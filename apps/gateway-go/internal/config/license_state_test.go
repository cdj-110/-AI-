package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testV2License(t *testing.T) (ActivationConfig, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	old := ActivationTrustedPublicKeys
	ring, _ := json.Marshal(map[string]string{"key-2026": base64.StdEncoding.EncodeToString(public)})
	ActivationTrustedPublicKeys = string(ring)
	t.Cleanup(func() { ActivationTrustedPublicKeys = old })
	enabled := true
	a := ActivationConfig{Enabled: &enabled, LicenseVersion: 2, KeyID: "key-2026", LicenseSequence: 2, HardwareID: "AABB", SN: "WK-1", DeviceSecret: "secret", Broker: "tcp://mqtt:1883", IssuedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Features: []string{"collection"}, Nonce: "nonce"}
	signV2(t, &a, private)
	return a, private
}

func signV2(t *testing.T, a *ActivationConfig, key ed25519.PrivateKey) {
	t.Helper()
	payload, err := activationPayload(*a)
	if err != nil {
		t.Fatal(err)
	}
	a.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
}

func TestV2PerpetualLicenseAndSignedExpiry(t *testing.T) {
	a, key := testV2License(t)
	timed := a
	a.ExpiresAt = ""
	signV2(t, &a, key)
	if err := validateActivationSignature(a, true, time.Now().AddDate(100, 0, 0)); err != nil {
		t.Fatalf("perpetual signed license rejected: %v", err)
	}
	payload, err := activationPayload(a)
	if err != nil || !strings.Contains(string(payload), `"expiresAt":""`) {
		t.Fatal("perpetual expiry missing from signed payload")
	}
	timed.ExpiresAt = ""
	if validateActivationSignature(timed, true, time.Now()) == nil {
		t.Fatal("erasing timed expiry bypassed signature")
	}
	a.ExpiresAt = time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339)
	if validateActivationSignature(a, true, time.Now()) == nil {
		t.Fatal("changing perpetual expiry bypassed signature")
	}
}

func TestV2LicenseKeyRotationAndTampering(t *testing.T) {
	a, key := testV2License(t)
	if err := validateActivationSignature(a, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ActivationConfig){func(a *ActivationConfig) { a.LicenseSequence++ }, func(a *ActivationConfig) { a.KeyID = "unknown" }, func(a *ActivationConfig) { a.Broker = "tcp://attacker:1883" }, func(a *ActivationConfig) { a.LicenseVersion = 3 }} {
		next := a
		mutate(&next)
		if validateActivationSignature(next, true, time.Now()) == nil {
			t.Fatal("tampered license accepted")
		}
	}
	ring := map[string]string{}
	_ = json.Unmarshal([]byte(ActivationTrustedPublicKeys), &ring)
	ring["key-2027"] = ring["key-2026"]
	raw, _ := json.Marshal(ring)
	ActivationTrustedPublicKeys = string(raw)
	a.KeyID = "key-2027"
	signV2(t, &a, key)
	if err := validateActivationSignature(a, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	delete(ring, "key-2027")
	raw, _ = json.Marshal(ring)
	ActivationTrustedPublicKeys = string(raw)
	if validateActivationSignature(a, true, time.Now()) == nil {
		t.Fatal("removed trust key accepted")
	}
}

func TestLicenseReplayPersistentAndIdempotent(t *testing.T) {
	a, _ := testV2License(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := AcceptLicense(path, a); err != nil {
		t.Fatal(err)
	}
	if err := AcceptLicense(path, a); err != nil {
		t.Fatalf("idempotent import rejected: %v", err)
	}
	for _, mutate := range []func(*ActivationConfig){func(a *ActivationConfig) { a.LicenseSequence = 1 }, func(a *ActivationConfig) { a.Nonce = "conflict" }, func(a *ActivationConfig) { a.SN = "another" }, func(a *ActivationConfig) { a.LicenseVersion = 1; a.LicenseSequence = 0; a.KeyID = "" }} {
		next := a
		mutate(&next)
		if CheckLicenseReplay(path, next) == nil {
			t.Fatal("old/conflicting authorization accepted")
		}
	}
	next := a
	next.LicenseSequence++
	if err := AcceptLicense(path, next); err != nil {
		t.Fatal(err)
	}
	if CheckLicenseReplay(path, a) == nil {
		t.Fatal("rollback accepted after renewed receipt")
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".license-state.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if CheckLicenseReplay(path, next) == nil {
		t.Fatal("corrupt receipt accepted")
	}
}

func TestLicenseV2CanonicalJSONDoesNotEscapeCloudURL(t *testing.T) {
	a, _ := testV2License(t)
	a.CloudAPI = "https://cloud/api?a=1&b=2"
	raw, _ := activationPayload(a)
	if !strings.Contains(string(raw), `"licenseVersion":2,"keyId":"key-2026","licenseSequence":2,"hardwareId"`) || strings.Contains(string(raw), `\u0026`) {
		t.Fatalf("incompatible canonical payload: %s", raw)
	}
}

func TestCloudSwitchDoesNotDisableInstalledLicense(t *testing.T) {
	a, _ := testV2License(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "license.json")
	a.HardwareID = ""
	a.LicenseVersion = 0
	a.KeyID = ""
	a.LicenseSequence = 0
	a.Signature = ""
	raw, _ := json.Marshal(a)
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	disabled := false
	cfg := Config{Activation: ActivationConfig{Enabled: &disabled, File: file}}
	if err := cfg.ApplyActivation(); err != nil {
		t.Fatal(err)
	}
	if !cfg.License.Valid || !cfg.Activation.IsEnabled() || cfg.CloudConnectionIsEnabled() {
		t.Fatalf("cloud-off broke authorization: %+v", cfg.License)
	}
}

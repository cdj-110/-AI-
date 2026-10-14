package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/hardware"
)

func signedTestActivation(t *testing.T) ActivationConfig {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	previousKey := ActivationSignaturePublicKey
	ActivationSignaturePublicKey = base64.StdEncoding.EncodeToString(publicDER)
	t.Cleanup(func() { ActivationSignaturePublicKey = previousKey })
	enabled := true
	activation := ActivationConfig{
		Enabled: &enabled, LicenseVersion: 1, HardwareID: "AABBCCDD", ChipUSID: "11223344",
		SN: "WK-TEST", DeviceSecret: "secret", Broker: "tcp://cloud.example:1883",
		CloudAPI: "https://cloud.example/api", RemoteAccessEndpoint: "wss://cloud.example/remote-agent",
		IssuedAt:  time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		Features:  []string{"collection", "forward", "cloud"}, Nonce: "test-nonce",
	}
	payload, err := activationPayload(activation)
	if err != nil {
		t.Fatal(err)
	}
	activation.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return activation
}

func TestActivationSignatureAcceptsUntamperedPackage(t *testing.T) {
	activation := signedTestActivation(t)
	if err := validateActivationSignature(activation, true, time.Now()); err != nil {
		t.Fatalf("valid signed activation rejected: %v", err)
	}
}

func TestActivationSignatureRejectsTamperedCloudSetting(t *testing.T) {
	activation := signedTestActivation(t)
	activation.Broker = "tcp://attacker.example:1883"
	err := validateActivationSignature(activation, true, time.Now())
	if err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("tampered activation was accepted: %v", err)
	}
}

func TestActivationSignatureRejectsMissingSignatureInStrictMode(t *testing.T) {
	activation := signedTestActivation(t)
	activation.Signature = ""
	err := validateActivationSignature(activation, true, time.Now())
	if err == nil || !strings.Contains(err.Error(), "signed activation") {
		t.Fatalf("unsigned activation was accepted: %v", err)
	}
}

func TestActivationSignatureRejectsExpiredPackage(t *testing.T) {
	activation := signedTestActivation(t)
	activation.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	err := validateActivationSignature(activation, true, time.Now())
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired activation was accepted: %v", err)
	}
}

func TestActivationPayloadMatchesManagementPlatformCanonicalOrder(t *testing.T) {
	enabled := true
	payload, err := activationPayload(ActivationConfig{
		Enabled: &enabled, LicenseVersion: 1, HardwareID: "aabb", ChipUSID: "ccdd",
		SN: " WK-1 ", DeviceSecret: "secret", Broker: " tcp://mqtt:1883 ",
		CloudAPI: " https://cloud/api ", RemoteAccessEndpoint: " wss://cloud/remote ",
		IssuedAt: "2026-09-16T00:00:00Z", ExpiresAt: "2027-09-16T00:00:00Z",
		Features: []string{"collection", "cloud"}, Nonce: "nonce",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"enabled":true,"licenseVersion":1,"hardwareId":"AABB","chipUsid":"CCDD","sn":"WK-1","deviceSecret":"secret","broker":"tcp://mqtt:1883","cloudApi":"https://cloud/api","remoteAccessEndpoint":"wss://cloud/remote","issuedAt":"2026-09-16T00:00:00Z","expiresAt":"2027-09-16T00:00:00Z","features":["collection","cloud"],"nonce":"nonce"}`
	if string(payload) != want {
		t.Fatalf("canonical payload changed:\n got: %s\nwant: %s", payload, want)
	}
}

func TestActivationPayloadIncludesSignedProductHardwareProfile(t *testing.T) {
	enabled := true
	payload, err := activationPayload(ActivationConfig{
		Enabled: &enabled, LicenseVersion: 2, KeyID: "key", LicenseSequence: 7,
		HardwareID: "aabb", ChipUSID: "ccdd", SN: "WK-1", DeviceSecret: "secret",
		Broker: "tcp://mqtt:1883", IssuedAt: "2026-09-22T00:00:00Z", Features: []string{"collection"},
		ProductModel: "EG100", HardwareProfile: &HardwareProfile{
			EthernetPortCount: 2, RS232PortCount: 2, RS485PortCount: 2,
			CANPortCount: 0, Supports4G: true, SupportsWiFi: true,
		}, Nonce: "nonce",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"enabled":true,"licenseVersion":2,"keyId":"key","licenseSequence":7,"hardwareId":"AABB","chipUsid":"CCDD","sn":"WK-1","deviceSecret":"secret","broker":"tcp://mqtt:1883","cloudApi":"","remoteAccessEndpoint":"","issuedAt":"2026-09-22T00:00:00Z","expiresAt":"","features":["collection"],"productModel":"EG100","hardwareProfile":{"ethernetPortCount":2,"rs232PortCount":2,"rs485PortCount":2,"canPortCount":0,"supports4G":true,"supportsWifi":true},"nonce":"nonce"}`
	if string(payload) != want {
		t.Fatalf("profile payload order changed:\n got: %s\nwant: %s", payload, want)
	}
}

func TestStrictBuildEntersRestrictedModeForUnsignedActivation(t *testing.T) {
	previousBinding := ActivationBindingRequired
	previousSignature := ActivationSignatureRequired
	previousKey := ActivationSignaturePublicKey
	previousChip := SecureChipRequired
	ActivationBindingRequired = "false"
	ActivationSignatureRequired = "true"
	ActivationSignaturePublicKey = ""
	SecureChipRequired = "false"
	t.Cleanup(func() {
		ActivationBindingRequired = previousBinding
		ActivationSignatureRequired = previousSignature
		ActivationSignaturePublicKey = previousKey
		SecureChipRequired = previousChip
	})
	enabled := true
	cfg := Config{Activation: ActivationConfig{
		Enabled: &enabled, HardwareID: "AABB", SN: "WK-TEST", DeviceSecret: "secret",
	}}
	if err := cfg.ApplyActivation(); err != nil {
		t.Fatalf("strict build should remain available for maintenance: %v", err)
	}
	if !cfg.License.Restricted || cfg.License.Valid {
		t.Fatalf("unsigned activation did not enter restricted mode: %+v", cfg.License)
	}
}

func TestActivationBindingAcceptsMatchingHardware(t *testing.T) {
	activation := ActivationConfig{
		HardwareID:   "AABBCCDD",
		SN:           "WK-TEST",
		DeviceSecret: "secret",
	}
	identity := hardware.Identity{ID: "aabbccdd", Available: true}
	if err := validateActivationBinding(activation, identity, true); err != nil {
		t.Fatalf("matching activation rejected: %v", err)
	}
}

func TestActivationBindingRejectsCopiedActivation(t *testing.T) {
	activation := ActivationConfig{
		HardwareID:   "ORIGINAL-HARDWARE",
		SN:           "WK-TEST",
		DeviceSecret: "secret",
	}
	identity := hardware.Identity{ID: "COPIED-TO-HARDWARE", Available: true}
	err := validateActivationBinding(activation, identity, true)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("copied activation was accepted: %v", err)
	}
}

func TestActivationBindingRequiredRejectsDisabledActivation(t *testing.T) {
	disabled := false
	err := validateActivationBinding(ActivationConfig{Enabled: &disabled}, hardware.Identity{}, true)
	if err == nil || !strings.Contains(err.Error(), "requires") {
		t.Fatalf("disabled activation was accepted by production binding: %v", err)
	}
}

func TestActivationBindingRejectsUnavailableHardwareIdentity(t *testing.T) {
	err := validateActivationBinding(
		ActivationConfig{HardwareID: "AABBCCDD", SN: "WK-TEST", DeviceSecret: "secret"},
		hardware.Identity{Available: false},
		true,
	)
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("activation without readable hardware identity was accepted: %v", err)
	}
}

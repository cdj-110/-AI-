package web

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func initialActivationFixture(t *testing.T) (*Server, []byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	oldRing := config.ActivationTrustedPublicKeys
	ring, _ := json.Marshal(map[string]string{"initial-test": base64.StdEncoding.EncodeToString(public)})
	config.ActivationTrustedPublicKeys = string(ring)
	t.Cleanup(func() { config.ActivationTrustedPublicKeys = oldRing })
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"gatewayKey":"test","activation":{"file":"activation.local.json"},"mqtt":{"enabled":false}}`), 0600); err != nil {
		t.Fatal(err)
	}
	id := hardware.ReadIdentity().ID
	if id == "" {
		id = "TEST-HARDWARE"
	}
	payload := fmt.Sprintf(`{"enabled":true,"licenseVersion":2,"keyId":"initial-test","licenseSequence":1,"hardwareId":%q,"chipUsid":"","sn":"WK-INITIAL","deviceSecret":"test-secret","broker":"tcp://127.0.0.1:1883","cloudApi":"","remoteAccessEndpoint":"","issuedAt":%q,"expiresAt":%q,"features":["collection"],"productModel":"EG100","hardwareProfile":{"ethernetPortCount":2,"rs232PortCount":2,"rs485PortCount":2,"canPortCount":0,"supports4G":true,"supportsWifi":true},"nonce":"initial-test"}`, id, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	var document map[string]interface{}
	_ = json.Unmarshal([]byte(payload), &document)
	document["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(payload)))
	content, _ := json.Marshal(document)
	server := &Server{configPath: path, store: state.New(config.Config{License: config.LicenseStatus{Restricted: true}}), onConfig: func(config.Config) error { return nil }}
	return server, content
}

func callInitial(server *Server, method string, content []byte, file string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"content": string(content), "file": file})
	r := httptest.NewRequest(method, "/api/activation/initial", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.routes().ServeHTTP(w, r)
	return w
}

func TestInitialActivationAnonymousSignedPreflightAndConcurrentCommit(t *testing.T) {
	s, content := initialActivationFixture(t)
	if !s.initialActivationAvailable() {
		t.Fatal("new device cannot activate")
	}
	status := httptest.NewRecorder()
	s.routes().ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/license/status", nil))
	if !bytes.Contains(status.Body.Bytes(), []byte(`"initialActivationAllowed":true`)) {
		t.Fatal("public entry missing first activation permission")
	}
	original, _ := os.ReadFile(s.configPath)
	preview := callInitial(s, http.MethodPost, content, "")
	if preview.Code != 200 {
		t.Fatalf("preflight: %d %s", preview.Code, preview.Body.String())
	}
	after, _ := os.ReadFile(s.configPath)
	if !bytes.Equal(original, after) || !s.initialActivationAvailable() {
		t.Fatal("preflight changed activation state")
	}
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- callInitial(s, http.MethodPut, content, "").Code }()
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for code := range results {
		counts[code]++
	}
	if counts[200] != 1 || counts[403] != 1 {
		t.Fatalf("concurrent import must allow exactly one commit: %v", counts)
	}
	if s.initialActivationAvailable() {
		t.Fatal("installed license left anonymous entry open")
	}
	working, _ := os.ReadFile(s.configPath)
	if result := callInitial(s, http.MethodPut, content, ""); result.Code != 403 {
		t.Fatalf("anonymous replacement allowed: %d", result.Code)
	}
	after, _ = os.ReadFile(s.configPath)
	if !bytes.Equal(working, after) {
		t.Fatal("replacement rejection modified config")
	}
	cfg, err := s.readRawConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cfg.Activation.File); err != nil {
		t.Fatal(err)
	}
	if s.initialActivationAvailable() {
		t.Fatal("missing installed license bypassed stored receipt")
	}
	if result := callInitial(s, http.MethodPut, content, ""); result.Code != 403 {
		t.Fatal("missing license allowed anonymous replacement")
	}
}

func TestInitialActivationEnablesCloudOnlyAfterConfirmedImport(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("factoryExplicitlyDisabled=%v", disabled), func(t *testing.T) {
			s, content := initialActivationFixture(t)
			if disabled {
				cfg, err := s.readRawConfig()
				if err != nil {
					t.Fatal(err)
				}
				off := false
				cfg.CloudConnectionEnabled = &off
				if err := config.Save(s.configPath, cfg); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(s.configPath)
			if preview := callInitial(s, http.MethodPost, content, ""); preview.Code != 200 {
				t.Fatalf("preview: %s", preview.Body.String())
			}
			afterPreview, _ := os.ReadFile(s.configPath)
			if !bytes.Equal(before, afterPreview) {
				t.Fatal("preflight changed cloud switch")
			}
			if imported := callInitial(s, http.MethodPut, content, ""); imported.Code != 200 {
				t.Fatalf("import: %s", imported.Body.String())
			}
			cfg, err := s.readRawConfig()
			if err != nil || cfg.CloudConnectionEnabled == nil || !*cfg.CloudConnectionEnabled {
				t.Fatalf("first activation did not persist cloud connection enabled: %v", err)
			}
			if cfg.ProductModel != "EG100" || cfg.HardwareProfile == nil || len(cfg.Resources) != 6 {
				t.Fatalf("first activation did not materialize EG100 hardware profile: model=%q profile=%#v resources=%d", cfg.ProductModel, cfg.HardwareProfile, len(cfg.Resources))
			}
			loaded, err := config.Load(s.configPath)
			if err != nil || !loaded.License.Valid || !loaded.CloudConnectionIsEnabled() {
				t.Fatalf("activated runtime did not enable cloud: %v", err)
			}
		})
	}
}

func TestInitialActivationRejectsUnsafePackagesWithoutChangingConfig(t *testing.T) {
	s, signed := initialActivationFixture(t)
	original, _ := os.ReadFile(s.configPath)
	for _, tc := range []struct {
		name, field string
		value       interface{}
		file        string
	}{
		{"tampered signature", "signature", "invalid", ""},
		{"no signature", "signature", "", ""},
		{"disabled package", "enabled", false, ""},
		{"legacy license", "licenseVersion", 1, ""},
		{"wrong hardware", "hardwareId", "OTHER-HARDWARE", ""},
		{"wrong chip", "chipUsid", "WRONG-CHIP", ""},
		{"expired license", "expiresAt", "2000-01-01T00:00:00Z", ""},
		{"caller path", "nonce", "initial-test", filepath.Join(filepath.Dir(s.configPath), "overwrite.json")},
		{"embedded path", "file", "overwrite.json", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]interface{}
			_ = json.Unmarshal(signed, &doc)
			doc[tc.field] = tc.value
			content, _ := json.Marshal(doc)
			result := callInitial(s, http.MethodPut, content, tc.file)
			if result.Code == 200 {
				t.Fatal("unsafe package accepted")
			}
			after, _ := os.ReadFile(s.configPath)
			if !bytes.Equal(original, after) || !s.initialActivationAvailable() {
				t.Fatal("rejected package modified new device")
			}
		})
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		if result := callInitial(s, method, signed, ""); result.Code != 405 {
			t.Fatal("anonymous endpoint allowed read/delete")
		}
	}
}

func TestInitialActivationFailsClosedForPreviousOrUnavailableAuthorization(t *testing.T) {
	for _, name := range []string{"receipt", "configured-file", "versioned-file", "inline", "activated", "unavailable", "corrupt-config", "missing-config"} {
		t.Run(name, func(t *testing.T) {
			s, content := initialActivationFixture(t)
			dir := filepath.Dir(s.configPath)
			switch name {
			case "receipt":
				if err := os.WriteFile(filepath.Join(dir, ".license-state.json"), []byte("damaged"), 0600); err != nil {
					t.Fatal(err)
				}
			case "configured-file":
				if err := os.WriteFile(filepath.Join(dir, "activation.local.json"), []byte("expired or damaged"), 0600); err != nil {
					t.Fatal(err)
				}
			case "versioned-file":
				if err := os.WriteFile(filepath.Join(dir, "activation-1-existing.json"), []byte("previous license"), 0600); err != nil {
					t.Fatal(err)
				}
			case "inline":
				if err := os.WriteFile(s.configPath, []byte(`{"activation":{"signature":"old"}}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "activated":
				s.store = state.New(config.Config{License: config.LicenseStatus{Valid: true}})
			case "unavailable":
				s.store = nil
			case "corrupt-config":
				if err := os.WriteFile(s.configPath, []byte("damaged"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing-config":
				if err := os.Remove(s.configPath); err != nil {
					t.Fatal(err)
				}
			}
			if s.initialActivationAvailable() {
				t.Fatal("unsafe device exposed first activation")
			}
			if result := callInitial(s, http.MethodPut, content, ""); result.Code == 200 {
				t.Fatal("anonymous replacement accepted")
			}
		})
	}
}

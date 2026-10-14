package web

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestDiagnosticBundleRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.local.json")
	cfg := config.Config{
		GatewayKey: "gw",
		Devices:    []config.DeviceConfig{{DeviceKey: "d1", Protocol: "opcda", Password: "do-not-leak"}},
		Security: config.SecurityConfig{Users: []config.UserConfig{{
			Username: "admin", PasswordHash: "password-hash-secret", Enabled: true,
			MFAEnabled: true, TOTPSecret: "totp-secret-value",
		}}},
	}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	server := &Server{configPath: configPath, store: state.New(cfg), audit: newAuditStore(configPath)}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/maintenance/diagnostic-bundle", nil)
	server.diagnosticBundle(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "application/zip") {
		t.Fatalf("status=%d type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	reader, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"do-not-leak", "password-hash-secret", "totp-secret-value"} {
		if bytes.Contains(response.Body.Bytes(), []byte(secret)) {
			t.Fatalf("diagnostic archive leaked %q", secret)
		}
	}
	foundConfig, foundManifest := false, false
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			foundManifest = true
		}
		if file.Name != "configuration/config.redacted.json" {
			continue
		}
		foundConfig = true
		stream, _ := file.Open()
		content, _ := io.ReadAll(stream)
		_ = stream.Close()
		if strings.Contains(string(content), "do-not-leak") || strings.Contains(string(content), "password-hash-secret") || strings.Contains(string(content), "totp-secret-value") || !strings.Contains(string(content), "***") {
			t.Fatalf("diagnostic config was not redacted: %s", content)
		}
	}
	if !foundConfig || !foundManifest {
		t.Fatalf("missing expected entries: config=%v manifest=%v", foundConfig, foundManifest)
	}
}

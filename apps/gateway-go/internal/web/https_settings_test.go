package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestHTTPSSettingsGeneratesCertificateAndPersistsConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{GatewayKey: "test-gateway", Web: configForAuthTest()}
	cfg.ApplyDefaults()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	server := New(configForAuthTest(), state.New(cfg), nil, path, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/security/https", strings.NewReader(`{"enabled":true}`))
	request.Host = "192.168.1.214:8088"
	authenticateTestRequest(server, request)
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var result httpsSettingsResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Enabled || !result.CertificateOK || !result.RequiresRestart || result.URL != "https://192.168.1.214:8088" {
		t.Fatalf("response = %#v", result)
	}
	updated, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Web.TLSCertFile == "" || updated.Web.TLSKeyFile == "" {
		t.Fatalf("TLS paths were not persisted: %#v", updated.Web)
	}
}

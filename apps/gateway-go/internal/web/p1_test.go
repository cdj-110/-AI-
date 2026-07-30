package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestSaveConfigRejectsStaleRevision(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{"gatewayKey":"old","activation":{"enabled":false},"mqtt":{"enabled":false}}`)
	if err := os.WriteFile(configPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	server := &Server{configPath: configPath, onConfig: func(config.Config) error { return nil }}
	body := []byte(`{"content":"{\"gatewayKey\":\"new\",\"activation\":{\"enabled\":false},\"mqtt\":{\"enabled\":false}}"}`)
	request := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	request.Header.Set("If-Match", `"stale-revision"`)
	response := httptest.NewRecorder()

	server.saveConfig(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(original) {
		t.Fatalf("stale save changed config: %s", raw)
	}
}

func TestSaveConfigAcceptsRawConfigDocument(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"gatewayKey":"old","activation":{"enabled":false},"mqtt":{"enabled":false}}`), 0600); err != nil {
		t.Fatal(err)
	}
	server := &Server{configPath: configPath, onConfig: func(config.Config) error { return nil }}
	body := []byte(`{"gatewayKey":"new","activation":{"enabled":false},"mqtt":{"enabled":false}}`)
	request := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/vnd.weikong.config+json")
	response := httptest.NewRecorder()

	server.saveConfig(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.GatewayKey != "new" {
		t.Fatalf("gatewayKey = %q, want new", saved.GatewayKey)
	}
}

func TestAuditStoreReturnsNewestEventsFirst(t *testing.T) {
	store := &auditStore{path: filepath.Join(t.TempDir(), "audit.jsonl")}
	if err := store.append(auditEvent{Path: "/first", Method: http.MethodPut, Status: 200, Result: "success"}); err != nil {
		t.Fatal(err)
	}
	if err := store.append(auditEvent{Path: "/second", Method: http.MethodPost, Status: 500, Result: "failed"}); err != nil {
		t.Fatal(err)
	}
	events, err := store.recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Path != "/second" || events[1].Path != "/first" {
		t.Fatalf("events = %#v", events)
	}
}

func TestVueFrontendIsEmbedded(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	vueFrontendHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q", contentType)
	}
}

func TestWiFiStatusMasksPassword(t *testing.T) {
	cfg := config.Config{GatewayKey: "gw", WiFi: config.WiFiConfig{Enabled: true, Interface: "wlan0", SSID: "factory", Password: "secret"}}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	server := &Server{runtime: gatewayruntime.NewManager(cfg, store)}
	request := httptest.NewRequest(http.MethodGet, "/api/network/wifi", nil)
	response := httptest.NewRecorder()
	server.wifiNetwork(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte(`"password":"secret"`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"password":"***"`)) {
		t.Fatalf("WiFi response did not mask password: %s", response.Body.String())
	}
}

func TestWiFiBootRequiresRestart(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "wireless-enabled")
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("GATEWAY_WIRELESS_MARKER", marker)
	cfg := config.Config{GatewayKey: "gw"}
	cfg.ApplyDefaults()
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	server := &Server{configPath: configPath, runtime: gatewayruntime.NewManager(cfg, state.New(cfg))}
	request := httptest.NewRequest(http.MethodPost, "/api/network/wifi/boot", bytes.NewBufferString(`{"enabled":true}`))
	response := httptest.NewRecorder()
	server.wifiBoot(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("wireless boot marker was not written: %v", err)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"restartRequired":true`)) {
		t.Fatalf("restart requirement missing: %s", response.Body.String())
	}
}

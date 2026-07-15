package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestWritePointRejectsReadOnlyPoint(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{{
		DeviceKey: "device-a", Metric: "input", Protocol: "modbus-tcp", Function: 4,
	}}}
	store := state.New(cfg)
	server := New(config.ListenerConfig{}, store, gatewayruntime.NewManager(cfg, store), "", nil)
	request := httptest.NewRequest(http.MethodPost, "/api/points/write", strings.NewReader(`{"deviceKey":"device-a","metric":"input","value":1}`))
	response := httptest.NewRecorder()
	server.writePoint(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "writable point not found") {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestWritePointValidatesRequest(t *testing.T) {
	cfg := config.Config{}
	store := state.New(cfg)
	server := New(config.ListenerConfig{}, store, gatewayruntime.NewManager(cfg, store), "", nil)
	request := httptest.NewRequest(http.MethodPost, "/api/points/write", strings.NewReader(`{"deviceKey":"","metric":"","value":null}`))
	response := httptest.NewRecorder()
	server.writePoint(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

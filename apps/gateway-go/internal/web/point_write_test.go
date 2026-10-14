package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestWritablePointsReportsOnlyExecutablePointsForDevice(t *testing.T) {
	disabled := false
	cfg := config.Config{Points: []config.PointConfig{
		{DeviceKey: "device-a", Metric: "coil", Protocol: "modbus-tcp", Function: 1},
		{DeviceKey: "device-a", Metric: "input", Protocol: "modbus-tcp", Function: 4},
		{DeviceKey: "device-a", Metric: "s7-output", Protocol: "siemens-s7", Area: "M", DataType: "bool"},
		{DeviceKey: "device-a", Metric: "disabled", Protocol: "modbus-rtu", Function: 3, Enabled: &disabled},
		{DeviceKey: "device-b", Metric: "coil", Protocol: "modbus-tcp", Function: 1},
	}}
	store := state.New(cfg)
	server := New(config.ListenerConfig{}, store, gatewayruntime.NewManager(cfg, store), "", nil)
	response := httptest.NewRecorder()
	server.writablePoints(response, httptest.NewRequest(http.MethodGet, "/api/points/writable?deviceKey=device-a", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Metrics []string `json:"metrics"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Metrics) != 2 || body.Metrics[0] != "coil" || body.Metrics[1] != "s7-output" {
		t.Fatalf("unexpected writable metrics: %+v", body.Metrics)
	}
	response = httptest.NewRecorder()
	server.writablePoints(response, httptest.NewRequest(http.MethodGet, "/api/points/writable", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing device key status = %d", response.Code)
	}
}

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

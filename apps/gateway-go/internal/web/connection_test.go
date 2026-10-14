package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestRTUConnectionTestsNeverFallThroughToTCP(t *testing.T) {
	body := `{"protocol":"modbus-rtu","address":"/dev/ttyS1","slaveId":0}`
	server := newAuthTestServer()
	request := httptest.NewRequest(http.MethodPost, "/api/connection/test", strings.NewReader(body))
	authenticateTestRequest(server, request)
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "从站 ID") || strings.Contains(response.Body.String(), "missing port") {
		t.Fatalf("HTTP probe used TCP: %d %s", response.Code, response.Body.String())
	}
	_, err := runConnectionTestCommand(context.Background(), json.RawMessage(body))
	if err == nil || !strings.Contains(err.Error(), "从站 ID") || strings.Contains(err.Error(), "missing port") {
		t.Fatalf("WebSocket probe used TCP: %v", err)
	}
}

func TestNormalizeConnectionAddressUsesProtocolPorts(t *testing.T) {
	tests := []struct {
		protocol string
		address  string
		want     string
	}{
		{protocol: "modbus-tcp", address: "192.168.1.10", want: "192.168.1.10:502"},
		{protocol: "iec104", address: "192.168.1.11", want: "192.168.1.11:2404"},
		{protocol: "siemens-s7", address: "192.168.1.12", want: "192.168.1.12:102"},
		{protocol: "iec61850", address: "192.168.1.13", want: "192.168.1.13:102"},
		{protocol: "modbus-tcp", address: "192.168.1.14:1502", want: "192.168.1.14:1502"},
	}
	for _, test := range tests {
		if got := normalizeConnectionAddress(test.protocol, test.address); got != test.want {
			t.Errorf("normalizeConnectionAddress(%q, %q)=%q, want %q", test.protocol, test.address, got, test.want)
		}
	}
}

func TestResolveMaskedOPCDAPasswordByDeviceKey(t *testing.T) {
	body := connectionTestRequest{DeviceKey: "opcda-1", Password: "***"}
	cfg := config.Config{Devices: []config.DeviceConfig{
		{DeviceKey: "other", Password: "wrong"},
		{DeviceKey: "opcda-1", Password: "secret"},
	}}

	if !resolveMaskedOPCDAPassword(&body, cfg) {
		t.Fatal("resolveMaskedOPCDAPassword returned false")
	}
	if body.Password != "secret" {
		t.Fatalf("resolved password = %q, want configured secret", body.Password)
	}
}

func TestResolveMaskedOPCDAPasswordRejectsUnknownDevice(t *testing.T) {
	body := connectionTestRequest{DeviceKey: "missing", Password: "***"}
	if resolveMaskedOPCDAPassword(&body, config.Config{}) {
		t.Fatal("resolveMaskedOPCDAPassword unexpectedly resolved an unknown device")
	}
	if body.Password != "***" {
		t.Fatal("unknown device changed the masked password")
	}
}

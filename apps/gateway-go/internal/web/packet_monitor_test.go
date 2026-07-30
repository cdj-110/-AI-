package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestPacketFramesFiltersCurrentDevice(t *testing.T) {
	packetmonitor.Record(packetmonitor.Frame{Protocol: "modbus-tcp", Direction: "tx", DeviceKey: "packet-test-one", Summary: "one"}, []byte{1})
	packetmonitor.Record(packetmonitor.Frame{Protocol: "modbus-tcp", Direction: "rx", DeviceKey: "packet-test-two", Summary: "two"}, []byte{2})
	server := New(config.ListenerConfig{}, state.New(config.Config{}), nil, "", nil)
	response := httptest.NewRecorder()
	server.packetFrames(response, httptest.NewRequest(http.MethodGet, "/api/packets?deviceKey=packet-test-two&limit=10", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("packet frames status = %d", response.Code)
	}
	var body struct {
		Frames []packetmonitor.Frame `json:"frames"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Frames) != 1 || body.Frames[0].DeviceKey != "packet-test-two" || body.Frames[0].Hex != "02" {
		t.Fatalf("unexpected packet frames: %#v", body.Frames)
	}
}

func TestPacketMonitorPageIsIndependentView(t *testing.T) {
	server := New(config.ListenerConfig{}, state.New(config.Config{}), nil, "", nil)
	response := httptest.NewRecorder()
	server.packetMonitorPage(response, httptest.NewRequest(http.MethodGet, "/packet-monitor", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "实时报文监控") || !strings.Contains(response.Body.String(), "EventSource") || !strings.Contains(response.Body.String(), "本次通信结束，将自动重连") {
		t.Fatalf("unexpected packet monitor page: status=%d", response.Code)
	}
}

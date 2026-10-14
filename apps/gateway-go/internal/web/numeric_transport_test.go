package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
	"weikong-iot-platform/apps/gateway-go/internal/mapper"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestNumericTransportAndEdgePreview(t *testing.T) {
	cfg := config.Config{GatewayKey: "test", Points: []config.PointConfig{
		{DeviceKey: "source", Metric: "a", DataType: "float64", Protocol: "modbus-tcp", Address: "127.0.0.1:502", Function: 3, Quantity: 2},
		{DeviceKey: "source", Metric: "b", DataType: "float64", Protocol: "modbus-tcp", Address: "127.0.0.1:502", Function: 3, Quantity: 2},
	}, EdgeComputing: config.EdgeComputingConfig{Enabled: true, Groups: []config.EdgeComputeGroupConfig{{GroupKey: "edge", Points: []config.EdgeComputedPointConfig{{
		Name: "result", Metric: "result", DataType: "float64", Decimals: 2, Expression: "a + b",
		Inputs: []config.EdgeComputeInputConfig{{Alias: "a", SourceDeviceKey: "source", SourceMetric: "a"}, {Alias: "b", SourceDeviceKey: "source", SourceMetric: "b"}},
	}}}}}}
	store := state.New(cfg)
	decoded, err := mapper.Decode(config.PointConfig{DataType: "int16", ByteOrder: "big", Scale: 0.1}, []byte{0x01, 0xb6})
	if err != nil {
		t.Fatal(err)
	}
	raw := decoded.(float64)
	if raw == 43.8 {
		t.Fatal("fixture must reproduce the reported Modbus scaling tail")
	}
	store.SetPointValue("source", "a", raw)
	store.SetPointValue("source", "b", 16.0)
	server := New(config.ListenerConfig{}, store, nil, "", nil)
	for _, url := range []string{"/api/point-status", "/api/point-status?deviceKey=source", "/api/point-status?deviceKey=source&metric=a", "/api/status"} {
		t.Run(url, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, url, nil)
			var points []state.PointStatus
			if url == "/api/status" {
				server.status(response, request)
				var snapshot state.Snapshot
				if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
					t.Fatal(err)
				}
				points = snapshot.Points
			} else {
				server.pointStatus(response, request)
				if err := json.Unmarshal(response.Body.Bytes(), &points); err != nil {
					t.Fatal(err)
				}
			}
			found := false
			for _, point := range points {
				if point.Metric == "a" {
					found = true
					if point.Value != float64(43.8) {
						t.Fatalf("value = %#v", point.Value)
					}
				}
			}
			if !found {
				t.Fatal("missing input point")
			}
		})
	}
	content, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"content": string(content), "groupKey": "edge", "metric": "result"})
	response := httptest.NewRecorder()
	server.validateEdgeCompute(response, httptest.NewRequest(http.MethodPost, "/api/edge-compute/validate", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("preview failed: %s", response.Body.String())
	}
	var preview struct {
		OK     bool               `json:"ok"`
		Value  float64            `json:"value"`
		Inputs map[string]float64 `json:"inputs"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.OK || preview.Value != 59.8 || preview.Inputs["a"] != 43.8 || preview.Inputs["b"] != 16 {
		t.Fatalf("preview = %#v", preview)
	}
	value, inputs, err := edgecompute.Preview(cfg, store, "edge", "result")
	if err != nil {
		t.Fatal(err)
	}
	if value != raw+16 || inputs["a"] != raw {
		t.Fatalf("calculation precision changed: %v, %v", value, inputs)
	}
	// Output decimals are presentation metadata, not rounding of CEL inputs/results.
	store.SetPointValue("source", "a", 1.2345678901234567)
	value, _, err = edgecompute.Preview(cfg, store, "edge", "result")
	if err != nil || value != 1.2345678901234567+16 {
		t.Fatalf("high precision preview = %v, %v", value, err)
	}
}

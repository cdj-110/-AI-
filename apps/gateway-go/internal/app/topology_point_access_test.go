package app

import (
	"testing"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestTopologyReportsInheritedPointAccess(t *testing.T) {
	cfg := config.Config{Devices: []config.DeviceConfig{{DeviceKey: "child-a", Protocol: "modbus-rtu", Points: []config.PointConfig{{Metric: "holding", Function: 3}, {Metric: "input", Function: 4}}}}}
	devices := topologyDevices(cfg)
	if len(devices) != 1 || devices[0].PointAccessVersion != 1 || !devices[0].Metrics[0].CanRead || !devices[0].Metrics[0].CanWrite || devices[0].Metrics[1].CanWrite {
		t.Fatalf("unexpected inherited access: %+v", devices)
	}
	// Prefer executable flattened settings over an incomplete nested definition.
	cfg.Points = []config.PointConfig{{DeviceKey: "child-a", Metric: "holding", Protocol: "modbus-rtu", Function: 4}}
	devices = topologyDevices(cfg)
	if len(devices[0].Metrics) != 1 || devices[0].Metrics[0].CanWrite {
		t.Fatalf("flattened runtime settings were ignored: %+v", devices)
	}
	if topologySnapshot(cfg)["pointAccessVersion"] != 1 {
		t.Fatal("snapshot must mark complete access reports, including an empty device list")
	}
}

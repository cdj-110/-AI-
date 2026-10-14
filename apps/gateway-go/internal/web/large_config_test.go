package web

import (
	"encoding/json"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestNextAvailableMetricKeepsUniqueRequestedValueAndIncrementsCopies(t *testing.T) {
	used := map[string]struct{}{"mbtcp-p01": {}}
	if got := nextAvailableMetric("custom", used); got != "custom" {
		t.Fatalf("unique requested metric = %q, want custom", got)
	}
	if got := nextAvailableMetric("mbtcp-P01", used); got != "mbtcp-P02" {
		t.Fatalf("copied metric = %q, want mbtcp-P02", got)
	}
}

func TestApplyPointBatchSupportsFilteredRegisterSequence(t *testing.T) {
	device := config.DeviceConfig{DeviceKey: "d1", Points: []config.PointConfig{
		{Metric: "p1", Function: 3, Register: 1},
		{Metric: "p2", Function: 3, Register: 2},
		{Metric: "p3", Function: 4, Register: 3},
	}}
	value, _ := json.Marshal(100)
	body := pointMutationRequest{Field: "register", Mode: "increment", Value: value, Step: 2}
	err := applyPointBatch(&device, body, func(point config.PointConfig) bool { return point.Function == 3 }, config.Config{Devices: []config.DeviceConfig{device}})
	if err != nil {
		t.Fatal(err)
	}
	if device.Points[0].Register != 100 || device.Points[1].Register != 102 || device.Points[2].Register != 3 {
		t.Fatalf("register sequence = %d, %d, %d", device.Points[0].Register, device.Points[1].Register, device.Points[2].Register)
	}
}

func TestCompactConfigForUIOmitsPointPayloads(t *testing.T) {
	cfg := config.Config{
		GatewayKey: "gateway",
		Devices: []config.DeviceConfig{{
			DeviceKey: "collect-1",
			Password:  "secret",
			Points: []config.PointConfig{
				{Metric: "p1"},
				{Metric: "p2"},
			},
		}},
		ForwardDevices: []config.ForwardDeviceConfig{{
			DeviceKey: "forward-1",
			Points:    []config.ForwardPointConfig{{Metric: "f1"}},
		}},
	}
	compact, counts, total := compactConfigForUI(cfg)
	if total != 3 || counts["collect-1"] != 2 || counts["forward-1"] != 1 {
		t.Fatalf("counts = %#v total=%d", counts, total)
	}
	if len(compact.Devices[0].Points) != 0 || len(compact.ForwardDevices[0].Points) != 0 {
		t.Fatal("compact config retained point payloads")
	}
	if compact.Devices[0].Password != "***" {
		t.Fatalf("device password was not redacted: %q", compact.Devices[0].Password)
	}
	if len(cfg.Devices[0].Points) != 2 || len(cfg.ForwardDevices[0].Points) != 1 {
		t.Fatal("source config was mutated")
	}
}

func TestDeletesWholeDeviceOnlyWithoutFilters(t *testing.T) {
	if !deletesWholeDevice(pointMutationRequest{Action: "delete", Range: "all"}) {
		t.Fatal("unfiltered all-point deletion should use the whole-device path")
	}
	if deletesWholeDevice(pointMutationRequest{Action: "delete", Range: "all", Function: 3}) {
		t.Fatal("function-filtered deletion must retain other function groups")
	}
	if deletesWholeDevice(pointMutationRequest{Action: "delete", Range: "all", Search: "temperature"}) {
		t.Fatal("search-filtered deletion must retain non-matching points")
	}
	if deletesWholeDevice(pointMutationRequest{Action: "batch", Range: "all"}) {
		t.Fatal("non-delete mutation must not use the deletion path")
	}
}

func TestRenamePointMetricRejectsDuplicateAndUpdatesReferences(t *testing.T) {
	cfg := config.Config{
		Devices: []config.DeviceConfig{
			{DeviceKey: "source", Points: []config.PointConfig{{Name: "设备@old", Metric: "old"}}},
			{DeviceKey: "other", Points: []config.PointConfig{{Name: "设备@used", Metric: "used"}}},
		},
		ForwardDevices: []config.ForwardDeviceConfig{{
			DeviceKey: "forward",
			Points: []config.ForwardPointConfig{{
				Name: "设备@old", Metric: "old", SourceDeviceKey: "source", SourceMetric: "old",
			}},
		}},
		EdgeComputing: config.EdgeComputingConfig{Groups: []config.EdgeComputeGroupConfig{{
			GroupKey: "edge",
			Points: []config.EdgeComputedPointConfig{{
				Metric: "result",
				Inputs: []config.EdgeComputeInputConfig{{Alias: "x", SourceDeviceKey: "source", SourceMetric: "old"}},
			}},
		}}},
	}
	if err := renamePointMetric(&cfg, &cfg.Devices[0], "old", "used"); err == nil {
		t.Fatal("duplicate identifier was accepted")
	}
	if err := renamePointMetric(&cfg, &cfg.Devices[0], "old", "new"); err != nil {
		t.Fatal(err)
	}
	if cfg.Devices[0].Points[0].Metric != "new" || cfg.Devices[0].Points[0].Name != "设备@new" {
		t.Fatalf("collection point was not renamed: %#v", cfg.Devices[0].Points[0])
	}
	forward := cfg.ForwardDevices[0].Points[0]
	if forward.Metric != "new" || forward.SourceMetric != "new" || forward.Name != "设备@new" {
		t.Fatalf("forward reference was not renamed: %#v", forward)
	}
	if got := cfg.EdgeComputing.Groups[0].Points[0].Inputs[0].SourceMetric; got != "new" {
		t.Fatalf("edge input metric = %q, want new", got)
	}
}

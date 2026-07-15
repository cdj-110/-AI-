package edgecompute

import (
	"context"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestManagerComputesChainAndRecoversFromInputError(t *testing.T) {
	cfg := config.Config{GatewayKey: "gw", CollectIntervalSeconds: 1, Points: []config.PointConfig{
		{DeviceKey: "source", Name: "A", Metric: "a", Protocol: "modbus-tcp", DataType: "float64"},
		{DeviceKey: "source", Name: "B", Metric: "b", Protocol: "modbus-tcp", DataType: "bool"},
	}, EdgeComputing: config.EdgeComputingConfig{Enabled: true, Groups: []config.EdgeComputeGroupConfig{{GroupKey: "edge", Name: "Edge", Points: []config.EdgeComputedPointConfig{
		{Name: "first", Metric: "first", DataType: "float64", Expression: "a * 2.0", Inputs: []config.EdgeComputeInputConfig{{Alias: "a", SourceDeviceKey: "source", SourceMetric: "a"}}},
		{Name: "alarm", Metric: "alarm", DataType: "bool", Expression: "first > 10.0 && b", Inputs: []config.EdgeComputeInputConfig{{Alias: "first", SourceDeviceKey: "edge", SourceMetric: "first"}, {Alias: "b", SourceDeviceKey: "source", SourceMetric: "b"}}},
	}}}}}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	manager, err := NewManager(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	waitForStatus(t, store, "edge", "first", func(status state.PointStatus) bool { return status.Error != "" })
	store.SetPointValue("source", "a", 6.0)
	store.SetPointValue("source", "b", true)
	waitForStatus(t, store, "edge", "alarm", func(status state.PointStatus) bool {
		value, _ := status.Value.(bool)
		return status.Error == "" && value
	})
	store.SetPointError(cfg.Points[0], context.DeadlineExceeded)
	waitForStatus(t, store, "edge", "first", func(status state.PointStatus) bool { return status.Error != "" && status.Value == float64(12) })
}

func TestManagerOnlyRecomputesAffectedRules(t *testing.T) {
	cfg := config.Config{GatewayKey: "gw", CollectIntervalSeconds: 60, Points: []config.PointConfig{
		{DeviceKey: "source", Metric: "a", DataType: "float64"},
		{DeviceKey: "source", Metric: "c", DataType: "float64"},
	}, EdgeComputing: config.EdgeComputingConfig{Enabled: true, Groups: []config.EdgeComputeGroupConfig{{GroupKey: "edge", Points: []config.EdgeComputedPointConfig{
		{Name: "A", Metric: "pa", DataType: "float64", Expression: "a * 2.0", Inputs: []config.EdgeComputeInputConfig{{Alias: "a", SourceDeviceKey: "source", SourceMetric: "a"}}},
		{Name: "C", Metric: "pc", DataType: "float64", Expression: "c * 2.0", Inputs: []config.EdgeComputeInputConfig{{Alias: "c", SourceDeviceKey: "source", SourceMetric: "c"}}},
	}}}}}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	manager, err := NewManager(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	store.SetPointValue("source", "a", 1.0)
	store.SetPointValue("source", "c", 1.0)
	waitForStatus(t, store, "edge", "pc", func(status state.PointStatus) bool { return status.UpdatedAt != nil })
	before := findStatus(store, "edge", "pc").UpdatedAt
	store.SetPointValue("source", "a", 2.0)
	waitForStatus(t, store, "edge", "pa", func(status state.PointStatus) bool { return status.Value == float64(4) })
	after := findStatus(store, "edge", "pc").UpdatedAt
	if before == nil || after == nil || !before.Equal(*after) {
		t.Fatalf("unrelated rule was recomputed: before=%v after=%v", before, after)
	}
}

func findStatus(store *state.Store, deviceKey, metric string) state.PointStatus {
	for _, status := range store.PointStatuses() {
		if status.DeviceKey == deviceKey && status.Metric == metric {
			return status
		}
	}
	return state.PointStatus{}
}

func waitForStatus(t *testing.T, store *state.Store, deviceKey, metric string, match func(state.PointStatus) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, status := range store.PointStatuses() {
			if status.DeviceKey == deviceKey && status.Metric == metric && match(status) {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status %s/%s did not match: %#v", deviceKey, metric, store.PointStatuses())
}

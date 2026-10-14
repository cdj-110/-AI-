package app

import (
	"math"
	"reflect"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

func TestHistoryGroupedValuesDefaultsToAllPoints(t *testing.T) {
	grouped := map[string]map[string]interface{}{"device-1": {"a": 1, "b": 2}}
	if result := historyGroupedValues(config.HistoryStorageConfig{}, grouped); !reflect.DeepEqual(result, grouped) {
		t.Fatalf("default history selection changed values: %#v", result)
	}
}

func TestHistorySavePolicyKeepsUint64Precision(t *testing.T) {
	config := config.HistoryStorageConfig{SaveMode: "change", ChangeDeadband: 1}
	last := map[string]historySavedSample{}
	base := time.Unix(1000, 0)
	applyHistorySavePolicy(config, base, map[string]map[string]interface{}{"device": {"counter": uint64(1<<63 + 10)}}, last)
	inside := applyHistorySavePolicy(config, base.Add(time.Second), map[string]map[string]interface{}{"device": {"counter": uint64(1<<63 + 11)}}, last)
	if len(inside) != 0 {
		t.Fatalf("one-count delta should remain inside deadband: %#v", inside)
	}
	changed := applyHistorySavePolicy(config, base.Add(2*time.Second), map[string]map[string]interface{}{"device": {"counter": uint64(1<<63 + 12)}}, last)
	if changed["device"]["counter"] != uint64(1<<63+12) {
		t.Fatalf("wide integer change was lost: %#v", changed)
	}
}

func TestHistoryIdentityPreservesSpecialFloat(t *testing.T) {
	nanA, _, _ := historyValueIdentity(math.NaN())
	nanB, _, _ := historyValueIdentity(math.NaN())
	positiveInfinity, _, _ := historyValueIdentity(math.Inf(1))
	if nanA != nanB || nanA == positiveInfinity {
		t.Fatalf("special float identity mismatch: nan=%q inf=%q", nanA, positiveInfinity)
	}
}

func TestHistorySamplesCarryProtocolNeutralMetadata(t *testing.T) {
	at := time.Date(2026, 9, 2, 10, 0, 0, 120000000, time.UTC)
	batch := telemetryBatch{
		Time:    at.Add(20 * time.Millisecond),
		Grouped: map[string]map[string]interface{}{"s7-device": {"pressure": 12.5}},
		Points: map[string]map[string]model.PointValue{"s7-device": {"pressure": {
			DeviceKey: "s7-device", Metric: "pressure", Value: 12.5, ActualDataType: "float32",
			Quality: model.QualityGood, SourceTimestamp: at, ReceiveTimestamp: at.Add(20 * time.Millisecond), CollectionMode: "poll",
		}}},
	}
	samples := historySamples(batch, batch.Grouped)
	if len(samples) != 1 || samples[0].ActualDataType != "float32" || samples[0].Quality != model.QualityGood || !samples[0].SourceTimestamp.Equal(at) || samples[0].CollectionMode != "poll" {
		t.Fatalf("protocol-neutral metadata was lost: %#v", samples)
	}
}

func TestHistorySavePolicyUsesIndependentPointIntervals(t *testing.T) {
	config := config.HistoryStorageConfig{SaveMode: "interval", SaveIntervalSeconds: 10}
	last := map[string]historySavedSample{}
	base := time.Unix(1000, 0)
	first := applyHistorySavePolicy(config, base, map[string]map[string]interface{}{"device": {"a": 1, "b": 2}}, last)
	if len(first["device"]) != 2 {
		t.Fatalf("first samples were not saved: %#v", first)
	}
	if next := applyHistorySavePolicy(config, base.Add(9*time.Second), map[string]map[string]interface{}{"device": {"a": 3, "b": 4}}, last); len(next) != 0 {
		t.Fatalf("samples inside interval were saved: %#v", next)
	}
	if next := applyHistorySavePolicy(config, base.Add(10*time.Second), map[string]map[string]interface{}{"device": {"a": 3}}, last); next["device"]["a"] != 3 {
		t.Fatalf("sample at interval was not saved: %#v", next)
	}
}

func TestHistorySavePolicyUsesChangeDeadband(t *testing.T) {
	config := config.HistoryStorageConfig{SaveMode: "change", ChangeDeadband: 1}
	last := map[string]historySavedSample{}
	base := time.Unix(1000, 0)
	applyHistorySavePolicy(config, base, map[string]map[string]interface{}{"device": {"temperature": 10.0, "state": "idle"}}, last)
	inside := applyHistorySavePolicy(config, base.Add(time.Second), map[string]map[string]interface{}{"device": {"temperature": 10.5, "state": "idle"}}, last)
	if len(inside) != 0 {
		t.Fatalf("unchanged/deadband samples were saved: %#v", inside)
	}
	changed := applyHistorySavePolicy(config, base.Add(2*time.Second), map[string]map[string]interface{}{"device": {"temperature": 11.1, "state": "running"}}, last)
	if len(changed["device"]) != 2 {
		t.Fatalf("changed samples were not saved: %#v", changed)
	}
}

func TestHistoryGroupedValuesFiltersSelectedPoints(t *testing.T) {
	grouped := map[string]map[string]interface{}{
		"device-1": {"a": 1, "b": 2},
		"device-2": {"a": 3},
	}
	result := historyGroupedValues(config.HistoryStorageConfig{
		SelectionMode: "selected",
		Points: []config.HistoryPointRef{
			{DeviceKey: "device-1", Metric: "b"},
			{DeviceKey: "device-2", Metric: "a"},
		},
	}, grouped)
	want := map[string]map[string]interface{}{
		"device-1": {"b": 2},
		"device-2": {"a": 3},
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("selected history values = %#v, want %#v", result, want)
	}
}

func TestHistoryGroupedValuesSelectedWithoutPointsWritesNothing(t *testing.T) {
	grouped := map[string]map[string]interface{}{"device-1": {"a": 1}}
	if result := historyGroupedValues(config.HistoryStorageConfig{SelectionMode: "selected"}, grouped); len(result) != 0 {
		t.Fatalf("empty selection should not persist values: %#v", result)
	}
}

func TestHistoryTasksRespectScheduleAndMergeSelectedPoints(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	enabled := true
	config := config.HistoryStorageConfig{Enabled: true, Tasks: []config.HistoryStorageTask{
		{TaskKey: "running", Name: "运行任务", Enabled: &enabled, SelectionMode: "selected", Points: []config.HistoryPointRef{{DeviceKey: "device", Metric: "a"}}, SaveMode: "interval", SaveIntervalSeconds: 1, StartAt: &past, EndAt: &future},
		{TaskKey: "completed", Name: "已结束任务", Enabled: &enabled, SelectionMode: "selected", Points: []config.HistoryPointRef{{DeviceKey: "device", Metric: "b"}}, SaveMode: "interval", SaveIntervalSeconds: 1, EndAt: &past},
	}}
	result := historyTaskGroupedValues(config, now, map[string]map[string]interface{}{"device": {"a": 1, "b": 2}}, map[string]historySavedSample{})
	if !reflect.DeepEqual(result, map[string]map[string]interface{}{"device": {"a": 1}}) {
		t.Fatalf("scheduled task values = %#v", result)
	}
}

package alarm

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func manualTestConfig() config.Config {
	threshold := 10.0
	cfg := config.Config{GatewayKey: "gw", Points: []config.PointConfig{{DeviceKey: "d1", Metric: "temp", Name: "温度", Unit: "℃", Protocol: "modbus-tcp", Address: "127.0.0.1:502", Function: 3, Quantity: 1, DataType: "float64", Scale: 1}}, Alarms: config.AlarmConfig{Rules: []config.AlarmRuleConfig{{RuleID: "high", Name: "高温", DeviceKey: "d1", Metric: "temp", Type: "threshold", Operator: ">=", Threshold: &threshold, Level: "WARNING"}}}}
	cfg.ApplyDefaults()
	return cfg
}

func TestManualRecoveryDefaultForbiddenAndLiveValue(t *testing.T) {
	cfg := manualTestConfig()
	store := state.New(cfg)
	manager := NewManager(cfg, store, "")
	now := time.Now()
	store.SetPointValue("d1", "temp", 10.0)
	manager.evaluate(now)
	id := manager.ActiveSnapshot().Active[0].EventID
	// Inclusive boundaries must not oscillate at exactly the threshold.
	for i := 0; i < 5; i++ {
		manager.evaluate(now.Add(time.Duration(i) * time.Second))
	}
	if len(manager.Snapshot().History) != 1 {
		t.Fatal("threshold equality repeatedly resolved/retriggered")
	}
	if _, err := manager.ManualRecover(id, "admin"); !errors.Is(err, ErrManualRecoveryDisabled) {
		t.Fatalf("manual recovery was not blocked: %v", err)
	}
	if _, err := manager.Acknowledge(id); err != nil {
		t.Fatal(err)
	}
	store.SetPointValue("d1", "temp", 12.0)
	manager.evaluate(now)
	active := manager.ActiveSnapshot().Active[0]
	if active.Value != 12.0 || active.Unit != "℃" || !active.Acknowledged {
		t.Fatalf("bad live snapshot: %+v", active)
	}
	if manager.Snapshot().History[1].Value != 10.0 {
		t.Fatal("trigger history was rewritten")
	}
	store.SetPointResult(model.PointValue{DeviceKey: "d1", Metric: "temp", Value: 0.0, Quality: model.QualityBad})
	manager.evaluate(now)
	if manager.ActiveSnapshot().Active[0].RecoveryStatus != "awaiting_data" {
		t.Fatal("invalid data incorrectly resolved alarm")
	}
	store.SetPointValue("d1", "temp", 9.0)
	manager.evaluate(now)
	if manager.ActiveSnapshot().ActiveCount != 0 {
		t.Fatal("valid normal data did not resolve")
	}
}

func TestManualRecoveryPersistsAndRearmsAfterValidRecovery(t *testing.T) {
	cfg := manualTestConfig()
	cfg.Alarms.Rules[0].AllowManualRecovery = true
	cfg.Alarms.Rules[0].Hysteresis = 2
	cfg.Alarms.Rules[0].RecoveryDelaySeconds = 2
	raw, _ := json.Marshal(cfg)
	var restored config.Config
	if err := json.Unmarshal(raw, &restored); err != nil || !restored.Alarms.Rules[0].AllowManualRecovery {
		t.Fatal("manual recovery setting lost")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	store := state.New(cfg)
	manager := NewManager(cfg, store, path)
	now := time.Now()
	store.SetPointValue("d1", "temp", 12.0)
	manager.evaluate(now)
	id := manager.ActiveSnapshot().Active[0].EventID
	record, err := manager.ManualRecover(id, "operator-test")
	if err != nil {
		t.Fatal(err)
	}
	if record.State != "MANUAL_RESOLVED" || !strings.Contains(record.Message, "operator-test") || manager.ActiveSnapshot().ActiveCount != 0 {
		t.Fatalf("invalid manual recovery: %+v", record)
	}
	if _, err := manager.ManualRecover(id, "operator-test"); err != nil {
		t.Fatal("duplicate recovery should be idempotent")
	}
	page, err := manager.History(HistoryQuery{State: "MANUAL_RESOLVED"})
	if err != nil || page.Total != 1 {
		t.Fatalf("manual history missing/duplicated: %+v %v", page, err)
	}
	manager.closeHistory()
	manager = NewManager(cfg, store, path)
	defer manager.closeHistory()
	manager.evaluate(now.Add(time.Second))
	if manager.ActiveSnapshot().ActiveCount != 0 {
		t.Fatal("same fault retriggered after restart")
	}
	store.SetPointResult(model.PointValue{DeviceKey: "d1", Metric: "temp", Value: 0.0, Quality: model.QualityBad})
	manager.evaluate(now.Add(2 * time.Second))
	store.SetPointValue("d1", "temp", 12.0)
	manager.evaluate(now.Add(5 * time.Second))
	if manager.ActiveSnapshot().ActiveCount != 0 {
		t.Fatal("bad data rearmed alarm")
	}
	store.SetPointValue("d1", "temp", 9.0)
	manager.evaluate(now.Add(6 * time.Second))
	manager.evaluate(now.Add(9 * time.Second))
	if manager.runtimes["high"].ManualReset == nil {
		t.Fatal("rearmed inside hysteresis band")
	}
	store.SetPointValue("d1", "temp", 8.0)
	manager.evaluate(now.Add(10 * time.Second))
	manager.evaluate(now.Add(11 * time.Second))
	if manager.runtimes["high"].ManualReset == nil {
		t.Fatal("ignored recovery delay")
	}
	manager.evaluate(now.Add(12 * time.Second))
	if manager.runtimes["high"].ManualReset != nil {
		t.Fatal("did not rearm after valid recovery")
	}
	store.SetPointValue("d1", "temp", 12.0)
	manager.evaluate(now.Add(13 * time.Second))
	if manager.ActiveSnapshot().ActiveCount != 1 || manager.ActiveSnapshot().Active[0].EventID == id {
		t.Fatal("new episode did not trigger")
	}
}

func TestManualRecoveryStorageFailureKeepsAlarm(t *testing.T) {
	cfg := manualTestConfig()
	cfg.Alarms.Rules[0].AllowManualRecovery = true
	store := state.New(cfg)
	manager := NewManager(cfg, store, filepath.Join(t.TempDir(), "config.json"))
	defer manager.closeHistory()
	store.SetPointValue("d1", "temp", 12.0)
	manager.evaluate(time.Now())
	id := manager.ActiveSnapshot().Active[0].EventID
	if manager.storage == nil {
		t.Fatal("database unavailable")
	}
	manager.storage.db.Close()
	if _, err := manager.ManualRecover(id, "admin"); err == nil {
		t.Fatal("recovered without durable audit record")
	}
	if manager.ActiveSnapshot().ActiveCount != 1 || len(manager.Snapshot().History) != 1 {
		t.Fatal("failed recovery changed alarm state")
	}
}

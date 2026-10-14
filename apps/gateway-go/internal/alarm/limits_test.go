package alarm

import (
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func testLimit(value float64) *config.AlarmLimitConfig {
	return &config.AlarmLimitConfig{Threshold: &value}
}
func limitsConfig() config.Config {
	cfg := manualTestConfig()
	cfg.Alarms.Rules[0].Limits = &config.AlarmLimitsConfig{LowLow: testLimit(-20), Low: testLimit(-10), High: testLimit(10), HighHigh: testLimit(20)}
	cfg.Alarms.Rules[0].Hysteresis = 2
	cfg.ApplyDefaults()
	return cfg
}
func assertLimits(t *testing.T, m *Manager, ids ...string) {
	t.Helper()
	actual := map[string]bool{}
	for _, e := range m.ActiveSnapshot().Active {
		actual[e.RuleID] = true
	}
	if len(actual) != len(ids) {
		t.Fatalf("active=%v expected=%v", actual, ids)
	}
	for _, id := range ids {
		if !actual["high::"+id] {
			t.Fatalf("missing %s: %v", id, actual)
		}
	}
}

func TestFourLimitsIndependentRecovery(t *testing.T) {
	cfg := limitsConfig()
	store := state.New(cfg)
	manager := NewManager(cfg, store, "")
	now := time.Now()
	set := func(value float64) { store.SetPointValue("d1", "temp", value); manager.evaluate(now) }
	set(10)
	assertLimits(t, manager)
	set(11)
	assertLimits(t, manager, "high")
	set(25)
	assertLimits(t, manager, "high", "highHigh")
	historyLen := len(manager.Snapshot().History)
	set(30)
	if len(manager.Snapshot().History) != historyLen {
		t.Fatal("continuous threshold creates duplicate events")
	}
	set(19)
	assertLimits(t, manager, "high", "highHigh")
	set(18)
	assertLimits(t, manager, "high")
	set(9)
	assertLimits(t, manager, "high")
	set(8)
	assertLimits(t, manager)
	set(-25)
	assertLimits(t, manager, "low", "lowLow")
	set(-19)
	assertLimits(t, manager, "low", "lowLow")
	set(-18)
	assertLimits(t, manager, "low")
	set(-8)
	assertLimits(t, manager)
	// Crossing from one side to the other resolves old alarms and triggers new ones.
	set(30)
	set(-30)
	assertLimits(t, manager, "low", "lowLow")
}

func TestFourLimitsDelayInvalidDataAndConfigReload(t *testing.T) {
	cfg := limitsConfig()
	cfg.Alarms.Rules[0].TriggerDelaySeconds = 1
	cfg.Alarms.Rules[0].RecoveryDelaySeconds = 2
	store := state.New(cfg)
	m := NewManager(cfg, store, "")
	now := time.Now()
	store.SetPointValue("d1", "temp", 30.0)
	m.evaluate(now)
	assertLimits(t, m)
	m.evaluate(now.Add(time.Second))
	assertLimits(t, m, "high", "highHigh")
	m.UpdateConfig(cfg, "")
	assertLimits(t, m, "high", "highHigh")
	store.SetPointResult(model.PointValue{DeviceKey: "d1", Metric: "temp", Value: 0.0, Quality: model.QualityBad})
	m.evaluate(now.Add(2 * time.Second))
	assertLimits(t, m, "high", "highHigh")
	store.SetPointValue("d1", "temp", 0.0)
	m.evaluate(now.Add(3 * time.Second))
	m.evaluate(now.Add(4 * time.Second))
	assertLimits(t, m, "high", "highHigh")
	m.evaluate(now.Add(5 * time.Second))
	assertLimits(t, m)
	store.SetPointValue("d1", "temp", 30.0)
	m.evaluate(now.Add(6 * time.Second))
	m.evaluate(now.Add(7 * time.Second))
	disabled := false
	cfg.Alarms.Rules[0].Limits.HighHigh = &config.AlarmLimitConfig{Enabled: &disabled}
	m.UpdateConfig(cfg, "")
	assertLimits(t, m, "high")
}

func TestFourLimitsManualRecoveryPersistsPerLimit(t *testing.T) {
	cfg := limitsConfig()
	cfg.Alarms.Rules[0].AllowManualRecovery = true
	store := state.New(cfg)
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewManager(cfg, store, path)
	now := time.Now()
	store.SetPointValue("d1", "temp", 15.0)
	m.evaluate(now)
	id := m.ActiveSnapshot().Active[0].EventID
	if _, err := m.ManualRecover(id, "operator"); err != nil {
		t.Fatal(err)
	}
	store.SetPointValue("d1", "temp", 30.0)
	m.evaluate(now)
	assertLimits(t, m, "highHigh")
	m.closeHistory()
	m = NewManager(cfg, store, path)
	defer m.closeHistory()
	m.evaluate(now)
	assertLimits(t, m, "highHigh")
	page, err := m.History(HistoryQuery{State: "MANUAL_RESOLVED"})
	if err != nil || page.Total != 1 || page.Items[0].RuleID != "high::high" {
		t.Fatalf("wrong per-limit history: %+v %v", page, err)
	}
	store.SetPointValue("d1", "temp", 8.0)
	m.evaluate(now)
	store.SetPointValue("d1", "temp", 15.0)
	m.evaluate(now)
	assertLimits(t, m, "high")
}

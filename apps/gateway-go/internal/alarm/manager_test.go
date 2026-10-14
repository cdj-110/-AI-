package alarm

import (
	"context"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestThresholdAlarmHysteresisAcknowledgeAndRecovery(t *testing.T) {
	enabled := true
	threshold := 10.0
	cfg := config.Config{
		GatewayKey: "gw",
		Points:     []config.PointConfig{{DeviceKey: "d1", Name: "温度", Metric: "temperature", Protocol: "modbus-tcp", Address: "127.0.0.1:502", Function: 3, Quantity: 1, DataType: "float64", Scale: 1}},
		Alarms:     config.AlarmConfig{HistoryLimit: 20, Rules: []config.AlarmRuleConfig{{RuleID: "high", Name: "温度过高", Enabled: &enabled, DeviceKey: "d1", Metric: "temperature", Type: "threshold", Operator: ">", Threshold: &threshold, Hysteresis: 2, Level: "CRITICAL"}}},
	}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	manager := NewManager(cfg, store, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)

	store.SetPointValue("d1", "temperature", 11.0)
	waitFor(t, func() bool { return manager.Snapshot().ActiveCount == 1 })
	active := manager.Snapshot().Active[0]
	if active.Level != "CRITICAL" || active.Value != 11.0 {
		t.Fatalf("unexpected active alarm: %#v", active)
	}
	lightweight := manager.ActiveSnapshot()
	if lightweight.ActiveCount != 1 || len(lightweight.Active) != 1 || len(lightweight.History) != 0 {
		t.Fatalf("unexpected lightweight alarm snapshot: %#v", lightweight)
	}
	if _, err := manager.Acknowledge(active.EventID); err != nil {
		t.Fatal(err)
	}
	if !manager.Snapshot().Active[0].Acknowledged {
		t.Fatal("alarm was not acknowledged")
	}

	store.SetPointValue("d1", "temperature", 9.0)
	time.Sleep(30 * time.Millisecond)
	if manager.Snapshot().ActiveCount != 1 {
		t.Fatal("alarm recovered inside hysteresis band")
	}
	store.SetPointValue("d1", "temperature", 8.0)
	waitFor(t, func() bool { return manager.Snapshot().ActiveCount == 0 })
	if manager.Snapshot().History[0].State != "RESOLVED" {
		t.Fatalf("missing resolved event: %#v", manager.Snapshot().History)
	}
}

func TestQualityAndBooleanRules(t *testing.T) {
	enabled, wanted := true, true
	cfg := config.Config{GatewayKey: "gw", Points: []config.PointConfig{{DeviceKey: "d1", Metric: "run", Name: "运行", Protocol: "modbus-tcp", Address: "127.0.0.1:502", Function: 1, Quantity: 1, DataType: "bool", Scale: 1}}, Alarms: config.AlarmConfig{HistoryLimit: 20, Rules: []config.AlarmRuleConfig{
		{RuleID: "bool", Name: "运行告警", Enabled: &enabled, DeviceKey: "d1", Metric: "run", Type: "boolean", BooleanValue: &wanted, Level: "WARNING"},
		{RuleID: "bad", Name: "质量告警", Enabled: &enabled, DeviceKey: "d1", Metric: "run", Type: "quality", Quality: "BAD", Level: "CRITICAL"},
	}}}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	manager := NewManager(cfg, store, "")
	store.SetPointValue("d1", "run", true)
	manager.evaluate(time.Now())
	if manager.Snapshot().ActiveCount != 1 {
		t.Fatalf("boolean alarm not active: %#v", manager.Snapshot())
	}
	store.SetPointResult(model.PointValue{DeviceKey: "d1", Metric: "run", Value: false, Quality: model.QualityBad})
	manager.evaluate(time.Now())
	if manager.Snapshot().ActiveCount != 2 {
		t.Fatalf("quality alarm not active: %#v", manager.Snapshot())
	}
	// A bad-quality false value must not resolve the existing boolean alarm.
	store.SetPointResult(model.PointValue{DeviceKey: "d1", Metric: "run", Value: false, Quality: model.QualityGood})
	manager.evaluate(time.Now())
	if manager.Snapshot().ActiveCount != 0 {
		t.Fatal("valid normal data must resolve both alarms")
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met")
}

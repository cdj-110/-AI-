package alarm

import (
	"encoding/json"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestRetainAcknowledgedPolicyDoesNotRetrigger(t *testing.T) {
	cfg := manualTestConfig()
	store := state.New(cfg)
	m := NewManager(cfg, store, "")
	store.SetPointValue("d1", "temp", 20.0)
	m.evaluate(time.Now())
	original := m.ActiveSnapshot().Active[0]
	if original.RetainAfterAcknowledged {
		t.Fatal("legacy rule must hide after acknowledgement")
	}
	if _, err := m.Acknowledge(original.EventID); err != nil {
		t.Fatal(err)
	}
	historyCount := len(m.Snapshot().History)
	for _, enabled := range []bool{true, false, true} {
		cfg.Alarms.Rules[0].RetainAfterAcknowledged = enabled
		m.UpdateConfig(cfg, "")
		m.evaluate(time.Now())
		s := m.ActiveSnapshot()
		e := s.Active[0]
		if e.EventID != original.EventID || !e.Acknowledged || e.RetainAfterAcknowledged != enabled || s.UnackedCount != 0 || len(m.Snapshot().History) != historyCount {
			t.Fatalf("presentation update changed alarm lifecycle: %+v", s)
		}
		raw, _ := json.Marshal(e)
		var api map[string]interface{}
		if err := json.Unmarshal(raw, &api); err != nil {
			t.Fatal(err)
		}
		if api["retainAfterAcknowledged"] != enabled {
			t.Fatal("event API omitted policy")
		}
	}
	store.SetPointValue("d1", "temp", 0.0)
	m.evaluate(time.Now())
	if m.ActiveSnapshot().ActiveCount != 0 {
		t.Fatal("retained alarm must still recover")
	}
	store.SetPointValue("d1", "temp", 20.0)
	m.evaluate(time.Now())
	next := m.ActiveSnapshot().Active[0]
	if next.Acknowledged || next.EventID == original.EventID || !next.RetainAfterAcknowledged {
		t.Fatal("new episode must require new acknowledgement")
	}
}

func TestRetainAcknowledgedFourLimits(t *testing.T) {
	cfg := limitsConfig()
	cfg.Alarms.Rules[0].RetainAfterAcknowledged = true
	store := state.New(cfg)
	m := NewManager(cfg, store, "")
	store.SetPointValue("d1", "temp", 30.0)
	m.evaluate(time.Now())
	if m.ActiveSnapshot().ActiveCount != 2 {
		t.Fatal("expected two limits")
	}
	for _, event := range m.ActiveSnapshot().Active {
		if !event.RetainAfterAcknowledged {
			t.Fatal("child must inherit policy")
		}
		if _, err := m.Acknowledge(event.EventID); err != nil {
			t.Fatal(err)
		}
	}
	if s := m.ActiveSnapshot(); s.ActiveCount != 2 || s.UnackedCount != 0 {
		t.Fatal("acknowledgement must not recover limits")
	}
}

func TestRetainAcknowledgedConfigCompatibility(t *testing.T) {
	for _, raw := range []string{`{}`, `{"retainAfterAcknowledged":false}`, `{"retainAfterAcknowledged":true}`} {
		var rule config.AlarmRuleConfig
		if err := json.Unmarshal([]byte(raw), &rule); err != nil {
			t.Fatal(err)
		}
		want := raw == `{"retainAfterAcknowledged":true}`
		if rule.RetainAfterAcknowledged != want {
			t.Fatal(raw)
		}
		encoded, _ := json.Marshal(rule)
		var restored config.AlarmRuleConfig
		if err := json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		if restored.RetainAfterAcknowledged != want {
			t.Fatal("round trip lost policy")
		}
	}
}

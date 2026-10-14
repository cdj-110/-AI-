package alarm

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestPopupPolicyKeepsAlarmEpisode(t *testing.T) {
	cfg := manualTestConfig()
	store := state.New(cfg)
	m := NewManager(cfg, store, "")
	store.SetPointValue("d1", "temp", 20.0)
	m.evaluate(time.Now())
	initial := m.ActiveSnapshot().Active[0]
	if !initial.PopupEnabled {
		t.Fatal("legacy rule must popup")
	}
	off := false
	cfg.Alarms.Rules[0].PopupEnabled = &off
	m.UpdateConfig(cfg, "")
	m.evaluate(time.Now())
	active := m.ActiveSnapshot().Active[0]
	if active.PopupEnabled || active.SoundWhenPopupDisabled || active.EventID != initial.EventID || len(m.Snapshot().History) != 1 {
		t.Fatal("popup change altered alarm episode")
	}
	cfg.Alarms.Rules[0].SoundWhenPopupDisabled = true
	m.UpdateConfig(cfg, "")
	active = m.ActiveSnapshot().Active[0]
	if !active.SoundWhenPopupDisabled || active.EventID != initial.EventID || len(m.Snapshot().History) != 1 {
		t.Fatal("sound-only policy altered alarm episode")
	}
	raw, _ := json.Marshal(active)
	var fields map[string]interface{}
	json.Unmarshal(raw, &fields)
	if fields["popupEnabled"] != false {
		t.Fatal("API must explicitly include false")
	}
	on := true
	cfg.Alarms.Rules[0].PopupEnabled = &on
	m.UpdateConfig(cfg, "")
	if e := m.ActiveSnapshot().Active[0]; !e.PopupEnabled || e.EventID != initial.EventID {
		t.Fatal("enabling popup retriggered alarm")
	}
	store.SetPointValue("d1", "temp", 0.0)
	m.evaluate(time.Now())
	if m.ActiveSnapshot().ActiveCount != 0 || len(m.Snapshot().History) != 2 {
		t.Fatal("normal recovery affected by popup policy")
	}
}

func TestPopupDisabledFourLimitsStillRecorded(t *testing.T) {
	cfg := limitsConfig()
	off := false
	cfg.Alarms.Rules[0].PopupEnabled = &off
	store := state.New(cfg)
	m := NewManager(cfg, store, "")
	store.SetPointValue("d1", "temp", 30.0)
	m.evaluate(time.Now())
	snapshot := m.Snapshot()
	if snapshot.ActiveCount != 2 || snapshot.UnackedCount != 2 || len(snapshot.History) != 2 {
		t.Fatal("disabled popup suppressed alarm")
	}
	for _, event := range snapshot.Active {
		if event.PopupEnabled {
			t.Fatal("child must inherit popup policy")
		}
	}
}

func TestPopupPolicyPreservesManualResetAfterRestart(t *testing.T) {
	cfg := manualTestConfig()
	cfg.Alarms.Rules[0].AllowManualRecovery = true
	cfg.Alarms.StoragePath = filepath.Join(t.TempDir(), "alarm.db")
	store := state.New(cfg)
	m := NewManager(cfg, store, "")
	store.SetPointValue("d1", "temp", 20.0)
	m.evaluate(time.Now())
	if _, err := m.ManualRecover(m.ActiveSnapshot().Active[0].EventID, "admin"); err != nil {
		t.Fatal(err)
	}
	off := false
	cfg.Alarms.Rules[0].PopupEnabled = &off
	m.UpdateConfig(cfg, "")
	m.evaluate(time.Now())
	if m.ActiveSnapshot().ActiveCount != 0 {
		t.Fatal("popup update cleared manual suppression")
	}
	m.closeHistory()
	reopened := NewManager(cfg, store, "")
	defer reopened.closeHistory()
	reopened.evaluate(time.Now())
	if reopened.ActiveSnapshot().ActiveCount != 0 {
		t.Fatal("popup setting broke persisted suppression")
	}
}

func TestPopupConfigJSONCompatibility(t *testing.T) {
	for _, raw := range []string{`{}`, `{"popupEnabled":true}`, `{"popupEnabled":false}`} {
		var rule config.AlarmRuleConfig
		if err := json.Unmarshal([]byte(raw), &rule); err != nil {
			t.Fatal(err)
		}
		want := raw != `{"popupEnabled":false}`
		if rule.IsPopupEnabled() != want {
			t.Fatal(raw)
		}
		encoded, _ := json.Marshal(rule)
		var restored config.AlarmRuleConfig
		json.Unmarshal(encoded, &restored)
		if restored.IsPopupEnabled() != want {
			t.Fatal("round trip changed setting")
		}
	}
}

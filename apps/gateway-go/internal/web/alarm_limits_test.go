package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestAlarmRulesAPIFourLimits(t *testing.T) {
	cfg := alarmWebConfig()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	server := New(config.ListenerConfig{}, state.New(cfg), nil, path, func(config.Config) error { return nil })
	rule := config.AlarmRuleConfig{RuleID: "four", Name: "四限", DeviceKey: "d1", Metric: "temperature", Type: "threshold", Limits: &config.AlarmLimitsConfig{High: &config.AlarmLimitConfig{Threshold: floatPointer(10)}, HighHigh: &config.AlarmLimitConfig{Threshold: floatPointer(20)}}}
	off := false
	rule.PopupEnabled = &off
	rule.SoundWhenPopupDisabled = true
	rule.RetainAfterAcknowledged = true
	put := func(permission string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]interface{}{"rules": []config.AlarmRuleConfig{rule}})
		r := httptest.NewRequest(http.MethodPut, "/api/alarm-rules", bytes.NewReader(raw))
		authorizeConfigTestRequest(server, r, permission)
		w := httptest.NewRecorder()
		server.routes().ServeHTTP(w, r)
		return w
	}
	if w := put("alarms.view"); w.Code != 403 {
		t.Fatal("viewer can edit limits")
	}
	if w := put("alarms.manage"); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	loaded, err := config.Load(path)
	if err != nil || len(loaded.Alarms.Rules) != 1 || loaded.Alarms.Rules[0].Limits.HighHigh.Level != "CRITICAL" {
		t.Fatalf("grouped config not persisted: %+v %v", loaded.Alarms.Rules, err)
	}
	if loaded.Alarms.Rules[0].PopupEnabled == nil || loaded.Alarms.Rules[0].IsPopupEnabled() {
		t.Fatal("popup policy did not persist")
	}
	if !loaded.Alarms.Rules[0].RetainAfterAcknowledged {
		t.Fatal("acknowledged card retention did not persist")
	}
	if !loaded.Alarms.Rules[0].SoundWhenPopupDisabled {
		t.Fatal("sound-only reminder policy did not persist")
	}
	rule.Limits.HighHigh.Threshold = floatPointer(5)
	if w := put("alarms.manage"); w.Code != 400 {
		t.Fatalf("invalid ordering accepted: %d %s", w.Code, w.Body)
	}
	loaded, _ = config.Load(path)
	if *loaded.Alarms.Rules[0].Limits.HighHigh.Threshold != 20 {
		t.Fatal("invalid save changed existing config")
	}
}

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestAlarmPopupSettings(t *testing.T) {
	cfg := alarmWebConfig()
	cfg.Alarms.Rules = []config.AlarmRuleConfig{{RuleID: "r", Name: "温度", DeviceKey: "d1", Metric: "temperature", Type: "threshold", Operator: ">", Threshold: floatPointer(10), Level: "WARNING"}}
	cfg.ApplyDefaults()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	store := state.New(cfg)
	manager := alarm.NewManager(cfg, store, "")
	server := New(config.ListenerConfig{}, store, nil, path, func(next config.Config) error { manager.UpdateConfig(next, ""); return nil }, manager)
	invoke := func(method, body, permission string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/alarms/settings", bytes.NewBufferString(body))
		authorizeConfigTestRequest(server, r, permission)
		w := httptest.NewRecorder()
		server.routes().ServeHTTP(w, r)
		return w
	}
	if cfg.Alarms.PopupDuration() != 10 {
		t.Fatal("default duration changed")
	}
	w := invoke(http.MethodGet, "", "alarms.view")
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"popupDurationSeconds":10`)) {
		t.Fatalf("default GET: %d %s", w.Code, w.Body)
	}
	if w := invoke(http.MethodPut, `{"popupDurationSeconds":0}`, "alarms.view"); w.Code != 403 {
		t.Fatal("viewer changed settings")
	}
	for _, body := range []string{`{}`, `{"popupDurationSeconds":null}`, `{"popupDurationSeconds":-1}`, `{"popupDurationSeconds":3601}`, `{"popupDurationSeconds":1.5}`} {
		if w := invoke(http.MethodPut, body, "alarms.manage"); w.Code != 400 {
			t.Fatalf("accepted %s: %d %s", body, w.Code, w.Body)
		}
	}
	for _, duration := range []int{0, 5, 3600, 10} {
		raw, _ := json.Marshal(map[string]int{"popupDurationSeconds": duration})
		w := invoke(http.MethodPut, string(raw), "alarms.manage")
		if w.Code != 200 {
			t.Fatalf("save failed: %d %s", w.Code, w.Body)
		}
		loaded, err := config.Load(path)
		if err != nil || loaded.Alarms.PopupDuration() != duration {
			t.Fatalf("duration not persisted: %v", err)
		}
		if !reflect.DeepEqual(loaded.Alarms.Rules, cfg.Alarms.Rules) {
			t.Fatal("popup settings rewrote alarm rules")
		}
		if manager.ActiveSnapshot().PopupDurationSeconds != duration {
			t.Fatal("popup duration not hot-loaded")
		}
	}
}

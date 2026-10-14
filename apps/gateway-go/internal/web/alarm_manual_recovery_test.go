package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestManualRecoveryAPIEnforcesRuleAndPermission(t *testing.T) {
	for _, allow := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic-only", true: "manual-allowed"}[allow], func(t *testing.T) {
			cfg := alarmWebConfig()
			cfg.Alarms.Rules = []config.AlarmRuleConfig{{RuleID: "r", Name: "温度", DeviceKey: "d1", Metric: "temperature", Type: "threshold", Operator: ">", Threshold: floatPointer(10), AllowManualRecovery: allow, Level: "WARNING"}}
			cfg.ApplyDefaults()
			path := filepath.Join(t.TempDir(), "config.json")
			if err := config.Save(path, cfg); err != nil {
				t.Fatal(err)
			}
			store := state.New(cfg)
			manager := alarm.NewManager(cfg, store, path)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { defer close(done); manager.Start(ctx) }()
			defer func() { cancel(); <-done }()
			server := New(config.ListenerConfig{}, store, nil, path, nil, manager)
			store.SetPointValue("d1", "temperature", 20.0)
			deadline := time.Now().Add(2 * time.Second)
			for manager.ActiveSnapshot().ActiveCount == 0 && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if manager.ActiveSnapshot().ActiveCount != 1 {
				t.Fatal("alarm not triggered")
			}
			id := manager.ActiveSnapshot().Active[0].EventID
			invoke := func(eventID string, perms ...string) *httptest.ResponseRecorder {
				body, _ := json.Marshal(map[string]string{"eventId": eventID})
				r := httptest.NewRequest(http.MethodPost, "/api/alarms/manual-recover", bytes.NewReader(body))
				authorizeConfigTestRequest(server, r, perms...)
				w := httptest.NewRecorder()
				server.routes().ServeHTTP(w, r)
				return w
			}
			if w := invoke(id, "alarms.view"); w.Code != http.StatusForbidden {
				t.Fatalf("viewer recovered alarm: %d %s", w.Code, w.Body)
			}
			if w := invoke("missing", "alarms.manage"); w.Code != http.StatusConflict {
				t.Fatalf("missing event: %d %s", w.Code, w.Body)
			}
			w := invoke(id, "alarms.manage")
			if !allow {
				if w.Code != http.StatusForbidden || manager.ActiveSnapshot().ActiveCount != 1 {
					t.Fatalf("automatic rule recovered: %d %s", w.Code, w.Body)
				}
				return
			}
			if w.Code != http.StatusOK || manager.ActiveSnapshot().ActiveCount != 0 {
				t.Fatalf("manual recovery failed: %d %s", w.Code, w.Body)
			}
			history, err := manager.History(alarm.HistoryQuery{State: "MANUAL_RESOLVED"})
			if err != nil || history.Total != 1 || !strings.Contains(history.Items[0].Message, "test") {
				t.Fatalf("missing actor history: %+v %v", history, err)
			}
		})
	}
}

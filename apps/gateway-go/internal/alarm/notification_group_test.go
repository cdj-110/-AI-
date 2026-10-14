package alarm

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestFourLimitNotificationParentMetadata(t *testing.T) {
	cfg := limitsConfig()
	cfg.Alarms.Rules[0].Name = "温度规则"
	store := state.New(cfg)
	manager := NewManager(cfg, store, "")
	store.SetPointValue("d1", "temp", 30.0)
	manager.evaluate(time.Now())
	snap := manager.ActiveSnapshot()
	if len(snap.Active) != 2 {
		t.Fatal("notification grouping must not merge active events")
	}
	for _, event := range snap.Active {
		if event.ParentRuleID != "high" || event.ParentRuleName != "温度规则" || event.LimitName == "" || event.LimitKey == "" {
			t.Fatalf("missing popup metadata: %+v", event)
		}
		raw, _ := json.Marshal(event)
		if !strings.Contains(string(raw), `"parentRuleId":"high"`) {
			t.Fatal("metadata missing from API")
		}
	}
	original := cfg.Alarms.RuntimeRules()[0]
	without := original
	without.ParentRuleID, without.ParentRuleName, without.LimitKey, without.LimitName = "", "", "", ""
	if ruleSignature(original) != ruleSignature(without) {
		t.Fatal("grouping metadata changed runtime/manual-reset signature")
	}
	raw, _ := json.Marshal(cfg)
	if strings.Contains(string(raw), "ParentRule") || strings.Contains(string(raw), "parentRuleId") {
		t.Fatal("runtime metadata leaked into configuration")
	}
	// Recover high-high independently while high remains active.
	store.SetPointValue("d1", "temp", 18.0)
	manager.evaluate(time.Now())
	active := manager.ActiveSnapshot().Active
	if len(active) != 1 || active[0].LimitKey != "high" || len(manager.Snapshot().History) != 3 {
		t.Fatal("independent limit recovery/history changed")
	}
}

package alarm

import (
	"reflect"
	"testing"
	"time"
)

func TestActiveSnapshotOrderStableForSameTimestamp(t *testing.T) {
	now := time.Now()
	manager := &Manager{runtimes: map[string]*ruleRuntime{}}
	events := []Event{
		{EventID: "b", RuleID: "r-b", OccurredAt: now},
		{EventID: "a2", RuleID: "r-a", OccurredAt: now},
		{EventID: "old", RuleID: "r-0", OccurredAt: now.Add(-time.Second)},
		{EventID: "a1", RuleID: "r-a", OccurredAt: now},
		{EventID: "new", RuleID: "r-z", OccurredAt: now.Add(time.Second)},
	}
	for i := range events {
		manager.runtimes[events[i].EventID] = &ruleRuntime{Active: &events[i]}
	}
	want := []string{"new", "a1", "a2", "b", "old"}
	for i := 0; i < 200; i++ {
		for _, r := range manager.runtimes {
			r.Active.UpdatedAt = now.Add(time.Duration(i) * time.Second)
			r.Active.Value = i
			r.Active.Acknowledged = i%2 == 0
		}
		var got []string
		for _, event := range manager.ActiveSnapshot().Active {
			got = append(got, event.EventID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("order changed: got %v want %v", got, want)
		}
	}
}

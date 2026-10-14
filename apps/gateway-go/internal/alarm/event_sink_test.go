package alarm

import (
	"reflect"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestEventSinkReceivesLifecycleTransitionsOnly(t *testing.T) {
	cfg := manualTestConfig()
	store := state.New(cfg)
	manager := NewManager(cfg, store, "")
	var states []string
	manager.SetEventSink(func(event Event) error {
		states = append(states, event.State)
		return nil
	})
	now := time.Now()
	store.SetPointValue("d1", "temp", 12.0)
	manager.evaluate(now)
	id := manager.ActiveSnapshot().Active[0].EventID
	// Live-value refreshes are intentionally not exported as new events.
	store.SetPointValue("d1", "temp", 13.0)
	manager.evaluate(now.Add(time.Second))
	if _, err := manager.Acknowledge(id); err != nil {
		t.Fatal(err)
	}
	store.SetPointValue("d1", "temp", 9.0)
	manager.evaluate(now.Add(2 * time.Second))
	if want := []string{"ACTIVE", "ACKNOWLEDGED", "RESOLVED"}; !reflect.DeepEqual(states, want) {
		t.Fatalf("states=%v want=%v", states, want)
	}
}

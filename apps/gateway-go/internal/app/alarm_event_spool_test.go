package app

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
)

func TestAlarmEventSpoolRetriesInOrder(t *testing.T) {
	spool := newAlarmEventSpool(filepath.Join(t.TempDir(), "alarm-events.jsonl"), 1024*1024)
	for _, state := range []string{"ACTIVE", "ACKNOWLEDGED", "RESOLVED"} {
		if err := spool.append(alarm.Event{EventID: "event-1", State: state, UpdatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	var first []string
	if err := spool.drain(20, func(event alarm.Event) error {
		first = append(first, event.State)
		if event.State == "ACKNOWLEDGED" {
			return errors.New("offline")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"ACTIVE", "ACKNOWLEDGED"}; !reflect.DeepEqual(first, want) {
		t.Fatalf("first attempts=%v want=%v", first, want)
	}
	var retried []string
	if err := spool.drain(20, func(event alarm.Event) error {
		retried = append(retried, event.State)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"ACKNOWLEDGED", "RESOLVED"}; !reflect.DeepEqual(retried, want) {
		t.Fatalf("retry=%v want=%v", retried, want)
	}
}

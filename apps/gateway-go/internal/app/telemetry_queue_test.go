package app

import (
	"context"
	"testing"
	"time"
)

func TestLatestTelemetryQueueCoalescesByDevice(t *testing.T) {
	queue := newLatestTelemetryQueue()
	queue.Put(telemetryBatch{Time: time.Unix(1, 0), Grouped: map[string]map[string]interface{}{
		"device-a": {"value": 1},
	}})
	queue.Put(telemetryBatch{Time: time.Unix(2, 0), Grouped: map[string]map[string]interface{}{
		"device-a": {"value": 2},
		"device-b": {"value": 3},
	}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	batch, ok := queue.Take(ctx)
	if !ok {
		t.Fatal("queue returned no batch")
	}
	if got := batch.Grouped["device-a"]["value"]; got != 2 {
		t.Fatalf("device-a value = %v, want latest value 2", got)
	}
	if got := batch.Grouped["device-b"]["value"]; got != 3 {
		t.Fatalf("device-b value = %v, want 3", got)
	}
	if queue.HasPending() {
		t.Fatal("queue retained data after take")
	}
}

func TestLatestTelemetryQueueMergesMetricsFromSeparateLanes(t *testing.T) {
	queue := newLatestTelemetryQueue()
	queue.Put(telemetryBatch{Time: time.Unix(1, 0), Grouped: map[string]map[string]interface{}{
		"device-a": {"coil-1": true},
	}})
	queue.Put(telemetryBatch{Time: time.Unix(2, 0), Grouped: map[string]map[string]interface{}{
		"device-a": {"register-1": 42},
	}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	batch, ok := queue.Take(ctx)
	if !ok {
		t.Fatal("queue returned no batch")
	}
	if got := batch.Grouped["device-a"]["coil-1"]; got != true {
		t.Fatalf("coil-1 = %v, want true", got)
	}
	if got := batch.Grouped["device-a"]["register-1"]; got != 42 {
		t.Fatalf("register-1 = %v, want 42", got)
	}
}

func TestPublishBackoffIsBoundedAndResets(t *testing.T) {
	var backoff publishBackoff
	now := time.Unix(10, 0)
	if delay := backoff.Fail(now); delay != time.Second {
		t.Fatalf("first delay = %v, want 1s", delay)
	}
	for index := 0; index < 10; index++ {
		backoff.Fail(now)
	}
	if delay := backoff.retryAt.Sub(now); delay != 30*time.Second {
		t.Fatalf("maximum delay = %v, want 30s", delay)
	}
	backoff.nextAt = now.Add(time.Second)
	backoff.Success()
	if backoff.Ready(now) || !backoff.Ready(now.Add(time.Second)) || backoff.failures != 0 {
		t.Fatal("success did not reset backoff")
	}
}

func TestManualPublishWaitsForInitialSnapshotInterval(t *testing.T) {
	app := &App{publishBackoff: map[string]*publishBackoff{}}
	now := time.Unix(20, 0)
	if app.manualPublishReady("manual-1", now, time.Second) {
		t.Fatal("first collection lane should not publish a partial snapshot")
	}
	if app.manualPublishReady("manual-1", now.Add(999*time.Millisecond), time.Second) {
		t.Fatal("manual MQTT became ready before its reporting interval")
	}
	if !app.manualPublishReady("manual-1", now.Add(time.Second), time.Second) {
		t.Fatal("manual MQTT was not ready after its reporting interval")
	}
	if app.manualPublishReady("manual-1", now.Add(time.Second), time.Second) {
		t.Fatal("manual MQTT allowed an overlapping publication")
	}
}

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
	backoff.Success()
	if !backoff.Ready(now) || backoff.failures != 0 {
		t.Fatal("success did not reset backoff")
	}
}

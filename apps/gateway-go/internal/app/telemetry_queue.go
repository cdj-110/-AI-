package app

import (
	"context"
	"sync"
	"time"
)

// latestTelemetryQueue bounds the live telemetry backlog by device count.
// A slow cloud connection never needs every intermediate realtime sample:
// retaining the newest sample per device is sufficient and prevents large
// point batches from accumulating until the kernel OOM killer intervenes.
type latestTelemetryQueue struct {
	mu      sync.Mutex
	pending map[string]pendingTelemetry
	wake    chan struct{}
}

type pendingTelemetry struct {
	at      time.Time
	metrics map[string]interface{}
}

func newLatestTelemetryQueue() *latestTelemetryQueue {
	return &latestTelemetryQueue{
		pending: make(map[string]pendingTelemetry),
		wake:    make(chan struct{}, 1),
	}
}

func (q *latestTelemetryQueue) Put(batch telemetryBatch) {
	if len(batch.Grouped) == 0 {
		return
	}
	q.mu.Lock()
	for deviceKey, metrics := range batch.Grouped {
		q.pending[deviceKey] = pendingTelemetry{at: batch.Time, metrics: metrics}
	}
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *latestTelemetryQueue) Take(ctx context.Context) (telemetryBatch, bool) {
	for {
		select {
		case <-ctx.Done():
			return telemetryBatch{}, false
		case <-q.wake:
		}

		q.mu.Lock()
		if len(q.pending) == 0 {
			q.mu.Unlock()
			continue
		}
		grouped := make(map[string]map[string]interface{}, len(q.pending))
		var latest time.Time
		for deviceKey, item := range q.pending {
			grouped[deviceKey] = item.metrics
			if item.at.After(latest) {
				latest = item.at
			}
		}
		q.pending = make(map[string]pendingTelemetry)
		q.mu.Unlock()
		return telemetryBatch{Time: latest, Grouped: grouped}, true
	}
}

func (q *latestTelemetryQueue) HasPending() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending) > 0
}

type publishBackoff struct {
	failures int
	retryAt  time.Time
}

func (b *publishBackoff) Ready(now time.Time) bool {
	return !now.Before(b.retryAt)
}

func (b *publishBackoff) Success() {
	b.failures = 0
	b.retryAt = time.Time{}
}

func (b *publishBackoff) Fail(now time.Time) time.Duration {
	b.failures++
	delay := time.Second
	for index := 1; index < b.failures && delay < 30*time.Second; index++ {
		delay *= 2
	}
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	b.retryAt = now.Add(delay)
	return delay
}

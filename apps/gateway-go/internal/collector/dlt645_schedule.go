package collector

import (
	"context"
	"sync"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// A zero-value, cancellable bus lock. No goroutine is left waiting after a
// request is cancelled; the response budget starts only after acquisition.
type dlt645BusMutex struct {
	once  sync.Once
	token chan struct{}
}

func (m *dlt645BusMutex) acquire(ctx context.Context) error {
	m.once.Do(func() { m.token = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case m.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			m.Unlock()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (m *dlt645BusMutex) Lock()   { _ = m.acquire(context.Background()) }
func (m *dlt645BusMutex) Unlock() { <-m.token }

// One batch per physical serial port, including different meter addresses.
// Each transaction has its own bounded timeout/retries, independent of the
// sampling frequency. Explicit caller cancellation still stops the batch.
func (DLT645) BatchKey(point config.PointConfig) string { return point.Address }
func (d DLT645) ReadPoints(ctx context.Context, points []config.PointConfig) []PointReadResult {
	results := make([]PointReadResult, 0, len(points))
	for index, point := range points {
		if ctx.Err() != nil {
			results = append(results, PointReadResult{Index: index, Err: ErrCollectionDeferred})
			continue
		}
		value, err := d.ReadPoint(ctx, point)
		if ctx.Err() != nil {
			// Reload/shutdown is not evidence of a meter failure.
			err = ErrCollectionDeferred
		}
		results = append(results, PointReadResult{Index: index, Value: value, Err: err})
	}
	return results
}

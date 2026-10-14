package collector

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goburrow/serial"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type delayedDLTPort struct {
	delay  time.Duration
	ready  time.Time
	reply  []byte
	writes *atomic.Int32
}

func (*delayedDLTPort) Open(*serial.Config) error { return nil }
func (*delayedDLTPort) Close() error              { return nil }
func (p *delayedDLTPort) Write(raw []byte) (int, error) {
	p.writes.Add(1)
	frame, _, _, err := parseDLT645Stream(raw)
	if err != nil {
		return 0, err
	}
	data := append(append([]byte{}, frame.Data...), 0x23, 0x01, 0, 0)
	p.reply, _ = buildDLT645Frame(frame.Address, 0x91, data, 0)
	p.ready = time.Now().Add(p.delay)
	return len(raw), nil
}
func (p *delayedDLTPort) Read(out []byte) (int, error) {
	if time.Now().Before(p.ready) {
		time.Sleep(time.Millisecond)
		return 0, serial.ErrTimeout
	}
	n := copy(out, p.reply)
	p.reply = p.reply[n:]
	return n, nil
}
func scheduledDLTPoint() config.PointConfig {
	return config.PointConfig{DeviceKey: "meter", Metric: "p", Protocol: "dlt645", Address: "schedule-test", BaudRate: 9600, DataBits: 8, StopBits: 1, Parity: "none", Scale: 1,
		DLT645: &config.DLT645Config{Version: "2007", MeterAddress: "000000000001", DataIdentifier: "00000000", ValueFormat: "bcd", DataLength: 4, Access: "read", ResponseTimeoutMilliseconds: 100, Retries: 1}}
}
func installDelayedDLT(t *testing.T, delay time.Duration) *atomic.Int32 {
	t.Helper()
	CloseDLT645Connections()
	old := dlt645OpenPort
	writes := &atomic.Int32{}
	dlt645OpenPort = func(cfg *serial.Config) (serial.Port, error) {
		if cfg.Timeout > 100*time.Millisecond {
			t.Errorf("driver read slice too long: %s", cfg.Timeout)
		}
		return &delayedDLTPort{delay: delay, writes: writes}, nil
	}
	t.Cleanup(func() { CloseDLT645Connections(); dlt645OpenPort = old })
	return writes
}
func TestDLT645BatchHasPerPointResponseBudgets(t *testing.T) {
	writes := installDelayedDLT(t, 40*time.Millisecond)
	p := scheduledDLTPoint()
	points := []config.PointConfig{p, p, p, p, p}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	results := (DLT645{}).ReadPoints(ctx, points)
	if len(results) != 5 || writes.Load() != 5 {
		t.Fatalf("results=%d writes=%d", len(results), writes.Load())
	}
	for i, r := range results {
		if r.Index != i || r.Err != nil {
			t.Fatalf("result %d: %+v", i, r)
		}
	}
	if time.Since(start) < 200*time.Millisecond {
		t.Fatal("reads were not serialized")
	}
}
func TestDLT645WaitingDoesNotConsumeResponseBudget(t *testing.T) {
	installDelayedDLT(t, 40*time.Millisecond)
	p := scheduledDLTPoint()
	bus, err := getDLT645Bus(p)
	if err != nil {
		t.Fatal(err)
	}
	bus.mu.Lock()
	done := make(chan error, 1)
	go func() { _, err := (DLT645{}).ReadPoint(context.Background(), p); done <- err }()
	time.Sleep(150 * time.Millisecond) // Longer than the 100ms response budget.
	bus.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("read did not finish")
	}
}
func TestDLT645CancelledQueueDoesNotSendOrCloseOwner(t *testing.T) {
	writes := installDelayedDLT(t, time.Millisecond)
	p := scheduledDLTPoint()
	bus, _ := getDLT645Bus(p)
	bus.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := (DLT645{}).ReadPoint(ctx, p)
	portIntact := bus.port != nil
	bus.mu.Unlock()
	if !errors.Is(err, ErrCollectionDeferred) || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "尚未发送") {
		t.Fatalf("wrong wait diagnostic: %v", err)
	}
	if writes.Load() != 0 || !portIntact {
		t.Fatal("waiter touched owner's port")
	}
	if _, err := (DLT645{}).ReadPoint(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}
func TestDLT645ResponseTimeoutRetriesAreBounded(t *testing.T) {
	writes := installDelayedDLT(t, 200*time.Millisecond)
	p := scheduledDLTPoint()
	p.DLT645.ResponseTimeoutMilliseconds = 20
	_, err := (DLT645{}).ReadPoint(context.Background(), p)
	if err == nil || !strings.Contains(err.Error(), "响应超时") || writes.Load() != 2 {
		t.Fatalf("writes=%d error=%v", writes.Load(), err)
	}
	// A failed point must not stop subsequent points on the same bus.
	p.DLT645.ResponseTimeoutMilliseconds = 300
	if _, err := (DLT645{}).ReadPoint(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}
func TestDLT645DifferentResponseTimeoutsShareBus(t *testing.T) {
	installDelayedDLT(t, time.Millisecond)
	p := scheduledDLTPoint()
	first, err := getDLT645Bus(p)
	if err != nil {
		t.Fatal(err)
	}
	p.DLT645.ResponseTimeoutMilliseconds = 1500
	second, err := getDLT645Bus(p)
	if err != nil || first != second {
		t.Fatalf("timeout must not change physical settings: %v", err)
	}
}

func TestDLT645CancelledActiveRoundDoesNotProduceMeterFailures(t *testing.T) {
	writes := installDelayedDLT(t, 200*time.Millisecond)
	p := scheduledDLTPoint()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	results := (DLT645{}).ReadPoints(ctx, []config.PointConfig{p, p, p})
	if len(results) != 3 || writes.Load() != 1 {
		t.Fatalf("unexpected cancelled round: writes=%d results=%+v", writes.Load(), results)
	}
	for _, result := range results {
		if !errors.Is(result.Err, ErrCollectionDeferred) {
			t.Fatalf("shutdown generated meter failure: %v", result.Err)
		}
	}
}

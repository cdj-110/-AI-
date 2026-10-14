package remotestore

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func testRecord(t *testing.T, value any) Record {
	t.Helper()
	r, err := NewRecord("gateway", state.PointStatus{DeviceKey: "d", Metric: "p", Value: value, Quality: "GOOD"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestQueuePersistentReplayAndDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	available := true
	open := func(identity string) *Queue {
		t.Helper()
		q, e := OpenQueue(path, identity, 4<<20, func() bool { return available })
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	q := open("original")
	r := testRecord(t, uint64(math.MaxUint64))
	if err := q.Append([]Record{r}); err != nil {
		t.Fatal(err)
	}
	q.Close()
	if wrong, e := OpenQueue(path, "other", 4<<20, func() bool { return true }); e == nil {
		wrong.Close()
		t.Fatal("retargeted pending records")
	}
	q = open("original")
	defer q.Close()
	batch, err := q.Peek(100)
	if err != nil || len(batch) != 1 || batch[0].Record.ID != r.ID || *batch[0].Record.Integer != "18446744073709551615" {
		t.Fatalf("replay corrupted: %+v %v", batch, err)
	}
	second, _ := q.Peek(100)
	if second[0].Record.ID != r.ID {
		t.Fatal("peek changed id")
	}
	available = false
	if err = q.Ack(batch); err == nil {
		t.Fatal("ack on disconnected volume")
	}
	if err = q.Append([]Record{r}); err == nil {
		t.Fatal("append on disconnected volume")
	}
	available = true
	if err = q.Ack(batch); err != nil {
		t.Fatal(err)
	}
	count, _, _ := q.Count()
	if count != 0 {
		t.Fatal(count)
	}
}
func TestQueueCapacityAtomicAndShrink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	q, err := OpenQueue(path, "same", 4<<20, func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	r := testRecord(t, "small")
	q.capacity = 1
	if err = q.Append([]Record{r, r}); err == nil {
		t.Fatal("ignored capacity")
	}
	n, _, _ := q.Count()
	if n != 0 {
		t.Fatal("partial batch")
	}
	q.Close()
	if q, e := OpenQueue(path, "same", 1024, func() bool { return true }); e == nil {
		q.Close()
		t.Fatal("ignored physical page bound")
	}
}
func TestRecordTypesAndTimestamps(t *testing.T) {
	for _, tc := range []struct {
		value         any
		kind, integer string
	}{{int64(math.MinInt64), "integer", "-9223372036854775808"}, {uint64(math.MaxUint64), "integer", "18446744073709551615"}, {json.Number("18446744073709551615"), "integer", "18446744073709551615"}, {-1.25, "float", ""}, {true, "boolean", ""}, {"设备文本", "string", ""}, {nil, "null", ""}, {math.NaN(), "special", ""}} {
		r := testRecord(t, tc.value)
		if r.Type != tc.kind {
			t.Fatalf("%v: %s", tc.value, r.Type)
		}
		if tc.integer != "" && (r.Integer == nil || *r.Integer != tc.integer) {
			t.Fatalf("integer lost: %+v", r)
		}
		if !json.Valid([]byte(r.ValueJSON)) {
			t.Fatal(r.ValueJSON)
		}
		if r.Source != nil {
			t.Fatal("invented source time")
		}
	}
	source, receive := time.Unix(123, 0), time.Unix(124, 0)
	r, e := NewRecord("g", state.PointStatus{Value: 7, SourceTimestamp: &source, ReceiveTimestamp: &receive, Error: "读取失败"}, receive)
	if e != nil || *r.Source != 123000 || *r.Received != 124000 || r.Quality != "BAD" {
		t.Fatalf("timestamps/quality: %+v %v", r, e)
	}
	if _, e = NewRecord("g", state.PointStatus{Value: strings.Repeat("x", 70<<10)}, receive); e == nil {
		t.Fatal("oversized value accepted")
	}
}
func TestManagerRejectsUnknownActiveOrPendingRetarget(t *testing.T) {
	target := config.RemoteStorageTarget{ID: "t", Driver: "mysql", Host: "a", Table: "x"}
	cfg := config.RemoteStorageConfig{StoragePath: "local", MaxSizeMB: 16, Targets: []config.RemoteStorageTarget{target}}
	w := &worker{target: target, status: Status{}}
	m := &Manager{cfg: cfg, workers: map[string]*worker{"t": w}}
	next := cfg
	next.Targets = nil
	if m.ValidateChange(next) == nil {
		t.Fatal("unknown cache deletion")
	}
	w.status.CacheReady = true
	w.status.Pending = 1
	if m.ValidateChange(next) == nil {
		t.Fatal("pending deletion")
	}
	w.status.Pending = 0
	w.target.Enabled = true
	if m.ValidateChange(next) == nil {
		t.Fatal("active deletion race")
	}
	w.target.Enabled = false
	if e := m.ValidateChange(next); e != nil {
		t.Fatal(e)
	}
}

type failingAdapter struct {
	calls     atomic.Int32
	recovered atomic.Bool
}

func (a *failingAdapter) Prepare(context.Context) error { return nil }
func (a *failingAdapter) Close() error                  { return nil }
func (a *failingAdapter) Write(ctx context.Context, records []Record) error {
	a.calls.Add(1)
	if !a.recovered.Load() {
		return errors.New("simulated disconnection")
	}
	return nil
}
func TestSenderKeepsFailedBatchAndRecovers(t *testing.T) {
	q, e := OpenQueue(filepath.Join(t.TempDir(), "queue.db"), "same", 4<<20, func() bool { return true })
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	q.Append([]Record{testRecord(t, 123)})
	mock := &failingAdapter{}
	original := factories["mock"]
	factories["mock"] = func(config.RemoteStorageTarget) (Adapter, error) { return mock, nil }
	defer func() {
		if original == nil {
			delete(factories, "mock")
		} else {
			factories["mock"] = original
		}
	}()
	w := &worker{queue: q, target: config.RemoteStorageTarget{Driver: "mock", BatchSize: 10, TimeoutSeconds: 2}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); w.send(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for mock.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	n, _, _ := q.Count()
	if n != 1 {
		t.Fatal("failed remote write was acknowledged")
	}
	mock.recovered.Store(true)
	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		n, _, _ = q.Count()
		if n == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("did not backfill")
}

func TestSamplingIndependentOfRemoteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	q, e := OpenQueue(path, "same", 4<<20, func() bool { return true })
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	cfg := config.Config{Points: []config.PointConfig{{DeviceKey: "d", Metric: "p", Protocol: "modbus-tcp", Address: "127.0.0.1:502", DataType: "float64"}}}
	store := state.New(cfg)
	store.SetPointValue("d", "p", 1.0)
	mock := &failingAdapter{}
	factories["sample-mock"] = func(config.RemoteStorageTarget) (Adapter, error) { return mock, nil }
	defer delete(factories, "sample-mock")
	w := &worker{queue: q, store: store, gateway: "test", target: config.RemoteStorageTarget{Driver: "sample-mock", BatchSize: 10, TimeoutSeconds: 2, IntervalSeconds: 1, Points: []config.HistoryPointRef{{DeviceKey: "d", Metric: "p"}}}}
	ctx, cancel := context.WithCancel(context.Background())
	sampleDone, sendDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(sampleDone); w.sample(ctx, path, func() bool { return true }) }()
	go func() { defer close(sendDone); w.send(ctx) }()
	defer func() { cancel(); <-sampleDone; <-sendDone }()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		store.SetPointValue("d", "p", time.Now().UnixMilli())
		n, _, _ := q.Count()
		if n >= 2 && mock.calls.Load() > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("remote failure prevented independent durable sampling")
}

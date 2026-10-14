package history

import "testing"

func TestWriterPreservesValuesAndTaskMembership(t *testing.T) {
	store := NewGuarded(t.TempDir(), 4*1024*1024, nil)
	w := &AsyncWriter{store: store, maxPending: 10}
	w.Enqueue([]Sample{{DeviceKey: "d", Metric: "p", Value: 1, TaskKeys: []string{"a"}}, {DeviceKey: "d", Metric: "p", Value: 0, TaskKeys: []string{"b"}}})
	if w.Status().Pending != 2 {
		t.Fatal("samples coalesced")
	}
	if !w.flush() {
		t.Fatal("flush failed")
	}
	stats, err := store.TaskStats()
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 || stats[0].Samples != 1 || stats[1].Samples != 1 {
		t.Fatalf("task samples lost: %+v", stats)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWriterCapacityIncludesFailedTransaction(t *testing.T) {
	w := &AsyncWriter{maxPending: 3}
	w.Enqueue([]Sample{{Value: 1}, {Value: 2}})
	// Simulate an in-flight transaction while the producer enqueues.
	batch := w.pending
	w.pending = nil
	w.inFlight = len(batch)
	w.Enqueue([]Sample{{Value: 3}, {Value: 4}})
	w.requeue(batch)
	if w.Status().Pending != 3 || w.Status().Dropped != 1 {
		t.Fatalf("capacity incorrect: %+v", w.Status())
	}
	for i, s := range w.pending {
		if s.Value != i+1 {
			t.Fatalf("retry reordered data: %+v", w.pending)
		}
	}
}

package collectdiag

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestStoreTracksReasonChangesAndRecovery(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "config.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	first := time.Date(2026, 9, 8, 12, 0, 0, 123000000, time.Local)
	base := state.PointIncidentEvent{Active: true, DeviceKey: "meter-1", PointName: "温度", Metric: "temperature", Protocol: "modbus-tcp", Address: "192.168.1.10:502/40001", At: first}
	base.Reason = "dial tcp 192.168.1.10:502: connect: connection refused"
	store.RecordPointIncident(base)
	flushIncidentStore(t, store)

	page, err := store.Query(Query{Page: 1, PageSize: 10, State: "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].Reason != base.Reason || page.Items[0].Explanation == "" {
		t.Fatalf("unexpected active incident: %#v", page)
	}
	if page.Items[0].OccurredAt.UnixNano() != first.UnixNano() {
		t.Fatalf("occurredAt = %s, want %s", page.Items[0].OccurredAt, first)
	}

	base.Reason = "read tcp: i/o timeout"
	base.At = first.Add(time.Minute)
	store.RecordPointIncident(base)
	flushIncidentStore(t, store)
	changed, err := store.Query(Query{Page: 1, PageSize: 10, State: "CHANGED"})
	if err != nil || changed.Total != 1 {
		t.Fatalf("changed incidents = %#v, err=%v", changed, err)
	}
	active, err := store.Query(Query{Page: 1, PageSize: 10, State: "ACTIVE", Protocol: "MODBUS-TCP", Keyword: "temperature"})
	if err != nil || active.Total != 1 || active.Items[0].Reason != base.Reason {
		t.Fatalf("new active incident = %#v, err=%v", active, err)
	}

	store.RecordPointIncident(state.PointIncidentEvent{Active: false, DeviceKey: base.DeviceKey, Metric: base.Metric, At: first.Add(2 * time.Minute)})
	flushIncidentStore(t, store)
	recovered, err := store.Query(Query{Page: 1, PageSize: 10, State: "RECOVERED"})
	if err != nil || recovered.Total != 1 || recovered.Items[0].RecoveredAt == nil {
		t.Fatalf("recovered incidents = %#v, err=%v", recovered, err)
	}
}

func TestOpenMarksUnfinishedIncidentInterrupted(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.local.json")
	store, err := Open(configPath)
	if err != nil {
		t.Fatal(err)
	}
	store.RecordPointIncident(state.PointIncidentEvent{Active: true, DeviceKey: "d1", Metric: "p1", Reason: "timeout", At: time.Now()})
	flushIncidentStore(t, store)
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(configPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	page, err := store.Query(Query{Page: 1, PageSize: 10, State: "INTERRUPTED"})
	if err != nil || page.Total != 1 {
		t.Fatalf("interrupted incidents = %#v, err=%v", page, err)
	}
}

func TestStoreIgnoresAndCleansLifecycleCancellation(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.local.json")
	store, err := Open(configPath)
	if err != nil {
		t.Fatal(err)
	}
	store.RecordPointIncident(state.PointIncidentEvent{Active: true, DeviceKey: "d1", Metric: "p1", Reason: "read failed: context canceled", At: time.Now()})
	flushIncidentStore(t, store)
	page, err := store.Query(Query{Page: 1, PageSize: 10})
	if err != nil || page.Total != 0 {
		t.Fatalf("lifecycle cancellation should be ignored: page=%#v err=%v", page, err)
	}
	now := sqliteTime(time.Now())
	if _, err = store.db.Exec(`INSERT INTO collection_incidents(device_key,metric,reason,state,occurred_at,last_seen_at) VALUES('d1','p1','OPC DA read: context canceled','ACTIVE',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(configPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	page, err = store.Query(Query{Page: 1, PageSize: 10})
	if err != nil || page.Total != 0 {
		t.Fatalf("stored lifecycle cancellation should be cleaned: page=%#v err=%v", page, err)
	}
}

func TestStoreClearRemovesExistingIncidents(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "config.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.RecordPointIncident(state.PointIncidentEvent{Active: true, DeviceKey: "d1", Metric: "p1", Reason: "connection refused", At: time.Now()})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err = store.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	page, err := store.Query(Query{Page: 1, PageSize: 10})
	if err != nil || page.Total != 0 {
		t.Fatalf("clear did not remove incidents: page=%#v err=%v", page, err)
	}
}

func TestExplainCommonFailureReasons(t *testing.T) {
	for _, reason := range []string{
		"HRESULT 0x800706BA: RPC server is unavailable",
		"OPC_E_UNKNOWNITEMID 0xC0040007",
		"context deadline exceeded",
		"connection refused",
		"network is unreachable",
		"access denied",
		"unknown item",
		"modbus exception 2",
		"x509: certificate has expired",
		"open /dev/ttyS2: no such file or directory",
		"connection reset by peer",
		"vendor specific failure",
	} {
		if explanation := Explain(reason); explanation == "" {
			t.Fatalf("empty explanation for %q", reason)
		}
	}
}

func flushIncidentStore(t *testing.T, store *Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := store.Flush(ctx); err != nil {
		t.Fatal(err)
	}
}

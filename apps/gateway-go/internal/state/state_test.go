package state

import (
	"fmt"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

func TestStoreChangeSubscriptionCoalescesWithoutBlockingCollector(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{{DeviceKey: "d1", Metric: "p1"}}}
	store := New(cfg)
	updates, unsubscribe := store.Subscribe()
	defer unsubscribe()

	store.SetPointValue("d1", "p1", 1)
	store.SetPointValue("d1", "p1", 2)
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("did not receive store change signal")
	}
	pointRevision, statusRevision := store.Revisions()
	if pointRevision != 2 || statusRevision != 0 {
		t.Fatalf("revisions = %d/%d, want 2/0", pointRevision, statusRevision)
	}

	store.MarkCollect()
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("did not receive status change signal")
	}
	_, statusRevision = store.Revisions()
	if statusRevision != 1 {
		t.Fatalf("status revision = %d, want 1", statusRevision)
	}
}

func TestPollingPointBecomesStale(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{{DeviceKey: "d1", Metric: "p1", Protocol: "modbus-tcp", CollectIntervalSeconds: 1}}}
	store := New(cfg)
	store.SetPointValue("d1", "p1", 1)
	store.mu.Lock()
	point := store.points[pointKey("d1", "p1")]
	old := time.Now().Add(-time.Minute)
	point.UpdatedAt = &old
	store.points[pointKey("d1", "p1")] = point
	store.mu.Unlock()
	points := store.PointStatuses()
	if len(points) != 1 || !points[0].Stale {
		t.Fatalf("point should be stale: %#v", points)
	}
	snapshot := store.Snapshot()
	if snapshot.StaleCount != 1 || snapshot.HealthyCount != 0 {
		t.Fatalf("unexpected stale counts: %#v", snapshot)
	}
}

func TestSubscriptionPointDoesNotBecomeStaleWithoutValueChange(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{{DeviceKey: "d1", Metric: "p1", Protocol: "opcda", CollectIntervalSeconds: 1}}}
	store := New(cfg)
	store.SetPointResult(model.PointValue{
		DeviceKey:      "d1",
		Metric:         "p1",
		Value:          1,
		CollectionMode: "subscription",
	})
	store.mu.Lock()
	point := store.points[pointKey("d1", "p1")]
	old := time.Now().Add(-time.Hour)
	point.UpdatedAt = &old
	point.ReceiveTimestamp = &old
	store.points[pointKey("d1", "p1")] = point
	store.mu.Unlock()

	points := store.PointStatuses()
	if len(points) != 1 || points[0].Stale {
		t.Fatalf("unchanged subscription point must remain healthy: %#v", points)
	}
	filtered := store.PointStatusesByMetric("d1", []string{"p1"})
	if len(filtered) != 1 || filtered[0].Stale {
		t.Fatalf("filtered subscription point must remain healthy: %#v", filtered)
	}
	for _, snapshot := range []Snapshot{store.Snapshot(), store.CompactSnapshot()} {
		if snapshot.StaleCount != 0 || snapshot.HealthyCount != 1 {
			t.Fatalf("unexpected subscription health counts: %#v", snapshot)
		}
	}
}

func TestMQTTChannelsRemainIndependent(t *testing.T) {
	enabled := true
	cfg := config.Config{
		GatewayKey: "gateway",
		Activation: config.ActivationConfig{Enabled: &enabled},
		MQTT:       config.MQTTConfig{Enabled: &enabled},
	}
	store := New(cfg)
	store.SetMQTTChannelConnected("manual-1", true)

	snapshot := store.Snapshot()
	if snapshot.MQTTChannels["activation"].Connected {
		t.Fatal("activation channel must remain disconnected")
	}
	if !snapshot.MQTTChannels["manual-1"].Connected {
		t.Fatal("manual channel should be connected")
	}
	if !snapshot.MQTTConnected {
		t.Fatal("overall MQTT state should be connected when one enabled channel is connected")
	}
}

func TestResetMQTTChannelsPreservesUnchangedConnections(t *testing.T) {
	enabled := true
	disabled := false
	cfg := config.Config{
		GatewayKey: "gateway",
		MQTTChannels: []config.MQTTConfig{
			{Name: "first", Enabled: &enabled, Broker: "tcp://one:1883", ClientID: "one", Username: "one"},
			{Name: "second", Enabled: &enabled, Broker: "tcp://two:1883", ClientID: "two", Username: "two"},
		},
	}
	store := New(cfg)
	store.SetMQTTChannelConnected("manual-1", true)
	store.SetMQTTChannelConnected("manual-2", true)

	cfg.MQTTChannels[0].Enabled = &disabled
	store.ResetMQTTChannels(cfg)

	snapshot := store.Snapshot()
	if snapshot.MQTTChannels["manual-1"].Connected {
		t.Fatal("disabled channel should not remain connected")
	}
	if !snapshot.MQTTChannels["manual-2"].Connected {
		t.Fatal("unchanged enabled channel should keep connected state")
	}
	if !snapshot.MQTTConnected {
		t.Fatal("overall MQTT state should remain connected while manual-2 is connected")
	}
}

func TestPointPresentationMetadataAppearsInSnapshot(t *testing.T) {
	cfg := config.Config{
		GatewayKey: "gateway",
		Points: []config.PointConfig{{
			DeviceKey: "meter-1",
			Name:      "噪声值",
			Metric:    "noise_db",
			Unit:      "dB",
			Decimals:  1,
		}},
	}
	store := New(cfg)
	store.SetPointValue("meter-1", "noise_db", 41.0)
	point := store.Snapshot().Points[0]
	if point.Unit != "dB" || point.Decimals != 1 {
		t.Fatalf("presentation metadata = %q/%d, want dB/1", point.Unit, point.Decimals)
	}
}

func TestPointProtocolQualityAndTimestampsArePreserved(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{{DeviceKey: "opc-1", Metric: "temperature", Protocol: "opcda"}}}
	store := New(cfg)
	source := time.Unix(1700000000, 123000000)
	received := source.Add(250 * time.Millisecond)
	store.SetPointResult(model.PointValue{
		DeviceKey: "opc-1", Metric: "temperature", Value: 25.8,
		ActualDataType: "float64",
		Quality:        model.QualityUncertain, RawQuality: 0x44,
		SourceTimestamp: source, ReceiveTimestamp: received,
		DiagnosticCode: 1, Diagnostic: "OPC_S_CLAMP",
	})
	point := store.Snapshot().Points[0]
	if point.Quality != model.QualityUncertain || point.RawQuality != 0x44 {
		t.Fatalf("quality = %s/0x%X, want UNCERTAIN/0x44", point.Quality, point.RawQuality)
	}
	if point.ActualDataType != "float64" {
		t.Fatalf("actual data type = %q, want float64", point.ActualDataType)
	}
	if point.SourceTimestamp == nil || !point.SourceTimestamp.Equal(source) || point.ReceiveTimestamp == nil || !point.ReceiveTimestamp.Equal(received) {
		t.Fatalf("timestamps were not preserved: %#v", point)
	}
	if point.UpdatedAt == nil || !point.UpdatedAt.Equal(received) || point.DiagnosticCode != 1 {
		t.Fatalf("receive metadata was not applied: %#v", point)
	}
}

func TestPointStatusPreservesExact64BitIntegerText(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{
		{DeviceKey: "opc-1", Metric: "signed", Protocol: "opcda"},
		{DeviceKey: "opc-1", Metric: "unsigned", Protocol: "opcda"},
	}}
	store := New(cfg)
	store.SetPointValue("opc-1", "signed", int64(-9223372036854775807-1))
	store.SetPointValue("opc-1", "unsigned", uint64(18446744073709551615))
	points := store.Snapshot().Points
	if len(points) != 2 {
		t.Fatalf("point count = %d, want 2", len(points))
	}
	byMetric := map[string]PointStatus{points[0].Metric: points[0], points[1].Metric: points[1]}
	if got := byMetric["signed"].ValueText; got != "-9223372036854775808" {
		t.Fatalf("signed exact text = %q", got)
	}
	if got := byMetric["unsigned"].ValueText; got != "18446744073709551615" {
		t.Fatalf("unsigned exact text = %q", got)
	}
}

type incidentRecorderStub struct {
	events []PointIncidentEvent
}

func (r *incidentRecorderStub) RecordPointIncident(event PointIncidentEvent) {
	r.events = append(r.events, event)
}

func TestPointIncidentRecorderReceivesFailureChangeAndRecovery(t *testing.T) {
	point := config.PointConfig{DeviceKey: "d1", Name: "温度", Metric: "temperature", Protocol: "modbus-tcp", Address: "40001"}
	store := New(config.Config{Points: []config.PointConfig{point}})
	recorder := &incidentRecorderStub{}
	store.SetPointIncidentRecorder(recorder)

	store.SetPointError(point, fmt.Errorf("timeout"))
	store.SetPointError(point, fmt.Errorf("timeout"))
	store.SetPointError(point, fmt.Errorf("connection refused"))
	store.SetPointResult(model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: 12})
	store.SetPointResult(model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: 13})

	if len(recorder.events) != 3 {
		t.Fatalf("incident event count = %d, want 3: %#v", len(recorder.events), recorder.events)
	}
	if !recorder.events[0].Active || recorder.events[0].Reason != "timeout" || recorder.events[0].Protocol != "modbus-tcp" {
		t.Fatalf("first event = %#v", recorder.events[0])
	}
	if !recorder.events[1].Active || recorder.events[1].Reason != "connection refused" {
		t.Fatalf("changed event = %#v", recorder.events[1])
	}
	if recorder.events[2].Active || recorder.events[2].Reason != "" {
		t.Fatalf("recovery event = %#v", recorder.events[2])
	}
}

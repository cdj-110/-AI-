package state

import (
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
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

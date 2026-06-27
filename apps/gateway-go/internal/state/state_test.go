package state

import (
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

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

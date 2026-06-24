package app

import (
	"encoding/json"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestMergeRemoteConfigAppliesPatchAndProtectsConnectivity(t *testing.T) {
	current := config.Config{
		GatewayKey:             "gateway-1",
		CollectIntervalSeconds: 1,
		MQTT:                   config.MQTTConfig{Broker: "tcp://broker:1883", ClientID: "client", Username: "user"},
		Web:                    config.ListenerConfig{Enabled: true, Listen: "0.0.0.0:8088"},
		NetworkPorts:           []config.NetworkPort{{Name: "eth0", Enabled: true}},
		Devices:                []config.DeviceConfig{{DeviceKey: "device-1", Name: "PLC", Protocol: "modbus-tcp"}},
	}
	patch := json.RawMessage(`{"collectIntervalSeconds":2,"gatewayKey":"changed","mqtt":{"broker":"tcp://bad:1883"},"devices":[{"deviceKey":"device-2","name":"Meter","protocol":"modbus-tcp","points":[]}]}`)

	merged, err := mergeRemoteConfig(current, patch)
	if err != nil {
		t.Fatal(err)
	}
	if merged.CollectIntervalSeconds != 2 {
		t.Fatalf("collect interval = %d, want 2", merged.CollectIntervalSeconds)
	}
	if merged.GatewayKey != current.GatewayKey || merged.MQTT.Broker != current.MQTT.Broker {
		t.Fatal("connectivity fields were changed by remote patch")
	}
	if len(merged.Devices) != 1 || merged.Devices[0].DeviceKey != "device-2" {
		t.Fatal("device list patch was not applied")
	}
}

func TestMergeRemoteConfigRejectsNonObject(t *testing.T) {
	_, err := mergeRemoteConfig(config.Config{GatewayKey: "gateway-1"}, json.RawMessage(`[]`))
	if err == nil {
		t.Fatal("expected non-object patch to be rejected")
	}
}

func TestMergeRemoteConfigPreservesMaskedPasswords(t *testing.T) {
	current := config.Config{
		GatewayKey: "gateway-1",
		Devices: []config.DeviceConfig{{
			DeviceKey: "opc-1", Protocol: "opcua", Address: "opc.tcp://localhost:4840", Password: "device-secret",
			Points: []config.PointConfig{{DeviceKey: "opc-1", Metric: "temperature", Protocol: "opcua", Password: "point-secret"}},
		}},
	}
	patch := json.RawMessage(`{"devices":[{"deviceKey":"opc-1","protocol":"opcua","address":"opc.tcp://localhost:4840","password":"***","points":[{"deviceKey":"opc-1","metric":"temperature","protocol":"opcua","password":"***"}]}]}`)

	merged, err := mergeRemoteConfig(current, patch)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Devices[0].Password != "device-secret" || merged.Devices[0].Points[0].Password != "device-secret" {
		t.Fatalf("masked passwords were not preserved: device=%q point=%q", merged.Devices[0].Password, merged.Devices[0].Points[0].Password)
	}
}

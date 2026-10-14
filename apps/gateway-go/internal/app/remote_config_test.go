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
			Points: []config.PointConfig{{DeviceKey: "opc-1", Metric: "temperature", Protocol: "opcua", NodeID: "ns=2;s=temperature", Password: "point-secret"}},
		}},
	}
	patch := json.RawMessage(`{"devices":[{"deviceKey":"opc-1","protocol":"opcua","address":"opc.tcp://localhost:4840","password":"***","points":[{"deviceKey":"opc-1","metric":"temperature","protocol":"opcua","nodeId":"ns=2;s=temperature","password":"***"}]}]}`)

	merged, err := mergeRemoteConfig(current, patch)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Devices[0].Password != "device-secret" || merged.Devices[0].Points[0].Password != "device-secret" {
		t.Fatalf("masked passwords were not preserved: device=%q point=%q", merged.Devices[0].Password, merged.Devices[0].Points[0].Password)
	}
}

func TestEffectiveDeviceCollectionIntervalUsesFlattenedRuntimePoint(t *testing.T) {
	cfg := config.Config{
		GatewayKey:             "gateway-1",
		CollectIntervalSeconds: 10,
		Channels: []config.ChannelConfig{{
			ChannelKey: "channel-1", Name: "采集通道", Type: "network", Role: "collect", Protocol: "modbus-tcp",
			CollectIntervalMilliseconds: 1500, Enabled: true,
		}},
		Devices: []config.DeviceConfig{{
			DeviceKey: "device-1", ChannelKey: "channel-1", Name: "设备1", Protocol: "modbus-tcp", Address: "192.168.1.10:502",
			Points: []config.PointConfig{{DeviceKey: "device-1", Metric: "value", Name: "值", Protocol: "modbus-tcp", Address: "192.168.1.10:502", Function: 3, Quantity: 1, DataType: "uint16"}},
		}},
	}
	cfg.ApplyDefaults()

	got := effectiveDeviceCollectionIntervalMilliseconds(cfg, cfg.Devices[0])
	if got != 1500 {
		t.Fatalf("effective interval = %dms, want channel-derived 1500ms", got)
	}
}

func TestEffectiveDeviceCollectionIntervalPrefersPointOverride(t *testing.T) {
	cfg := config.Config{
		CollectIntervalSeconds: 10,
		Points: []config.PointConfig{
			{DeviceKey: "device-1", ChannelKey: "channel-1", CollectIntervalMilliseconds: 2500},
			{DeviceKey: "device-1", ChannelKey: "channel-1", CollectIntervalMilliseconds: 1200},
		},
	}
	device := config.DeviceConfig{DeviceKey: "device-1", ChannelKey: "channel-1", CollectIntervalMilliseconds: 3000}

	got := effectiveDeviceCollectionIntervalMilliseconds(cfg, device)
	if got != 1200 {
		t.Fatalf("effective interval = %dms, want fastest point interval 1200ms", got)
	}
}

func TestMQTTGroupedValuesPreservesBooleanAndNumericTypes(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{
		{DeviceKey: "device-1", Metric: "coil", Protocol: "modbus-tcp", Function: 1, DataType: "bool"},
		{DeviceKey: "device-1", Metric: "holding_bool", Protocol: "modbus-tcp", Function: 3, DataType: "bool"},
		{DeviceKey: "device-2", Metric: "opc_bool", Protocol: "opcua", DataType: "bool"},
		{DeviceKey: "device-2", Metric: "rtu_coil", Protocol: "modbus-rtu", Function: 1, DataType: "bool"},
		{DeviceKey: "device-2", Metric: "discrete", Protocol: "modbus-rtu", Function: 2, DataType: "bool"},
	}}
	grouped := map[string]map[string]interface{}{
		"device-1": {"coil": true, "holding_bool": true, "zero": 0, "one": 1},
		"device-2": {"opc_bool": false, "rtu_coil": false, "discrete": true},
	}

	converted := mqttGroupedValues(cfg, grouped)
	if converted["device-1"]["coil"] != true {
		t.Fatalf("coil = %#v, want true", converted["device-1"]["coil"])
	}
	if converted["device-1"]["holding_bool"] != true {
		t.Fatalf("holding bool should remain bool: %#v", converted["device-1"]["holding_bool"])
	}
	if converted["device-2"]["opc_bool"] != false {
		t.Fatalf("opc bool should remain bool: %#v", converted["device-2"]["opc_bool"])
	}
	if converted["device-2"]["rtu_coil"] != false || converted["device-2"]["discrete"] != true {
		t.Fatal("RTU coil/discrete boolean types changed")
	}
	if converted["device-1"]["zero"] != 0 || converted["device-1"]["one"] != 1 {
		t.Fatal("numeric values converted to booleans")
	}
	if grouped["device-1"]["coil"] != true || grouped["device-2"]["rtu_coil"] != false {
		t.Fatal("source values changed")
	}
}

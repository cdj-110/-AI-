package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDeviceConnectionOverridesStalePointConnection(t *testing.T) {
	raw := []byte(`{
  "gatewayKey": "gw-test",
  "activation": {"enabled": false},
  "mqtt": {"enabled": false},
  "devices": [{
    "deviceKey": "device-1",
    "protocol": "modbus-tcp",
    "address": "192.168.1.207:502",
    "slaveId": 2,
    "points": [{
      "metric": "temperature",
      "protocol": "modbus-tcp",
      "address": "192.168.1.202:502",
      "slaveId": 1,
      "function": 3,
      "register": 0,
      "dataType": "uint16"
    }]
  }]
}`)

	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(cfg.Points) != 1 {
		t.Fatalf("len(Points) = %d, want 1", len(cfg.Points))
	}
	point := cfg.Points[0]
	if point.Address != "192.168.1.207:502" {
		t.Errorf("point address = %q, want device address", point.Address)
	}
	if point.SlaveID != 2 {
		t.Errorf("point slaveId = %d, want 2", point.SlaveID)
	}
	if cfg.Devices[0].Points[0].Address != "192.168.1.207:502" {
		t.Errorf("stored point address = %q, want normalized device address", cfg.Devices[0].Points[0].Address)
	}
}

func TestS7DeviceDefaultsToSmartModel(t *testing.T) {
	device := DeviceConfig{Protocol: "siemens-s7"}
	device.ApplyDefaults()
	if device.PLCModel != "s7-200-smart" {
		t.Fatalf("PLCModel = %q, want s7-200-smart", device.PLCModel)
	}
}

func TestActivationSNBecomesEffectiveGatewayKey(t *testing.T) {
	cfg, err := Parse([]byte(`{
  "gatewayKey": "OLD-SN",
  "activation": {
    "enabled": true,
    "hardwareId": "0123456789ABCDEF0123456789ABCDEF",
    "sn": "NEW-SN",
    "deviceSecret": "secret",
    "broker": "tcp://127.0.0.1:1883"
  },
  "mqtt": {"enabled": false}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GatewayKey != "NEW-SN" {
		t.Fatalf("GatewayKey = %q, want activation SN", cfg.GatewayKey)
	}
}

func TestSaveReplacesConfigWithValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"gatewayKey":"old"}`), 0644); err != nil {
		t.Fatal(err)
	}

	disabled := false
	cfg := Config{GatewayKey: "网关-测试", Activation: ActivationConfig{Enabled: &disabled}}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}
	if loaded.GatewayKey != cfg.GatewayKey {
		t.Fatalf("GatewayKey = %q, want %q", loaded.GatewayKey, cfg.GatewayKey)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".gateway-config-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary config files remain: %v", matches)
	}
}

func TestOPCUADeviceConnectionFlowsToPoints(t *testing.T) {
	raw := []byte(`{
  "gatewayKey": "gw-opcua",
  "activation": {"enabled": false},
  "mqtt": {"enabled": false},
  "devices": [{
    "deviceKey": "opcua-1",
    "protocol": "opcua",
    "address": "opc.tcp://192.168.1.10:4840",
    "username": "operator",
    "password": "secret",
    "points": [{"metric": "temperature", "nodeId": "ns=2;s=Temperature"}]
  }]
}`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(cfg.Points) != 1 {
		t.Fatalf("len(Points) = %d, want 1", len(cfg.Points))
	}
	point := cfg.Points[0]
	if point.Protocol != "opcua" || point.NodeID != "ns=2;s=Temperature" {
		t.Fatalf("unexpected OPC UA point: %#v", point)
	}
	if point.Username != "operator" || point.Password != "secret" {
		t.Fatalf("OPC UA credentials were not inherited from device")
	}
}

func TestRTUDeviceUsesNamedSerialPort(t *testing.T) {
	raw := []byte(`{
  "gatewayKey": "gw-rtu",
  "activation": {"enabled": false},
  "mqtt": {"enabled": false},
  "serialPorts": [
    {"name":"RS485-1","port":"/dev/ttyS1","baudRate":9600,"dataBits":8,"stopBits":1,"parity":"none","enabled":true},
    {"name":"RS485-2","port":"/dev/ttyS2","baudRate":19200,"dataBits":8,"stopBits":1,"parity":"even","enabled":true}
  ],
  "devices": [{
    "deviceKey": "meter-2",
    "interfaceType": "serial",
    "interfaceName": "RS485-2",
    "protocol": "modbus-rtu",
    "address": "/dev/stale",
    "slaveId": 2,
    "points": [{"metric":"energy","function":3,"register":0,"dataType":"uint16"}]
  }]
}`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	point := cfg.Points[0]
	if point.Address != "/dev/ttyS2" {
		t.Fatalf("Address = %q, want /dev/ttyS2", point.Address)
	}
	if point.BaudRate != 19200 || point.DataBits != 8 || point.StopBits != 1 || point.Parity != "even" {
		t.Fatalf("unexpected serial settings: %#v", point)
	}
}

func TestChannelDeviceExpandsToLegacyRuntimeConfig(t *testing.T) {
	raw := []byte(`{
  "gatewayKey": "gw-channel",
  "activation": {"enabled": false},
  "mqtt": {"enabled": false},
  "channels": [{
    "channelKey": "rs485-1",
    "name": "RS485-1",
    "type": "serial",
    "protocol": "modbus-rtu",
    "enabled": true,
    "serial": {"port": "/dev/ttyS1", "baudRate": 19200, "dataBits": 8, "stopBits": 1, "parity": "even", "enabled": true},
    "devices": [{
      "deviceKey": "meter-1",
      "slaveId": 3,
      "points": [{"metric": "energy", "function": 3, "register": 0, "dataType": "uint16"}]
    }]
  }]
}`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Devices) != 1 || cfg.Devices[0].ChannelKey != "rs485-1" {
		t.Fatalf("channel devices were not expanded: %#v", cfg.Devices)
	}
	point := cfg.Points[0]
	if point.Protocol != "modbus-rtu" || point.Address != "/dev/ttyS1" || point.SlaveID != 3 {
		t.Fatalf("unexpected expanded point connection: %#v", point)
	}
	if point.BaudRate != 19200 || point.Parity != "even" {
		t.Fatalf("unexpected channel serial settings: %#v", point)
	}
}

func TestResourceChannelDeviceExpandsToRuntimeConfig(t *testing.T) {
	raw := []byte(`{
  "gatewayKey": "gw-resource",
  "activation": {"enabled": false},
  "mqtt": {"enabled": false},
  "resources": [{
    "resourceKey": "eth0",
    "name": "网口1",
    "type": "network",
    "enabled": true,
    "network": {"interface": "eth0", "mode": "static", "ipAddress": "192.168.1.10", "prefixLength": 24, "enabled": true}
  }],
  "channels": [{
    "channelKey": "modbus-eth0",
    "resourceKey": "eth0",
    "name": "Modbus TCP",
    "protocol": "modbus-tcp",
    "network": {"address": "192.168.1.50:502"},
    "enabled": true,
    "devices": [{
      "deviceKey": "meter-1",
      "slaveId": 2,
      "points": [{"metric": "voltage", "function": 3, "register": 0, "dataType": "uint16"}]
    }]
  }]
}`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Resources) != 1 || cfg.Resources[0].ResourceKey != "eth0" {
		t.Fatalf("resources not preserved: %#v", cfg.Resources)
	}
	if len(cfg.Devices) != 1 || cfg.Devices[0].ChannelKey != "modbus-eth0" {
		t.Fatalf("channel devices were not expanded: %#v", cfg.Devices)
	}
	point := cfg.Points[0]
	if point.Protocol != "modbus-tcp" || point.Address != "192.168.1.50:502" || point.SlaveID != 2 {
		t.Fatalf("unexpected expanded point: %#v", point)
	}
}

func TestLegacyFlatRTUPointKeepsAddress(t *testing.T) {
	raw := []byte(`{
  "gatewayKey": "gw-legacy-rtu",
  "activation": {"enabled": false},
  "mqtt": {"enabled": false},
  "points": [{
    "deviceKey":"meter-1","metric":"value","protocol":"modbus-rtu",
    "address":"/dev/ttyUSB9","slaveId":1,"function":3,"register":0,"dataType":"uint16"
  }]
}`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	point := cfg.Points[0]
	if point.Address != "/dev/ttyUSB9" {
		t.Fatalf("Address = %q, want legacy point address", point.Address)
	}
	if point.BaudRate != 9600 || point.Parity != "none" {
		t.Fatalf("unexpected RTU defaults: %#v", point)
	}
}

func TestPointPresentationMetadataIsPreserved(t *testing.T) {
	cfg, err := Parse([]byte(`{
  "gatewayKey":"gw-display",
  "activation":{"enabled":false},
  "mqtt":{"enabled":false},
  "points":[{
    "deviceKey":"meter-1","name":"噪声值","metric":"noise_db",
    "protocol":"modbus-rtu","address":"/dev/ttyS1","slaveId":2,
    "function":3,"register":0,"dataType":"uint16","scale":0.1,
    "unit":"dB","decimals":1
  }]
}`))
	if err != nil {
		t.Fatal(err)
	}
	point := cfg.Points[0]
	if point.Unit != "dB" || point.Decimals != 1 {
		t.Fatalf("presentation metadata = %q/%d, want dB/1", point.Unit, point.Decimals)
	}
}

func TestLegacyNetworkPortsMapToPhysicalInterfaces(t *testing.T) {
	port := NetworkPort{Name: "net2", Address: "192.168.1.51:502", Mode: "tcp-client", Enabled: true}
	port.ApplyDefaults()
	if port.Interface != "eth1" || port.Mode != "static" || port.PrefixLength != 24 {
		t.Fatalf("legacy network port was not migrated: %#v", port)
	}
}

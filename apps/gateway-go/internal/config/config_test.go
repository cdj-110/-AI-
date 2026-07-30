package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMillisecondCollectionIntervalTakesPrecedence(t *testing.T) {
	cfg := Config{
		CollectIntervalSeconds: 5,
		Channels: []ChannelConfig{{
			CollectIntervalSeconds:      1,
			CollectIntervalMilliseconds: 100,
		}},
	}
	if got := cfg.CollectInterval(); got != 100*time.Millisecond {
		t.Fatalf("CollectInterval() = %s, want 100ms", got)
	}
	point := PointConfig{CollectIntervalSeconds: 1, CollectIntervalMilliseconds: 80}
	if got := point.CollectInterval(time.Second); got != 80*time.Millisecond {
		t.Fatalf("PointConfig.CollectInterval() = %s, want 80ms", got)
	}
}

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

func TestDeviceReconnectIntervalDefaultsToThirtySeconds(t *testing.T) {
	device := DeviceConfig{}
	device.ApplyDefaults()
	if device.ReconnectIntervalSeconds != 30 {
		t.Fatalf("ReconnectIntervalSeconds = %d, want 30", device.ReconnectIntervalSeconds)
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
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatalf("config permissions = %o, want no group/other access", info.Mode().Perm())
	}
}

func TestRedactAndPreserveSecrets(t *testing.T) {
	current := Config{
		GatewayKey: "gw",
		MQTT:       MQTTConfig{Password: "mqtt-secret"},
		WiFi:       WiFiConfig{Password: "wifi-secret"},
		Security:   SecurityConfig{Users: []UserConfig{{Username: "admin", PasswordHash: "$2a$hash", Enabled: true}}},
		Devices:    []DeviceConfig{{DeviceKey: "opc", Protocol: "opcua", Address: "opc.tcp://127.0.0.1:4840", Password: "opc-secret", Points: []PointConfig{{Metric: "p", NodeID: "ns=1;i=1"}}}},
	}
	redacted := RedactSecrets(current)
	if redacted.MQTT.Password != "***" || redacted.WiFi.Password != "***" || redacted.Devices[0].Password != "***" || len(redacted.Security.Users) != 0 {
		t.Fatalf("secrets were not redacted: %#v", redacted)
	}
	PreserveMaskedSecrets(current, &redacted)
	if redacted.MQTT.Password != "mqtt-secret" || redacted.WiFi.Password != "wifi-secret" || redacted.Devices[0].Password != "opc-secret" || len(redacted.Security.Users) != 1 {
		t.Fatalf("masked secrets were not restored: %#v", redacted)
	}
}

func TestValidateChecksNestedProtocolFields(t *testing.T) {
	_, err := Parse([]byte(`{"gatewayKey":"gw","mqtt":{"enabled":false},"devices":[{"deviceKey":"d1","protocol":"opcua","address":"opc.tcp://127.0.0.1:4840","points":[{"metric":"temperature"}]}]}`))
	if err == nil || !strings.Contains(err.Error(), "nodeId") {
		t.Fatalf("expected nested OPC UA nodeId validation, got %v", err)
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

func TestDuplicateChannelKeysAreMadeUniqueAndDevicesAreRemapped(t *testing.T) {
	raw := []byte(`{
  "gatewayKey": "gw-duplicate-channel",
  "activation": {"enabled": false},
  "mqtt": {"enabled": false},
  "resources": [{
    "resourceKey": "eth0",
    "name": "eth0",
    "type": "network",
    "enabled": true,
    "network": {"interface": "eth0", "mode": "static", "ipAddress": "192.168.1.10", "prefixLength": 24, "enabled": true}
  }],
  "channels": [
    {"channelKey": "eth0-channel-05", "resourceKey": "eth0", "name": "OPC UA", "protocol": "opcua", "enabled": true},
    {"channelKey": "eth0-channel-05", "resourceKey": "eth0", "name": "IEC104", "protocol": "iec104", "enabled": true}
  ],
  "devices": [{
    "deviceKey": "iec104-device",
    "channelKey": "eth0-channel-05",
    "protocol": "iec104",
    "address": "192.168.1.141:2404",
    "points": [{"metric": "DY1", "register": 1, "dataType": "float32"}]
  }]
}`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Channels) != 2 {
		t.Fatalf("len(Channels) = %d, want 2", len(cfg.Channels))
	}
	if cfg.Channels[0].ChannelKey == cfg.Channels[1].ChannelKey {
		t.Fatalf("channel keys were not made unique: %#v", cfg.Channels)
	}
	if cfg.Devices[0].ChannelKey != cfg.Channels[1].ChannelKey {
		t.Fatalf("device channelKey = %q, want renamed IEC104 key %q", cfg.Devices[0].ChannelKey, cfg.Channels[1].ChannelKey)
	}
	if cfg.Points[0].ChannelKey != cfg.Channels[1].ChannelKey {
		t.Fatalf("point channelKey = %q, want renamed IEC104 key %q", cfg.Points[0].ChannelKey, cfg.Channels[1].ChannelKey)
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

func TestIEC104ExpandedAddressesAndLegacyFallback(t *testing.T) {
	cfg := Config{
		CollectIntervalSeconds: 5,
		Channels:               []ChannelConfig{{ChannelKey: "iec", Protocol: "iec104", IEC104: IEC104Config{Configured: true}}},
		Devices: []DeviceConfig{
			{DeviceKey: "new", ChannelKey: "iec", Protocol: "iec104", Address: "192.168.1.10:2404", CommonAddress: 1024, Points: []PointConfig{{Metric: "large", IOA: 70000, DataType: "float32"}}},
			{DeviceKey: "legacy", ChannelKey: "iec", Protocol: "iec104", Address: "192.168.1.11:2404", SlaveID: 7, Points: []PointConfig{{Metric: "old", Register: 123, DataType: "float32"}}},
		},
	}
	cfg.ApplyDefaults()
	points := cfg.FlattenPoints()
	if points[0].CommonAddress != 1024 || points[0].IOA != 70000 {
		t.Fatalf("expanded IEC104 addresses = CA %d IOA %d", points[0].CommonAddress, points[0].IOA)
	}
	if points[1].CommonAddress != 7 || points[1].IOA != 123 {
		t.Fatalf("legacy IEC104 fallback = CA %d IOA %d", points[1].CommonAddress, points[1].IOA)
	}
}

func TestIEC104ConfiguredAllOffIsPreserved(t *testing.T) {
	options := IEC104Config{Configured: true}
	options.ApplyDefaults()
	if options.AcquisitionMode != "auto" || options.GeneralInterrogationOnStart || options.ClockSyncOnStart || options.CounterInterrogationOnStart || options.ClockSyncIntervalSeconds != 0 {
		t.Fatalf("explicit all-off IEC104 options were changed: %#v", options)
	}
}

func TestIEC104AcquisitionModeDefaultsAndLegacyPeriodicMigration(t *testing.T) {
	automatic := IEC104Config{}
	automatic.ApplyDefaults()
	if automatic.AcquisitionMode != "auto" || !automatic.GeneralInterrogationOnStart || automatic.GeneralInterrogationIntervalSeconds != 0 || automatic.ClockSyncOnStart || automatic.CounterInterrogationOnStart {
		t.Fatalf("automatic IEC104 defaults = %#v", automatic)
	}
	legacyPeriodic := IEC104Config{Configured: true, GeneralInterrogationIntervalSeconds: 30}
	legacyPeriodic.ApplyDefaults()
	if legacyPeriodic.AcquisitionMode != "periodic" || legacyPeriodic.GeneralInterrogationIntervalSeconds != 30 {
		t.Fatalf("legacy periodic IEC104 config was not preserved: %#v", legacyPeriodic)
	}
	explicitPeriodic := IEC104Config{Configured: true, AcquisitionMode: "periodic"}
	explicitPeriodic.ApplyDefaults()
	if explicitPeriodic.GeneralInterrogationIntervalSeconds != 5 {
		t.Fatalf("periodic IEC104 default interval = %d, want 5", explicitPeriodic.GeneralInterrogationIntervalSeconds)
	}
}

func TestIEC104RejectsIOAOutside24BitRange(t *testing.T) {
	_, err := Parse([]byte(`{
  "gatewayKey":"gw-iec104-invalid",
  "activation":{"enabled":false},
  "mqtt":{"enabled":false},
  "points":[{
    "deviceKey":"station","metric":"bad","protocol":"iec104",
    "address":"192.168.1.10:2404","commonAddress":1,
    "ioa":16777216,"dataType":"float32"
  }]
}`))
	if err == nil {
		t.Fatal("expected IOA range validation error")
	}
}

func TestForwardDeviceDefaultsAndEnablesServer(t *testing.T) {
	cfg := Config{ForwardDevices: []ForwardDeviceConfig{{Protocol: "modbus-tcp-slave", Points: []ForwardPointConfig{{SourceDeviceKey: "d1", SourceMetric: "p1"}}}}}
	cfg.ApplyDefaults()
	if cfg.ForwardSlave.IEC61850Listen != "0.0.0.0:102" {
		t.Fatalf("IEC61850 MMS default listen = %q, want standard port 102", cfg.ForwardSlave.IEC61850Listen)
	}
	device := cfg.ForwardDevices[0]
	if !cfg.ForwardSlave.Enabled || device.DeviceKey == "" || device.UnitID != 1 {
		t.Fatalf("forward defaults not applied: %#v", device)
	}
	if device.Points[0].Function != 3 || device.Points[0].Metric != "p1" || device.Points[0].Quantity != 1 || device.Points[0].Scale != 1 {
		t.Fatalf("forward point defaults not applied: %#v", device.Points[0])
	}
}

func TestParseRejectsInvalidForwardListenPort(t *testing.T) {
	raw := []byte(`{"gatewayKey":"gw","forwardSlave":{"modbusListen":"0.0.0.0:70000","iec104Listen":"0.0.0.0:2404"}}`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("Parse() accepted an invalid Modbus forwarding port")
	}
}

func TestForwardOnlyChannelDoesNotGetCollectionProtocol(t *testing.T) {
	cfg := Config{Channels: []ChannelConfig{{ChannelKey: "forward", Role: "forward", ForwardProtocol: "iec104-server"}}}
	cfg.ApplyDefaults()
	channel := cfg.Channels[0]
	if channel.Role != "forward" || channel.Protocol != "none" || channel.ForwardProtocol != "iec104-server" {
		t.Fatalf("forward-only channel defaults = %#v", channel)
	}
}

func TestIEC61850MMSForwardDefaultsCreateCanonicalObjectReference(t *testing.T) {
	cfg := Config{
		GatewayKey: "gw",
		ForwardDevices: []ForwardDeviceConfig{{
			Protocol: "iec61850-mms-server",
			Points: []ForwardPointConfig{{
				SourceDeviceKey: "source", SourceMetric: "temperature-1", DataType: "float32",
			}, {
				SourceDeviceKey: "source", SourceMetric: "alarm", DataType: "bool",
			}},
		}},
	}
	cfg.ApplyDefaults()
	device := cfg.ForwardDevices[0]
	if device.IEDName != "WEIKONG" || device.LogicalDevice != "LD1" {
		t.Fatalf("IEC61850 MMS device defaults = %#v", device)
	}
	if got := device.Points[0]; got.ObjectRef != "WEIKONGLD1/GGIO1.temperature1.mag.f" || got.FC != "MX" {
		t.Fatalf("numeric IEC61850 MMS point defaults = %#v", got)
	}
	if got := device.Points[1]; got.ObjectRef != "WEIKONGLD1/GGIO1.alarm.stVal" || got.FC != "ST" {
		t.Fatalf("boolean IEC61850 MMS point defaults = %#v", got)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate IEC61850 MMS forwarding: %v", err)
	}
}

func TestLegacyCollectionChannelKeepsCollectionRole(t *testing.T) {
	cfg := Config{Channels: []ChannelConfig{{ChannelKey: "collect", Protocol: "modbus-tcp"}}}
	cfg.ApplyDefaults()
	if cfg.Channels[0].Role != "collect" || cfg.Channels[0].Protocol != "modbus-tcp" {
		t.Fatalf("legacy channel defaults = %#v", cfg.Channels[0])
	}
}

func TestLegacyForwardOnlyChannelIsInferredWithoutDevices(t *testing.T) {
	cfg := Config{Channels: []ChannelConfig{{ChannelKey: "legacy-forward", Protocol: "modbus-tcp", ForwardProtocol: "modbus-tcp-slave"}}}
	cfg.ApplyDefaults()
	if cfg.Channels[0].Role != "forward" || cfg.Channels[0].Protocol != "none" {
		t.Fatalf("legacy forward channel was not inferred: %#v", cfg.Channels[0])
	}
}

func TestValidateGlobalDeviceKeysAcrossDeviceKinds(t *testing.T) {
	cfg := Config{
		Devices:        []DeviceConfig{{DeviceKey: "mbtcp-01"}},
		ForwardDevices: []ForwardDeviceConfig{{DeviceKey: "iec104-01"}},
		EdgeComputing:  EdgeComputingConfig{Groups: []EdgeComputeGroupConfig{{GroupKey: "edge-01"}}},
	}
	if err := cfg.ValidateGlobalDeviceKeys(); err != nil {
		t.Fatalf("unique keys rejected: %v", err)
	}
	cfg.EdgeComputing.Groups[0].GroupKey = "MBTCP-01"
	if err := cfg.ValidateGlobalDeviceKeys(); err == nil {
		t.Fatal("case-insensitive duplicate key accepted")
	}
}

func TestLegacyDuplicateDeviceKeysRemainLoadable(t *testing.T) {
	raw := []byte(`{
  "gatewayKey":"gw",
  "activation":{"enabled":false},
  "mqtt":{"enabled":false},
  "devices":[
    {"deviceKey":"device-006","protocol":"modbus-tcp","points":[]},
    {"deviceKey":"device-006","protocol":"iec104","points":[]}
  ]
}`)
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("legacy duplicate config should load: %v", err)
	}
	if err := cfg.ValidateNewGlobalIdentifierDuplicates(cfg); err != nil {
		t.Fatalf("unchanged legacy duplicate should remain saveable: %v", err)
	}
	changed := cfg
	changed.ForwardDevices = append(changed.ForwardDevices, ForwardDeviceConfig{DeviceKey: "device-006"})
	if err := changed.ValidateNewGlobalIdentifierDuplicates(cfg); err == nil {
		t.Fatal("new duplicate was accepted")
	}
}

func TestNewGlobalPointMetricDuplicateIsRejected(t *testing.T) {
	previous := Config{Devices: []DeviceConfig{
		{DeviceKey: "mbtcp-01", Points: []PointConfig{{Metric: "temperature"}}},
		{DeviceKey: "iec104-01", Points: []PointConfig{{Metric: "running"}}},
	}}
	changed := previous
	changed.Devices = append([]DeviceConfig(nil), previous.Devices...)
	changed.Devices[1].Points = append([]PointConfig(nil), previous.Devices[1].Points...)
	changed.Devices[1].Points = append(changed.Devices[1].Points, PointConfig{Metric: "Temperature"})
	if err := changed.ValidateNewGlobalIdentifierDuplicates(previous); err == nil {
		t.Fatal("new case-insensitive point metric duplicate was accepted")
	}
}

func TestForwardAliasMayKeepSourceMetric(t *testing.T) {
	previous := Config{Devices: []DeviceConfig{{DeviceKey: "source", Points: []PointConfig{{Metric: "temperature"}}}}}
	changed := previous
	changed.ForwardDevices = []ForwardDeviceConfig{{DeviceKey: "forward", Points: []ForwardPointConfig{{Metric: "temperature", SourceDeviceKey: "source", SourceMetric: "temperature"}}}}
	if err := changed.ValidateNewGlobalIdentifierDuplicates(previous); err != nil {
		t.Fatalf("forward alias should keep its source metric: %v", err)
	}
}

func TestLegacyConfigIsMigratedToCurrentSchema(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gw","activation":{"enabled":false},"mqtt":{"enabled":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", cfg.SchemaVersion, CurrentSchemaVersion)
	}
}

func TestFutureConfigSchemaIsRejected(t *testing.T) {
	_, err := Parse([]byte(`{"schemaVersion":999,"gatewayKey":"gw","activation":{"enabled":false},"mqtt":{"enabled":false}}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported config schemaVersion") {
		t.Fatalf("error = %v, want unsupported schema error", err)
	}
}

package config

import (
	"reflect"
	"testing"
)

func TestApplyInitialHardwareProfileBuildsEG100InventoryAndPreservesNetworkSettings(t *testing.T) {
	cfg := Config{
		Resources: []ResourceConfig{{
			ResourceKey: "网口1", Name: "网口1", Type: "network", Enabled: true,
			Network: &NetworkPort{Name: "网口1", Interface: "eth0", Mode: "static", IPAddress: "192.168.1.100", PrefixLength: 24, Gateway: "192.168.1.1", Enabled: true},
		}},
		WiFi:     WiFiConfig{Enabled: false, SSID: "factory-network"},
		Cellular: CellularConfig{Enabled: boolPointer(false)},
	}
	profile := HardwareProfile{
		EthernetPortCount: 2, RS232PortCount: 2, RS485PortCount: 2,
		CANPortCount: 0, Supports4G: true, SupportsWiFi: true,
	}
	if err := ApplyInitialHardwareProfile(&cfg, "EG100", profile); err != nil {
		t.Fatal(err)
	}
	if cfg.ProductModel != "EG100" || cfg.HardwareProfile == nil || *cfg.HardwareProfile != profile {
		t.Fatalf("hardware metadata was not persisted: %#v %#v", cfg.ProductModel, cfg.HardwareProfile)
	}
	if len(cfg.Resources) != 6 {
		t.Fatalf("resources = %d, want 6", len(cfg.Resources))
	}
	if got := cfg.Resources[0].Network; got == nil || got.IPAddress != "192.168.1.100" || got.Gateway != "192.168.1.1" {
		t.Fatalf("existing eth0 settings were lost: %#v", got)
	}
	wantSerial := []struct {
		name, kind, port string
	}{
		{"RS485-1", "rs485", "/dev/ttyS1"},
		{"RS485-2", "rs485", "/dev/ttyS2"},
		{"RS232-1", "rs232", "/dev/ttyS3"},
		{"RS232-2", "rs232", "/dev/ttyS5"},
	}
	for index, want := range wantSerial {
		serial := cfg.Resources[index+2].Serial
		if serial == nil || serial.Name != want.name || serial.ElectricalType != want.kind || serial.Port != want.port {
			t.Fatalf("serial %d = %#v, want %+v", index, serial, want)
		}
	}
	if !cfg.WiFi.Enabled || !cfg.Cellular.IsEnabled() {
		t.Fatalf("licensed radios were not enabled: wifi=%v cellular=%v", cfg.WiFi.Enabled, cfg.Cellular.IsEnabled())
	}
	if cfg.WiFi.SSID != "factory-network" {
		t.Fatal("existing Wi-Fi settings were cleared")
	}
	if cfg.Channels != nil || cfg.SerialPorts != nil || cfg.NetworkPorts != nil {
		t.Fatal("legacy derived inventory was not cleared")
	}
}

func TestApplyInitialHardwareProfileDisablesUnsupportedRadios(t *testing.T) {
	cfg := Config{WiFi: WiFiConfig{Enabled: true}, Cellular: CellularConfig{Enabled: boolPointer(true)}}
	if err := ApplyInitialHardwareProfile(&cfg, "TEST", HardwareProfile{}); err != nil {
		t.Fatal(err)
	}
	if cfg.WiFi.Enabled || cfg.Cellular.IsEnabled() {
		t.Fatal("unsupported radios remained enabled")
	}
}

func boolPointer(value bool) *bool { return &value }

func TestInitialHardwareProfilePreservesProjectBindingsByPhysicalPort(t *testing.T) {
	cfg := Config{
		Resources: []ResourceConfig{
			{ResourceKey: "factory-serial", Type: "serial", Serial: &SerialPort{Port: "/dev/ttyS1", Enabled: true}},
			{ResourceKey: "factory-lan", Type: "network", Network: &NetworkPort{Interface: "eth0", Enabled: true}},
		},
		Channels: []ChannelConfig{
			{ChannelKey: "serial-custom", ResourceKey: "factory-serial", Protocol: "modbus-rtu", Enabled: true},
			{ChannelKey: "channel-001", ResourceKey: "factory-lan", Protocol: "siemens-s7", Enabled: true},
		},
		Devices: []DeviceConfig{{DeviceKey: "device-001", ChannelKey: "serial-custom"}, {DeviceKey: "device-003", ChannelKey: "channel-001"}},
		Points:  []PointConfig{{DeviceKey: "device-003", ChannelKey: "channel-001", Metric: "p1"}},
	}
	devices, points := append([]DeviceConfig(nil), cfg.Devices...), append([]PointConfig(nil), cfg.Points...)
	if err := ApplyInitialHardwareProfile(&cfg, "EG100", HardwareProfile{EthernetPortCount: 2, RS485PortCount: 2}); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Channels) != 2 || cfg.Channels[0].ResourceKey != "RS485-1" || cfg.Channels[1].ResourceKey != "网口1" {
		t.Fatalf("physical port mapping lost: %+v", cfg.Channels)
	}
	if !reflect.DeepEqual(cfg.Devices, devices) || !reflect.DeepEqual(cfg.Points, points) {
		t.Fatal("first activation modified device/point references")
	}
	cfg.ApplyDefaults()
	if cfg.Devices[0].ChannelKey != "serial-custom" || cfg.Devices[1].ChannelKey != "channel-001" || cfg.Channels[1].Protocol != "siemens-s7" {
		t.Fatal("normalization changed existing project channel identities")
	}
}

func TestInitialHardwareProfileDoesNotRebindRemovedPortByName(t *testing.T) {
	cfg := Config{
		Resources: []ResourceConfig{{ResourceKey: "RS485-1", Type: "serial", Serial: &SerialPort{Port: "/dev/ttyS9"}}},
		Channels:  []ChannelConfig{{ChannelKey: "custom", ResourceKey: "RS485-1", Protocol: "modbus-rtu", Enabled: true}},
		Devices:   []DeviceConfig{{DeviceKey: "d1", ChannelKey: "custom", Points: []PointConfig{{Metric: "value"}}}},
	}
	if err := ApplyInitialHardwareProfile(&cfg, "EG100", HardwareProfile{RS485PortCount: 1}); err != nil {
		t.Fatal(err)
	}
	if cfg.Channels[0].ResourceKey != "__unbound:custom" || cfg.Channels[0].Enabled || len(cfg.Devices) != 1 {
		t.Fatalf("removed port must retain a disabled, unbound project channel: %+v", cfg.Channels)
	}
	if cfg.Devices[0].IsEnabled() || len(cfg.FlattenPoints()) != 0 {
		t.Fatal("devices on removed hardware must not keep collecting")
	}
}

func TestInitialHardwareProfileZeroPortsDoesNotResurrectLegacyInventory(t *testing.T) {
	cfg := Config{Channels: []ChannelConfig{{ChannelKey: "old", ResourceKey: "removed", Serial: &SerialPort{Port: "/dev/ttyS1"}}}}
	if err := ApplyInitialHardwareProfile(&cfg, "EG100", HardwareProfile{}); err != nil {
		t.Fatal(err)
	}
	cfg.ApplyDefaults()
	if len(cfg.Resources) != 0 || len(cfg.Channels) != 1 || cfg.Channels[0].Enabled {
		t.Fatal("removed resources were reconstructed from legacy channels")
	}
}

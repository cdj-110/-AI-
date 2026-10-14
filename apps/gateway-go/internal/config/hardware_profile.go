package config

import (
	"fmt"
	"strings"
)

const maximumHardwarePortCount = 64

func (p HardwareProfile) Validate() error {
	counts := []struct {
		name  string
		value int
	}{
		{"ethernetPortCount", p.EthernetPortCount},
		{"rs232PortCount", p.RS232PortCount},
		{"rs485PortCount", p.RS485PortCount},
		{"canPortCount", p.CANPortCount},
	}
	for _, count := range counts {
		if count.value < 0 || count.value > maximumHardwarePortCount {
			return fmt.Errorf("%s must be between 0 and %d", count.name, maximumHardwarePortCount)
		}
	}
	return nil
}

func hardwareProfilesEqual(left, right *HardwareProfile) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

// ApplyInitialHardwareProfile materializes the licensed product capabilities
// into the gateway configuration. It is intended for the first activation
// only; replacement licenses must not rebuild resources or change the user's
// Wi-Fi/mobile-network choices.
func ApplyInitialHardwareProfile(cfg *Config, productModel string, profile HardwareProfile) error {
	if cfg == nil {
		return fmt.Errorf("gateway config is required")
	}
	if strings.TrimSpace(productModel) == "" {
		return fmt.Errorf("productModel is required")
	}
	if err := profile.Validate(); err != nil {
		return err
	}

	existingNetworks := map[string]NetworkPort{}
	existingSerial := map[string]SerialPort{}
	for _, resource := range cfg.Resources {
		if resource.Network != nil {
			network := *resource.Network
			network.ApplyDefaults()
			existingNetworks[strings.ToLower(network.Interface)] = network
		}
		if resource.Serial != nil {
			serial := *resource.Serial
			existingSerial[serial.Port] = serial
		}
	}
	for _, network := range cfg.NetworkPorts {
		network.ApplyDefaults()
		existingNetworks[strings.ToLower(network.Interface)] = network
	}
	for _, serial := range cfg.SerialPorts {
		existingSerial[serial.Port] = serial
	}

	resources := make([]ResourceConfig, 0, profile.EthernetPortCount+profile.RS485PortCount+profile.RS232PortCount)
	for index := 0; index < profile.EthernetPortCount; index++ {
		name := fmt.Sprintf("网口%d", index+1)
		interfaceName := fmt.Sprintf("eth%d", index)
		network, ok := existingNetworks[strings.ToLower(interfaceName)]
		if !ok {
			network = NetworkPort{Name: name, Interface: interfaceName, Mode: "static", PrefixLength: 24, Enabled: true}
		} else {
			network.Name = name
			network.Interface = interfaceName
		}
		resources = append(resources, ResourceConfig{
			ResourceKey: name, Name: name, Type: "network", Enabled: network.Enabled, Network: &network,
		})
	}
	for index := 0; index < profile.RS485PortCount; index++ {
		resources = append(resources, licensedSerialResource(productModel, "RS485", index, profile, existingSerial))
	}
	for index := 0; index < profile.RS232PortCount; index++ {
		resources = append(resources, licensedSerialResource(productModel, "RS232", index, profile, existingSerial))
	}

	cfg.ProductModel = strings.TrimSpace(productModel)
	profileCopy := profile
	cfg.HardwareProfile = &profileCopy
	// Keep project channel keys (and therefore device/point references) stable.
	// Match physical ports, not display names that may be reused by a new model.
	remapLicensedChannels(cfg, resources)
	cfg.Resources = resources
	// Resources are now authoritative. Clearing the derived views prevents old
	// factory defaults from being merged back into the licensed inventory.
	cfg.SerialPorts = nil
	cfg.NetworkPorts = nil
	cfg.WiFi.Enabled = profile.SupportsWiFi
	cellularEnabled := profile.Supports4G
	cfg.Cellular.Enabled = &cellularEnabled
	return nil
}

func remapLicensedChannels(cfg *Config, resources []ResourceConfig) {
	oldResources := map[string]ResourceConfig{}
	for _, resource := range cfg.Resources {
		oldResources[resource.ResourceKey] = resource
	}
	identity := func(resource ResourceConfig) string {
		if resource.Network != nil && resource.Network.Interface != "" {
			return "network:" + strings.ToLower(resource.Network.Interface)
		}
		if resource.Serial != nil && resource.Serial.Port != "" {
			return "serial:" + resource.Serial.Port
		}
		return ""
	}
	for i := range cfg.Channels {
		channel := &cfg.Channels[i]
		old, exists := oldResources[channel.ResourceKey]
		if !exists {
			old = ResourceConfig{ResourceKey: channel.ResourceKey, Serial: channel.Serial, Network: channel.Network}
		}
		oldIdentity := identity(old)
		matched := false
		for _, resource := range resources {
			if (oldIdentity != "" && oldIdentity == identity(resource)) ||
				(oldIdentity == "" && channel.ResourceKey == resource.ResourceKey) {
				channel.ResourceKey = resource.ResourceKey
				matched = true
				break
			}
		}
		if !matched {
			// Retain configuration for manual recovery, but never silently bind a
			// removed port to the new model's similarly named resource or run it.
			channel.ResourceKey = "__unbound:" + channel.ChannelKey
			channel.Enabled = false
			disabled := false
			for j := range channel.Devices {
				channel.Devices[j].Enabled = &disabled
			}
			for j := range cfg.Devices {
				if cfg.Devices[j].ChannelKey == channel.ChannelKey {
					cfg.Devices[j].Enabled = &disabled
				}
			}
			for j := range cfg.ForwardDevices {
				if cfg.ForwardDevices[j].ChannelKey == channel.ChannelKey {
					cfg.ForwardDevices[j].Enabled = &disabled
				}
			}
			for j := range cfg.Points {
				if cfg.Points[j].ChannelKey == channel.ChannelKey {
					cfg.Points[j].Enabled = &disabled
				}
			}
		}
	}
}

func licensedSerialResource(productModel, electricalType string, index int, profile HardwareProfile, existing map[string]SerialPort) ResourceConfig {
	name := fmt.Sprintf("%s-%d", electricalType, index+1)
	port := licensedSerialDevice(productModel, electricalType, index, profile)
	serial, ok := existing[port]
	if !ok {
		serial = SerialPort{Name: name, Port: port, BaudRate: 9600, DataBits: 8, StopBits: 1, Parity: "none", Enabled: true}
	}
	serial.Name = name
	serial.Port = port
	serial.ElectricalType = strings.ToLower(electricalType)
	serial.ApplyDefaults()
	return ResourceConfig{ResourceKey: name, Name: name, Type: "serial", Enabled: serial.Enabled, Serial: &serial}
}

func licensedSerialDevice(productModel, electricalType string, index int, profile HardwareProfile) string {
	if strings.EqualFold(strings.TrimSpace(productModel), "EG100") {
		if electricalType == "RS485" {
			ports := []string{"/dev/ttyS1", "/dev/ttyS2"}
			if index < len(ports) {
				return ports[index]
			}
		}
		if electricalType == "RS232" {
			ports := []string{"/dev/ttyS3", "/dev/ttyS5"}
			if index < len(ports) {
				return ports[index]
			}
		}
	}
	offset := index + 1
	if electricalType == "RS232" {
		offset += profile.RS485PortCount
	}
	return fmt.Sprintf("/dev/ttyS%d", offset)
}

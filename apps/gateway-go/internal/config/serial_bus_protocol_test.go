package config

import (
	"strings"
	"testing"
)

func serialBusTestConfig(channelProtocols ...string) Config {
	channels := make([]ChannelConfig, 0, len(channelProtocols))
	for index, protocol := range channelProtocols {
		channels = append(channels, ChannelConfig{
			ChannelKey:  "rs485-channel-" + string(rune('a'+index)),
			ResourceKey: "rs485-2",
			Name:        "RS485-2",
			Type:        "serial",
			Role:        "collect",
			Protocol:    protocol,
			Enabled:     true,
		})
	}
	return Config{
		Resources: []ResourceConfig{{
			ResourceKey: "rs485-2",
			Name:        "RS485-2",
			Type:        "serial",
			Enabled:     true,
			Serial:      &SerialPort{Name: "RS485-2", Port: "/dev/ttyS2", Enabled: true},
		}},
		Channels: channels,
	}
}

func TestValidateSerialBusProtocolsAcceptsOneProtocol(t *testing.T) {
	cfg := serialBusTestConfig("dlt645", "dlt645")
	if err := cfg.validateSerialBusProtocols(); err != nil {
		t.Fatalf("expected one bus protocol to pass: %v", err)
	}
}

func TestValidateSerialBusProtocolsRejectsMixedProtocols(t *testing.T) {
	cfg := serialBusTestConfig("modbus-rtu", "dlt645")
	err := cfg.validateSerialBusProtocols()
	if err == nil || !strings.Contains(err.Error(), "不能同时使用") {
		t.Fatalf("expected mixed serial bus protocols to fail, got %v", err)
	}
}

func TestValidateSerialBusProtocolsRejectsDeviceMismatch(t *testing.T) {
	cfg := serialBusTestConfig("dlt645")
	cfg.Devices = []DeviceConfig{{
		DeviceKey:  "meter-1",
		ChannelKey: cfg.Channels[0].ChannelKey,
		Name:       "电表1",
		Protocol:   "modbus-rtu",
	}}
	err := cfg.validateSerialBusProtocols()
	if err == nil || !strings.Contains(err.Error(), "所属 RS485 总线协议") {
		t.Fatalf("expected device mismatch to fail, got %v", err)
	}
}

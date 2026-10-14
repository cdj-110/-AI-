package config

import "testing"

func TestGOOSEDeviceDefaultsPropagateToPoints(t *testing.T) {
	device := DeviceConfig{
		DeviceKey: "goose-source-1",
		Protocol:  "iec61850-goose",
		Address:   "eth0",
		GoCBRef:   "IEDLD1/LLN0$GO$gcb01",
		Points: []PointConfig{{
			Metric:     "breaker_closed",
			GooseIndex: 0,
		}},
	}
	device.ApplyDefaults()
	if device.AppID != 0x1000 || device.DestinationMAC != "01:0c:cd:01:00:01" {
		t.Fatalf("unexpected GOOSE defaults: %#v", device)
	}
	point := device.Points[0]
	if point.Protocol != "iec61850-goose" || point.Address != "eth0" || point.GoCBRef != device.GoCBRef {
		t.Fatalf("GOOSE connection was not propagated: %#v", point)
	}
	if point.DataType != "bool" {
		t.Fatalf("GOOSE default data type = %q, want bool", point.DataType)
	}
}

func TestGOOSEPublisherDefaults(t *testing.T) {
	device := ForwardDeviceConfig{
		DeviceKey:     "goose-publisher-1",
		Protocol:      "iec61850-goose-publisher",
		InterfaceName: "eth0",
		Points: []ForwardPointConfig{{
			Metric:          "breaker_closed",
			SourceDeviceKey: "source-1",
			SourceMetric:    "breaker_closed",
			DataType:        "bool",
		}},
	}
	device.ApplyDefaults(0)
	if device.GoCBRef == "" || device.DataSetRef == "" {
		t.Fatalf("expected generated GOOSE references: %#v", device)
	}
	if device.TimeAllowedToLiveMS != 2000 || device.ConfRev != 1 {
		t.Fatalf("unexpected GOOSE publisher timing defaults: %#v", device)
	}
	cfg := Config{ForwardDevices: []ForwardDeviceConfig{device}}
	if err := cfg.validateIEC61850GOOSEForwarding(); err != nil {
		t.Fatalf("valid GOOSE publisher rejected: %v", err)
	}
}

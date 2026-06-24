package collector

import (
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestNormalizeParity(t *testing.T) {
	tests := map[string]string{
		"none": "N",
		"N":    "N",
		"even": "E",
		"E":    "E",
		"odd":  "O",
		"O":    "O",
	}
	for input, want := range tests {
		if got := normalizeParity(input); got != want {
			t.Errorf("normalizeParity(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRTUSettingsKeyUsesSerialFraming(t *testing.T) {
	point := config.PointConfig{Protocol: "modbus-rtu", Address: "/dev/ttyS1"}
	if got := rtuSettingsKey(point); got != "9600-8-1-N" {
		t.Fatalf("rtuSettingsKey() = %q", got)
	}
	point.SlaveID = 7
	if got := rtuSettingsKey(point); got != "9600-8-1-N" {
		t.Fatalf("slave ID must not split a physical bus, got %q", got)
	}
}

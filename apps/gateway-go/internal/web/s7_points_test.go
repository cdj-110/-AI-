package web

import (
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestIsZeroNumericS7ScanValue(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		value    interface{}
		want     bool
	}{
		{name: "zero integer", dataType: "int32", value: float64(0), want: true},
		{name: "non-zero integer", dataType: "uint16", value: float64(12), want: false},
		{name: "zero float", dataType: "float32", value: float32(0), want: true},
		{name: "false bool remains visible", dataType: "bool", value: false, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			point := config.PointConfig{DataType: test.dataType}
			if got := isZeroNumericS7ScanValue(point, test.value); got != test.want {
				t.Fatalf("isZeroNumericS7ScanValue() = %v, want %v", got, test.want)
			}
		})
	}
}

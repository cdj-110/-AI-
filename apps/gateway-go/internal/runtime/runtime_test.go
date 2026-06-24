package runtime

import (
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestGroupBatchableRTUPointsBySlave(t *testing.T) {
	points := []config.PointConfig{
		{Protocol: "modbus-rtu", Address: "/dev/ttyS1", SlaveID: 1, Function: 3, Metric: "a"},
		{Protocol: "modbus-rtu", Address: "/dev/ttyS1", SlaveID: 1, Function: 3, Metric: "b"},
		{Protocol: "modbus-rtu", Address: "/dev/ttyS1", SlaveID: 2, Function: 3, Metric: "c"},
	}
	groups := groupBatchablePoints(points)
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if got := len(groups[groupKey(points[0])]); got != 2 {
		t.Fatalf("slave 1 group size = %d, want 2", got)
	}
}

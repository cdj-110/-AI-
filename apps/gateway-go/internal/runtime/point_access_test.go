package runtime

import (
	"testing"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestReportedPointAccessMatchesExecutableCapabilities(t *testing.T) {
	disabled := false
	cases := []struct {
		name        string
		point       config.PointConfig
		read, write bool
	}{
		{"coil", config.PointConfig{Protocol: "modbus-rtu", Function: 1}, true, true},
		{"holding", config.PointConfig{Protocol: "modbus-tcp", Function: 3}, true, true},
		{"discrete", config.PointConfig{Protocol: "modbus-rtu", Function: 2}, true, false},
		{"input", config.PointConfig{Protocol: "modbus-tcp", Function: 4}, true, false},
		{"s7-marker", config.PointConfig{Protocol: "siemens-s7", Area: "M", DataType: "bool"}, true, true},
		{"s7-db", config.PointConfig{Protocol: "siemens-s7", Area: "DB", DataType: "uint16"}, true, true},
		{"s7-input", config.PointConfig{Protocol: "siemens-s7", Area: "I", DataType: "bool"}, true, false},
		{"opc-read", config.PointConfig{Protocol: "opcda", Access: "read"}, true, false},
		{"opc-write", config.PointConfig{Protocol: "opcda", Access: "write"}, false, true},
		{"opc-rw", config.PointConfig{Protocol: "opcda", Access: "read_write"}, true, true},
		{"meter-rw", config.PointConfig{Protocol: "dlt645", DLT645: &config.DLT645Config{Access: "read_write"}}, true, true},
		{"meter-write", config.PointConfig{Protocol: "dlt645", DLT645: &config.DLT645Config{Access: "write"}}, false, true},
		{"disabled", config.PointConfig{Protocol: "modbus-rtu", Function: 3, Enabled: &disabled}, false, false},
		{"iec-read", config.PointConfig{Protocol: "iec61850"}, true, false},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			read, write := PointAccess(item.point)
			if read != item.read || write != item.write {
				t.Fatalf("read/write = %v/%v, want %v/%v", read, write, item.read, item.write)
			}
		})
	}
	cfg := config.Config{Points: []config.PointConfig{{DeviceKey: "child-a", Metric: "same", Protocol: "modbus-rtu", Function: 3}, {DeviceKey: "child-b", Metric: "same", Protocol: "modbus-rtu", Function: 4}}}
	manager := NewManager(cfg, state.New(cfg))
	access := manager.PointAccesses()
	if len(access) != 2 || access[0]["deviceKey"] != "child-a" || access[0]["canWrite"] != true || access[1]["canWrite"] != false {
		t.Fatalf("unexpected scoped report: %+v", access)
	}
}

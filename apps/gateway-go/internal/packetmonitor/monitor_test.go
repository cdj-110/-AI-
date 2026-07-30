package packetmonitor

import "testing"

func TestMonitorKeepsBoundedFilteredFrames(t *testing.T) {
	monitor := New(2)
	monitor.Record(Frame{Protocol: "MODBUS-TCP", DeviceKey: "one", Direction: "TX", Summary: "a"}, []byte{1})
	monitor.Record(Frame{Protocol: "modbus-tcp", DeviceKey: "two", Direction: "rx", Summary: "b"}, []byte{2})
	monitor.Record(Frame{Protocol: "modbus-tcp", DeviceKey: "one", Direction: "rx", Summary: "c"}, []byte{3})
	frames := monitor.Snapshot("one", "modbus-tcp", 10)
	if len(frames) != 1 || frames[0].Summary != "c" || frames[0].Hex != "03" {
		t.Fatalf("unexpected filtered frames: %#v", frames)
	}
}

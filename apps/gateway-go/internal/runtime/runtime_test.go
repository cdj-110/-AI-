package runtime

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestInvalidModbusAreaDoesNotBlockHealthyRegisters(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var registerValue atomic.Uint32
	go serveMixedModbusResponses(listener, &registerValue)

	cfg := config.Config{CollectIntervalSeconds: 1, Points: []config.PointConfig{
		{DeviceKey: "d1", ChannelKey: "c1", Metric: "holding", Protocol: "modbus-tcp", Address: listener.Addr().String(), SlaveID: 1, Function: 3, Register: 0, Quantity: 1, DataType: "uint16", Scale: 1},
		{DeviceKey: "d1", ChannelKey: "c1", Metric: "unsupported", Protocol: "modbus-tcp", Address: listener.Addr().String(), SlaveID: 1, Function: 2, Register: 0, Quantity: 1, DataType: "bool", Scale: 1},
	}}
	store := state.New(cfg)
	manager := NewManager(cfg, store)
	laneKey := collectionLaneKey(cfg.Points[0])
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	manager.CollectLane(ctx, laneKey, true)
	batch := manager.CollectLaneDetailed(ctx, laneKey, true)

	statuses := store.PointStatuses()
	if len(statuses) != 2 || fmt.Sprint(statuses[0].Value) != "2" {
		t.Fatalf("holding point did not continue updating: %#v", statuses)
	}
	if statuses[1].Error == "" {
		t.Fatalf("unsupported point should retain its protocol error: %#v", statuses[1])
	}
	detailed := batch.Points["d1"]["holding"]
	if detailed.ActualDataType != "uint16" || detailed.Quality != "GOOD" || detailed.ReceiveTimestamp.IsZero() || detailed.CollectionMode != "poll" {
		t.Fatalf("non-OPC collector metadata defaults were not applied: %#v", detailed)
	}
}

func TestDeviceReconnectDelayUsesDeviceSetting(t *testing.T) {
	point := config.PointConfig{DeviceKey: "device-1"}
	devices := []config.DeviceConfig{{DeviceKey: "device-1", ReconnectIntervalSeconds: 12}}
	if got := deviceReconnectDelay(point, devices); got != 12*time.Second {
		t.Fatalf("deviceReconnectDelay() = %s, want 12s", got)
	}
	if got := deviceReconnectDelay(point, nil); got != 30*time.Second {
		t.Fatalf("default deviceReconnectDelay() = %s, want 30s", got)
	}
}

func TestOPCDAV11ReconnectBackoff(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, 30 * time.Second}
	for index, expected := range want {
		if got := opcdaReconnectDelay(index + 1); got != expected {
			t.Fatalf("attempt %d delay = %s, want %s", index+1, got, expected)
		}
	}
}

func TestProtocolPointsEqualIgnoresUnrelatedOPCDAHotReload(t *testing.T) {
	previous := []config.PointConfig{
		{DeviceKey: "rtu", Metric: "holding", Protocol: "modbus-rtu", Address: "/dev/ttyS1", Function: 3},
		{DeviceKey: "opc", Metric: "value", Protocol: "opcda", ReadMode: "sync", ItemID: "Random.Real8"},
	}
	next := append([]config.PointConfig(nil), previous...)
	next[1].ReadMode = "subscription"
	if !protocolPointsEqual(previous, next, "modbus-rtu") {
		t.Fatal("OPC DA read mode hot reload must not reset the Modbus RTU session")
	}
	next[0].Address = "/dev/ttyS2"
	if protocolPointsEqual(previous, next, "modbus-rtu") {
		t.Fatal("Modbus RTU connection change must reset the RTU session")
	}
}

func TestOPCDAV11WritablePointRequiresExplicitAccess(t *testing.T) {
	cfg := config.Config{Points: []config.PointConfig{
		{DeviceKey: "opcda-1", Metric: "read-only", Protocol: "opcda", Access: "read"},
		{DeviceKey: "opcda-1", Metric: "command", Protocol: "opcda", Access: "read_write"},
	}}
	if _, ok := findWritablePoint(cfg, "opcda-1", "read-only"); ok {
		t.Fatal("read-only OPC DA point was writable")
	}
	if _, ok := findWritablePoint(cfg, "opcda-1", "command"); !ok {
		t.Fatal("read_write OPC DA point was not writable")
	}
}

func TestOPCDAWriteOnlyPointsAreExcludedFromCollection(t *testing.T) {
	points := []config.PointConfig{
		{DeviceKey: "opcda-1", ChannelKey: "channel-1", Metric: "read", Protocol: "opcda", Access: "read"},
		{DeviceKey: "opcda-1", ChannelKey: "channel-1", Metric: "write", Protocol: "opcda", Access: "write"},
		{DeviceKey: "opcda-1", ChannelKey: "channel-1", Metric: "read-write", Protocol: "opcda", Access: "read_write"},
	}
	cfg := config.Config{Points: points}
	lanes, order := buildCollectionLanes(cfg)
	if len(order) != 1 || len(lanes[order[0]].Points) != 2 {
		t.Fatalf("collection lanes must contain only readable points: order=%v lanes=%+v", order, lanes)
	}
	for _, point := range lanes[order[0]].Points {
		if point.Access == "write" {
			t.Fatalf("write-only point leaked into collection lane: %+v", point)
		}
	}
	results := collectPoints(context.Background(), points)
	if len(results) != 2 {
		t.Fatalf("collectPoints returned %d results, want two readable points: %#v", len(results), results)
	}
	for _, result := range results {
		if result.Index == 1 {
			t.Fatalf("write-only point was collected: %#v", results)
		}
	}
}

func TestSuccessfulWriteClearsDeviceBackoffAndRejectsStaleCollection(t *testing.T) {
	points := []config.PointConfig{
		{DeviceKey: "device-1", Metric: "p1"},
		{DeviceKey: "device-1", Metric: "p2"},
	}
	manager := NewManager(config.Config{Points: points}, state.New(config.Config{Points: points}))
	manager.retryAfter[pointKey(points[0])] = time.Now().Add(time.Minute)
	manager.retryAfter[pointKey(points[1])] = time.Now().Add(time.Minute)
	manager.lastCollected[pointKey(points[0])] = time.Now()
	collectionStartedAt := time.Now()
	time.Sleep(time.Millisecond)
	manager.markWritten(points[0])
	if len(manager.retryAfter) != 0 || len(manager.lastCollected) != 0 {
		t.Fatalf("successful write did not clear device scheduling state: retry=%v collected=%v", manager.retryAfter, manager.lastCollected)
	}
	if !manager.wasWrittenAfter(points[0], collectionStartedAt) {
		t.Fatal("collection started before write was not detected as stale")
	}
}

func serveMixedModbusResponses(listener net.Listener, value *atomic.Uint32) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer connection.Close()
			for {
				header := make([]byte, 7)
				if _, err := io.ReadFull(connection, header); err != nil {
					return
				}
				remaining := int(binary.BigEndian.Uint16(header[4:6])) - 1
				request := make([]byte, remaining)
				if _, err := io.ReadFull(connection, request); err != nil || len(request) == 0 {
					return
				}
				var pdu []byte
				if request[0] == 3 {
					current := uint16(value.Add(1))
					pdu = []byte{3, 2, byte(current >> 8), byte(current)}
				} else {
					pdu = []byte{request[0] | 0x80, 1}
				}
				response := make([]byte, 7+len(pdu))
				copy(response[:4], header[:4])
				binary.BigEndian.PutUint16(response[4:6], uint16(1+len(pdu)))
				response[6] = header[6]
				copy(response[7:], pdu)
				if _, err := connection.Write(response); err != nil {
					return
				}
			}
		}()
	}
}

func TestGroupBatchableRTUPointsBySlave(t *testing.T) {
	points := []config.PointConfig{
		{Protocol: "modbus-rtu", Address: "/dev/ttyS1", SlaveID: 1, Function: 3, Metric: "a"},
		{Protocol: "modbus-rtu", Address: "/dev/ttyS1", SlaveID: 1, Function: 3, Metric: "b"},
		{Protocol: "modbus-rtu", Address: "/dev/ttyS1", SlaveID: 2, Function: 3, Metric: "c"},
	}
	groups, order := groupBatchablePoints(points)
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if len(order) != 2 || order[0] != groupKey(points[0]) || order[1] != groupKey(points[2]) {
		t.Fatalf("group order = %v, want first-seen point order", order)
	}
	if got := len(groups[groupKey(points[0])]); got != 2 {
		t.Fatalf("slave 1 group size = %d, want 2", got)
	}
}

func TestIEC61850CollectionTimeoutScalesWithBatchCount(t *testing.T) {
	points := make([]config.PointConfig, 1568)
	for index := range points {
		points[index] = config.PointConfig{Protocol: "iec61850"}
	}
	timeout := collectTimeoutForPoints(points, time.Second)
	if timeout < 11*time.Second {
		t.Fatalf("IEC61850 timeout %s is too short for 49 MMS batches", timeout)
	}
	if timeout > 30*time.Second {
		t.Fatalf("IEC61850 timeout %s exceeds the safety cap", timeout)
	}
	if got := collectTimeoutForPoints([]config.PointConfig{{Protocol: "modbus-tcp"}}, time.Second); got != 900*time.Millisecond {
		t.Fatalf("non-IEC61850 timeout changed to %s", got)
	}
}

func TestCollectionLanesAreSplitByDeviceAndChannel(t *testing.T) {
	cfg := config.Config{
		CollectIntervalSeconds: 1,
		Points: []config.PointConfig{
			{DeviceKey: "device-a", ChannelKey: "tcp-1", Metric: "a", Protocol: "modbus-tcp"},
			{DeviceKey: "device-a", ChannelKey: "tcp-1", Metric: "b", Protocol: "modbus-tcp"},
			{DeviceKey: "device-b", ChannelKey: "tcp-1", Metric: "c", Protocol: "modbus-tcp", CollectIntervalSeconds: 5},
			{DeviceKey: "device-a", ChannelKey: "tcp-2", Metric: "d", Protocol: "modbus-tcp", CollectIntervalSeconds: 2},
		},
	}
	manager := NewManager(cfg, state.New(cfg))
	lanes := manager.CollectionLanes()
	if len(lanes) != 3 {
		t.Fatalf("len(lanes) = %d, want 3", len(lanes))
	}
	if lanes[0].DeviceKey != "device-a" || lanes[0].Channel != "tcp-1" || len(lanes[0].Points) != 2 || lanes[0].Interval != time.Second {
		t.Fatalf("lane[0] = %+v, want device-a tcp-1 with 2 points and 1s interval", lanes[0])
	}
	if lanes[1].DeviceKey != "device-b" || lanes[1].Channel != "tcp-1" || lanes[1].Interval != 5*time.Second {
		t.Fatalf("lane[1] = %+v, want device-b tcp-1 with 5s interval", lanes[1])
	}
	if lanes[2].DeviceKey != "device-a" || lanes[2].Channel != "tcp-2" || lanes[2].Interval != 2*time.Second {
		t.Fatalf("lane[2] = %+v, want device-a tcp-2 with 2s interval", lanes[2])
	}
}

func TestProtectedLargeLaneInterval(t *testing.T) {
	if got := protectedLargeLaneInterval(50000, 250*time.Millisecond); got != 250*time.Millisecond {
		t.Fatalf("small lane interval = %s", got)
	}
	if got := protectedLargeLaneInterval(300000, 250*time.Millisecond); got != 6*time.Second {
		t.Fatalf("300k lane interval = %s, want 6s", got)
	}
	if got := protectedLargeLaneInterval(300000, 10*time.Second); got != 10*time.Second {
		t.Fatalf("slower configured interval = %s", got)
	}
}

func TestOPCDABatchCollectorReturnsOneResultPerPointWithoutNativeBuild(t *testing.T) {
	points := []config.PointConfig{
		{DeviceKey: "opc-1", Metric: "a", Protocol: "opcda", Address: "192.168.1.100", CLSID: "clsid", ItemID: "Channel.A"},
		{DeviceKey: "opc-1", Metric: "b", Protocol: "opcda", Address: "192.168.1.100", CLSID: "clsid", ItemID: "Channel.B"},
	}
	results := collectPoints(context.Background(), points)
	if len(results) != len(points) {
		t.Fatalf("len(results) = %d, want %d: %#v", len(results), len(points), results)
	}
	for _, result := range results {
		if result.Err == nil {
			t.Fatalf("default build should report native OPC DA capability error: %#v", result)
		}
	}
}

func TestFindWritablePointOnlyReturnsModbusOutputAreas(t *testing.T) {
	disabled := false
	cfg := config.Config{Points: []config.PointConfig{
		{DeviceKey: "device-a", Metric: "coil", Protocol: "modbus-tcp", Function: 1},
		{DeviceKey: "device-a", Metric: "discrete", Protocol: "modbus-tcp", Function: 2},
		{DeviceKey: "device-a", Metric: "holding", Protocol: "modbus-rtu", Function: 3},
		{DeviceKey: "device-a", Metric: "input", Protocol: "modbus-rtu", Function: 4},
		{DeviceKey: "device-a", Metric: "disabled", Protocol: "modbus-rtu", Function: 3, Enabled: &disabled},
	}}
	for _, metric := range []string{"coil", "holding"} {
		if point, ok := findWritablePoint(cfg, "device-a", metric); !ok || point.Metric != metric {
			t.Fatalf("findWritablePoint(%q) = %+v, %v", metric, point, ok)
		}
	}
	for _, metric := range []string{"discrete", "input", "disabled"} {
		if point, ok := findWritablePoint(cfg, "device-a", metric); ok {
			t.Fatalf("findWritablePoint(%q) unexpectedly returned %+v", metric, point)
		}
	}
	if _, ok := findWritablePoint(cfg, "device-b", "coil"); ok {
		t.Fatal("device key must be used to disambiguate writable points")
	}
}

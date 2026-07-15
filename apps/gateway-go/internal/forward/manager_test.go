package forward

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

type recordedWrite struct {
	deviceKey string
	metric    string
	value     interface{}
}

type recordingPointWriter struct {
	writes []recordedWrite
	err    error
}

func (w *recordingPointWriter) WritePoint(_ context.Context, deviceKey, metric string, value interface{}) (config.PointConfig, error) {
	w.writes = append(w.writes, recordedWrite{deviceKey: deviceKey, metric: metric, value: value})
	return config.PointConfig{}, w.err
}

func TestModbusResponseUsesForwardedPointValue(t *testing.T) {
	cfg, store := forwardTestConfig()
	manager := NewManager(store)
	manager.cfg = cfg

	response := manager.modbusResponse([]byte{3, 0, 10, 0, 2})
	if len(response) != 6 || response[0] != 3 || response[1] != 4 {
		t.Fatalf("unexpected response: %v", response)
	}
	got := math.Float32frombits(binary.BigEndian.Uint32(response[2:6]))
	if got != 12.5 {
		t.Fatalf("register value = %v, want 12.5", got)
	}
}

func TestIEC104InterrogationReturnsForwardedValue(t *testing.T) {
	cfg, store := forwardTestConfig()
	manager := NewManager(store)
	manager.cfg = cfg

	server, client := net.Pipe()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.handleIEC104Conn(context.Background(), server)
	}()

	if _, err := client.Write([]byte{0x68, 0x04, 0x07, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	readIEC104TestPacket(t, client)
	asdu := []byte{100, 1, 6, 0, 1, 0, 0, 0, 0, 20}
	packet := make([]byte, 6+len(asdu))
	packet[0] = 0x68
	packet[1] = byte(4 + len(asdu))
	copy(packet[6:], asdu)
	if _, err := client.Write(packet); err != nil {
		t.Fatal(err)
	}
	readIEC104TestPacket(t, client)
	valuePacket := readIEC104TestPacket(t, client)
	if len(valuePacket) < 20 || valuePacket[6] != 13 {
		t.Fatalf("unexpected IEC104 value packet: %v", valuePacket)
	}
	value := math.Float32frombits(binary.LittleEndian.Uint32(valuePacket[15:19]))
	if value != 12.5 {
		t.Fatalf("IEC104 value = %v, want 12.5", value)
	}
	_ = client.Close()
	<-done
}

func TestIEC104PushesChangedValueSpontaneously(t *testing.T) {
	cfg, store := forwardTestConfig()
	manager := NewManager(store)
	manager.cfg = cfg

	server, client := net.Pipe()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.handleIEC104Conn(context.Background(), server)
	}()

	if _, err := client.Write([]byte{0x68, 0x04, 0x07, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	readIEC104TestPacket(t, client)
	asdu := []byte{100, 1, 6, 0, 1, 0, 0, 0, 0, 20}
	packet := make([]byte, 6+len(asdu))
	packet[0] = 0x68
	packet[1] = byte(4 + len(asdu))
	copy(packet[6:], asdu)
	if _, err := client.Write(packet); err != nil {
		t.Fatal(err)
	}
	readIEC104TestPacket(t, client) // interrogation activation confirmation
	readIEC104TestPacket(t, client) // current value
	readIEC104TestPacket(t, client) // interrogation termination

	store.SetPointValue("d2", "p2", 27.75)
	spontaneous := readIEC104TestPacket(t, client)
	if len(spontaneous) < 20 || spontaneous[6] != 13 || spontaneous[8] != 3 {
		t.Fatalf("unexpected spontaneous IEC104 packet: %v", spontaneous)
	}
	if value := math.Float32frombits(binary.LittleEndian.Uint32(spontaneous[15:19])); value != 27.75 {
		t.Fatalf("spontaneous value = %v, want 27.75", value)
	}
	_ = client.Close()
	<-done
}

func TestIEC104BooleanPointUsesSinglePointASDU(t *testing.T) {
	cfg, store := forwardTestConfig()
	cfg.ForwardDevices = []config.ForwardDeviceConfig{{
		DeviceKey: "iec-bool", Protocol: ProtocolIEC104Server, CommonAddress: 1,
		Points: []config.ForwardPointConfig{{SourceDeviceKey: "d1", SourceMetric: "p1", IOA: 10, DataType: "bool"}},
	}}
	store.SetPointValue("d1", "p1", true)
	manager := NewManager(store)
	manager.cfg = cfg

	server, client := net.Pipe()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.handleIEC104Conn(context.Background(), server)
	}()
	if _, err := client.Write([]byte{0x68, 0x04, 0x07, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	readIEC104TestPacket(t, client)
	asdu := []byte{100, 1, 6, 0, 1, 0, 0, 0, 0, 20}
	packet := make([]byte, 6+len(asdu))
	packet[0], packet[1] = 0x68, byte(4+len(asdu))
	copy(packet[6:], asdu)
	if _, err := client.Write(packet); err != nil {
		t.Fatal(err)
	}
	readIEC104TestPacket(t, client)
	valuePacket := readIEC104TestPacket(t, client)
	if len(valuePacket) < 16 || valuePacket[6] != 1 || valuePacket[15] != 1 {
		t.Fatalf("unexpected single point packet: %v", valuePacket)
	}
	_ = client.Close()
	<-done
}

func TestForwardDeviceMapsOnlySelectedPointAndUnit(t *testing.T) {
	cfg, store := forwardTestConfig()
	cfg.ForwardDevices = []config.ForwardDeviceConfig{{
		DeviceKey: "slave-7", Protocol: ProtocolModbusSlave, UnitID: 7,
		Points: []config.ForwardPointConfig{{SourceDeviceKey: "d1", SourceMetric: "p1", Function: 3, Register: 20, DataType: "float32"}},
	}}
	manager := NewManager(store)
	manager.cfg = cfg

	wrongUnit := manager.modbusResponseForUnit(8, []byte{3, 0, 20, 0, 2})
	if got := math.Float32frombits(binary.BigEndian.Uint32(wrongUnit[2:6])); got != 0 {
		t.Fatalf("wrong unit returned %v, want 0", got)
	}
	response := manager.modbusResponseForUnit(7, []byte{3, 0, 20, 0, 2})
	if got := math.Float32frombits(binary.BigEndian.Uint32(response[2:6])); got != 12.5 {
		t.Fatalf("mapped value = %v, want 12.5", got)
	}
	if response := manager.modbusResponseForUnit(7, []byte{4, 0, 20, 0, 2}); math.Float32frombits(binary.BigEndian.Uint32(response[2:6])) != 0 {
		t.Fatal("function 3 mapping leaked into function 4")
	}
}

func TestModbusForwardWritesCoilAndSingleRegisterToSourcePoints(t *testing.T) {
	cfg, store := modbusWriteTestConfig()
	writer := &recordingPointWriter{}
	manager := NewManager(store, writer)
	manager.cfg = cfg

	coilRequest := []byte{5, 0, 5, 0xff, 0}
	if response := manager.modbusResponseForUnit(7, coilRequest); string(response) != string(coilRequest) {
		t.Fatalf("coil response = %v, want echo %v", response, coilRequest)
	}
	registerRequest := []byte{6, 0, 10, 0, 42}
	if response := manager.modbusResponseForUnit(7, registerRequest); string(response) != string(registerRequest) {
		t.Fatalf("register response = %v, want echo %v", response, registerRequest)
	}
	if len(writer.writes) != 2 {
		t.Fatalf("writes = %d, want 2", len(writer.writes))
	}
	if got := writer.writes[0]; got.deviceKey != "source-device" || got.metric != "coil" || got.value != true {
		t.Fatalf("coil write = %#v", got)
	}
	if got := writer.writes[1]; got.deviceKey != "source-device" || got.metric != "holding" || got.value != float64(42) {
		t.Fatalf("register write = %#v", got)
	}
}

func TestModbusForwardWritesMultipleRegisterValue(t *testing.T) {
	cfg, store := modbusWriteTestConfig()
	writer := &recordingPointWriter{}
	manager := NewManager(store, writer)
	manager.cfg = cfg
	raw := math.Float32bits(12.5)
	request := []byte{16, 0, 20, 0, 2, 4, byte(raw >> 24), byte(raw >> 16), byte(raw >> 8), byte(raw)}

	response := manager.modbusResponseForUnit(7, request)
	if want := []byte{16, 0, 20, 0, 2}; string(response) != string(want) {
		t.Fatalf("response = %v, want %v", response, want)
	}
	if len(writer.writes) != 1 || writer.writes[0].metric != "float" || writer.writes[0].value != float64(12.5) {
		t.Fatalf("writes = %#v", writer.writes)
	}
}

func TestModbusForwardRejectsReadOnlyAndReportsDownstreamFailure(t *testing.T) {
	cfg, store := modbusWriteTestConfig()
	writer := &recordingPointWriter{}
	manager := NewManager(store, writer)
	manager.cfg = cfg

	if response := manager.modbusResponseForUnit(7, []byte{6, 0, 30, 0, 1}); len(response) != 2 || response[0] != 0x86 || response[1] != 0x02 {
		t.Fatalf("read-only response = %v, want illegal data address", response)
	}
	writer.err = errors.New("device offline")
	if response := manager.modbusResponseForUnit(7, []byte{6, 0, 10, 0, 1}); len(response) != 2 || response[0] != 0x86 || response[1] != 0x04 {
		t.Fatalf("failed write response = %v, want slave device failure", response)
	}
}

func modbusWriteTestConfig() (config.Config, *state.Store) {
	cfg := config.Config{
		ForwardSlave: config.FeatureConfig{Enabled: true},
		ForwardDevices: []config.ForwardDeviceConfig{{
			DeviceKey: "modbus-forward", Protocol: ProtocolModbusSlave, UnitID: 7,
			Points: []config.ForwardPointConfig{
				{SourceDeviceKey: "source-device", SourceMetric: "coil", Function: 1, Register: 5, Quantity: 1, DataType: "bool", Scale: 1},
				{SourceDeviceKey: "source-device", SourceMetric: "holding", Function: 3, Register: 10, Quantity: 1, DataType: "uint16", Scale: 1},
				{SourceDeviceKey: "source-device", SourceMetric: "float", Function: 3, Register: 20, Quantity: 2, DataType: "float32", Scale: 1},
				{SourceDeviceKey: "source-device", SourceMetric: "input", Function: 4, Register: 30, Quantity: 1, DataType: "uint16", Scale: 1},
			},
		}},
	}
	return cfg, state.New(cfg)
}

func forwardTestConfig() (config.Config, *state.Store) {
	cfg := config.Config{
		GatewayKey: "gw",
		ForwardSlave: config.FeatureConfig{
			Enabled: true,
		},
		Channels: []config.ChannelConfig{{
			ChannelKey:      "ch1",
			Protocol:        "iec104",
			ForwardProtocol: ProtocolModbusSlave,
		}, {
			ChannelKey:      "ch2",
			Protocol:        "modbus-tcp",
			ForwardProtocol: ProtocolIEC104Server,
		}},
		Points: []config.PointConfig{{
			DeviceKey:  "d1",
			ChannelKey: "ch1",
			Metric:     "p1",
			Register:   10,
			DataType:   "float32",
		}, {
			DeviceKey:  "d2",
			ChannelKey: "ch2",
			Metric:     "p2",
			Register:   11,
			DataType:   "float32",
		}},
	}
	store := state.New(cfg)
	store.SetPointValue("d1", "p1", 12.5)
	store.SetPointValue("d2", "p2", 12.5)
	return cfg, store
}

func readIEC104TestPacket(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 2)
	if _, err := conn.Read(header); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, int(header[1]))
	if _, err := conn.Read(body); err != nil {
		t.Fatal(err)
	}
	return append(header, body...)
}

package forward

import (
	"context"
	"encoding/binary"
	"math"
	"net"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

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

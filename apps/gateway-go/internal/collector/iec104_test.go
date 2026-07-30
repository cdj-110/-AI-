package collector

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestIEC104ConcurrentPointsReuseOneConnection(t *testing.T) {
	CloseIEC104Connections()
	t.Cleanup(CloseIEC104Connections)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	var accepted atomic.Int32
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted.Add(1)
			go func() {
				defer conn.Close()
				start := make([]byte, 6)
				if _, readErr := io.ReadFull(conn, start); readErr != nil {
					return
				}
				if _, writeErr := conn.Write([]byte{0x68, 0x04, 0x0b, 0x00, 0x00, 0x00}); writeErr != nil {
					return
				}
				_, _ = io.Copy(io.Discard, conn)
			}()
		}
	}()

	point := config.PointConfig{Address: listener.Addr().String(), CommonAddress: 1}
	const readers = 8
	clients := make([]*iec104Client, readers)
	errs := make([]error, readers)
	ready := make(chan struct{})
	var wg sync.WaitGroup
	for i := range readers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-ready
			clients[index], errs[index] = getIEC104Client(context.Background(), point)
		}(i)
	}
	close(ready)
	wg.Wait()

	for i, connectErr := range errs {
		if connectErr != nil {
			t.Fatalf("reader %d connect: %v", i, connectErr)
		}
		if clients[i] != clients[0] {
			t.Fatalf("reader %d received a different IEC104 client", i)
		}
	}
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted connections = %d, want 1", got)
	}
}

func TestIEC104ValueAfterRejectsCachedValue(t *testing.T) {
	ioa := uint32(16385)
	oldUpdate := time.Now().Add(-time.Second)
	client := &iec104Client{
		values:       map[uint32]interface{}{ioa: float32(61)},
		valueUpdated: map[uint32]time.Time{ioa: oldUpdate},
	}

	if _, ok := client.valueAfter(ioa, oldUpdate); ok {
		t.Fatal("cached value was accepted as a fresh interrogation result")
	}
	client.valueUpdated[ioa] = oldUpdate.Add(time.Millisecond)
	if value, ok := client.valueAfter(ioa, oldUpdate); !ok || value != float32(61) {
		t.Fatalf("fresh value = %v, %v", value, ok)
	}
}

func TestIEC104AutoModeDetectsShortInterrogationSession(t *testing.T) {
	client := &iec104Client{
		options:                  config.IEC104Config{AcquisitionMode: "auto"},
		connectedAt:              time.Now(),
		initialInterrogationSent: true,
		updated:                  time.Now(),
	}
	if !client.shouldPreferSpontaneous(io.EOF) {
		t.Fatal("automatic mode did not switch to spontaneous receive after a short interrogation session")
	}
	client.options.AcquisitionMode = "periodic"
	if client.shouldPreferSpontaneous(io.EOF) {
		t.Fatal("periodic mode must not switch to spontaneous-only receive")
	}
	client.options.AcquisitionMode = "auto"
	client.skipInitialInterrogation = true
	if client.shouldPreferSpontaneous(io.EOF) {
		t.Fatal("a spontaneous receive session must not retrigger automatic mode detection")
	}
	client.updated = time.Time{}
	if !client.shouldRestoreInterrogation(io.EOF) {
		t.Fatal("a short spontaneous session without point data must restore interrogation")
	}
}

func TestIEC104AutoModeFallsBackToSpontaneousLongConnection(t *testing.T) {
	CloseIEC104Connections()
	t.Cleanup(CloseIEC104Connections)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	serverErr := make(chan error, 1)
	go func() {
		for connectionIndex := 0; connectionIndex < 2; connectionIndex++ {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				serverErr <- acceptErr
				return
			}
			start, readErr := readIEC104Packet(conn)
			if readErr != nil || len(start) < 3 || start[2] != 0x07 {
				serverErr <- fmt.Errorf("connection %d STARTDT: %x, %v", connectionIndex, start, readErr)
				_ = conn.Close()
				return
			}
			if _, writeErr := conn.Write([]byte{0x68, 0x04, 0x0b, 0, 0, 0}); writeErr != nil {
				serverErr <- writeErr
				_ = conn.Close()
				return
			}
			if connectionIndex == 0 {
				command, commandErr := readIEC104Packet(conn)
				if commandErr != nil || len(command) < 7 || command[6] != 100 {
					serverErr <- fmt.Errorf("initial interrogation: %x, %v", command, commandErr)
					_ = conn.Close()
					return
				}
				activation := []byte{0x68, 0x0e, 0, 0, 2, 0, 100, 1, 7, 0, 1, 0, 0, 0, 0, 20}
				measurement := testIEC104FloatPacket(20, 41.5)
				measurement[2] = 2
				termination := []byte{0x68, 0x0e, 4, 0, 2, 0, 100, 1, 10, 0, 1, 0, 0, 0, 0, 20}
				response := append(append(activation, measurement...), termination...)
				_, _ = conn.Write(response)
				_, _ = readIEC104Packet(conn)
				_ = conn.Close()
				continue
			}
			_ = conn.SetReadDeadline(time.Now().Add(120 * time.Millisecond))
			if command, commandErr := readIEC104Packet(conn); commandErr == nil {
				serverErr <- fmt.Errorf("automatic fallback unexpectedly sent command: %x", command)
				_ = conn.Close()
				return
			}
			_ = conn.SetReadDeadline(time.Time{})
			_, _ = conn.Write(testIEC104FloatPacket(3, 42.5))
			time.Sleep(100 * time.Millisecond)
			_ = conn.Close()
			serverErr <- nil
			return
		}
	}()

	options := config.IEC104Config{Configured: true, AcquisitionMode: "auto", GeneralInterrogationOnStart: true}
	point := config.PointConfig{DeviceKey: "auto-device", Metric: "value", Protocol: "iec104", Address: listener.Addr().String(), CommonAddress: 1, IOA: 0x4001, IEC104: &options}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	first, err := (IEC104{}).ReadPoint(ctx, point)
	cancel()
	if err != nil || first.Value != float64(41.5) {
		t.Fatalf("initial interrogation value = %#v, %v", first, err)
	}
	key := fmt.Sprintf("%s#%d", listener.Addr().String(), 1)
	deadline := time.Now().Add(time.Second)
	for {
		iec104Pool.Lock()
		passive := iec104Pool.preferSpontaneous[key]
		_, connected := iec104Pool.items[key]
		iec104Pool.Unlock()
		if passive && !connected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("automatic mode did not remember the short interrogation session")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	second, err := (IEC104{}).ReadPoint(ctx, point)
	cancel()
	if err != nil || second.Value != float64(42.5) {
		t.Fatalf("spontaneous value = %#v, %v", second, err)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func testIEC104FloatPacket(cause uint16, value float32) []byte {
	packet := []byte{0x68, 0x12, 0, 0, 0, 0, 13, 1, byte(cause), byte(cause >> 8), 1, 0, 1, 0x40, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(packet[15:19], math.Float32bits(value))
	return packet
}

func TestIEC104AddressFieldsPreferExpandedValues(t *testing.T) {
	point := config.PointConfig{SlaveID: 7, Register: 8, CommonAddress: 1024, IOA: 70000}
	if got := iec104CommonAddress(point); got != 1024 {
		t.Fatalf("common address = %d, want 1024", got)
	}
	if got := iec104PointIOA(point); got != 70000 {
		t.Fatalf("IOA = %d, want 70000", got)
	}
	point.CommonAddress = 0
	point.IOA = 0
	if got := iec104CommonAddress(point); got != 7 {
		t.Fatalf("legacy common address = %d, want 7", got)
	}
	if got := iec104PointIOA(point); got != 8 {
		t.Fatalf("legacy IOA = %d, want 8", got)
	}
}

func TestIEC104ConfiguredCommandsUseExpectedTypeIDs(t *testing.T) {
	tests := []struct {
		name       string
		invoke     func(*iec104Client) error
		wantTypeID byte
		wantLast   byte
	}{
		{name: "general interrogation", invoke: func(c *iec104Client) error { return c.interrogate() }, wantTypeID: 100, wantLast: 20},
		{name: "counter interrogation", invoke: func(c *iec104Client) error { return c.counterInterrogate() }, wantTypeID: 101, wantLast: 5},
		{name: "clock sync", invoke: func(c *iec104Client) error {
			return c.clockSync(time.Date(2026, 7, 13, 10, 11, 12, 13000000, time.UTC))
		}, wantTypeID: 103, wantLast: 26},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer clientConn.Close()
			defer serverConn.Close()
			client := &iec104Client{conn: clientConn, commonAS: 1024}
			errCh := make(chan error, 1)
			go func() { errCh <- tt.invoke(client) }()
			packet := make([]byte, 64)
			n, err := serverConn.Read(packet)
			if err != nil {
				t.Fatalf("read command: %v", err)
			}
			if err := <-errCh; err != nil {
				t.Fatalf("send command: %v", err)
			}
			packet = packet[:n]
			if len(packet) < 12 || packet[6] != tt.wantTypeID {
				t.Fatalf("packet type = %v, want %d", packet, tt.wantTypeID)
			}
			if packet[10] != 0x00 || packet[11] != 0x04 {
				t.Fatalf("common address bytes = %02x %02x, want 00 04", packet[10], packet[11])
			}
			if packet[len(packet)-1] != tt.wantLast {
				t.Fatalf("last byte = %d, want %d", packet[len(packet)-1], tt.wantLast)
			}
		})
	}
}

func TestIEC104RepliesToTestFrameActivation(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := &iec104Client{conn: clientConn}
	done := make(chan struct{})
	go func() {
		client.handlePacket([]byte{0x68, 0x04, 0x43, 0x00, 0x00, 0x00})
		close(done)
	}()
	response := make([]byte, 6)
	if _, err := io.ReadFull(serverConn, response); err != nil {
		t.Fatal(err)
	}
	<-done
	want := []byte{0x68, 0x04, 0x83, 0x00, 0x00, 0x00}
	for index := range want {
		if response[index] != want[index] {
			t.Fatalf("test frame response = %x, want %x", response, want)
		}
	}
}

func TestIEC104TestFrameSummaries(t *testing.T) {
	if got := iec104PacketSummary([]byte{0x68, 0x04, 0x43, 0, 0, 0}); got != "TESTFR 激活" {
		t.Fatalf("activation summary = %q", got)
	}
	if got := iec104PacketSummary([]byte{0x68, 0x04, 0x83, 0, 0, 0}); got != "TESTFR 确认" {
		t.Fatalf("confirmation summary = %q", got)
	}
}

func TestIEC104PacketSummaryIncludesTypeAndCause(t *testing.T) {
	packet := []byte{0x68, 0x12, 0x00, 0x00, 0x00, 0x00, 13, 1, 3, 0}
	if got := iec104PacketSummary(packet); got != "I帧 短浮点数(M_ME_NC_1) 对象数=1 原因=自发(3)" {
		t.Fatalf("packet summary = %q", got)
	}
}

package collector

import (
	"context"
	"io"
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

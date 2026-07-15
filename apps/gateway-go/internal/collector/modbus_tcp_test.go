package collector

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestTCPConnectionLockHonorsCollectionDeadline(t *testing.T) {
	connection := &tcpConnection{gate: newPriorityConnectionLock()}
	if err := connection.lockRead(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer connection.unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := connection.lockRead(ctx); err == nil {
		t.Fatal("lock unexpectedly succeeded")
	}
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("lock cancellation took %s", elapsed)
	}
}

func TestReadLockTimeoutIsReportedAsDeferredCollection(t *testing.T) {
	connection := &tcpConnection{gate: newPriorityConnectionLock()}
	if err := connection.lockWrite(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer connection.unlock()
	tcpPool.Lock()
	previous := tcpPool.items["deferred-test#1"]
	tcpPool.items["deferred-test#1"] = connection
	tcpPool.Unlock()
	defer func() {
		tcpPool.Lock()
		defer tcpPool.Unlock()
		if previous == nil {
			delete(tcpPool.items, "deferred-test#1")
		} else {
			tcpPool.items["deferred-test#1"] = previous
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := (ModbusTCP{}).ReadPoint(ctx, config.PointConfig{Address: "deferred-test", SlaveID: 1})
	if !errors.Is(err, ErrCollectionDeferred) {
		t.Fatalf("ReadPoint() error = %v, want ErrCollectionDeferred", err)
	}
}

func TestTCPConnectionPrioritizesWaitingWrite(t *testing.T) {
	connection := &tcpConnection{gate: newPriorityConnectionLock()}
	if err := connection.lockRead(context.Background()); err != nil {
		t.Fatal(err)
	}
	order := make(chan string, 2)
	go func() {
		if err := connection.lockWrite(context.Background()); err == nil {
			order <- "write"
			connection.unlock()
		}
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		if err := connection.lockRead(context.Background()); err == nil {
			order <- "read"
			connection.unlock()
		}
	}()
	connection.unlock()
	if first := <-order; first != "write" {
		t.Fatalf("first lock owner = %q, want write", first)
	}
	if second := <-order; second != "read" {
		t.Fatalf("second lock owner = %q, want read", second)
	}
}

func TestStaleTCPErrorDoesNotCloseReplacementConnection(t *testing.T) {
	point := config.PointConfig{Address: "127.0.0.1:502", SlaveID: 7}
	stale := &tcpConnection{}
	replacement := &tcpConnection{}
	key := tcpConnectionKey(point)
	tcpPool.Lock()
	previous := tcpPool.items[key]
	tcpPool.items[key] = replacement
	tcpPool.Unlock()
	t.Cleanup(func() {
		tcpPool.Lock()
		if previous == nil {
			delete(tcpPool.items, key)
		} else {
			tcpPool.items[key] = previous
		}
		tcpPool.Unlock()
	})

	closeTCPConnection(point, stale)
	tcpPool.Lock()
	current := tcpPool.items[key]
	tcpPool.Unlock()
	if current != replacement {
		t.Fatal("stale request removed the healthy replacement connection")
	}
}

func TestModbusTCPUsesSeparateCollectionAndControlConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 2)
	go func() {
		for i := 0; i < 2; i++ {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted <- connection
		}
	}()
	point := config.PointConfig{Address: listener.Addr().String(), SlaveID: 1}
	collectionConnection, err := getTCPConnection(point)
	if err != nil {
		t.Fatal(err)
	}
	controlConnection, err := getTCPWriteConnection(point)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTCPConnection(point, collectionConnection)
	defer closeTCPWriteConnection(point, controlConnection)
	if collectionConnection == controlConnection {
		t.Fatal("collection and control unexpectedly share one Modbus TCP connection")
	}
	for i := 0; i < 2; i++ {
		select {
		case connection := <-accepted:
			defer connection.Close()
		case <-time.After(time.Second):
			t.Fatal("server did not receive both TCP connections")
		}
	}
}

type recordingModbusClient struct {
	method   string
	address  uint16
	quantity uint16
	value    uint16
	raw      []byte
}

func (c *recordingModbusClient) recordRead(method string, address, quantity uint16) ([]byte, error) {
	c.method, c.address, c.quantity = method, address, quantity
	return []byte{0, 1}, nil
}

func (c *recordingModbusClient) ReadCoils(address, quantity uint16) ([]byte, error) {
	return c.recordRead("read-coils-01", address, quantity)
}
func (c *recordingModbusClient) ReadDiscreteInputs(address, quantity uint16) ([]byte, error) {
	return c.recordRead("read-discrete-inputs-02", address, quantity)
}
func (c *recordingModbusClient) ReadHoldingRegisters(address, quantity uint16) ([]byte, error) {
	return c.recordRead("read-holding-registers-03", address, quantity)
}
func (c *recordingModbusClient) ReadInputRegisters(address, quantity uint16) ([]byte, error) {
	return c.recordRead("read-input-registers-04", address, quantity)
}
func (c *recordingModbusClient) WriteSingleCoil(address, value uint16) ([]byte, error) {
	c.method, c.address, c.value = "write-single-coil-05", address, value
	return nil, nil
}
func (c *recordingModbusClient) WriteMultipleCoils(address, quantity uint16, value []byte) ([]byte, error) {
	c.method = "write-multiple-coils-15"
	return nil, nil
}
func (c *recordingModbusClient) WriteSingleRegister(address, value uint16) ([]byte, error) {
	c.method, c.address, c.value = "write-single-register-06", address, value
	return nil, nil
}
func (c *recordingModbusClient) WriteMultipleRegisters(address, quantity uint16, value []byte) ([]byte, error) {
	c.method, c.address, c.quantity, c.raw = "write-multiple-registers-16", address, quantity, append([]byte(nil), value...)
	return nil, nil
}
func (c *recordingModbusClient) ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity uint16, value []byte) ([]byte, error) {
	c.method = "read-write-multiple-registers-23"
	return nil, nil
}
func (c *recordingModbusClient) MaskWriteRegister(address, andMask, orMask uint16) ([]byte, error) {
	c.method = "mask-write-register-22"
	return nil, nil
}
func (c *recordingModbusClient) ReadFIFOQueue(address uint16) ([]byte, error) {
	c.method = "read-fifo-24"
	return nil, nil
}

func TestReadByFunctionUsesConfiguredDataArea(t *testing.T) {
	tests := []struct {
		function uint8
		method   string
	}{
		{1, "read-coils-01"},
		{2, "read-discrete-inputs-02"},
		{3, "read-holding-registers-03"},
		{4, "read-input-registers-04"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			client := &recordingModbusClient{}
			point := config.PointConfig{Function: tt.function, Register: 12, Quantity: 3}
			if _, err := readByFunction(context.Background(), client, point); err != nil {
				t.Fatalf("readByFunction() error = %v", err)
			}
			if client.method != tt.method || client.address != 12 || client.quantity != 3 {
				t.Fatalf("read call = %s(%d, %d), want %s(12, 3)", client.method, client.address, client.quantity, tt.method)
			}
		})
	}
}

func TestWriteByFunctionAutomaticallySelectsWriteFunction(t *testing.T) {
	tests := []struct {
		name       string
		point      config.PointConfig
		value      interface{}
		wantMethod string
		wantQty    uint16
		wantValue  uint16
	}{
		{
			name:       "coil uses function 05",
			point:      config.PointConfig{Function: 1, Register: 8, DataType: "bool", Scale: 1},
			value:      true,
			wantMethod: "write-single-coil-05",
			wantValue:  0xff00,
		},
		{
			name:       "one register uses function 06",
			point:      config.PointConfig{Function: 3, Register: 9, DataType: "uint16", Scale: 1},
			value:      42,
			wantMethod: "write-single-register-06",
			wantValue:  42,
		},
		{
			name:       "multiple registers use function 16",
			point:      config.PointConfig{Function: 3, Register: 10, DataType: "float32", Scale: 1},
			value:      12.5,
			wantMethod: "write-multiple-registers-16",
			wantQty:    2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &recordingModbusClient{}
			if err := writeByFunction(context.Background(), client, tt.point, tt.value); err != nil {
				t.Fatalf("writeByFunction() error = %v", err)
			}
			if client.method != tt.wantMethod {
				t.Fatalf("method = %q, want %q", client.method, tt.wantMethod)
			}
			if client.address != tt.point.Register || client.quantity != tt.wantQty || client.value != tt.wantValue {
				t.Fatalf("write args address=%d quantity=%d value=%#x", client.address, client.quantity, client.value)
			}
		})
	}
}

func TestWriteByFunctionRejectsReadOnlyDataAreas(t *testing.T) {
	for _, function := range []uint8{2, 4} {
		client := &recordingModbusClient{}
		point := config.PointConfig{Metric: "readonly", Function: function, DataType: "uint16", Scale: 1}
		err := writeByFunction(context.Background(), client, point, 1)
		if err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("function %d error = %v, want read-only error", function, err)
		}
		if client.method != "" {
			t.Fatalf("function %d unexpectedly called %s", function, client.method)
		}
	}
}

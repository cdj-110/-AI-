package collector

import (
	"context"
	"errors"
	"github.com/goburrow/modbus"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type rtuProbeClient struct {
	modbus.Client
	reads              int
	register, quantity uint16
	err                error
}

func (c *rtuProbeClient) ReadHoldingRegisters(register, quantity uint16) ([]byte, error) {
	c.reads++
	c.register = register
	c.quantity = quantity
	return []byte{0, 42}, c.err
}

func TestRTUConnectionProbeReadsOnSharedBus(t *testing.T) {
	point := config.PointConfig{Protocol: "modbus-rtu", Address: "rtu-probe-test", SlaveID: 2, Function: 3, Register: 17}
	client := &rtuProbeClient{}
	bus := &rtuBus{handler: modbus.NewRTUClientHandler(point.Address), client: client, settings: rtuSettingsKey(point)}
	rtuPool.Lock()
	rtuPool.items[point.Address] = bus
	rtuPool.Unlock()
	t.Cleanup(func() { rtuPool.Lock(); delete(rtuPool.items, point.Address); rtuPool.Unlock() })
	result, err := TestModbusRTUConnection(context.Background(), point)
	if err != nil || result.SlaveID != 2 || client.reads != 1 || client.register != 17 || client.quantity != 1 || bus.handler.SlaveId != 2 {
		t.Fatalf("bad RTU probe: %+v %v %+v", result, err, client)
	}
	bus.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = TestModbusRTUConnection(ctx, point)
	bus.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || client.reads != 1 {
		t.Fatalf("busy bus ignored cancellation: %v", err)
	}
	client.err = errors.New("i/o timeout")
	_, err = TestModbusRTUConnection(context.Background(), point)
	if err == nil || strings.Contains(err.Error(), "Modbus TCP") {
		t.Fatalf("serial timeout misreported: %v", err)
	}
}

func TestRTUProbeRejectsBroadcastAndWriteFunctions(t *testing.T) {
	for _, point := range []config.PointConfig{{Address: "test", SlaveID: 0}, {Address: "test", SlaveID: 248}, {Address: "test", SlaveID: 1, Function: 6}, {SlaveID: 1}} {
		if _, err := TestModbusRTUConnection(context.Background(), point); err == nil {
			t.Fatalf("unsafe or invalid probe accepted: %+v", point)
		}
	}
}

func TestNormalizeParity(t *testing.T) {
	tests := map[string]string{
		"none": "N",
		"N":    "N",
		"even": "E",
		"E":    "E",
		"odd":  "O",
		"O":    "O",
	}
	for input, want := range tests {
		if got := normalizeParity(input); got != want {
			t.Errorf("normalizeParity(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRTUSettingsKeyUsesSerialFraming(t *testing.T) {
	point := config.PointConfig{Protocol: "modbus-rtu", Address: "/dev/ttyS1"}
	if got := rtuSettingsKey(point); got != "9600-8-1-N" {
		t.Fatalf("rtuSettingsKey() = %q", got)
	}
	point.SlaveID = 7
	if got := rtuSettingsKey(point); got != "9600-8-1-N" {
		t.Fatalf("slave ID must not split a physical bus, got %q", got)
	}
}

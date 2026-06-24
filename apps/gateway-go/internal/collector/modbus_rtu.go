package collector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/goburrow/modbus"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/mapper"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type ModbusRTU struct{}

type rtuBus struct {
	handler  *modbus.RTUClientHandler
	client   modbus.Client
	settings string
	mu       sync.Mutex
}

var rtuPool = struct {
	sync.Mutex
	items map[string]*rtuBus
}{items: map[string]*rtuBus{}}

func (ModbusRTU) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	raw, err := readRTU(ctx, point)
	if err != nil {
		return model.PointValue{}, err
	}
	value, err := mapper.Decode(point, raw)
	if err != nil {
		return model.PointValue{}, err
	}
	return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
}

func (ModbusRTU) ReadRegisterRange(ctx context.Context, point config.PointConfig, start uint16, quantity uint16) ([]byte, error) {
	rangePoint := point
	rangePoint.Register = start
	rangePoint.Quantity = quantity
	return readRTU(ctx, rangePoint)
}

func readRTU(ctx context.Context, point config.PointConfig) ([]byte, error) {
	if point.Address == "" {
		return nil, fmt.Errorf("modbus RTU serial port is required")
	}
	bus, err := getRTUBus(point)
	if err != nil {
		return nil, err
	}
	bus.mu.Lock()
	defer bus.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	bus.handler.SlaveId = point.SlaveID
	raw, err := readByFunction(ctx, bus.client, point)
	if err != nil {
		_ = bus.handler.Close()
		return nil, fmt.Errorf("Modbus RTU %s slave %d read failed: %w", point.Address, point.SlaveID, err)
	}
	return raw, nil
}

func getRTUBus(point config.PointConfig) (*rtuBus, error) {
	point.ApplyDefaults()
	key := point.Address
	settings := rtuSettingsKey(point)
	rtuPool.Lock()
	defer rtuPool.Unlock()
	if bus := rtuPool.items[key]; bus != nil {
		if bus.settings != settings {
			return nil, fmt.Errorf("serial port %s has conflicting settings: %s and %s", point.Address, bus.settings, settings)
		}
		return bus, nil
	}
	handler := modbus.NewRTUClientHandler(point.Address)
	handler.BaudRate = point.BaudRate
	handler.DataBits = point.DataBits
	handler.StopBits = point.StopBits
	handler.Parity = normalizeParity(point.Parity)
	handler.Timeout = 1500 * time.Millisecond
	handler.IdleTimeout = 60 * time.Second
	bus := &rtuBus{handler: handler, client: modbus.NewClient(handler), settings: settings}
	rtuPool.items[key] = bus
	return bus, nil
}

func rtuSettingsKey(point config.PointConfig) string {
	point.ApplyDefaults()
	return fmt.Sprintf("%d-%d-%d-%s", point.BaudRate, point.DataBits, point.StopBits, normalizeParity(point.Parity))
}

func normalizeParity(parity string) string {
	switch strings.ToUpper(strings.TrimSpace(parity)) {
	case "E", "EVEN":
		return "E"
	case "O", "ODD":
		return "O"
	default:
		return "N"
	}
}

func CloseRTUConnections() {
	rtuPool.Lock()
	buses := make([]*rtuBus, 0, len(rtuPool.items))
	for _, bus := range rtuPool.items {
		buses = append(buses, bus)
	}
	rtuPool.items = map[string]*rtuBus{}
	rtuPool.Unlock()

	for _, bus := range buses {
		bus.mu.Lock()
		_ = bus.handler.Close()
		bus.mu.Unlock()
	}
}

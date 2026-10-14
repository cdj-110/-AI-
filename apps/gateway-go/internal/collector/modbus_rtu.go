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

type ModbusRTUConnectionResult struct {
	Address   string `json:"address"`
	SlaveID   byte   `json:"slaveId"`
	Function  uint8  `json:"function"`
	Register  uint16 `json:"register"`
	ElapsedMS int64  `json:"elapsedMilliseconds"`
}

// Probe with a single read only, reusing the collection bus and its lock.
func TestModbusRTUConnection(ctx context.Context, point config.PointConfig) (ModbusRTUConnectionResult, error) {
	point.Protocol = "modbus-rtu"
	point.Address = strings.TrimSpace(point.Address)
	point.ApplyDefaults()
	if point.Function == 0 {
		point.Function = 3
	}
	point.Quantity = 1
	result := ModbusRTUConnectionResult{Address: point.Address, SlaveID: point.SlaveID, Function: point.Function, Register: point.Register}
	if point.Address == "" {
		return result, fmt.Errorf("请配置串口设备路径")
	}
	if point.SlaveID < 1 || point.SlaveID > 247 {
		return result, fmt.Errorf("Modbus RTU 从站 ID 必须为 1–247，连接测试不使用广播地址")
	}
	if point.Function < 1 || point.Function > 4 {
		return result, fmt.Errorf("连接测试仅允许读取功能码 1/2/3/4")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	bus, err := getRTUBus(point)
	if err != nil {
		return result, fmt.Errorf("串口参数冲突，请先保存配置再测试：%w", err)
	}
	for !bus.mu.TryLock() {
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("串口忙，等待连接测试超时：%w", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer bus.mu.Unlock()
	started := time.Now()
	_, err = readRTULocked(ctx, bus, point)
	result.ElapsedMS = time.Since(started).Milliseconds()
	if err != nil {
		return result, fmt.Errorf("串口通信测试失败，请检查接线、从站 ID、串口参数和点位地址：%w", err)
	}
	return result, nil
}

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

func (ModbusRTU) WritePoint(ctx context.Context, point config.PointConfig, value interface{}) error {
	if point.Address == "" {
		return fmt.Errorf("modbus RTU serial port is required")
	}
	bus, err := getRTUBus(point)
	if err != nil {
		return err
	}
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.handler.SlaveId = point.SlaveID
	if err := writeByFunction(ctx, bus.client, point, value); err != nil {
		recordModbusWrite("modbus-rtu", point, value, err)
		_ = bus.handler.Close()
		return fmt.Errorf("Modbus RTU %s slave %d write failed: %w", point.Address, point.SlaveID, err)
	}
	recordModbusWrite("modbus-rtu", point, value, nil)
	return nil
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
	return readRTULocked(ctx, bus, point)
}

func readRTULocked(ctx context.Context, bus *rtuBus, point config.PointConfig) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	bus.handler.SlaveId = point.SlaveID
	recordModbusReadRequest("modbus-rtu", point)
	raw, err := readByFunction(ctx, bus.client, point)
	recordModbusReadResponse("modbus-rtu", point, raw, err)
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

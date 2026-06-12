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

type ModbusTCP struct{}

type tcpConnection struct {
	handler *modbus.TCPClientHandler
	client  modbus.Client
	mu      sync.Mutex
}

var tcpPool = struct {
	sync.Mutex
	items map[string]*tcpConnection
}{items: map[string]*tcpConnection{}}

func (ModbusTCP) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	conn, err := getTCPConnection(point)
	if err != nil {
		return model.PointValue{}, err
	}

	conn.mu.Lock()
	raw, err := readByFunction(ctx, conn.client, point)
	conn.mu.Unlock()
	if err != nil {
		closeTCPConnection(point)
		return model.PointValue{}, err
	}
	value, err := mapper.Decode(point, raw)
	if err != nil {
		return model.PointValue{}, err
	}
	return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
}

func (ModbusTCP) ReadRegisterRange(ctx context.Context, point config.PointConfig, start uint16, quantity uint16) ([]byte, error) {
	conn, err := getTCPConnection(point)
	if err != nil {
		return nil, err
	}

	rangePoint := point
	rangePoint.Register = start
	rangePoint.Quantity = quantity

	conn.mu.Lock()
	raw, err := readByFunction(ctx, conn.client, rangePoint)
	conn.mu.Unlock()
	if err != nil {
		closeTCPConnection(point)
		return nil, err
	}
	return raw, nil
}

func getTCPConnection(point config.PointConfig) (*tcpConnection, error) {
	key := tcpConnectionKey(point)
	tcpPool.Lock()
	if conn := tcpPool.items[key]; conn != nil {
		tcpPool.Unlock()
		return conn, nil
	}
	tcpPool.Unlock()

	handler := modbus.NewTCPClientHandler(point.Address)
	handler.SlaveId = point.SlaveID
	handler.Timeout = 1200 * time.Millisecond
	if err := handler.Connect(); err != nil {
		return nil, err
	}
	conn := &tcpConnection{handler: handler, client: modbus.NewClient(handler)}

	tcpPool.Lock()
	if existing := tcpPool.items[key]; existing != nil {
		tcpPool.Unlock()
		_ = handler.Close()
		return existing, nil
	}
	tcpPool.items[key] = conn
	tcpPool.Unlock()
	return conn, nil
}

func closeTCPConnection(point config.PointConfig) {
	key := tcpConnectionKey(point)
	tcpPool.Lock()
	conn := tcpPool.items[key]
	delete(tcpPool.items, key)
	tcpPool.Unlock()
	if conn != nil {
		_ = conn.handler.Close()
	}
}

func tcpConnectionKey(point config.PointConfig) string {
	return fmt.Sprintf("%s#%d", point.Address, point.SlaveID)
}

func readByFunction(ctx context.Context, client modbus.Client, point config.PointConfig) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	switch point.Function {
	case 1:
		raw, err := client.ReadCoils(point.Register, point.Quantity)
		return readWithHint(point, raw, err)
	case 2:
		raw, err := client.ReadDiscreteInputs(point.Register, point.Quantity)
		return readWithHint(point, raw, err)
	case 3:
		raw, err := client.ReadHoldingRegisters(point.Register, point.Quantity)
		return readWithHint(point, raw, err)
	case 4:
		raw, err := client.ReadInputRegisters(point.Register, point.Quantity)
		return readWithHint(point, raw, err)
	default:
		return nil, fmt.Errorf("unsupported modbus function %d", point.Function)
	}
}

func readWithHint(point config.PointConfig, raw []byte, err error) ([]byte, error) {
	if err == nil {
		return raw, nil
	}
	message := err.Error()
	if strings.Contains(message, "illegal function") {
		return nil, fmt.Errorf("设备不支持当前 Modbus 功能码 %d，请检查点表功能码是否应为 1/2/3/4：%w", point.Function, err)
	}
	if strings.Contains(message, "illegal data address") {
		return nil, fmt.Errorf("设备不支持当前寄存器地址 %d，请检查寄存器起始地址是否需要减 1：%w", point.Register, err)
	}
	if strings.Contains(message, "connection refused") || strings.Contains(message, "i/o timeout") {
		return nil, fmt.Errorf("无法连接 Modbus TCP 设备 %s，请检查 IP、端口和网络：%w", point.Address, err)
	}
	return nil, err
}

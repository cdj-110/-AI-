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

// Some industrial Modbus TCP slaves only produce a response on their own
// scan boundary. Keep the socket timeout longer than the UI collection period;
// collection lanes isolate a slow slave so it cannot delay other devices.
const modbusTCPTimeout = 3 * time.Second

type tcpConnection struct {
	handler *modbus.TCPClientHandler
	client  modbus.Client
	gate    *priorityConnectionLock
}

type priorityConnectionLock struct {
	mu           sync.Mutex
	held         bool
	writeWaiters int
	changed      chan struct{}
}

func newPriorityConnectionLock() *priorityConnectionLock {
	return &priorityConnectionLock{changed: make(chan struct{})}
}

type tcpConnectionPool struct {
	sync.Mutex
	items map[string]*tcpConnection
}

var tcpPool = tcpConnectionPool{items: map[string]*tcpConnection{}}
var tcpWritePool = tcpConnectionPool{items: map[string]*tcpConnection{}}

func (ModbusTCP) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	conn, err := getTCPConnection(point)
	if err != nil {
		return model.PointValue{}, err
	}

	if err := conn.lockRead(ctx); err != nil {
		return model.PointValue{}, fmt.Errorf("%w: %v", ErrCollectionDeferred, err)
	}
	recordModbusReadRequest("modbus-tcp", point)
	raw, err := readByFunction(ctx, conn.client, point)
	recordModbusReadResponse("modbus-tcp", point, raw, err)
	conn.unlock()
	if err != nil {
		closeTCPConnection(point, conn)
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

	if err := conn.lockRead(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCollectionDeferred, err)
	}
	recordModbusReadRequest("modbus-tcp", rangePoint)
	raw, err := readByFunction(ctx, conn.client, rangePoint)
	recordModbusReadResponse("modbus-tcp", rangePoint, raw, err)
	conn.unlock()
	if err != nil {
		closeTCPConnection(point, conn)
		return nil, err
	}
	return raw, nil
}

func (ModbusTCP) WritePoint(ctx context.Context, point config.PointConfig, value interface{}) error {
	conn, err := getTCPWriteConnection(point)
	if err != nil {
		return err
	}
	if err := conn.lockWrite(ctx); err != nil {
		return err
	}
	err = writeByFunction(ctx, conn.client, point, value)
	recordModbusWrite("modbus-tcp", point, value, err)
	conn.unlock()
	if err != nil {
		closeTCPWriteConnection(point, conn)
	}
	return err
}

func getTCPConnection(point config.PointConfig) (*tcpConnection, error) {
	return getTCPConnectionFromPool(&tcpPool, point)
}

func getTCPWriteConnection(point config.PointConfig) (*tcpConnection, error) {
	return getTCPConnectionFromPool(&tcpWritePool, point)
}

func getTCPConnectionFromPool(pool *tcpConnectionPool, point config.PointConfig) (*tcpConnection, error) {
	key := tcpConnectionKey(point)
	pool.Lock()
	if conn := pool.items[key]; conn != nil {
		pool.Unlock()
		return conn, nil
	}
	pool.Unlock()

	handler := modbus.NewTCPClientHandler(point.Address)
	handler.SlaveId = point.SlaveID
	handler.Timeout = modbusTCPTimeout
	if err := handler.Connect(); err != nil {
		return nil, err
	}
	conn := &tcpConnection{handler: handler, client: modbus.NewClient(handler), gate: newPriorityConnectionLock()}

	pool.Lock()
	if existing := pool.items[key]; existing != nil {
		pool.Unlock()
		_ = handler.Close()
		return existing, nil
	}
	pool.items[key] = conn
	pool.Unlock()
	return conn, nil
}

func closeTCPConnection(point config.PointConfig, target *tcpConnection) {
	closeTCPConnectionFromPool(&tcpPool, point, target)
}

func closeTCPWriteConnection(point config.PointConfig, target *tcpConnection) {
	closeTCPConnectionFromPool(&tcpWritePool, point, target)
}

func closeTCPConnectionFromPool(pool *tcpConnectionPool, point config.PointConfig, target *tcpConnection) {
	key := tcpConnectionKey(point)
	pool.Lock()
	conn := pool.items[key]
	if conn == target {
		delete(pool.items, key)
	}
	pool.Unlock()
	if conn == target {
		_ = target.handler.Close()
	}
}

func (c *tcpConnection) lockRead(ctx context.Context) error {
	return c.gate.lock(ctx, false)
}

func (c *tcpConnection) lockWrite(ctx context.Context) error {
	return c.gate.lock(ctx, true)
}

func (c *tcpConnection) unlock() {
	c.gate.unlock()
}

func (l *priorityConnectionLock) lock(ctx context.Context, write bool) error {
	l.mu.Lock()
	if write {
		l.writeWaiters++
	}
	for l.held || (!write && l.writeWaiters > 0) {
		changed := l.changed
		l.mu.Unlock()
		select {
		case <-changed:
			l.mu.Lock()
		case <-ctx.Done():
			l.mu.Lock()
			if write {
				l.writeWaiters--
				l.notifyLocked()
			}
			l.mu.Unlock()
			return ctx.Err()
		}
	}
	if write {
		l.writeWaiters--
	}
	l.held = true
	l.mu.Unlock()
	return nil
}

func (l *priorityConnectionLock) unlock() {
	l.mu.Lock()
	l.held = false
	l.notifyLocked()
	l.mu.Unlock()
}

func (l *priorityConnectionLock) notifyLocked() {
	close(l.changed)
	l.changed = make(chan struct{})
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

func writeByFunction(ctx context.Context, client modbus.Client, point config.PointConfig, value interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	raw, err := mapper.Encode(point, value)
	if err != nil {
		return err
	}
	switch point.Function {
	case 1:
		if len(raw) < 2 {
			return fmt.Errorf("coil write requires 2 bytes")
		}
		_, err = client.WriteSingleCoil(point.Register, uint16(raw[0])<<8|uint16(raw[1]))
		return err
	case 3:
		if len(raw) == 2 {
			_, err = client.WriteSingleRegister(point.Register, uint16(raw[0])<<8|uint16(raw[1]))
			return err
		}
		_, err = client.WriteMultipleRegisters(point.Register, uint16(len(raw)/2), raw)
		return err
	default:
		return fmt.Errorf("point %s function %d is read-only or unsupported for write", point.Metric, point.Function)
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

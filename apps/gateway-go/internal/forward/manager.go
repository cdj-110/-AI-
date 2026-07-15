package forward

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/mapper"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

const (
	ProtocolNone         = "none"
	ProtocolModbusSlave  = "modbus-tcp-slave"
	ProtocolIEC104Server = "iec104-server"
)

type Manager struct {
	store  *state.Store
	writer PointWriter
	mu     sync.RWMutex
	cfg    config.Config
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type forwardPoint struct {
	Config          config.PointConfig
	Status          state.PointStatus
	Value           interface{}
	UnitID          byte
	CommonAddress   uint16
	SourceDeviceKey string
	SourceMetric    string
}

type PointWriter interface {
	WritePoint(ctx context.Context, deviceKey, metric string, value interface{}) (config.PointConfig, error)
}

func NewManager(store *state.Store, writers ...PointWriter) *Manager {
	manager := &Manager{store: store}
	if len(writers) > 0 {
		manager.writer = writers[0]
	}
	return manager
}

func (m *Manager) Update(ctx context.Context, cfg config.Config) {
	m.stop()
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	if !cfg.ForwardSlave.Enabled {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	if m.hasForwardProtocol(ProtocolModbusSlave) {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			if err := m.serveModbus(runCtx, cfg.ForwardSlave.ModbusListen); err != nil && runCtx.Err() == nil {
				log.Printf("modbus forward server stopped: %v", err)
				m.store.AddError("modbus forward server stopped: " + err.Error())
			}
		}()
	}
	if m.hasForwardProtocol(ProtocolIEC104Server) {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			if err := m.serveIEC104(runCtx, cfg.ForwardSlave.IEC104Listen); err != nil && runCtx.Err() == nil {
				log.Printf("iec104 forward server stopped: %v", err)
				m.store.AddError("iec104 forward server stopped: " + err.Error())
			}
		}()
	}
}

func (m *Manager) Stop() {
	m.stop()
}

func (m *Manager) stop() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.wg.Wait()
}

func (m *Manager) config() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) hasForwardProtocol(protocol string) bool {
	cfg := m.config()
	if len(cfg.ForwardDevices) > 0 {
		for _, device := range cfg.ForwardDevices {
			if device.IsEnabled() && strings.EqualFold(device.Protocol, protocol) {
				return true
			}
		}
		return false
	}
	for _, channel := range cfg.Channels {
		if strings.EqualFold(channel.ForwardProtocol, protocol) {
			return true
		}
	}
	return false
}

func (m *Manager) points(protocol string) []forwardPoint {
	cfg := m.config()
	statusByKey := map[string]state.PointStatus{}
	for _, point := range m.store.PointStatuses() {
		statusByKey[point.DeviceKey+"::"+point.Metric] = point
	}
	var points []forwardPoint
	if len(cfg.ForwardDevices) > 0 {
		for _, device := range cfg.ForwardDevices {
			if !device.IsEnabled() || !strings.EqualFold(device.Protocol, protocol) {
				continue
			}
			for _, mapping := range device.Points {
				status := statusByKey[mapping.SourceDeviceKey+"::"+mapping.SourceMetric]
				if status.Error != "" || status.UpdatedAt == nil {
					continue
				}
				point := config.PointConfig{Name: mapping.Name, Metric: mapping.Metric, PointType: mapping.PointType, Function: mapping.Function, Register: mapping.Register, IOA: mapping.IOA, Quantity: mapping.Quantity, DataType: mapping.DataType, ByteOrder: mapping.ByteOrder, WordOrder: mapping.WordOrder, Scale: mapping.Scale, Offset: mapping.Offset, Unit: mapping.Unit, Decimals: mapping.Decimals}
				points = append(points, forwardPoint{Config: point, Status: status, Value: status.Value, UnitID: device.UnitID, CommonAddress: device.CommonAddress, SourceDeviceKey: mapping.SourceDeviceKey, SourceMetric: mapping.SourceMetric})
			}
		}
		return points
	}
	channelForward := map[string]string{}
	for _, channel := range cfg.Channels {
		channelForward[channel.ChannelKey] = channel.ForwardProtocol
	}
	for _, point := range cfg.Points {
		if !strings.EqualFold(channelForward[point.ChannelKey], protocol) {
			continue
		}
		status := statusByKey[point.DeviceKey+"::"+point.Metric]
		if status.Error != "" || status.UpdatedAt == nil {
			continue
		}
		points = append(points, forwardPoint{Config: point, Status: status, Value: status.Value, UnitID: point.SlaveID, CommonAddress: point.CommonAddress, SourceDeviceKey: point.DeviceKey, SourceMetric: point.Metric})
	}
	return points
}

func (m *Manager) serveModbus(ctx context.Context, listen string) error {
	if listen == "" {
		listen = "0.0.0.0:1502"
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	log.Printf("protocol forward Modbus TCP slave listening on %s", listen)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go m.handleModbusConn(ctx, conn)
	}
}

func (m *Manager) handleModbusConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		header := make([]byte, 7)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		length := int(binary.BigEndian.Uint16(header[4:6]))
		if length <= 1 || length > 260 {
			return
		}
		pdu := make([]byte, length-1)
		if _, err := io.ReadFull(conn, pdu); err != nil {
			return
		}
		response := m.modbusResponseForUnitContext(ctx, header[6], pdu)
		adu := make([]byte, 7+len(response))
		copy(adu[0:4], header[0:4])
		binary.BigEndian.PutUint16(adu[4:6], uint16(len(response)+1))
		adu[6] = header[6]
		copy(adu[7:], response)
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn.Write(adu); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (m *Manager) modbusResponse(pdu []byte) []byte {
	return m.modbusResponseForUnit(0, pdu)
}

func (m *Manager) modbusResponseForUnit(unitID byte, pdu []byte) []byte {
	return m.modbusResponseForUnitContext(context.Background(), unitID, pdu)
}

func (m *Manager) modbusResponseForUnitContext(ctx context.Context, unitID byte, pdu []byte) []byte {
	if len(pdu) < 5 {
		return modbusException(pdu, 0x03)
	}
	function := pdu[0]
	start := binary.BigEndian.Uint16(pdu[1:3])
	switch function {
	case 1, 2:
		quantity := binary.BigEndian.Uint16(pdu[3:5])
		if quantity == 0 || quantity > 2000 {
			return modbusException(pdu, 0x03)
		}
		return m.modbusBits(unitID, function, start, quantity)
	case 3, 4:
		quantity := binary.BigEndian.Uint16(pdu[3:5])
		if quantity == 0 || quantity > 125 {
			return modbusException(pdu, 0x03)
		}
		return m.modbusRegisters(unitID, function, start, quantity)
	case 5:
		return m.writeSingleCoil(ctx, unitID, pdu)
	case 6:
		return m.writeSingleRegister(ctx, unitID, pdu)
	case 16:
		return m.writeMultipleRegisters(ctx, unitID, pdu)
	default:
		return modbusException(pdu, 0x01)
	}
}

func (m *Manager) writableModbusPoints(unitID byte, readFunction uint8) []forwardPoint {
	cfg := m.config()
	var points []forwardPoint
	if len(cfg.ForwardDevices) > 0 {
		for _, device := range cfg.ForwardDevices {
			if !device.IsEnabled() || !strings.EqualFold(device.Protocol, ProtocolModbusSlave) || (unitID != 0 && device.UnitID != unitID) {
				continue
			}
			for _, mapping := range device.Points {
				if mapping.Function != readFunction {
					continue
				}
				point := config.PointConfig{Name: mapping.Name, Metric: mapping.Metric, PointType: mapping.PointType, Function: mapping.Function, Register: mapping.Register, IOA: mapping.IOA, Quantity: mapping.Quantity, DataType: mapping.DataType, ByteOrder: mapping.ByteOrder, WordOrder: mapping.WordOrder, Scale: mapping.Scale, Offset: mapping.Offset, Unit: mapping.Unit, Decimals: mapping.Decimals}
				points = append(points, forwardPoint{Config: point, UnitID: device.UnitID, SourceDeviceKey: mapping.SourceDeviceKey, SourceMetric: mapping.SourceMetric})
			}
		}
		return points
	}
	channelForward := map[string]string{}
	for _, channel := range cfg.Channels {
		channelForward[channel.ChannelKey] = channel.ForwardProtocol
	}
	for _, point := range cfg.Points {
		if !strings.EqualFold(channelForward[point.ChannelKey], ProtocolModbusSlave) || point.Function != readFunction || (unitID != 0 && point.SlaveID != unitID) {
			continue
		}
		points = append(points, forwardPoint{Config: point, UnitID: point.SlaveID, SourceDeviceKey: point.DeviceKey, SourceMetric: point.Metric})
	}
	return points
}

func (m *Manager) writeSingleCoil(ctx context.Context, unitID byte, pdu []byte) []byte {
	raw := binary.BigEndian.Uint16(pdu[3:5])
	if raw != 0xff00 && raw != 0x0000 {
		return modbusException(pdu, 0x03)
	}
	point, ok := singlePointAt(m.writableModbusPoints(unitID, 1), binary.BigEndian.Uint16(pdu[1:3]), 1)
	if !ok {
		return modbusException(pdu, 0x02)
	}
	if !m.writeForwardPoint(ctx, point, raw == 0xff00) {
		return modbusException(pdu, 0x04)
	}
	return append([]byte(nil), pdu[:5]...)
}

func (m *Manager) writeSingleRegister(ctx context.Context, unitID byte, pdu []byte) []byte {
	point, ok := singlePointAt(m.writableModbusPoints(unitID, 3), binary.BigEndian.Uint16(pdu[1:3]), 1)
	if !ok {
		return modbusException(pdu, 0x02)
	}
	value, err := mapper.Decode(forwardDecodeConfig(point.Config), pdu[3:5])
	if err != nil {
		return modbusException(pdu, 0x03)
	}
	if !m.writeForwardPoint(ctx, point, value) {
		return modbusException(pdu, 0x04)
	}
	return append([]byte(nil), pdu[:5]...)
}

func (m *Manager) writeMultipleRegisters(ctx context.Context, unitID byte, pdu []byte) []byte {
	if len(pdu) < 6 {
		return modbusException(pdu, 0x03)
	}
	start := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])
	byteCount := int(pdu[5])
	if quantity == 0 || quantity > 123 || byteCount != int(quantity)*2 || len(pdu) != 6+byteCount {
		return modbusException(pdu, 0x03)
	}
	points := m.writableModbusPoints(unitID, 3)
	sort.SliceStable(points, func(i, j int) bool { return points[i].Config.Register < points[j].Config.Register })
	covered := make([]bool, quantity)
	selected := make([]forwardPoint, 0)
	values := make([]interface{}, 0)
	requestEnd := uint32(start) + uint32(quantity)
	for _, point := range points {
		width := forwardRegisterCount(point.Config)
		pointStart := uint32(point.Config.Register)
		pointEnd := pointStart + uint32(width)
		if pointEnd <= uint32(start) || pointStart >= requestEnd {
			continue
		}
		if pointStart < uint32(start) || pointEnd > requestEnd {
			return modbusException(pdu, 0x02)
		}
		offset := int(pointStart - uint32(start))
		for index := 0; index < int(width); index++ {
			if covered[offset+index] {
				return modbusException(pdu, 0x02)
			}
			covered[offset+index] = true
		}
		raw := pdu[6+offset*2 : 6+(offset+int(width))*2]
		value, err := mapper.Decode(forwardDecodeConfig(point.Config), raw)
		if err != nil {
			return modbusException(pdu, 0x03)
		}
		selected = append(selected, point)
		values = append(values, value)
	}
	for _, isCovered := range covered {
		if !isCovered {
			return modbusException(pdu, 0x02)
		}
	}
	for index, point := range selected {
		if !m.writeForwardPoint(ctx, point, values[index]) {
			return modbusException(pdu, 0x04)
		}
	}
	return append([]byte(nil), pdu[:5]...)
}

func singlePointAt(points []forwardPoint, register uint16, width uint16) (forwardPoint, bool) {
	var matched forwardPoint
	count := 0
	for _, point := range points {
		if point.Config.Register == register && forwardRegisterCount(point.Config) == width {
			matched = point
			count++
		}
	}
	return matched, count == 1
}

func forwardRegisterCount(point config.PointConfig) uint16 {
	if point.Quantity > 0 {
		return point.Quantity
	}
	switch strings.ToLower(strings.TrimSpace(point.DataType)) {
	case "uint32", "int32", "float32":
		return 2
	default:
		return 1
	}
}

func forwardDecodeConfig(point config.PointConfig) config.PointConfig {
	point.Quantity = forwardRegisterCount(point)
	if point.DataType == "" {
		point.DataType = "uint16"
	}
	if point.Scale == 0 {
		point.Scale = 1
	}
	return point
}

func (m *Manager) writeForwardPoint(ctx context.Context, point forwardPoint, value interface{}) bool {
	if m.writer == nil || point.SourceDeviceKey == "" || point.SourceMetric == "" {
		return false
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := m.writer.WritePoint(writeCtx, point.SourceDeviceKey, point.SourceMetric, value); err != nil {
		log.Printf("modbus forward write %s/%s failed: %v", point.SourceDeviceKey, point.SourceMetric, err)
		m.store.AddError(fmt.Sprintf("Modbus 转发写入 %s/%s 失败: %v", point.SourceDeviceKey, point.SourceMetric, err))
		return false
	}
	return true
}

func (m *Manager) modbusBits(unitID byte, function byte, start uint16, quantity uint16) []byte {
	byteCount := int((quantity + 7) / 8)
	data := make([]byte, byteCount)
	for _, point := range m.points(ProtocolModbusSlave) {
		if (unitID != 0 && point.UnitID != unitID) || (point.Config.Function != 0 && point.Config.Function != function) {
			continue
		}
		if point.Config.Register < start || point.Config.Register >= start+quantity {
			continue
		}
		if truthy(point.Value) {
			offset := point.Config.Register - start
			data[offset/8] |= 1 << (offset % 8)
		}
	}
	return append([]byte{function, byte(byteCount)}, data...)
}

func (m *Manager) modbusRegisters(unitID byte, function byte, start uint16, quantity uint16) []byte {
	registers := make([]uint16, quantity)
	for _, point := range m.points(ProtocolModbusSlave) {
		if (unitID != 0 && point.UnitID != unitID) || (point.Config.Function != 0 && point.Config.Function != function) {
			continue
		}
		values := modbusValueRegisters(point)
		offset := int(point.Config.Register) - int(start)
		for index, value := range values {
			target := offset + index
			if target >= 0 && target < len(registers) {
				registers[target] = value
			}
		}
	}
	data := make([]byte, len(registers)*2)
	for index, value := range registers {
		binary.BigEndian.PutUint16(data[index*2:index*2+2], value)
	}
	return append([]byte{function, byte(len(data))}, data...)
}

func modbusException(pdu []byte, code byte) []byte {
	function := byte(0)
	if len(pdu) > 0 {
		function = pdu[0]
	}
	return []byte{function | 0x80, code}
}

func modbusValueRegisters(point forwardPoint) []uint16 {
	value := numeric(point.Value)
	switch strings.ToLower(point.Config.DataType) {
	case "float32", "float":
		raw := math.Float32bits(float32(value))
		return []uint16{uint16(raw >> 16), uint16(raw)}
	case "uint32", "int32":
		raw := uint32(int64(value))
		return []uint16{uint16(raw >> 16), uint16(raw)}
	default:
		return []uint16{uint16(int64(value))}
	}
}

func (m *Manager) serveIEC104(ctx context.Context, listen string) error {
	if listen == "" {
		listen = "0.0.0.0:2404"
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	log.Printf("protocol forward IEC104 server listening on %s", listen)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go m.handleIEC104Conn(ctx, conn)
	}
}

func (m *Manager) handleIEC104Conn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	session := &iec104Session{conn: conn, commonAS: 1, baseline: map[string]string{}}
	go session.pushChanges(sessionCtx, m)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		packet, err := readForwardIEC104Packet(conn)
		if err != nil {
			return
		}
		if len(packet) < 6 {
			continue
		}
		control := packet[2]
		if control == 0x07 {
			session.setStarted(true)
			_ = session.writeRaw([]byte{0x68, 0x04, 0x0b, 0x00, 0x00, 0x00})
			continue
		}
		if control == 0x13 {
			session.setStarted(false)
			_ = session.writeRaw([]byte{0x68, 0x04, 0x23, 0x00, 0x00, 0x00})
			return
		}
		if control == 0x43 {
			_ = session.writeRaw([]byte{0x68, 0x04, 0x83, 0x00, 0x00, 0x00})
			continue
		}
		if control&0x01 == 0 {
			session.setRecvSeq((binary.LittleEndian.Uint16(packet[2:4]) >> 1) + 1)
			session.handleASDU(m, packet[6:])
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

type iec104Session struct {
	conn          net.Conn
	writeMu       sync.Mutex
	stateMu       sync.RWMutex
	sendSeq       uint16
	recvSeq       uint16
	commonAS      uint16
	started       bool
	baseline      map[string]string
	baselineReady bool
}

func (s *iec104Session) handleASDU(m *Manager, asdu []byte) {
	if len(asdu) < 10 || asdu[0] != 100 {
		_ = s.sendAck()
		return
	}
	commonAS := binary.LittleEndian.Uint16(asdu[4:6])
	if commonAS != 0 {
		s.setCommonAS(commonAS)
	}
	_ = s.sendInterrogation(0x07)
	points := s.forwardPoints(m)
	for _, point := range points {
		_ = s.sendPoint(point, 0x14)
	}
	_ = s.sendInterrogation(0x0a)
	s.captureBaseline(points)
}

func (s *iec104Session) sendAck() error {
	_, recvSeq, _ := s.state()
	recv := recvSeq << 1
	return s.writeRaw([]byte{0x68, 0x04, 0x01, 0x00, byte(recv), byte(recv >> 8)})
}

func (s *iec104Session) sendInterrogation(cot byte) error {
	commonAS, _, _ := s.state()
	asdu := []byte{100, 1, cot, 0, byte(commonAS), byte(commonAS >> 8), 0, 0, 0, 20}
	return s.writeI(asdu)
}

func (s *iec104Session) sendPoint(point forwardPoint, cot byte) error {
	ioa := point.Config.IOA
	if ioa == 0 {
		ioa = uint32(point.Config.Register)
	}
	if ioa == 0 {
		ioa = 1
	}
	commonAS, _, _ := s.state()
	dataType := strings.ToLower(strings.TrimSpace(point.Config.DataType))
	if dataType == "bool" || dataType == "boolean" || dataType == "single" {
		value := byte(0)
		if truthy(point.Value) {
			value = 1
		}
		asdu := []byte{
			1, 1,
			cot, 0,
			byte(commonAS), byte(commonAS >> 8),
			byte(ioa), byte(ioa >> 8), byte(ioa >> 16),
			value,
		}
		return s.writeI(asdu)
	}
	value := float32(numeric(point.Value))
	asdu := []byte{
		13, 1,
		cot, 0,
		byte(commonAS), byte(commonAS >> 8),
		byte(ioa), byte(ioa >> 8), byte(ioa >> 16),
	}
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, math.Float32bits(value))
	asdu = append(asdu, raw...)
	asdu = append(asdu, 0)
	return s.writeI(asdu)
}

func (s *iec104Session) writeI(asdu []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, recvSeq, _ := s.state()
	send := s.sendSeq << 1
	recv := recvSeq << 1
	s.sendSeq++
	packet := make([]byte, 6+len(asdu))
	packet[0] = 0x68
	packet[1] = byte(4 + len(asdu))
	binary.LittleEndian.PutUint16(packet[2:4], send)
	binary.LittleEndian.PutUint16(packet[4:6], recv)
	copy(packet[6:], asdu)
	return s.writeRawLocked(packet)
}

func (s *iec104Session) writeRaw(packet []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.writeRawLocked(packet)
}

func (s *iec104Session) writeRawLocked(packet []byte) error {
	_ = s.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_, err := s.conn.Write(packet)
	return err
}

func (s *iec104Session) setStarted(started bool) {
	s.stateMu.Lock()
	s.started = started
	s.baselineReady = false
	s.baseline = map[string]string{}
	s.stateMu.Unlock()
}

func (s *iec104Session) setRecvSeq(sequence uint16) {
	s.stateMu.Lock()
	s.recvSeq = sequence
	s.stateMu.Unlock()
}

func (s *iec104Session) setCommonAS(commonAS uint16) {
	s.stateMu.Lock()
	if s.commonAS != commonAS {
		s.commonAS = commonAS
		s.baselineReady = false
		s.baseline = map[string]string{}
	}
	s.stateMu.Unlock()
}

func (s *iec104Session) state() (commonAS uint16, recvSeq uint16, started bool) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.commonAS, s.recvSeq, s.started
}

func (s *iec104Session) forwardPoints(m *Manager) []forwardPoint {
	commonAS, _, _ := s.state()
	points := m.points(ProtocolIEC104Server)
	filtered := make([]forwardPoint, 0, len(points))
	for _, point := range points {
		if point.CommonAddress != 0 && point.CommonAddress != commonAS {
			continue
		}
		filtered = append(filtered, point)
	}
	return filtered
}

func forwardPointKey(point forwardPoint) string {
	ioa := point.Config.IOA
	if ioa == 0 {
		ioa = uint32(point.Config.Register)
	}
	return fmt.Sprintf("%d:%d:%s", point.CommonAddress, ioa, point.Config.Metric)
}

func forwardPointFingerprint(point forwardPoint) string {
	return fmt.Sprintf("%T:%v", point.Value, point.Value)
}

func (s *iec104Session) captureBaseline(points []forwardPoint) {
	baseline := make(map[string]string, len(points))
	for _, point := range points {
		baseline[forwardPointKey(point)] = forwardPointFingerprint(point)
	}
	s.stateMu.Lock()
	s.baseline = baseline
	s.baselineReady = true
	s.stateMu.Unlock()
}

func (s *iec104Session) changedPoints(points []forwardPoint) ([]forwardPoint, bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if !s.baselineReady {
		s.baseline = make(map[string]string, len(points))
		for _, point := range points {
			s.baseline[forwardPointKey(point)] = forwardPointFingerprint(point)
		}
		s.baselineReady = true
		return nil, false
	}
	var changed []forwardPoint
	for _, point := range points {
		key := forwardPointKey(point)
		fingerprint := forwardPointFingerprint(point)
		if previous, exists := s.baseline[key]; exists && previous == fingerprint {
			continue
		}
		s.baseline[key] = fingerprint
		changed = append(changed, point)
	}
	return changed, true
}

func (s *iec104Session) pushChanges(ctx context.Context, m *Manager) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, started := s.state()
			if !started {
				continue
			}
			changed, ready := s.changedPoints(s.forwardPoints(m))
			if !ready {
				continue
			}
			for _, point := range changed {
				if err := s.sendPoint(point, 0x03); err != nil {
					_ = s.conn.Close()
					return
				}
			}
		}
	}
}

func readForwardIEC104Packet(reader io.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if header[0] != 0x68 {
		return nil, fmt.Errorf("invalid IEC104 start byte 0x%02x", header[0])
	}
	body := make([]byte, int(header[1]))
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}

func numeric(value interface{}) float64 {
	switch typed := value.(type) {
	case bool:
		if typed {
			return 1
		}
		return 0
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int8:
		return float64(typed)
	case int16:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint8:
		return float64(typed)
	case uint16:
		return float64(typed)
	case uint32:
		return float64(typed)
	case uint64:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

func truthy(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "1" || strings.EqualFold(typed, "true") || strings.EqualFold(typed, "on")
	default:
		return numeric(value) != 0
	}
}

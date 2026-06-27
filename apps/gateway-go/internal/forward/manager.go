package forward

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

const (
	ProtocolNone         = "none"
	ProtocolModbusSlave  = "modbus-tcp-slave"
	ProtocolIEC104Server = "iec104-server"
)

type Manager struct {
	store  *state.Store
	mu     sync.RWMutex
	cfg    config.Config
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type forwardPoint struct {
	Config config.PointConfig
	Status state.PointStatus
	Value  interface{}
}

func NewManager(store *state.Store) *Manager {
	return &Manager{store: store}
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
	for _, channel := range cfg.Channels {
		if strings.EqualFold(channel.ForwardProtocol, protocol) {
			return true
		}
	}
	return false
}

func (m *Manager) points(protocol string) []forwardPoint {
	cfg := m.config()
	channelForward := map[string]string{}
	for _, channel := range cfg.Channels {
		channelForward[channel.ChannelKey] = channel.ForwardProtocol
	}
	statusByKey := map[string]state.PointStatus{}
	for _, point := range m.store.Snapshot().Points {
		statusByKey[point.DeviceKey+"::"+point.Metric] = point
	}
	var points []forwardPoint
	for _, point := range cfg.Points {
		if !strings.EqualFold(channelForward[point.ChannelKey], protocol) {
			continue
		}
		status := statusByKey[point.DeviceKey+"::"+point.Metric]
		if status.Error != "" || status.UpdatedAt == nil {
			continue
		}
		points = append(points, forwardPoint{Config: point, Status: status, Value: status.Value})
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
		response := m.modbusResponse(pdu)
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
	if len(pdu) < 5 {
		return modbusException(pdu, 0x03)
	}
	function := pdu[0]
	start := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])
	if quantity == 0 || quantity > 125 {
		return modbusException(pdu, 0x03)
	}
	switch function {
	case 1, 2:
		return m.modbusBits(function, start, quantity)
	case 3, 4:
		return m.modbusRegisters(function, start, quantity)
	default:
		return modbusException(pdu, 0x01)
	}
}

func (m *Manager) modbusBits(function byte, start uint16, quantity uint16) []byte {
	byteCount := int((quantity + 7) / 8)
	data := make([]byte, byteCount)
	for _, point := range m.points(ProtocolModbusSlave) {
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

func (m *Manager) modbusRegisters(function byte, start uint16, quantity uint16) []byte {
	registers := make([]uint16, quantity)
	for _, point := range m.points(ProtocolModbusSlave) {
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
	session := &iec104Session{conn: conn, commonAS: 1}
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
			_ = session.writeRaw([]byte{0x68, 0x04, 0x0b, 0x00, 0x00, 0x00})
			continue
		}
		if control == 0x13 {
			_ = session.writeRaw([]byte{0x68, 0x04, 0x23, 0x00, 0x00, 0x00})
			return
		}
		if control == 0x43 {
			_ = session.writeRaw([]byte{0x68, 0x04, 0x83, 0x00, 0x00, 0x00})
			continue
		}
		if control&0x01 == 0 {
			session.recvSeq = (binary.LittleEndian.Uint16(packet[2:4]) >> 1) + 1
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
	conn     net.Conn
	sendSeq  uint16
	recvSeq  uint16
	commonAS uint16
}

func (s *iec104Session) handleASDU(m *Manager, asdu []byte) {
	if len(asdu) < 10 || asdu[0] != 100 {
		_ = s.sendAck()
		return
	}
	commonAS := binary.LittleEndian.Uint16(asdu[4:6])
	if commonAS != 0 {
		s.commonAS = commonAS
	}
	_ = s.sendInterrogation(0x07)
	for _, point := range m.points(ProtocolIEC104Server) {
		_ = s.sendMeasuredValue(point)
	}
	_ = s.sendInterrogation(0x0a)
}

func (s *iec104Session) sendAck() error {
	recv := s.recvSeq << 1
	return s.writeRaw([]byte{0x68, 0x04, 0x01, 0x00, byte(recv), byte(recv >> 8)})
}

func (s *iec104Session) sendInterrogation(cot byte) error {
	asdu := []byte{100, 1, cot, 0, byte(s.commonAS), byte(s.commonAS >> 8), 0, 0, 0, 20}
	return s.writeI(asdu)
}

func (s *iec104Session) sendMeasuredValue(point forwardPoint) error {
	ioa := uint32(point.Config.Register)
	if ioa == 0 {
		ioa = 1
	}
	value := float32(numeric(point.Value))
	asdu := []byte{
		13, 1,
		0x14, 0,
		byte(s.commonAS), byte(s.commonAS >> 8),
		byte(ioa), byte(ioa >> 8), byte(ioa >> 16),
	}
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, math.Float32bits(value))
	asdu = append(asdu, raw...)
	asdu = append(asdu, 0)
	return s.writeI(asdu)
}

func (s *iec104Session) writeI(asdu []byte) error {
	send := s.sendSeq << 1
	recv := s.recvSeq << 1
	s.sendSeq++
	packet := make([]byte, 6+len(asdu))
	packet[0] = 0x68
	packet[1] = byte(4 + len(asdu))
	binary.LittleEndian.PutUint16(packet[2:4], send)
	binary.LittleEndian.PutUint16(packet[4:6], recv)
	copy(packet[6:], asdu)
	return s.writeRaw(packet)
}

func (s *iec104Session) writeRaw(packet []byte) error {
	_ = s.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_, err := s.conn.Write(packet)
	return err
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

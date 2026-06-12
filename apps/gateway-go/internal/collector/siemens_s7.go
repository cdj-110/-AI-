package collector

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/mapper"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type SiemensS7 struct{}

type s7Connection struct {
	conn net.Conn
	mu   sync.Mutex
	seq  uint16
}

type s7Endpoint struct {
	rack       uint8
	slot       uint8
	localTSAP  uint16
	remoteTSAP uint16
	label      string
}

type S7ConnectionResult struct {
	Address    string
	Endpoint   string
	LocalTSAP  string
	RemoteTSAP string
}

var s7Pool = struct {
	sync.Mutex
	items map[string]*s7Connection
}{items: map[string]*s7Connection{}}

func (SiemensS7) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	conn, err := getS7Connection(point)
	if err != nil {
		return model.PointValue{}, err
	}

	conn.mu.Lock()
	raw, err := conn.readBytes(ctx, point)
	conn.mu.Unlock()
	if err != nil {
		closeS7Connection(point)
		return model.PointValue{}, err
	}

	value, err := decodeS7Value(point, raw)
	if err != nil {
		return model.PointValue{}, err
	}
	return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
}

func getS7Connection(point config.PointConfig) (*s7Connection, error) {
	key := s7ConnectionKey(point)
	s7Pool.Lock()
	if conn := s7Pool.items[key]; conn != nil {
		s7Pool.Unlock()
		return conn, nil
	}
	s7Pool.Unlock()

	address := s7Address(point.Address)
	var lastErr error
	var s7 *s7Connection
	for _, endpoint := range s7EndpointCandidates(point) {
		conn, err := net.DialTimeout("tcp", address, 1500*time.Millisecond)
		if err != nil {
			lastErr = err
			continue
		}
		candidate := &s7Connection{conn: conn, seq: 1}
		_ = conn.SetDeadline(time.Now().Add(2500 * time.Millisecond))
		if err := candidate.handshake(endpoint); err != nil {
			lastErr = err
			_ = conn.Close()
			continue
		}
		_ = conn.SetDeadline(time.Time{})
		s7 = candidate
		break
	}
	if s7 == nil {
		return nil, fmt.Errorf("无法连接 Siemens S7 PLC %s，请检查 Rack/Slot、PUT/GET 和 102 端口：%w", address, lastErr)
	}

	s7Pool.Lock()
	if existing := s7Pool.items[key]; existing != nil {
		s7Pool.Unlock()
		_ = s7.conn.Close()
		return existing, nil
	}
	s7Pool.items[key] = s7
	s7Pool.Unlock()
	return s7, nil
}

func closeS7Connection(point config.PointConfig) {
	key := s7ConnectionKey(point)
	s7Pool.Lock()
	conn := s7Pool.items[key]
	delete(s7Pool.items, key)
	s7Pool.Unlock()
	if conn != nil {
		_ = conn.conn.Close()
	}
}

func TestS7Connection(ctx context.Context, point config.PointConfig) (S7ConnectionResult, error) {
	address := s7Address(point.Address)
	var lastErr error
	for _, endpoint := range s7EndpointCandidates(point) {
		dialer := net.Dialer{Timeout: 1500 * time.Millisecond}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			lastErr = fmt.Errorf("%s TCP 连接失败：%w", endpoint.label, err)
			continue
		}
		candidate := &s7Connection{conn: conn, seq: 1}
		_ = conn.SetDeadline(time.Now().Add(2500 * time.Millisecond))
		err = candidate.handshake(endpoint)
		_ = conn.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return S7ConnectionResult{
			Address:    address,
			Endpoint:   endpoint.label,
			LocalTSAP:  fmt.Sprintf("0x%04x", endpoint.localTSAP),
			RemoteTSAP: fmt.Sprintf("0x%04x", endpoint.remoteTSAP),
		}, nil
	}
	return S7ConnectionResult{Address: address}, fmt.Errorf("无法完成 Siemens S7 连接握手，TCP 端口已尝试但 PLC 未接受当前通信参数：%w", lastErr)
}

func s7ConnectionKey(point config.PointConfig) string {
	return fmt.Sprintf("%s#%d#%d#%s#%s", s7Address(point.Address), point.Rack, point.Slot, point.LocalTSAP, point.RemoteTSAP)
}

func s7Address(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return "127.0.0.1:102"
	}
	if strings.Contains(address, ":") {
		return address
	}
	return address + ":102"
}

func s7EndpointCandidates(point config.PointConfig) []s7Endpoint {
	var candidates []s7Endpoint
	if local, remote, ok := customTSAP(point); ok {
		candidates = append(candidates, s7Endpoint{rack: point.Rack, slot: point.Slot, localTSAP: local, remoteTSAP: remote, label: "custom-tsap"})
	}
	rack := point.Rack
	candidates = append(candidates, []s7Endpoint{
		{rack: rack, slot: 0, localTSAP: 0x0200, remoteTSAP: 0x0200, label: "smart200-tsap"},
		{rack: rack, slot: 0, localTSAP: 0x1000, remoteTSAP: 0x0300, label: "smart200-tsap-alt"},
		{rack: rack, slot: 0, localTSAP: 0x0300, remoteTSAP: 0x0300, label: "smart200-tsap-alt2"},
		{rack: rack, slot: 0, localTSAP: 0x1000, remoteTSAP: 0x1000, label: "smart200-tsap-alt3"},
	}...)
	if point.Slot != 0 {
		candidates = append(candidates, s7RackSlotEndpoint(rack, point.Slot))
	}
	candidates = append(candidates,
		s7RackSlotEndpoint(rack, 1),
		s7RackSlotEndpoint(rack, 2),
		s7RackSlotEndpoint(rack, 0),
		s7RackSlotEndpoint(rack, 3),
	)
	return dedupeS7Endpoints(candidates)
}

func dedupeS7Endpoints(candidates []s7Endpoint) []s7Endpoint {
	seen := map[string]bool{}
	var result []s7Endpoint
	for _, endpoint := range candidates {
		key := fmt.Sprintf("%d#%d#%04x#%04x", endpoint.rack, endpoint.slot, endpoint.localTSAP, endpoint.remoteTSAP)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, endpoint)
	}
	return result
}

func s7RackSlotEndpoint(rack uint8, slot uint8) s7Endpoint {
	return s7Endpoint{
		rack:       rack,
		slot:       slot,
		localTSAP:  0x0100,
		remoteTSAP: 0x0100 + uint16(rack)*0x20 + uint16(slot),
		label:      fmt.Sprintf("rack-slot-%d-%d", rack, slot),
	}
}

func customTSAP(point config.PointConfig) (uint16, uint16, bool) {
	local, localOK := parseTSAP(point.LocalTSAP)
	remote, remoteOK := parseTSAP(point.RemoteTSAP)
	return local, remote, localOK && remoteOK
}

func parseTSAP(value string) (uint16, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "0x")
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(parsed), true
}

func (c *s7Connection) handshake(endpoint s7Endpoint) error {
	if err := c.writePacket([]byte{
		0x03, 0x00, 0x00, 0x16,
		0x11, 0xe0, 0x00, 0x00, 0x00, 0x01, 0x00,
		0xc0, 0x01, 0x0a,
		0xc1, 0x02, byte(endpoint.localTSAP >> 8), byte(endpoint.localTSAP),
		0xc2, 0x02, byte(endpoint.remoteTSAP >> 8), byte(endpoint.remoteTSAP),
	}); err != nil {
		return err
	}
	if _, err := c.readPacket(); err != nil {
		return fmt.Errorf("Siemens S7 COTP 握手失败，%s localTSAP=0x%04x remoteTSAP=0x%04x：%w", endpoint.label, endpoint.localTSAP, endpoint.remoteTSAP, err)
	}
	if err := c.writePacket([]byte{
		0x03, 0x00, 0x00, 0x19,
		0x02, 0xf0, 0x80,
		0x32, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x08, 0x00, 0x00,
		0xf0, 0x00, 0x00, 0x01, 0x00, 0x01, 0x03, 0xc0,
	}); err != nil {
		return err
	}
	if _, err := c.readPacket(); err != nil {
		return fmt.Errorf("Siemens S7 通信参数协商失败，%s localTSAP=0x%04x remoteTSAP=0x%04x：%w", endpoint.label, endpoint.localTSAP, endpoint.remoteTSAP, err)
	}
	return nil
}

func (c *s7Connection) readBytes(ctx context.Context, point config.PointConfig) ([]byte, error) {
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetDeadline(deadline)
	} else {
		_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	}
	request, err := buildS7ReadRequest(c.nextSeq(), point)
	if err != nil {
		return nil, err
	}
	if err := c.writePacket(request); err != nil {
		return nil, err
	}
	response, err := c.readPacket()
	if err != nil {
		return nil, err
	}
	return parseS7ReadResponse(response)
}

func (c *s7Connection) nextSeq() uint16 {
	c.seq++
	if c.seq == 0 {
		c.seq = 1
	}
	return c.seq
}

func (c *s7Connection) writePacket(packet []byte) error {
	_, err := c.conn.Write(packet)
	return err
}

func (c *s7Connection) readPacket() ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, err
	}
	if header[0] != 0x03 {
		return nil, fmt.Errorf("invalid TPKT version %d", header[0])
	}
	length := int(binary.BigEndian.Uint16(header[2:4]))
	if length < 4 || length > 4096 {
		return nil, fmt.Errorf("invalid TPKT length %d", length)
	}
	body := make([]byte, length-4)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}

func buildS7ReadRequest(seq uint16, point config.PointConfig) ([]byte, error) {
	area, err := s7AreaCode(point.Area)
	if err != nil {
		return nil, err
	}
	size := s7ByteLength(point)
	transportSize := byte(0x02)
	if point.DataType == "bool" {
		transportSize = 0x01
		size = 1
	}
	bitAddress := uint32(point.Register) * 8
	if point.DataType == "bool" && point.BitIndex != nil {
		if *point.BitIndex > 7 {
			return nil, fmt.Errorf("Siemens S7 位索引必须在 0-7 之间")
		}
		bitAddress += uint32(*point.BitIndex)
	}

	return []byte{
		0x03, 0x00, 0x00, 0x1f,
		0x02, 0xf0, 0x80,
		0x32, 0x01, 0x00, 0x00, byte(seq >> 8), byte(seq),
		0x00, 0x0e, 0x00, 0x00,
		0x04, 0x01,
		0x12, 0x0a, 0x10, transportSize,
		byte(size >> 8), byte(size),
		byte(point.DBNumber >> 8), byte(point.DBNumber),
		area,
		byte(bitAddress >> 16), byte(bitAddress >> 8), byte(bitAddress),
	}, nil
}

func parseS7ReadResponse(packet []byte) ([]byte, error) {
	if len(packet) < 21 {
		return nil, fmt.Errorf("Siemens S7 响应过短")
	}
	s7 := packet[7:]
	if len(s7) < 12 || s7[0] != 0x32 {
		return nil, fmt.Errorf("Siemens S7 响应格式无效")
	}
	paramLength := int(binary.BigEndian.Uint16(s7[6:8]))
	dataStart := 10 + paramLength
	if dataStart+4 > len(s7) {
		return nil, fmt.Errorf("Siemens S7 响应数据区缺失")
	}
	item := s7[dataStart:]
	if item[0] != 0xff {
		if item[0] == 0x04 {
			return nil, fmt.Errorf("Siemens S7 读取失败，返回码 0x04：PLC 已建立连接但拒绝访问该数据区，请检查 SMART 200 是否启用 S7 通信/PUT-GET、数据区地址是否存在")
		}
		return nil, fmt.Errorf("Siemens S7 读取失败，返回码 0x%02x", item[0])
	}
	bitLength := int(binary.BigEndian.Uint16(item[2:4]))
	byteLength := (bitLength + 7) / 8
	if item[1] != 0x03 && bitLength%8 == 0 {
		byteLength = bitLength / 8
	}
	if byteLength <= 0 || 4+byteLength > len(item) {
		return nil, fmt.Errorf("Siemens S7 响应数据长度无效")
	}
	return item[4 : 4+byteLength], nil
}

func s7AreaCode(area string) (byte, error) {
	switch strings.ToUpper(strings.TrimSpace(area)) {
	case "", "DB", "V":
		return 0x84, nil
	case "M", "MK", "MERKER":
		return 0x83, nil
	case "I", "E", "INPUT":
		return 0x81, nil
	case "Q", "A", "OUTPUT":
		return 0x82, nil
	default:
		return 0, fmt.Errorf("unsupported Siemens S7 area %s", area)
	}
}

func s7ByteLength(point config.PointConfig) uint16 {
	switch point.DataType {
	case "uint32", "int32", "float32":
		return 4
	case "bool":
		return 1
	default:
		return 2
	}
}

func decodeS7Value(point config.PointConfig, raw []byte) (interface{}, error) {
	if point.DataType == "bool" {
		if len(raw) == 0 {
			return nil, fmt.Errorf("bool requires 1 byte")
		}
		if point.BitIndex == nil {
			return raw[0] != 0, nil
		}
		return raw[0]&(1<<*point.BitIndex) != 0, nil
	}
	point.ByteOrder = "big"
	point.WordOrder = "normal"
	return mapper.Decode(point, raw)
}

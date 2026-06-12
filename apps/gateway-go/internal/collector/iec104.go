package collector

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type IEC104 struct{}

type IEC104ConnectionResult struct {
	Address       string
	CommonAddress uint16
}

type iec104Client struct {
	conn     net.Conn
	mu       sync.Mutex
	values   map[uint32]interface{}
	sendSeq  uint16
	recvSeq  uint16
	commonAS uint16
	updated  time.Time
}

var iec104Pool = struct {
	sync.Mutex
	items map[string]*iec104Client
}{items: map[string]*iec104Client{}}

func (IEC104) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	client, err := getIEC104Client(point)
	if err != nil {
		return model.PointValue{}, err
	}
	ioa := uint32(point.Register)
	if ioa == 0 {
		return model.PointValue{}, fmt.Errorf("IEC104 IOA 信息对象地址不能为空")
	}
	_ = client.interrogate()

	deadline := time.Now().Add(4500 * time.Millisecond)
	for {
		client.mu.Lock()
		value, ok := client.values[ioa]
		client.mu.Unlock()
		if ok {
			value = applyIEC104Scale(point, value)
			return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
		}
		if time.Now().After(deadline) {
			return model.PointValue{}, fmt.Errorf("IEC104 IOA %d 暂无数据，请确认设备是否响应总召唤或有该点位", ioa)
		}
		select {
		case <-ctx.Done():
			return model.PointValue{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestIEC104Connection(ctx context.Context, address string, commonAS uint16) (IEC104ConnectionResult, error) {
	address = iec104Address(address)
	if commonAS == 0 {
		commonAS = 1
	}
	result := IEC104ConnectionResult{Address: address, CommonAddress: commonAS}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return result, fmt.Errorf("IEC104 连接失败 %s：%w", address, err)
	}
	defer conn.Close()
	client := &iec104Client{
		conn:     conn,
		values:   map[uint32]interface{}{},
		commonAS: commonAS,
	}
	if err := client.start(); err != nil {
		return result, err
	}
	return result, nil
}

func applyIEC104Scale(point config.PointConfig, value interface{}) interface{} {
	scale := point.Scale
	if scale == 0 {
		scale = 1
	}
	switch typed := value.(type) {
	case float32:
		return float64(typed)*scale + point.Offset
	case float64:
		return typed*scale + point.Offset
	case int16:
		return float64(typed)*scale + point.Offset
	case int:
		return float64(typed)*scale + point.Offset
	default:
		return value
	}
}

func getIEC104Client(point config.PointConfig) (*iec104Client, error) {
	address := iec104Address(point.Address)
	commonAS := uint16(point.SlaveID)
	if commonAS == 0 {
		commonAS = 1
	}
	key := fmt.Sprintf("%s#%d", address, commonAS)

	iec104Pool.Lock()
	if client := iec104Pool.items[key]; client != nil {
		iec104Pool.Unlock()
		return client, nil
	}
	iec104Pool.Unlock()

	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("IEC104 连接失败 %s：%w", address, err)
	}
	client := &iec104Client{
		conn:     conn,
		values:   map[uint32]interface{}{},
		commonAS: commonAS,
	}
	if err := client.start(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	go client.readLoop(key)
	_ = client.interrogate()

	iec104Pool.Lock()
	if existing := iec104Pool.items[key]; existing != nil {
		iec104Pool.Unlock()
		_ = conn.Close()
		return existing, nil
	}
	iec104Pool.items[key] = client
	iec104Pool.Unlock()
	return client, nil
}

func iec104Address(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return "127.0.0.1:2404"
	}
	if strings.Contains(address, ":") {
		return address
	}
	return address + ":2404"
}

func (c *iec104Client) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.conn.Write([]byte{0x68, 0x04, 0x07, 0x00, 0x00, 0x00}); err != nil {
		return fmt.Errorf("IEC104 STARTDT 发送失败：%w", err)
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	packet, err := readIEC104Packet(c.conn)
	_ = c.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return fmt.Errorf("IEC104 STARTDT 响应失败：%w", err)
	}
	if len(packet) < 6 || packet[2] != 0x0b {
		return fmt.Errorf("IEC104 STARTDT 响应无效")
	}
	return nil
}

func (c *iec104Client) interrogate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	asdu := []byte{
		100, 0x01,
		0x06, 0x00,
		byte(c.commonAS), byte(c.commonAS >> 8),
		0x00, 0x00, 0x00,
		20,
	}
	packet := c.iFrame(asdu)
	_, err := c.conn.Write(packet)
	return err
}

func (c *iec104Client) iFrame(asdu []byte) []byte {
	send := c.sendSeq << 1
	recv := c.recvSeq << 1
	c.sendSeq++
	packet := make([]byte, 6+len(asdu))
	packet[0] = 0x68
	packet[1] = byte(4 + len(asdu))
	binary.LittleEndian.PutUint16(packet[2:4], send)
	binary.LittleEndian.PutUint16(packet[4:6], recv)
	copy(packet[6:], asdu)
	return packet
}

func (c *iec104Client) readLoop(key string) {
	defer func() {
		_ = c.conn.Close()
		iec104Pool.Lock()
		delete(iec104Pool.items, key)
		iec104Pool.Unlock()
	}()
	for {
		packet, err := readIEC104Packet(c.conn)
		if err != nil {
			return
		}
		c.handlePacket(packet)
	}
}

func readIEC104Packet(reader io.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if header[0] != 0x68 {
		return nil, fmt.Errorf("IEC104 报文起始字节无效：0x%02x", header[0])
	}
	body := make([]byte, int(header[1]))
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}

func (c *iec104Client) handlePacket(packet []byte) {
	if len(packet) < 6 {
		return
	}
	control := packet[2]
	if control&0x01 == 0 {
		c.recvSeq = (binary.LittleEndian.Uint16(packet[2:4]) >> 1) + 1
		c.handleASDU(packet[6:])
		_ = c.sendAck()
	}
}

func (c *iec104Client) sendAck() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	recv := c.recvSeq << 1
	packet := []byte{0x68, 0x04, 0x01, 0x00, byte(recv), byte(recv >> 8)}
	_, err := c.conn.Write(packet)
	return err
}

func (c *iec104Client) handleASDU(asdu []byte) {
	if len(asdu) < 6 {
		return
	}
	typeID := asdu[0]
	vsq := asdu[1]
	count := int(vsq & 0x7f)
	sequence := vsq&0x80 != 0
	commonAS := binary.LittleEndian.Uint16(asdu[4:6])
	if commonAS != c.commonAS {
		return
	}

	offset := 6
	var baseIOA uint32
	for index := 0; index < count; index++ {
		var ioa uint32
		if sequence {
			if index == 0 {
				if offset+3 > len(asdu) {
					return
				}
				baseIOA = uint32(asdu[offset]) | uint32(asdu[offset+1])<<8 | uint32(asdu[offset+2])<<16
				offset += 3
			}
			ioa = baseIOA + uint32(index)
		} else {
			if offset+3 > len(asdu) {
				return
			}
			ioa = uint32(asdu[offset]) | uint32(asdu[offset+1])<<8 | uint32(asdu[offset+2])<<16
			offset += 3
		}
		value, size, ok := decodeIEC104Value(typeID, asdu[offset:])
		if !ok {
			return
		}
		offset += size
		c.mu.Lock()
		c.values[ioa] = value
		c.updated = time.Now()
		c.mu.Unlock()
	}
}

func decodeIEC104Value(typeID byte, raw []byte) (interface{}, int, bool) {
	switch typeID {
	case 1, 30:
		if len(raw) < 1 {
			return nil, 0, false
		}
		return raw[0]&0x01 == 1, iec104ValueSize(typeID, 1), true
	case 3, 31:
		if len(raw) < 1 {
			return nil, 0, false
		}
		return int(raw[0] & 0x03), iec104ValueSize(typeID, 1), true
	case 9, 34:
		if len(raw) < 3 {
			return nil, 0, false
		}
		return float64(int16(binary.LittleEndian.Uint16(raw[0:2]))) / 32768.0, iec104ValueSize(typeID, 3), true
	case 11, 35:
		if len(raw) < 3 {
			return nil, 0, false
		}
		return int16(binary.LittleEndian.Uint16(raw[0:2])), iec104ValueSize(typeID, 3), true
	case 13, 36:
		if len(raw) < 5 {
			return nil, 0, false
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(raw[0:4])), iec104ValueSize(typeID, 5), true
	default:
		return nil, 0, false
	}
}

func iec104ValueSize(typeID byte, base int) int {
	switch typeID {
	case 30, 31, 34, 35, 36:
		return base + 7
	default:
		return base
	}
}

package collector

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
)

type IEC104 struct{}

const iec104TestFrameInterval = 5 * time.Second
const iec104InitialDataGrace = 5 * time.Second
const iec104ShortInterrogationSession = 3 * time.Second
const iec104AckWindow = 3
const iec104AckDelay = 2 * time.Second
const iec104ReconnectGrace = 30 * time.Second

type IEC104ConnectionResult struct {
	Address       string
	CommonAddress uint16
}

type iec104Client struct {
	conn                     net.Conn
	deviceKey                string
	address                  string
	mu                       sync.Mutex
	values                   map[uint32]interface{}
	valueUpdated             map[uint32]time.Time
	sendSeq                  uint16
	recvSeq                  uint16
	commonAS                 uint16
	options                  config.IEC104Config
	done                     chan struct{}
	updated                  time.Time
	connectedAt              time.Time
	skipInitialInterrogation bool
	initialInterrogationSent bool
	unackedRecv              int
	ackTimer                 *time.Timer
	poolKey                  string
}

type iec104ConnectCall struct {
	done       chan struct{}
	client     *iec104Client
	err        error
	generation uint64
}

var iec104Pool = struct {
	sync.Mutex
	items              map[string]*iec104Client
	connecting         map[string]*iec104ConnectCall
	preferSpontaneous  map[string]bool
	forceInterrogation map[string]bool
	lastSuccess        map[string]time.Time
	generation         uint64
}{
	items:              map[string]*iec104Client{},
	connecting:         map[string]*iec104ConnectCall{},
	preferSpontaneous:  map[string]bool{},
	forceInterrogation: map[string]bool{},
	lastSuccess:        map[string]time.Time{},
}

func (IEC104) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	client, err := getIEC104Client(ctx, point)
	if err != nil {
		return model.PointValue{}, err
	}
	ioa := iec104PointIOA(point)
	if ioa == 0 {
		return model.PointValue{}, fmt.Errorf("IEC104 IOA 信息对象地址不能为空")
	}
	deadline := time.Now().Add(4500 * time.Millisecond)
	for {
		value, ok := client.value(ioa)
		if ok {
			value = applyIEC104Scale(point, value)
			return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
		}
		if time.Now().After(deadline) {
			return model.PointValue{}, fmt.Errorf("IEC104 IOA %d 暂无数据，请确认设备是否响应总召唤或有该点位", ioa)
		}
		select {
		case <-ctx.Done():
			if client.options.AcquisitionMode == "auto" && time.Since(client.connectedAt) < iec104InitialDataGrace {
				return model.PointValue{}, fmt.Errorf("%w: IEC104 自动兼容模式正在等待首个自发报文", ErrCollectionDeferred)
			}
			return model.PointValue{}, fmt.Errorf("IEC104 IOA %d 暂无数据，请确认设备是否响应总召或自发上送该点位", ioa)
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
		conn:         conn,
		address:      address,
		values:       map[uint32]interface{}{},
		valueUpdated: map[uint32]time.Time{},
		commonAS:     commonAS,
		done:         make(chan struct{}),
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

func getIEC104Client(ctx context.Context, point config.PointConfig) (result *iec104Client, resultErr error) {
	address := iec104Address(point.Address)
	commonAS := iec104CommonAddress(point)
	if commonAS == 0 {
		commonAS = 1
	}
	key := fmt.Sprintf("%s#%d", address, commonAS)
	options := iec104Options(point)

	iec104Pool.Lock()
	if client := iec104Pool.items[key]; client != nil {
		iec104Pool.Unlock()
		return client, nil
	}
	if call := iec104Pool.connecting[key]; call != nil {
		iec104Pool.Unlock()
		select {
		case <-call.done:
			return call.client, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &iec104ConnectCall{
		done:       make(chan struct{}),
		generation: iec104Pool.generation,
	}
	lastSuccess := iec104Pool.lastSuccess[key]
	skipInitialInterrogation := options.AcquisitionMode == "auto" && iec104Pool.preferSpontaneous[key] && !iec104Pool.forceInterrogation[key]
	iec104Pool.connecting[key] = call
	iec104Pool.Unlock()

	defer func() {
		iec104Pool.Lock()
		if call.generation != iec104Pool.generation && resultErr == nil {
			_ = result.conn.Close()
			result = nil
			resultErr = fmt.Errorf("IEC104 connection was closed while being established")
		}
		if resultErr == nil {
			iec104Pool.items[key] = result
		}
		call.client = result
		call.err = resultErr
		delete(iec104Pool.connecting, key)
		close(call.done)
		iec104Pool.Unlock()

		if resultErr == nil {
			go result.readLoop(key)
			result.startConfiguredCommands()
		}
	}()

	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		if options.AcquisitionMode == "auto" && !lastSuccess.IsZero() && time.Since(lastSuccess) < iec104ReconnectGrace {
			return nil, fmt.Errorf("%w: IEC104 正在重连 %s: %v", ErrCollectionDeferred, address, err)
		}
		return nil, fmt.Errorf("IEC104 连接失败 %s：%w", address, err)
	}
	client := &iec104Client{
		conn:                     conn,
		deviceKey:                point.DeviceKey,
		address:                  address,
		values:                   map[uint32]interface{}{},
		valueUpdated:             map[uint32]time.Time{},
		commonAS:                 commonAS,
		options:                  options,
		done:                     make(chan struct{}),
		connectedAt:              time.Now(),
		skipInitialInterrogation: skipInitialInterrogation,
		poolKey:                  key,
	}
	if err := client.start(); err != nil {
		_ = conn.Close()
		if options.AcquisitionMode == "auto" && !lastSuccess.IsZero() && time.Since(lastSuccess) < iec104ReconnectGrace {
			return nil, fmt.Errorf("%w: IEC104 正在恢复会话: %v", ErrCollectionDeferred, err)
		}
		return nil, err
	}
	return client, nil
}

func iec104CommonAddress(point config.PointConfig) uint16 {
	if point.CommonAddress != 0 {
		return point.CommonAddress
	}
	if point.SlaveID != 0 {
		return uint16(point.SlaveID)
	}
	return 1
}

func iec104PointIOA(point config.PointConfig) uint32 {
	if point.IOA != 0 {
		return point.IOA
	}
	return uint32(point.Register)
}

func iec104Options(point config.PointConfig) config.IEC104Config {
	if point.IEC104 != nil {
		options := *point.IEC104
		options.ApplyDefaults()
		return options
	}
	options := config.IEC104Config{}
	options.ApplyDefaults()
	return options
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
	start := []byte{0x68, 0x04, 0x07, 0x00, 0x00, 0x00}
	c.recordPacket("tx", start, "STARTDT 激活", nil)
	if _, err := c.conn.Write(start); err != nil {
		return fmt.Errorf("IEC104 STARTDT 发送失败：%w", err)
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	packet, err := readIEC104Packet(c.conn)
	c.recordPacket("rx", packet, "STARTDT 确认", err)
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
	asdu := []byte{
		100, 0x01,
		0x06, 0x00,
		byte(c.commonAS), byte(c.commonAS >> 8),
		0x00, 0x00, 0x00,
		20,
	}
	return c.sendASDU(asdu)
}

func (c *iec104Client) counterInterrogate() error {
	return c.sendASDU([]byte{
		101, 0x01,
		0x06, 0x00,
		byte(c.commonAS), byte(c.commonAS >> 8),
		0x00, 0x00, 0x00,
		0x05,
	})
}

func (c *iec104Client) clockSync(now time.Time) error {
	local := now.Local()
	milliseconds := uint16(local.Second()*1000 + local.Nanosecond()/int(time.Millisecond))
	dayOfWeek := int(local.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}
	cp56 := []byte{
		byte(milliseconds), byte(milliseconds >> 8),
		byte(local.Minute()), byte(local.Hour()),
		byte(local.Day() | dayOfWeek<<5), byte(local.Month()), byte(local.Year() % 100),
	}
	asdu := []byte{
		103, 0x01,
		0x06, 0x00,
		byte(c.commonAS), byte(c.commonAS >> 8),
		0x00, 0x00, 0x00,
	}
	return c.sendASDU(append(asdu, cp56...))
}

func (c *iec104Client) sendASDU(asdu []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	packet := c.iFrame(asdu)
	c.recordPacket("tx", packet, iec104PacketSummary(packet), nil)
	_, err := c.conn.Write(packet)
	return err
}

func (c *iec104Client) startConfiguredCommands() {
	if c.options.AcquisitionMode == "auto" && c.skipInitialInterrogation {
		c.recordPacket("rx", nil, "自动兼容：已转为长连接，等待设备自发上送", nil)
	}
	if c.options.GeneralInterrogationOnStart && !c.skipInitialInterrogation {
		if c.interrogate() == nil {
			c.mu.Lock()
			c.initialInterrogationSent = true
			c.mu.Unlock()
		}
	}
	if c.options.ClockSyncOnStart {
		_ = c.clockSync(time.Now())
	}
	if c.options.CounterInterrogationOnStart {
		_ = c.counterInterrogate()
	}
	if c.options.AcquisitionMode == "periodic" {
		c.startPeriodicCommand(c.options.GeneralInterrogationIntervalSeconds, c.interrogate)
	}
	c.startPeriodicCommand(c.options.ClockSyncIntervalSeconds, func() error { return c.clockSync(time.Now()) })
	c.startPeriodicCommand(c.options.CounterInterrogationIntervalSeconds, c.counterInterrogate)
	c.startTestFrames()
}

func (c *iec104Client) startTestFrames() {
	go func() {
		ticker := time.NewTicker(iec104TestFrameInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.sendUFrame(0x43, "TESTFR 激活"); err != nil {
					return
				}
			case <-c.done:
				return
			}
		}
	}()
}

func (c *iec104Client) startPeriodicCommand(seconds int, command func() error) {
	if seconds <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = command()
			case <-c.done:
				return
			}
		}
	}()
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
		c.mu.Lock()
		if c.ackTimer != nil {
			c.ackTimer.Stop()
			c.ackTimer = nil
		}
		c.mu.Unlock()
		_ = c.conn.Close()
		close(c.done)
		iec104Pool.Lock()
		if iec104Pool.items[key] == c {
			delete(iec104Pool.items, key)
		}
		iec104Pool.Unlock()
	}()
	for {
		packet, err := readIEC104Packet(c.conn)
		if err != nil {
			if c.shouldPreferSpontaneous(err) {
				iec104Pool.Lock()
				switchToSpontaneous := !iec104Pool.forceInterrogation[key]
				if switchToSpontaneous {
					iec104Pool.preferSpontaneous[key] = true
				}
				iec104Pool.Unlock()
				if switchToSpontaneous {
					c.recordPacket("rx", nil, "自动兼容：检测到总召后结束会话，下次连接改为接收自发上送", nil)
				}
			} else if c.shouldRestoreInterrogation(err) {
				iec104Pool.Lock()
				delete(iec104Pool.preferSpontaneous, key)
				iec104Pool.forceInterrogation[key] = true
				iec104Pool.Unlock()
				c.recordPacket("rx", nil, "自动兼容：自发会话未收到有效点位，已回退为总召方式", nil)
			}
			c.recordPacket("rx", nil, "接收失败", err)
			return
		}
		c.recordPacket("rx", packet, iec104PacketSummary(packet), nil)
		c.handlePacket(packet)
	}
}

func CloseIEC104Connections() {
	iec104Pool.Lock()
	items := iec104Pool.items
	iec104Pool.items = map[string]*iec104Client{}
	iec104Pool.preferSpontaneous = map[string]bool{}
	iec104Pool.forceInterrogation = map[string]bool{}
	iec104Pool.lastSuccess = map[string]time.Time{}
	iec104Pool.generation++
	iec104Pool.Unlock()
	for _, client := range items {
		_ = client.conn.Close()
	}
}

func (c *iec104Client) shouldPreferSpontaneous(err error) bool {
	if c.options.AcquisitionMode != "auto" || c.skipInitialInterrogation || !errors.Is(err, io.EOF) || time.Since(c.connectedAt) > iec104ShortInterrogationSession {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initialInterrogationSent && !c.updated.IsZero()
}

func (c *iec104Client) shouldRestoreInterrogation(err error) bool {
	if c.options.AcquisitionMode != "auto" || !c.skipInitialInterrogation || !errors.Is(err, io.EOF) || time.Since(c.connectedAt) > iec104ShortInterrogationSession {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.updated.IsZero()
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
		c.queueAck()
		return
	}
	if control == 0x43 {
		_ = c.sendUFrame(0x83, "TESTFR 确认")
	}
}

func (c *iec104Client) sendUFrame(control byte, summary string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	packet := []byte{0x68, 0x04, control, 0x00, 0x00, 0x00}
	c.recordPacket("tx", packet, summary, nil)
	_, err := c.conn.Write(packet)
	return err
}

func (c *iec104Client) sendAck() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendAckLocked()
}

func (c *iec104Client) sendAckLocked() error {
	recv := c.recvSeq << 1
	packet := []byte{0x68, 0x04, 0x01, 0x00, byte(recv), byte(recv >> 8)}
	c.recordPacket("tx", packet, "S帧确认", nil)
	_, err := c.conn.Write(packet)
	c.unackedRecv = 0
	if c.ackTimer != nil {
		c.ackTimer.Stop()
		c.ackTimer = nil
	}
	return err
}

func (c *iec104Client) queueAck() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unackedRecv++
	if c.unackedRecv >= iec104AckWindow {
		_ = c.sendAckLocked()
		return
	}
	if c.ackTimer == nil {
		c.ackTimer = time.AfterFunc(iec104AckDelay, c.flushAck)
	}
}

func (c *iec104Client) flushAck() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.unackedRecv > 0 {
		_ = c.sendAckLocked()
	}
}

func (c *iec104Client) recordPacket(direction string, packet []byte, summary string, err error) {
	frame := packetmonitor.Frame{Protocol: "iec104", Direction: direction, DeviceKey: c.deviceKey, Address: c.address, Summary: summary}
	if err != nil {
		frame.Error = err.Error()
	}
	packetmonitor.Record(frame, packet)
}

func iec104PacketSummary(packet []byte) string {
	if len(packet) < 6 {
		return "IEC104 报文"
	}
	if packet[2]&0x01 == 0 && len(packet) > 7 {
		typeID := packet[6]
		count := packet[7] & 0x7f
		if len(packet) >= 10 {
			cause := binary.LittleEndian.Uint16(packet[8:10]) & 0x3f
			return fmt.Sprintf("I帧 %s 对象数=%d 原因=%s", iec104TypeName(typeID), count, iec104CauseName(cause))
		}
		return fmt.Sprintf("I帧 %s 对象数=%d", iec104TypeName(typeID), count)
	}
	if packet[2]&0x03 == 1 {
		return "S帧确认"
	}
	switch packet[2] {
	case 0x07:
		return "STARTDT 激活"
	case 0x0b:
		return "STARTDT 确认"
	case 0x13:
		return "STOPDT 激活"
	case 0x23:
		return "STOPDT 确认"
	case 0x43:
		return "TESTFR 激活"
	case 0x83:
		return "TESTFR 确认"
	default:
		return "U帧控制"
	}
}

func iec104TypeName(typeID byte) string {
	switch typeID {
	case 1:
		return "单点信息(M_SP_NA_1)"
	case 3:
		return "双点信息(M_DP_NA_1)"
	case 9:
		return "归一化值(M_ME_NA_1)"
	case 11:
		return "标度化值(M_ME_NB_1)"
	case 13:
		return "短浮点数(M_ME_NC_1)"
	case 100:
		return "总召命令(C_IC_NA_1)"
	case 101:
		return "电度召唤(C_CI_NA_1)"
	case 103:
		return "时钟同步(C_CS_NA_1)"
	default:
		return fmt.Sprintf("ASDU类型=%d", typeID)
	}
}

func iec104CauseName(cause uint16) string {
	switch cause {
	case 3:
		return "自发(3)"
	case 6:
		return "激活(6)"
	case 7:
		return "激活确认(7)"
	case 10:
		return "激活终止(10)"
	case 20:
		return "响应总召(20)"
	default:
		return fmt.Sprintf("%d", cause)
	}
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
		now := time.Now()
		c.valueUpdated[ioa] = now
		c.updated = now
		c.mu.Unlock()
		if c.poolKey != "" {
			iec104Pool.Lock()
			iec104Pool.lastSuccess[c.poolKey] = now
			iec104Pool.Unlock()
		}
	}
}

func (c *iec104Client) valueAfter(ioa uint32, after time.Time) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values[ioa]
	return value, ok && c.valueUpdated[ioa].After(after)
}

func (c *iec104Client) value(ioa uint32) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values[ioa]
	return value, ok
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

package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goburrow/serial"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type DLT645 struct{}

type DLT645ConnectionResult struct {
	Address           string `json:"address"`
	MeterAddress      string `json:"meterAddress"`
	ConfiguredVersion string `json:"configuredVersion"`
	DetectedVersion   string `json:"detectedVersion"`
	ElapsedMS         int64  `json:"elapsedMilliseconds"`
}

type dlt645Frame struct {
	Address string
	Control byte
	Data    []byte // decoded data (0x33 removed)
	Raw     []byte
}

type dlt645Bus struct {
	port     serial.Port
	settings string
	mu       dlt645BusMutex
	// Protected by mu; survives port close/reopen and applies across all meters.
	resyncWindow    time.Duration
	resyncSensitive bool
}

var dlt645OpenPort = serial.Open

var dlt645Pool = struct {
	sync.Mutex
	items    map[string]*dlt645Bus
	versions map[string]string
}{items: map[string]*dlt645Bus{}, versions: map[string]string{}}

func (DLT645) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	if err := ctx.Err(); err != nil {
		return model.PointValue{}, err
	}
	point.ApplyDefaults()
	if point.DLT645 == nil {
		return model.PointValue{}, errors.New("DL/T 645 点位缺少协议配置")
	}
	versions := dlt645VersionsForPoint(point)
	var failures []error
	for _, version := range versions {
		value, actualType, err := readDLT645Version(ctx, point, version)
		if err == nil {
			if point.DLT645.Version == "auto" {
				cacheDLT645Version(point, version)
			}
			return model.PointValue{
				DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value,
				ActualDataType: actualType, Quality: model.QualityGood,
				ReceiveTimestamp: time.Now(), CollectionMode: "poll", Diagnostic: "DL/T 645-" + version,
			}, nil
		}
		failures = append(failures, fmt.Errorf("%s: %w", version, err))
		if ctx.Err() != nil {
			break
		}
	}
	return model.PointValue{}, fmt.Errorf("DL/T 645 读取失败（%w）", errors.Join(failures...))
}

func (DLT645) WritePoint(ctx context.Context, point config.PointConfig, value interface{}) error {
	point.ApplyDefaults()
	if point.DLT645 == nil || (point.DLT645.Access != "write" && point.DLT645.Access != "read_write") {
		return errors.New("DL/T 645 点位未配置写权限")
	}
	versions := dlt645VersionsForPoint(point)
	if len(versions) != 1 {
		return errors.New("DL/T 645 自动版本尚未识别，请先测试连接或完成一次读取")
	}
	version := versions[0]
	descriptor, di, err := resolveDLT645Descriptor(point, version)
	if err != nil {
		return err
	}
	if dlt645WriteIsRestricted(point, version) {
		return errors.New("DL/T 645 安全策略禁止校时、改地址、清零和拉合闸等高风险命令")
	}
	encoded, err := encodeDLT645Value(value, descriptor.ValueFormat, descriptor.DataLength, descriptor.Decimals)
	if err != nil {
		return fmt.Errorf("DL/T 645 写入值无效：%w", err)
	}
	password, err := fixedHex(point.DLT645.WritePassword, 4, "写密码")
	if err != nil {
		return err
	}
	data := append(append([]byte{}, di...), password...)
	control := byte(0x14)
	if version == "2007" {
		operator, operatorErr := fixedHex(point.DLT645.OperatorCode, 4, "操作者代码")
		if operatorErr != nil {
			return operatorErr
		}
		data = append(data, operator...)
	} else {
		control = 0x04
	}
	data = append(data, encoded...)
	response, err := transactDLT645(ctx, point, version, control, data)
	if err != nil {
		return fmt.Errorf("DL/T 645 写入失败：%w", err)
	}
	if response.Control&0x1f != control&0x1f {
		return fmt.Errorf("DL/T 645 写入响应控制码不匹配：0x%02X", response.Control)
	}
	return nil
}

func dlt645WriteIsRestricted(point config.PointConfig, version string) bool {
	if point.DLT645.ItemKey == "meter-date" || point.DLT645.ItemKey == "meter-time" {
		return true
	}
	di := strings.ToUpper(strings.TrimSpace(point.DLT645.DataIdentifier))
	if version == "1997" {
		return di == "C010" || di == "C011"
	}
	return di == "04000101" || di == "04000102" || di == "0400010C"
}

func TestDLT645Connection(ctx context.Context, point config.PointConfig) (DLT645ConnectionResult, error) {
	point.ApplyDefaults()
	result := DLT645ConnectionResult{Address: point.Address}
	if point.DLT645 == nil {
		return result, errors.New("缺少 DL/T 645 设备配置")
	}
	result.MeterAddress = point.DLT645.MeterAddress
	result.ConfiguredVersion = point.DLT645.Version
	probe := point
	probe.Metric = "__dlt645_probe"
	copyConfig := *point.DLT645
	copyConfig.ItemKey = "forward-active-total"
	copyConfig.DataIdentifier = ""
	copyConfig.Access = "read"
	probe.DLT645 = &copyConfig
	started := time.Now()
	_, err := (DLT645{}).ReadPoint(ctx, probe)
	result.ElapsedMS = time.Since(started).Milliseconds()
	if err != nil {
		return result, err
	}
	result.DetectedVersion = detectedDLT645Version(probe)
	if result.DetectedVersion == "" {
		result.DetectedVersion = point.DLT645.Version
	}
	return result, nil
}

// ReadDLT645Address uses the 2007 point-to-point address-read command. It must
// only be used with one meter connected, otherwise responses can collide.
func ReadDLT645Address(ctx context.Context, point config.PointConfig) (string, error) {
	point.ApplyDefaults()
	bus, err := getDLT645Bus(point)
	if err != nil {
		return "", err
	}
	if err := bus.mu.acquire(ctx); err != nil {
		return "", fmt.Errorf("等待串口总线取消，尚未发送读地址请求: %w", err)
	}
	defer bus.mu.Unlock()
	if err := ensureDLT645BusPortLocked(point, bus); err != nil {
		return "", err
	}
	request := buildDLT645FrameRaw(bytes.Repeat([]byte{0xaa}, 6), 0x13, nil, point.DLT645.WakeupBytes)
	response, err := exchangeDLT645(ctx, bus, point, "2007", request, "", 0x13)
	if err != nil {
		return "", fmt.Errorf("读取通信地址失败；DL/T 645-1997 不支持此广播命令，请手动输入表地址：%w", err)
	}
	return response.Address, nil
}

func readDLT645Version(ctx context.Context, point config.PointConfig, version string) (interface{}, string, error) {
	descriptor, di, err := resolveDLT645Descriptor(point, version)
	if err != nil {
		return nil, "", err
	}
	control := byte(0x11)
	if version == "1997" {
		control = 0x01
	}
	response, err := transactDLT645(ctx, point, version, control, di)
	if err != nil {
		return nil, "", err
	}
	if len(response.Data) < len(di) || !bytes.Equal(response.Data[:len(di)], di) {
		return nil, "", errors.New("响应中的数据标识与请求不一致")
	}
	payload := append([]byte{}, response.Data[len(di):]...)
	for fragments := 0; response.Control&0x20 != 0; fragments++ {
		if fragments >= 16 || len(payload) > 200 {
			return nil, "", errors.New("后续数据帧超过安全上限（16 帧 / 200 字节）")
		}
		if len(payload) == 0 {
			return nil, "", errors.New("后续帧标志存在但缺少帧序号")
		}
		sequence := payload[len(payload)-1]
		payload = payload[:len(payload)-1]
		nextControl := byte(0x12)
		if version == "1997" {
			nextControl = 0x02
		}
		response, err = transactDLT645(ctx, point, version, nextControl, append(append([]byte{}, di...), sequence))
		if err != nil {
			return nil, "", err
		}
		if len(response.Data) < len(di)+1 || !bytes.Equal(response.Data[:len(di)], di) {
			return nil, "", errors.New("后续响应中的数据标识不一致")
		}
		part := response.Data[len(di):]
		if response.Control&0x20 != 0 {
			part = part[:len(part)-1]
		}
		payload = append(payload, part...)
	}
	if len(payload) > 200 {
		return nil, "", errors.New("响应数据超过安全上限（200 字节）")
	}
	if descriptor.DataLength > 0 && len(payload) != descriptor.DataLength {
		return nil, "", fmt.Errorf("数据长度应为 %d 字节，实际为 %d 字节", descriptor.DataLength, len(payload))
	}
	value, actualType, err := decodeDLT645Value(payload, descriptor.ValueFormat, descriptor.Decimals)
	if err != nil {
		return nil, "", err
	}
	if number, ok := value.(float64); ok {
		value = number*point.Scale + point.Offset
	}
	return value, actualType, nil
}

func transactDLT645(ctx context.Context, point config.PointConfig, version string, control byte, data []byte) (dlt645Frame, error) {
	bus, err := getDLT645Bus(point)
	if err != nil {
		return dlt645Frame{}, err
	}
	addressBytes, err := encodeDLT645Address(point.DLT645.MeterAddress)
	if err != nil {
		return dlt645Frame{}, err
	}
	request := buildDLT645FrameRaw(addressBytes, control, data, point.DLT645.WakeupBytes)
	waitStarted := time.Now()
	if err := bus.mu.acquire(ctx); err != nil {
		failure := fmt.Errorf("等待串口总线 %s 已取消（等待 %dms，尚未发送请求）: %w", point.Address, time.Since(waitStarted).Milliseconds(), errors.Join(ErrCollectionDeferred, err))
		recordDLT645Frame(point, version, "rx", nil, control, failure)
		return dlt645Frame{}, failure
	}
	defer bus.mu.Unlock()
	var lastErr error
	for attempt := 0; attempt <= point.DLT645.Retries; attempt++ {
		if err := ctx.Err(); err != nil {
			return dlt645Frame{}, fmt.Errorf("采集任务已取消: %w", err)
		}
		if openErr := ensureDLT645BusPortLocked(point, bus); openErr != nil {
			lastErr = openErr
			break
		}
		response, exchangeErr := exchangeDLT645(ctx, bus, point, version, request, point.DLT645.MeterAddress, control)
		if exchangeErr == nil {
			return response, nil
		}
		lastErr = exchangeErr
		_ = closeDLT645BusLocked(point.Address, bus)
	}
	return dlt645Frame{}, lastErr
}

func exchangeDLT645(ctx context.Context, bus *dlt645Bus, point config.PointConfig, version string, request []byte, address string, requestControl byte) (dlt645Frame, error) {
	if err := ctx.Err(); err != nil {
		return dlt645Frame{}, err
	}
	if bus == nil || bus.port == nil {
		err := fmt.Errorf("DL/T 645 串口 %s 尚未打开或已断开", point.Address)
		recordDLT645Frame(point, version, "rx", nil, requestControl, err)
		return dlt645Frame{}, err
	}
	if err := drainDLT645Residual(ctx, bus, point, version, requestControl); err != nil {
		return dlt645Frame{}, err
	}
	requestFrame, _, complete, parseErr := parseDLT645Stream(request)
	if parseErr != nil || !complete {
		return dlt645Frame{}, errors.New("DL/T 645 请求帧无效，未发送")
	}
	// Only a matched response completes the exchange. A failed/cancelled send
	// leaves the bus quarantined even when the next caller uses another point.
	matched := false
	defer func() {
		if !matched {
			bus.resyncWindow = max(bus.resyncWindow, time.Duration(point.DLT645.ResponseTimeoutMilliseconds)*time.Millisecond, 100*time.Millisecond)
			bus.resyncSensitive = bus.resyncSensitive || requestControl == 0x14 || requestControl == 0x04
		}
	}()
	recordDLT645Frame(point, version, "tx", request, requestControl, nil)
	n, err := bus.port.Write(request)
	if err == nil && n != len(request) {
		err = io.ErrShortWrite
	}
	if err != nil {
		recordDLT645Frame(point, version, "rx", nil, requestControl, err)
		return dlt645Frame{}, fmt.Errorf("串口发送失败：%w", err)
	}
	started := time.Now()
	deadline := started.Add(time.Duration(point.DLT645.ResponseTimeoutMilliseconds) * time.Millisecond)
	buffer := make([]byte, 0, 256)
	chunk := make([]byte, 256)
	var lastRejected error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			failure := fmt.Errorf("等待电表响应时采集任务被取消（已发送，等待 %dms）: %w", time.Since(started).Milliseconds(), err)
			recordDLT645Frame(point, version, "rx", buffer, requestControl, failure)
			return dlt645Frame{}, failure
		}
		n, readErr := bus.port.Read(chunk)
		// A blocking driver read may return after the deadline. Do not accept a
		// late frame as a successful response for an already expired request.
		if !time.Now().Before(deadline) {
			if n > 0 {
				recordDLT645Frame(point, version, "rx", chunk[:n], requestControl, errors.New("已丢弃：超过当前请求响应期限的报文"))
			}
			break
		}
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			for {
				frame, consumed, complete, parseErr := parseDLT645Stream(buffer)
				if consumed > 0 {
					buffer = buffer[consumed:]
				}
				if parseErr != nil {
					lastRejected = parseErr
					continue
				}
				if !complete {
					break
				}
				if frame.Control&0x80 == 0 || frame.Control&0x1f != requestControl&0x1f || (address != "" && frame.Address != address) {
					lastRejected = errors.New("已忽略：回显或响应地址/控制码与当前请求不匹配")
					recordDLT645Frame(point, version, "rx", frame.Raw, frame.Control, lastRejected)
					continue
				}
				if frame.Control&0x40 != 0 {
					failure := dlt645Exception(frame.Data)
					recordDLT645Frame(point, version, "rx", frame.Raw, frame.Control, failure)
					return dlt645Frame{}, failure
				}
				if mismatch := matchDLT645ReadResponse(version, requestFrame, frame); mismatch != nil {
					lastRejected = mismatch
					recordDLT645Frame(point, version, "rx", frame.Raw, frame.Control, mismatch)
					continue // Keep waiting within the original, unextended deadline.
				}
				recordDLT645Frame(point, version, "rx", frame.Raw, frame.Control, nil)
				matched = true
				if len(buffer) > 0 {
					recordDLT645Frame(point, version, "rx", buffer, requestControl, errors.New("已丢弃：有效响应之后的残留报文"))
					bus.resyncWindow = 100 * time.Millisecond
					bus.resyncSensitive = requestControl == 0x14 || requestControl == 0x04
				}
				return frame, nil
			}
		}
		if readErr != nil && !errors.Is(readErr, serial.ErrTimeout) && !errors.Is(readErr, io.EOF) {
			recordDLT645Frame(point, version, "rx", buffer, requestControl, readErr)
			return dlt645Frame{}, fmt.Errorf("串口接收失败：%w", readErr)
		}
	}
	err = fmt.Errorf("等待电表 %s 响应超时（请求已发送，响应预算 %dms，实际等待 %dms）", address, point.DLT645.ResponseTimeoutMilliseconds, time.Since(started).Milliseconds())
	if lastRejected != nil {
		err = fmt.Errorf("%w；未收到匹配响应，最后忽略原因：%v", err, lastRejected)
	}
	recordDLT645Frame(point, version, "rx", buffer, requestControl, err)
	return dlt645Frame{}, err
}

func getDLT645Bus(point config.PointConfig) (*dlt645Bus, error) {
	settings := dlt645Settings(point)
	dlt645Pool.Lock()
	defer dlt645Pool.Unlock()
	if bus := dlt645Pool.items[point.Address]; bus != nil {
		if bus.settings != settings {
			return nil, fmt.Errorf("串口 %s 存在冲突参数：%s 与 %s", point.Address, bus.settings, settings)
		}
		return bus, nil
	}
	bus, err := openDLT645Bus(point)
	if err != nil {
		return nil, err
	}
	dlt645Pool.items[point.Address] = bus
	return bus, nil
}

func openDLT645Bus(point config.PointConfig) (*dlt645Bus, error) {
	// Poll in short slices so cancellation, retries and shutdown are not
	// blocked by a meter's full response timeout. Wire deadlines are per request.
	timeout := 100 * time.Millisecond
	port, err := dlt645OpenPort(&serial.Config{Address: point.Address, BaudRate: point.BaudRate, DataBits: point.DataBits, StopBits: point.StopBits, Parity: normalizeParity(point.Parity), Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("打开 DL/T 645 串口 %s 失败：%w", point.Address, err)
	}
	return &dlt645Bus{port: port, settings: dlt645Settings(point)}, nil
}

// ensureDLT645BusPortLocked reopens a shared serial port after a failed
// transaction. Callers must hold bus.mu: another point on the same RS-485 bus
// may already have obtained the shared bus and be waiting for that mutex while
// the current transaction closes the port. Checking and reopening only while
// holding bus.mu keeps that waiter from dereferencing a nil port.
func ensureDLT645BusPortLocked(point config.PointConfig, bus *dlt645Bus) error {
	if bus == nil {
		return fmt.Errorf("DL/T 645 串口 %s 总线未初始化", point.Address)
	}
	if bus.port != nil {
		return nil
	}
	opened, err := openDLT645Bus(point)
	if err != nil {
		return err
	}
	bus.port = opened.port
	bus.settings = opened.settings
	return nil
}

func dlt645Settings(point config.PointConfig) string {
	return fmt.Sprintf("%d-%d-%d-%s", point.BaudRate, point.DataBits, point.StopBits, normalizeParity(point.Parity))
}

func closeDLT645BusLocked(address string, bus *dlt645Bus) error {
	if bus.port == nil {
		return nil
	}
	err := bus.port.Close()
	bus.port = nil
	return err
}

func CloseDLT645Connections() {
	dlt645Pool.Lock()
	buses := make([]*dlt645Bus, 0, len(dlt645Pool.items))
	for _, bus := range dlt645Pool.items {
		buses = append(buses, bus)
	}
	dlt645Pool.items = map[string]*dlt645Bus{}
	dlt645Pool.versions = map[string]string{}
	dlt645Pool.Unlock()
	for _, bus := range buses {
		bus.mu.Lock()
		if bus.port != nil {
			_ = bus.port.Close()
		}
		bus.mu.Unlock()
	}
}

func dlt645VersionsForPoint(point config.PointConfig) []string {
	version := strings.ToLower(strings.TrimSpace(point.DLT645.Version))
	if version == "2007" || version == "1997" {
		return []string{version}
	}
	if detected := detectedDLT645Version(point); detected != "" {
		return []string{detected}
	}
	if point.DLT645.ItemKey == "" {
		switch len(strings.TrimSpace(point.DLT645.DataIdentifier)) {
		case 8:
			return []string{"2007"}
		case 4:
			return []string{"1997"}
		}
	}
	return []string{"2007", "1997"}
}

func dlt645VersionKey(point config.PointConfig) string {
	return point.Address + "::" + point.DLT645.MeterAddress
}

func detectedDLT645Version(point config.PointConfig) string {
	dlt645Pool.Lock()
	defer dlt645Pool.Unlock()
	return dlt645Pool.versions[dlt645VersionKey(point)]
}

func cacheDLT645Version(point config.PointConfig, version string) {
	dlt645Pool.Lock()
	dlt645Pool.versions[dlt645VersionKey(point)] = version
	dlt645Pool.Unlock()
}

func resolveDLT645Descriptor(point config.PointConfig, version string) (DLT645DataItem, []byte, error) {
	options := point.DLT645
	descriptor := DLT645DataItem{ValueFormat: options.ValueFormat, DataLength: options.DataLength, Decimals: point.Decimals, DataType: point.DataType, Access: options.Access}
	diText := options.DataIdentifier
	if options.ItemKey != "" {
		item, ok := findDLT645Item(options.ItemKey)
		if !ok {
			return descriptor, nil, fmt.Errorf("未知的标准数据项 %q", options.ItemKey)
		}
		descriptor = item
		if version == "2007" {
			diText = item.DI2007
		} else {
			diText = item.DI1997
		}
		// Never silently retarget a saved standard point after a dictionary fix.
		configured := strings.ToUpper(strings.TrimSpace(options.DataIdentifier))
		if configured != "" && len(configured) == len(diText) && configured != diText {
			return descriptor, nil, fmt.Errorf("标准点位 %s 的已保存 DI %s 与当前字典 %s 不一致，请重新选择标准点位或改为手动 DI", item.Name, configured, diText)
		}
	}
	want := 8
	if version == "1997" {
		want = 4
	}
	if len(diText) != want {
		return descriptor, nil, fmt.Errorf("数据项不支持 DL/T 645-%s（DI=%s）", version, diText)
	}
	di, err := hex.DecodeString(diText)
	if err != nil {
		return descriptor, nil, fmt.Errorf("数据标识 %q 无效：%w", diText, err)
	}
	reverseBytes(di)
	return descriptor, di, nil
}

func buildDLT645Frame(address string, control byte, data []byte, wakeup int) ([]byte, error) {
	addressBytes, err := encodeDLT645Address(address)
	if err != nil {
		return nil, err
	}
	return buildDLT645FrameRaw(addressBytes, control, data, wakeup), nil
}

func buildDLT645FrameRaw(address []byte, control byte, data []byte, wakeup int) []byte {
	frame := make([]byte, 0, wakeup+12+len(data))
	frame = append(frame, bytes.Repeat([]byte{0xfe}, wakeup)...)
	start := len(frame)
	frame = append(frame, 0x68)
	frame = append(frame, address...)
	frame = append(frame, 0x68, control, byte(len(data)))
	for _, value := range data {
		frame = append(frame, value+0x33)
	}
	var checksum byte
	for _, value := range frame[start:] {
		checksum += value
	}
	return append(frame, checksum, 0x16)
}

func parseDLT645Stream(buffer []byte) (dlt645Frame, int, bool, error) {
	start := bytes.IndexByte(buffer, 0x68)
	if start < 0 {
		return dlt645Frame{}, len(buffer), false, nil
	}
	if len(buffer)-start < 10 {
		return dlt645Frame{}, start, false, nil
	}
	if buffer[start+7] != 0x68 {
		return dlt645Frame{}, start + 1, false, errors.New("第二帧起始符错误")
	}
	length := int(buffer[start+9])
	total := 12 + length
	if len(buffer)-start < total {
		return dlt645Frame{}, start, false, nil
	}
	raw := append([]byte(nil), buffer[start:start+total]...)
	if raw[len(raw)-1] != 0x16 {
		return dlt645Frame{}, start + 1, false, errors.New("帧结束符错误")
	}
	var checksum byte
	for _, value := range raw[:len(raw)-2] {
		checksum += value
	}
	if checksum != raw[len(raw)-2] {
		return dlt645Frame{}, start + 1, false, errors.New("帧校验和错误")
	}
	data := append([]byte(nil), raw[10:10+length]...)
	for index := range data {
		data[index] -= 0x33
	}
	return dlt645Frame{Address: decodeDLT645Address(raw[1:7]), Control: raw[8], Data: data, Raw: raw}, start + total, true, nil
}

func encodeDLT645Address(address string) ([]byte, error) {
	if len(address) != 12 {
		return nil, errors.New("表地址必须为 12 位十进制数字")
	}
	result := make([]byte, 6)
	for index := 0; index < 6; index++ {
		position := 10 - index*2
		value, err := strconv.ParseUint(address[position:position+2], 10, 8)
		if err != nil {
			return nil, errors.New("表地址只能包含十进制数字")
		}
		result[index] = byte(value/10)<<4 | byte(value%10)
	}
	return result, nil
}

func decodeDLT645Address(address []byte) string {
	var builder strings.Builder
	for index := len(address) - 1; index >= 0; index-- {
		if address[index] == 0xaa {
			builder.WriteString("AA")
		} else {
			fmt.Fprintf(&builder, "%d%d", address[index]>>4, address[index]&0x0f)
		}
	}
	return builder.String()
}

func decodeDLT645Value(raw []byte, format string, decimals int) (interface{}, string, error) {
	switch format {
	case "bcd", "signed-bcd":
		negative := false
		copyRaw := append([]byte(nil), raw...)
		if format == "signed-bcd" && len(copyRaw) > 0 && copyRaw[len(copyRaw)-1]&0x80 != 0 {
			negative = true
			copyRaw[len(copyRaw)-1] &= 0x7f
		}
		var digits strings.Builder
		for index := len(copyRaw) - 1; index >= 0; index-- {
			high, low := copyRaw[index]>>4, copyRaw[index]&0x0f
			if high > 9 || low > 9 {
				return nil, "", fmt.Errorf("BCD 数据包含无效数字：% X", raw)
			}
			fmt.Fprintf(&digits, "%d%d", high, low)
		}
		integer, err := strconv.ParseUint(digits.String(), 10, 64)
		if err != nil {
			return nil, "", err
		}
		value := float64(integer) / math.Pow10(decimals)
		if negative {
			value = -value
		}
		return value, "float64", nil
	case "binary":
		if len(raw) > 8 {
			return nil, "", errors.New("二进制整数长度不能超过 8 字节")
		}
		var padded [8]byte
		copy(padded[:], raw)
		return binary.LittleEndian.Uint64(padded[:]), "uint64", nil
	case "ascii":
		return strings.TrimRight(string(raw), "\x00 "), "string", nil
	case "date", "time", "datetime":
		value, err := decodeDLT645DateTime(raw, format)
		return value, "string", err
	case "demand-time":
		if len(raw) != 8 {
			return nil, "", errors.New("最大需量及发生时间数据必须为 8 字节")
		}
		demand, _, err := decodeDLT645Value(raw[:3], "bcd", decimals)
		if err != nil {
			return nil, "", err
		}
		date := bcdByteValues(raw[3:])
		for _, value := range raw[3:] {
			if value>>4 > 9 || value&0x0f > 9 {
				return nil, "", fmt.Errorf("发生时间包含无效 BCD：% X", raw[3:])
			}
		}
		return map[string]interface{}{"demand": demand, "occurredAt": fmt.Sprintf("20%02d-%02d-%02d %02d:%02d", date[4], date[3], date[2], date[1], date[0])}, "object", nil
	case "raw-hex", "raw_hex":
		return strings.ToUpper(hex.EncodeToString(raw)), "string", nil
	default:
		return nil, "", fmt.Errorf("不支持的数据格式 %q", format)
	}
}

func encodeDLT645Value(value interface{}, format string, length, decimals int) ([]byte, error) {
	if length < 1 || length > 200 || decimals < 0 || decimals > 9 {
		return nil, errors.New("数据长度必须为 1～200 字节，小数位必须为 0～9")
	}
	if format == "ascii" {
		raw := []byte(fmt.Sprint(value))
		if length > 0 && len(raw) != length {
			return nil, fmt.Errorf("ASCII 长度应为 %d 字节", length)
		}
		return raw, nil
	}
	if format == "binary" {
		if length <= 0 || length > 8 {
			return nil, errors.New("二进制整数长度必须为 1～8 字节")
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if number, ok := value.(json.Number); ok {
			text = number.String()
		}
		number, err := strconv.ParseUint(text, 10, length*8)
		if err != nil {
			return nil, fmt.Errorf("%q 不是有效的 %d 位无符号整数", text, length*8)
		}
		raw := make([]byte, length)
		for index := range raw {
			raw[index] = byte(number >> (8 * index))
		}
		return raw, nil
	}
	if format == "raw-hex" || format == "raw_hex" || format == "date" || format == "time" || format == "datetime" {
		text := strings.ReplaceAll(strings.TrimSpace(fmt.Sprint(value)), " ", "")
		raw, err := hex.DecodeString(text)
		if err != nil {
			return nil, errors.New("请输入偶数位十六进制原始数据")
		}
		if length > 0 && len(raw) != length {
			return nil, fmt.Errorf("数据长度应为 %d 字节", length)
		}
		if format == "date" || format == "time" || format == "datetime" {
			if _, err := decodeDLT645DateTime(raw, format); err != nil {
				return nil, err
			}
		}
		return raw, nil
	}
	if format != "bcd" && format != "signed-bcd" {
		return nil, fmt.Errorf("不支持的数据格式 %q", format)
	}
	text := fmt.Sprint(value)
	if number, ok := value.(json.Number); ok {
		text = number.String()
	}
	if len(text) > 512 {
		return nil, errors.New("数字输入过长")
	}
	finite, finiteErr := strconv.ParseFloat(text, 64)
	if finiteErr != nil || math.IsNaN(finite) || math.IsInf(finite, 0) {
		return nil, errors.New("请输入有限数值")
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("%q 不是有效数字", text)
	}
	negative := number.Sign() < 0
	if negative && format != "signed-bcd" {
		return nil, errors.New("该数据项不允许负数")
	}
	number.Abs(number)
	number.Mul(number, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)))
	// Round exact decimal input without float-to-uint overflow or lost digits.
	scaled, remainder := new(big.Int), new(big.Int)
	scaled.QuoRem(number.Num(), number.Denom(), remainder)
	if remainder.Lsh(remainder, 1).Cmp(number.Denom()) >= 0 {
		scaled.Add(scaled, big.NewInt(1))
	}
	digits := scaled.String()
	if len(digits) > length*2 {
		return nil, fmt.Errorf("数值超出 %d 字节 BCD 范围", length)
	}
	digits = strings.Repeat("0", length*2-len(digits)) + digits
	if format == "signed-bcd" && digits[0] >= '8' {
		return nil, fmt.Errorf("数值超出 %d 字节带符号 BCD 范围（最高位保留为符号位）", length)
	}
	raw := make([]byte, length)
	for index := 0; index < length; index++ {
		position := len(digits) - 2 - index*2
		pair, _ := strconv.ParseUint(digits[position:position+2], 10, 8)
		raw[index] = byte(pair/10)<<4 | byte(pair%10)
	}
	if negative {
		if format != "signed-bcd" {
			return nil, errors.New("该数据项不允许负数")
		}
		raw[len(raw)-1] |= 0x80
	}
	return raw, nil
}

// Manual datetime layout: ss mm hh DD MM YY; the seven-byte variant has
// weekday between hh and DD. Date is DD MM YY or weekday DD MM YY.
func decodeDLT645DateTime(raw []byte, format string) (string, error) {
	validLength := (format == "date" && (len(raw) == 3 || len(raw) == 4)) || (format == "time" && len(raw) == 3) || (format == "datetime" && (len(raw) == 6 || len(raw) == 7))
	if !validLength {
		return "", fmt.Errorf("%s 数据长度不正确", format)
	}
	for _, b := range raw {
		if b>>4 > 9 || b&15 > 9 {
			return "", errors.New("日期时间包含无效 BCD")
		}
	}
	v := bcdByteValues(raw)
	if format == "time" {
		if v[0] > 59 || v[1] > 59 || v[2] > 23 {
			return "", errors.New("时间超出有效范围")
		}
		return fmt.Sprintf("%02d:%02d:%02d", v[2], v[1], v[0]), nil
	}
	if (format == "date" && len(v) == 4 && v[0] > 6) || (format == "datetime" && len(v) == 7 && v[3] > 6) {
		return "", errors.New("星期超出有效范围")
	}
	n := len(v)
	day, month, year := v[n-3], v[n-2], 2000+v[n-1]
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return "", errors.New("日期超出有效范围")
	}
	result := date.Format("2006-01-02")
	if format == "datetime" {
		clock, err := decodeDLT645DateTime(raw[:3], "time")
		if err != nil {
			return "", err
		}
		result += " " + clock
	}
	return result, nil
}

func fixedHex(value string, length int, label string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "***" {
		return nil, fmt.Errorf("%s必须配置为 %d 字节十六进制", label, length)
	}
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != length {
		return nil, fmt.Errorf("%s必须是 %d 字节（%d 位）十六进制", label, length, length*2)
	}
	reverseBytes(raw)
	return raw, nil
}

func bcdByteValues(raw []byte) []int {
	result := make([]int, len(raw))
	for index, value := range raw {
		result[index] = int(value>>4)*10 + int(value&0x0f)
	}
	return result
}

func reverseBytes(raw []byte) {
	for left, right := 0, len(raw)-1; left < right; left, right = left+1, right-1 {
		raw[left], raw[right] = raw[right], raw[left]
	}
}

func dlt645Exception(data []byte) error {
	if len(data) == 0 {
		return errors.New("电表返回异常但未提供错误码")
	}
	code := data[0]
	var reasons []string
	for bit, text := range map[byte]string{0x01: "其他错误", 0x02: "无请求数据", 0x04: "密码错误或未授权", 0x08: "通信速率不能更改", 0x10: "年时区数超限", 0x20: "日时段数超限", 0x40: "费率数超限"} {
		if code&bit != 0 {
			reasons = append(reasons, text)
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "未知异常")
	}
	return fmt.Errorf("电表异常响应 0x%02X：%s", code, strings.Join(reasons, "、"))
}

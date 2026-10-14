package collector

import (
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
)

func recordDLT645Frame(point config.PointConfig, version, direction string, raw []byte, control byte, err error) {
	di := "-"
	meterAddress := "-"
	if point.DLT645 != nil {
		meterAddress = point.DLT645.MeterAddress
		di = point.DLT645.DataIdentifier
		if item, ok := findDLT645Item(point.DLT645.ItemKey); ok {
			if version == "1997" {
				di = item.DI1997
			} else {
				di = item.DI2007
			}
		}
	}
	checksum := "无完整帧"
	parsed, _, complete, parseErr := parseDLT645Stream(raw)
	if parseErr != nil {
		checksum = "失败"
	} else if complete {
		checksum = "通过"
	}
	summary := fmt.Sprintf("DL/T 645-%s %s 表地址=%s 控制码=0x%02X DI=%s 长度=%d 校验=%s", version, map[string]string{"tx": "请求", "rx": "响应"}[direction], meterAddress, control, di, len(raw), checksum)
	if direction == "rx" && complete && parseErr == nil && parsed.Control&0x40 == 0 {
		size := 4
		if version == "1997" {
			size = 2
		}
		if (parsed.Control&0x1f == 0x11 || parsed.Control&0x1f == 0x12 || parsed.Control&0x1f == 0x01 || parsed.Control&0x1f == 0x02) && len(parsed.Data) >= size {
			summary += " 收到DI=" + dlt645DIText(parsed.Data[:size])
		}
	}
	frame := packetmonitor.Frame{Protocol: "dlt645", Direction: direction, DeviceKey: point.DeviceKey, Address: point.Address, Summary: summary, Event: "DLT645", Mode: "Poll", ItemID: di}
	if control&0x1f == 0x14 || control&0x1f == 0x04 || (complete && (parsed.Control&0x1f == 0x14 || parsed.Control&0x1f == 0x04)) {
		frame.Summary += "（写入凭据及原始帧已脱敏）"
		frame.ValueText = "***"
		if err != nil {
			frame.Error = err.Error()
		}
		packetmonitor.Record(frame, nil)
		return
	}
	if len(raw) > 0 {
		frame.ValueText = strings.ToUpper(hex.EncodeToString(raw))
	}
	if err != nil {
		frame.Error = err.Error()
	}
	packetmonitor.Record(frame, raw)
}

func recordOPCDAOnDataChange(deviceKey, address, itemID string, value interface{}, quality model.PointQuality, sourceTime, receiveTime time.Time, initial bool, err error) {
	valueText := formatOPCDAMonitorValue(value)
	summary := fmt.Sprintf(
		"[OPCDA][ASYNC] OnDataChange ItemID=%s Value=%s Quality=%s SourceTime=%s ReceiveTime=%s",
		itemID, valueText, quality,
		formatOPCDAMonitorTime(sourceTime), formatOPCDAMonitorTime(receiveTime),
	)
	frame := packetmonitor.Frame{
		Protocol: "opcda", Direction: "rx", DeviceKey: deviceKey,
		Address: address, Summary: summary, Event: "OnDataChange", Mode: "Async",
		Initial: initial, ItemID: itemID, ValueText: valueText, Quality: string(quality),
	}
	if !sourceTime.IsZero() {
		value := sourceTime
		frame.SourceTimestamp = &value
	}
	if !receiveTime.IsZero() {
		value := receiveTime
		frame.ReceiveTimestamp = &value
	}
	if !sourceTime.IsZero() && !receiveTime.IsZero() {
		latency := receiveTime.Sub(sourceTime).Milliseconds()
		frame.CallbackLatencyMilliseconds = &latency
	}
	if err != nil {
		frame.Error = err.Error()
	}
	log.Print(summary)
	packetmonitor.Record(frame, nil)
}

func recordOPCDASyncRead(deviceKey, address, source string, itemCount int, direction string, err error) {
	summary := fmt.Sprintf("[OPCDA][SYNC] IOPCSyncIO::Read Source=%s Items=%d", source, itemCount)
	if direction == "rx" {
		summary += " Completed"
	}
	frame := packetmonitor.Frame{
		Protocol: "opcda", Direction: direction, DeviceKey: deviceKey,
		Address: address, Summary: summary, Event: "IOPCSyncIO::Read", Mode: "Sync",
	}
	if err != nil {
		frame.Error = err.Error()
	}
	packetmonitor.Record(frame, nil)
}

func formatOPCDAMonitorValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	case time.Time:
		return typed.Format("2006-01-02 15:04:05.000")
	default:
		return fmt.Sprint(value)
	}
}

func formatOPCDAMonitorTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05.000")
}

func recordModbusReadRequest(protocol string, point config.PointConfig) {
	payload := []byte{point.SlaveID, byte(point.Function), byte(point.Register >> 8), byte(point.Register), byte(point.Quantity >> 8), byte(point.Quantity)}
	if protocol == "modbus-rtu" {
		payload = appendModbusCRC(payload)
	}
	packetmonitor.Record(packetmonitor.Frame{
		Protocol: protocol, Direction: "tx", DeviceKey: point.DeviceKey, Address: point.Address,
		Summary: fmt.Sprintf("读请求 功能码=%d 起始地址=%d 数量=%d", point.Function, point.Register, point.Quantity),
	}, payload)
}

func recordModbusReadResponse(protocol string, point config.PointConfig, raw []byte, err error) {
	frame := packetmonitor.Frame{
		Protocol: protocol, Direction: "rx", DeviceKey: point.DeviceKey, Address: point.Address,
		Summary: fmt.Sprintf("读响应 功能码=%d 数据长度=%d", point.Function, len(raw)),
	}
	if err != nil {
		frame.Summary = fmt.Sprintf("读响应失败 功能码=%d", point.Function)
		frame.Error = err.Error()
		packetmonitor.Record(frame, nil)
		return
	}
	payload := append([]byte{point.SlaveID, byte(point.Function), byte(len(raw))}, raw...)
	if protocol == "modbus-rtu" {
		payload = appendModbusCRC(payload)
	}
	packetmonitor.Record(frame, payload)
}

func recordModbusWrite(protocol string, point config.PointConfig, value interface{}, err error) {
	frame := packetmonitor.Frame{
		Protocol: protocol, Direction: "tx", DeviceKey: point.DeviceKey, Address: point.Address,
		Summary: fmt.Sprintf("写请求 功能码=%d 地址=%d 值=%v", point.Function, point.Register, value),
	}
	packetmonitor.Record(frame, []byte{point.SlaveID, byte(point.Function), byte(point.Register >> 8), byte(point.Register)})
	response := packetmonitor.Frame{
		Protocol: protocol, Direction: "rx", DeviceKey: point.DeviceKey, Address: point.Address,
		Summary: fmt.Sprintf("写响应 功能码=%d 地址=%d", point.Function, point.Register),
	}
	if err != nil {
		response.Error = err.Error()
	}
	packetmonitor.Record(response, nil)
}

func recordSemanticRequest(protocol string, point config.PointConfig, summary string) {
	packetmonitor.Record(packetmonitor.Frame{Protocol: protocol, Direction: "tx", DeviceKey: point.DeviceKey, Address: point.Address, Summary: summary}, nil)
}

func recordSemanticResponse(protocol string, point config.PointConfig, summary string, err error) {
	frame := packetmonitor.Frame{Protocol: protocol, Direction: "rx", DeviceKey: point.DeviceKey, Address: point.Address, Summary: summary}
	if err != nil {
		frame.Error = err.Error()
	}
	packetmonitor.Record(frame, nil)
}

func appendModbusCRC(payload []byte) []byte {
	crc := uint16(0xffff)
	for _, value := range payload {
		crc ^= uint16(value)
		for bit := 0; bit < 8; bit++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xa001
			} else {
				crc >>= 1
			}
		}
	}
	return append(payload, byte(crc), byte(crc>>8))
}

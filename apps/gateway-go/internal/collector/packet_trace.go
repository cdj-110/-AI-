package collector

import (
	"fmt"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
)

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

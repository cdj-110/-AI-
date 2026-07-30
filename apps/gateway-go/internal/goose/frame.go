package goose

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

const EtherType uint16 = 0x88b8

type Value struct {
	Kind  string      `json:"kind"`
	Value interface{} `json:"value"`
}

type Message struct {
	AppID             uint16
	GoCBRef           string
	TimeAllowedToLive uint32
	DataSetRef        string
	GoID              string
	Timestamp         time.Time
	StateNumber       uint32
	SequenceNumber    uint32
	Test              bool
	ConfRev           uint32
	NeedsCommission   bool
	Values            []Value
}

func DecodeFrame(frame []byte) (Message, error) {
	var message Message
	offset := 14
	if len(frame) < offset {
		return message, fmt.Errorf("GOOSE Ethernet frame is too short")
	}
	etherType := binary.BigEndian.Uint16(frame[12:14])
	if etherType == 0x8100 {
		offset = 18
		if len(frame) < offset {
			return message, fmt.Errorf("GOOSE VLAN frame is too short")
		}
		etherType = binary.BigEndian.Uint16(frame[16:18])
	}
	if etherType != EtherType || len(frame) < offset+8 {
		return message, fmt.Errorf("not an IEC61850 GOOSE frame")
	}
	message.AppID = binary.BigEndian.Uint16(frame[offset : offset+2])
	length := int(binary.BigEndian.Uint16(frame[offset+2 : offset+4]))
	if length < 8 || offset+length > len(frame) {
		return message, fmt.Errorf("invalid GOOSE frame length %d", length)
	}
	apdu := frame[offset+8 : offset+length]
	tag, content, rest, err := readTLV(apdu)
	if err != nil || tag != 0x61 || len(rest) != 0 {
		return message, fmt.Errorf("invalid GOOSE APDU")
	}
	for len(content) > 0 {
		var field []byte
		tag, field, content, err = readTLV(content)
		if err != nil {
			return message, err
		}
		switch tag {
		case 0x80:
			message.GoCBRef = string(field)
		case 0x81:
			message.TimeAllowedToLive = uint32(decodeUnsigned(field))
		case 0x82:
			message.DataSetRef = string(field)
		case 0x83:
			message.GoID = string(field)
		case 0x84:
			message.Timestamp = decodeUTCTime(field)
		case 0x85:
			message.StateNumber = uint32(decodeUnsigned(field))
		case 0x86:
			message.SequenceNumber = uint32(decodeUnsigned(field))
		case 0x87:
			message.Test = decodeBool(field)
		case 0x88:
			message.ConfRev = uint32(decodeUnsigned(field))
		case 0x89:
			message.NeedsCommission = decodeBool(field)
		case 0xab:
			message.Values, err = decodeValues(field)
			if err != nil {
				return message, err
			}
		}
	}
	return message, nil
}

func EncodeFrame(message Message, sourceMAC, destinationMAC net.HardwareAddr, vlanID uint16, vlanPriority uint8) ([]byte, error) {
	if len(sourceMAC) != 6 || len(destinationMAC) != 6 {
		return nil, fmt.Errorf("source and destination MAC must contain 6 bytes")
	}
	values := make([]byte, 0)
	for _, value := range message.Values {
		encoded, err := encodeValue(value)
		if err != nil {
			return nil, err
		}
		values = append(values, encoded...)
	}
	apduContent := make([]byte, 0)
	apduContent = append(apduContent, encodeTLV(0x80, []byte(message.GoCBRef))...)
	apduContent = append(apduContent, encodeTLV(0x81, encodeUnsigned(uint64(message.TimeAllowedToLive)))...)
	apduContent = append(apduContent, encodeTLV(0x82, []byte(message.DataSetRef))...)
	apduContent = append(apduContent, encodeTLV(0x83, []byte(message.GoID))...)
	apduContent = append(apduContent, encodeTLV(0x84, encodeUTCTime(message.Timestamp))...)
	apduContent = append(apduContent, encodeTLV(0x85, encodeUnsigned(uint64(message.StateNumber)))...)
	apduContent = append(apduContent, encodeTLV(0x86, encodeUnsigned(uint64(message.SequenceNumber)))...)
	apduContent = append(apduContent, encodeTLV(0x87, encodeBool(message.Test))...)
	apduContent = append(apduContent, encodeTLV(0x88, encodeUnsigned(uint64(message.ConfRev)))...)
	apduContent = append(apduContent, encodeTLV(0x89, encodeBool(message.NeedsCommission))...)
	apduContent = append(apduContent, encodeTLV(0x8a, encodeUnsigned(uint64(len(message.Values))))...)
	apduContent = append(apduContent, encodeTLV(0xab, values)...)
	apdu := encodeTLV(0x61, apduContent)
	ethernetLength := 14
	if vlanID > 0 {
		ethernetLength = 18
	}
	frame := make([]byte, ethernetLength+8+len(apdu))
	copy(frame[0:6], destinationMAC)
	copy(frame[6:12], sourceMAC)
	offset := 14
	if vlanID > 0 {
		binary.BigEndian.PutUint16(frame[12:14], 0x8100)
		tci := (uint16(vlanPriority&7) << 13) | (vlanID & 0x0fff)
		binary.BigEndian.PutUint16(frame[14:16], tci)
		binary.BigEndian.PutUint16(frame[16:18], EtherType)
		offset = 18
	} else {
		binary.BigEndian.PutUint16(frame[12:14], EtherType)
	}
	binary.BigEndian.PutUint16(frame[offset:offset+2], message.AppID)
	binary.BigEndian.PutUint16(frame[offset+2:offset+4], uint16(8+len(apdu)))
	copy(frame[offset+8:], apdu)
	return frame, nil
}

func decodeValues(content []byte) ([]Value, error) {
	values := make([]Value, 0)
	for len(content) > 0 {
		tag, field, rest, err := readTLV(content)
		if err != nil {
			return nil, err
		}
		content = rest
		switch tag {
		case 0x83:
			values = append(values, Value{Kind: "bool", Value: decodeBool(field)})
		case 0x84:
			values = append(values, Value{Kind: "uint32", Value: decodeUnsigned(field)})
		case 0x85:
			values = append(values, Value{Kind: "int64", Value: decodeSigned(field)})
		case 0x86:
			values = append(values, Value{Kind: "uint64", Value: decodeUnsigned(field)})
		case 0x87:
			if len(field) == 5 {
				values = append(values, Value{Kind: "float32", Value: float64(math.Float32frombits(binary.BigEndian.Uint32(field[1:])))})
			} else if len(field) == 9 {
				values = append(values, Value{Kind: "float64", Value: math.Float64frombits(binary.BigEndian.Uint64(field[1:]))})
			} else {
				return nil, fmt.Errorf("unsupported GOOSE floating-point length %d", len(field))
			}
		case 0x8a, 0x90:
			values = append(values, Value{Kind: "string", Value: string(field)})
		case 0x91:
			values = append(values, Value{Kind: "timestamp", Value: decodeUTCTime(field).UnixMilli()})
		default:
			values = append(values, Value{Kind: fmt.Sprintf("tag-0x%02x", tag), Value: append([]byte(nil), field...)})
		}
	}
	return values, nil
}

func encodeValue(value Value) ([]byte, error) {
	switch value.Kind {
	case "bool":
		typed, _ := value.Value.(bool)
		return encodeTLV(0x83, encodeBool(typed)), nil
	case "int8", "int16", "int32", "int64":
		return encodeTLV(0x85, encodeSigned(asInt64(value.Value))), nil
	case "uint8", "uint16", "uint32", "uint64":
		return encodeTLV(0x86, encodeUnsigned(asUint64(value.Value))), nil
	case "float32":
		raw := make([]byte, 5)
		raw[0] = 8
		binary.BigEndian.PutUint32(raw[1:], math.Float32bits(float32(asFloat64(value.Value))))
		return encodeTLV(0x87, raw), nil
	case "float64":
		raw := make([]byte, 9)
		raw[0] = 11
		binary.BigEndian.PutUint64(raw[1:], math.Float64bits(asFloat64(value.Value)))
		return encodeTLV(0x87, raw), nil
	case "string":
		return encodeTLV(0x8a, []byte(fmt.Sprint(value.Value))), nil
	default:
		return nil, fmt.Errorf("unsupported GOOSE value kind %q", value.Kind)
	}
}

func readTLV(raw []byte) (byte, []byte, []byte, error) {
	if len(raw) < 2 {
		return 0, nil, nil, fmt.Errorf("truncated BER TLV")
	}
	tag := raw[0]
	length, used, err := decodeLength(raw[1:])
	if err != nil || 1+used+length > len(raw) {
		return 0, nil, nil, fmt.Errorf("invalid BER length")
	}
	start := 1 + used
	return tag, raw[start : start+length], raw[start+length:], nil
}

func encodeTLV(tag byte, content []byte) []byte {
	result := []byte{tag}
	result = append(result, encodeLength(len(content))...)
	return append(result, content...)
}

func decodeLength(raw []byte) (int, int, error) {
	if len(raw) == 0 {
		return 0, 0, fmt.Errorf("missing BER length")
	}
	if raw[0]&0x80 == 0 {
		return int(raw[0]), 1, nil
	}
	count := int(raw[0] & 0x7f)
	if count == 0 || count > 4 || len(raw) < count+1 {
		return 0, 0, fmt.Errorf("unsupported BER length")
	}
	length := 0
	for _, value := range raw[1 : count+1] {
		length = length<<8 | int(value)
	}
	return length, count + 1, nil
}

func encodeLength(length int) []byte {
	if length < 128 {
		return []byte{byte(length)}
	}
	var buffer [4]byte
	index := len(buffer)
	for length > 0 {
		index--
		buffer[index] = byte(length)
		length >>= 8
	}
	return append([]byte{0x80 | byte(len(buffer)-index)}, buffer[index:]...)
}

func decodeBool(raw []byte) bool { return len(raw) > 0 && raw[len(raw)-1] != 0 }

func encodeBool(value bool) []byte {
	if value {
		return []byte{0xff}
	}
	return []byte{0}
}

func decodeUnsigned(raw []byte) uint64 {
	var value uint64
	for _, item := range raw {
		value = value<<8 | uint64(item)
	}
	return value
}

func decodeSigned(raw []byte) int64 {
	if len(raw) == 0 {
		return 0
	}
	value := int64(0)
	for _, item := range raw {
		value = value<<8 | int64(item)
	}
	if raw[0]&0x80 != 0 && len(raw) < 8 {
		value |= ^int64(0) << uint(len(raw)*8)
	}
	return value
}

func encodeUnsigned(value uint64) []byte {
	var buffer [9]byte
	index := len(buffer)
	for {
		index--
		buffer[index] = byte(value)
		value >>= 8
		if value == 0 {
			break
		}
	}
	if buffer[index]&0x80 != 0 {
		index--
		buffer[index] = 0
	}
	return buffer[index:]
}

func encodeSigned(value int64) []byte {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(value))
	index := 0
	if value >= 0 {
		for index < 7 && buffer[index] == 0 && buffer[index+1]&0x80 == 0 {
			index++
		}
	} else {
		for index < 7 && buffer[index] == 0xff && buffer[index+1]&0x80 != 0 {
			index++
		}
	}
	return buffer[index:]
}

func encodeUTCTime(value time.Time) []byte {
	if value.IsZero() {
		value = time.Now()
	}
	seconds := uint32(value.Unix())
	fraction := uint32((uint64(value.Nanosecond()) << 24) / 1_000_000_000)
	result := make([]byte, 8)
	binary.BigEndian.PutUint32(result[:4], seconds)
	result[4] = byte(fraction >> 16)
	result[5] = byte(fraction >> 8)
	result[6] = byte(fraction)
	result[7] = 0x0a
	return result
}

func decodeUTCTime(raw []byte) time.Time {
	if len(raw) < 7 {
		return time.Time{}
	}
	seconds := int64(binary.BigEndian.Uint32(raw[:4]))
	fraction := uint32(raw[4])<<16 | uint32(raw[5])<<8 | uint32(raw[6])
	nanos := int64((uint64(fraction) * 1_000_000_000) >> 24)
	return time.Unix(seconds, nanos)
}

func asFloat64(value interface{}) float64 {
	switch typed := value.(type) {
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
	case uint64:
		return float64(typed)
	case uint32:
		return float64(typed)
	default:
		return 0
	}
}

func asInt64(value interface{}) int64 { return int64(asFloat64(value)) }
func asUint64(value interface{}) uint64 {
	number := asFloat64(value)
	if number < 0 {
		return 0
	}
	return uint64(number)
}

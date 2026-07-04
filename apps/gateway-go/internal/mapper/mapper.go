package mapper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func Decode(point config.PointConfig, raw []byte) (interface{}, error) {
	normalized := normalizeBytes(raw, point.ByteOrder, point.WordOrder)
	if point.Scale == 0 {
		point.Scale = 1
	}

	var value float64
	switch point.DataType {
	case "bool":
		return decodeBool(normalized, point.BitIndex)
	case "uint16":
		if len(normalized) < 2 {
			return nil, fmt.Errorf("uint16 requires 2 bytes")
		}
		value = float64(binary.BigEndian.Uint16(normalized[:2]))
	case "int16":
		if len(normalized) < 2 {
			return nil, fmt.Errorf("int16 requires 2 bytes")
		}
		value = float64(int16(binary.BigEndian.Uint16(normalized[:2])))
	case "uint32":
		if len(normalized) < 4 {
			return nil, fmt.Errorf("uint32 requires 4 bytes")
		}
		value = float64(binary.BigEndian.Uint32(normalized[:4]))
	case "int32":
		if len(normalized) < 4 {
			return nil, fmt.Errorf("int32 requires 4 bytes")
		}
		value = float64(int32(binary.BigEndian.Uint32(normalized[:4])))
	case "float32":
		if len(normalized) < 4 {
			return nil, fmt.Errorf("float32 requires 4 bytes")
		}
		value = float64(math.Float32frombits(binary.BigEndian.Uint32(normalized[:4])))
	default:
		return nil, fmt.Errorf("unsupported dataType %s", point.DataType)
	}

	return applyNumericScale(point, value), nil
}

func applyNumericScale(point config.PointConfig, value float64) float64 {
	scaled := value*point.Scale + point.Offset
	if point.Decimals > 0 {
		factor := math.Pow10(point.Decimals)
		scaled = math.Round(scaled*factor) / factor
	}
	return scaled
}

func Encode(point config.PointConfig, value interface{}) ([]byte, error) {
	if point.Scale == 0 {
		point.Scale = 1
	}
	if point.DataType == "bool" {
		boolValue, err := toBool(value)
		if err != nil {
			return nil, err
		}
		if boolValue {
			return []byte{0xff, 0x00}, nil
		}
		return []byte{0x00, 0x00}, nil
	}
	normalizedValue, err := toFloat64(value)
	if err != nil {
		return nil, err
	}
	rawValue := (normalizedValue - point.Offset) / point.Scale
	var raw []byte
	switch point.DataType {
	case "uint16":
		raw = make([]byte, 2)
		binary.BigEndian.PutUint16(raw, uint16(rawValue))
	case "int16":
		raw = make([]byte, 2)
		binary.BigEndian.PutUint16(raw, uint16(int16(rawValue)))
	case "uint32":
		raw = make([]byte, 4)
		binary.BigEndian.PutUint32(raw, uint32(rawValue))
	case "int32":
		raw = make([]byte, 4)
		binary.BigEndian.PutUint32(raw, uint32(int32(rawValue)))
	case "float32":
		raw = make([]byte, 4)
		binary.BigEndian.PutUint32(raw, math.Float32bits(float32(rawValue)))
	default:
		return nil, fmt.Errorf("unsupported dataType %s", point.DataType)
	}
	return denormalizeBytes(raw, point.ByteOrder, point.WordOrder), nil
}

func normalizeBytes(raw []byte, byteOrder string, wordOrder string) []byte {
	normalized := append([]byte(nil), raw...)
	if byteOrder == "little" {
		for i := 0; i+1 < len(normalized); i += 2 {
			normalized[i], normalized[i+1] = normalized[i+1], normalized[i]
		}
	}
	if wordOrder == "swap" && len(normalized) >= 4 {
		for i := 0; i+3 < len(normalized); i += 4 {
			normalized[i], normalized[i+1], normalized[i+2], normalized[i+3] = normalized[i+2], normalized[i+3], normalized[i], normalized[i+1]
		}
	}
	return normalized
}

func denormalizeBytes(raw []byte, byteOrder string, wordOrder string) []byte {
	return normalizeBytes(raw, byteOrder, wordOrder)
}

func decodeBool(raw []byte, bitIndex *uint8) (bool, error) {
	if len(raw) == 0 {
		return false, fmt.Errorf("bool requires at least 1 byte")
	}
	if bitIndex == nil {
		return raw[0] != 0, nil
	}
	if *bitIndex > 15 {
		return false, fmt.Errorf("bitIndex must be between 0 and 15")
	}
	if len(raw) < 2 {
		return false, fmt.Errorf("bit bool requires 2 bytes")
	}
	value := binary.BigEndian.Uint16(raw[:2])
	return value&(1<<*bitIndex) != 0, nil
}

func toFloat64(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case uint64:
		return float64(typed), nil
	case json.Number:
		return typed.Float64()
	case string:
		return strconv.ParseFloat(typed, 64)
	default:
		return 0, fmt.Errorf("value %v is not numeric", value)
	}
}

func toBool(value interface{}) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case float64:
		return typed != 0, nil
	case int:
		return typed != 0, nil
	case string:
		if typed == "1" || typed == "true" || typed == "on" {
			return true, nil
		}
		if typed == "0" || typed == "false" || typed == "off" {
			return false, nil
		}
	}
	return false, fmt.Errorf("value %v is not boolean", value)
}

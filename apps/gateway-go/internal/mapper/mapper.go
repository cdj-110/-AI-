package mapper

import (
	"encoding/binary"
	"fmt"
	"math"

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

	return value*point.Scale + point.Offset, nil
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

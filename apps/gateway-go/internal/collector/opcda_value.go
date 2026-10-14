package collector

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// opcdaAccessName converts OPC_ACCESS_RIGHTS into the PointConfig enum.
// Unknown or omitted access rights use the safest usable default: read-only.
func opcdaAccessName(rights uint32) string {
	switch rights {
	case 1:
		return "read"
	case 2:
		return "write"
	case 3:
		return "read_write"
	default:
		return "read"
	}
}

func normalizeOPCDAWriteValue(value interface{}, targetType string) (interface{}, error) {
	target := strings.ToLower(strings.TrimSpace(targetType))
	if strings.HasSuffix(target, "[]") {
		return nil, fmt.Errorf("OPC DA Write does not support ARRAY target type %s", targetType)
	}
	if target == "" || target == "auto" || target == "empty" || target == "variant" {
		if number, ok := value.(json.Number); ok {
			return parseOPCDAUntypedNumber(number.String())
		}
		return value, nil
	}

	text := strings.TrimSpace(opcdaWriteText(value))
	switch target {
	case "bool", "boolean":
		switch strings.ToLower(text) {
		case "1", "true", "on", "yes":
			return true, nil
		case "0", "false", "off", "no":
			return false, nil
		default:
			return nil, fmt.Errorf("OPC DA BOOL write value %q must be true/false or 1/0", text)
		}
	case "int8", "int16", "int32", "int64", "int":
		bits := map[string]int{"int8": 8, "int16": 16, "int32": 32, "int": 32, "int64": 64}[target]
		parsed, err := strconv.ParseInt(text, 10, bits)
		if err != nil {
			return nil, fmt.Errorf("OPC DA %s write value %q is invalid or out of range: %w", target, text, err)
		}
		return parsed, nil
	case "uint8", "uint16", "uint32", "uint64", "uint":
		bits := map[string]int{"uint8": 8, "uint16": 16, "uint32": 32, "uint": 32, "uint64": 64}[target]
		parsed, err := strconv.ParseUint(text, 10, bits)
		if err != nil {
			return nil, fmt.Errorf("OPC DA %s write value %q is invalid or out of range: %w", target, text, err)
		}
		return parsed, nil
	case "float32", "float64", "currency":
		bits := 64
		if target == "float32" {
			bits = 32
		}
		parsed, err := strconv.ParseFloat(text, bits)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			if err == nil {
				err = fmt.Errorf("non-finite number")
			}
			return nil, fmt.Errorf("OPC DA %s write value %q is invalid: %w", target, text, err)
		}
		return parsed, nil
	case "string":
		if typed, ok := value.(string); ok {
			return typed, nil
		}
		return opcdaWriteText(value), nil
	case "date", "datetime":
		if typed, ok := value.(time.Time); ok {
			return typed, nil
		}
		if milliseconds, err := strconv.ParseInt(text, 10, 64); err == nil {
			return time.UnixMilli(milliseconds), nil
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999", "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.ParseInLocation(layout, text, time.Local); err == nil {
				return parsed, nil
			}
		}
		return nil, fmt.Errorf("OPC DA DATETIME write value %q must be RFC3339, local date/time, or Unix milliseconds", text)
	default:
		return nil, fmt.Errorf("OPC DA Write does not support target type %s", targetType)
	}
}

func opcdaWriteText(value interface{}) string {
	if number, ok := value.(json.Number); ok {
		return number.String()
	}
	return fmt.Sprint(value)
}

func parseOPCDAUntypedNumber(text string) (interface{}, error) {
	if strings.ContainsAny(text, ".eE") {
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, fmt.Errorf("OPC DA numeric write value %q is invalid", text)
		}
		return parsed, nil
	}
	if strings.HasPrefix(text, "-") {
		parsed, err := strconv.ParseInt(text, 10, 64)
		return parsed, err
	}
	if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
		return parsed, nil
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("OPC DA numeric write value %q is invalid or out of range: %w", text, err)
	}
	return parsed, nil
}

func applyOPCDAScale(point config.PointConfig, value interface{}) interface{} {
	if point.Scale == 1 && point.Offset == 0 {
		return value
	}
	if values, ok := value.([]float64); ok {
		scaled := make([]float64, len(values))
		factor := math.Pow10(point.Decimals)
		for index, number := range values {
			scaled[index] = number*point.Scale + point.Offset
			if point.Decimals > 0 {
				scaled[index] = math.Round(scaled[index]*factor) / factor
			}
		}
		return scaled
	}
	var number float64
	switch typed := value.(type) {
	case int:
		number = float64(typed)
	case int8:
		number = float64(typed)
	case int16:
		number = float64(typed)
	case int32:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case uint:
		number = float64(typed)
	case uint8:
		number = float64(typed)
	case uint16:
		number = float64(typed)
	case uint32:
		number = float64(typed)
	case uint64:
		number = float64(typed)
	case float32:
		number = float64(typed)
	case float64:
		number = typed
	default:
		return value
	}
	scaled := number*point.Scale + point.Offset
	if point.Decimals > 0 {
		factor := math.Pow10(point.Decimals)
		scaled = math.Round(scaled*factor) / factor
	}
	return scaled
}

func opcdaCanonicalDataType(value uint16) string {
	const arrayFlag = uint16(0x2000)
	base := value &^ arrayFlag
	names := map[uint16]string{
		0: "empty", 2: "int16", 3: "int32", 4: "float32", 5: "float64",
		6: "currency", 7: "datetime", 8: "string", 11: "bool", 16: "int8",
		17: "uint8", 18: "uint16", 19: "uint32", 20: "int64", 21: "uint64",
		22: "int", 23: "uint",
	}
	name := names[base]
	if name == "" {
		return "variant"
	}
	if value&arrayFlag != 0 {
		return name + "[]"
	}
	return name
}

func opcdaNativeValueTypeName(value int) string {
	switch value {
	case 0:
		return "empty"
	case 1:
		return "bool"
	case 2:
		return "int64"
	case 3:
		return "uint64"
	case 4:
		return "float64"
	case 5:
		return "string"
	case 6:
		return "datetime"
	case 7:
		return "float64[]"
	case 8:
		return "string[]"
	default:
		return "variant"
	}
}

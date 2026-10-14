package history

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

// Sample is the protocol-neutral historical representation of one point.
// Protocol collectors populate PointValue; the history layer never needs to
// know whether the source was OPC DA, OPC UA, Modbus, S7 or another driver.
type Sample struct {
	Time             time.Time
	DeviceKey        string
	Metric           string
	Value            interface{}
	ActualDataType   string
	Quality          model.PointQuality
	RawQuality       uint32
	SourceTimestamp  time.Time
	ReceiveTimestamp time.Time
	CollectionMode   string
	DiagnosticCode   int64
	Diagnostic       string
	TaskKeys         []string
}

type encodedValue struct {
	ValueType    string
	NumericValue interface{}
	IntegerValue interface{}
	SpecialValue interface{}
	JSON         string
	Identity     string
	Numeric      *big.Rat
}

// ValueIdentity provides a stable, lossless identity and an exact rational
// value for change/deadband decisions. It deliberately does not coerce int64
// or uint64 through float64.
func ValueIdentity(value interface{}) (string, *big.Rat, bool) {
	encoded, err := encodeValue(value)
	if err != nil {
		return fmt.Sprintf("%T:%v", value, value), nil, false
	}
	return encoded.Identity, encoded.Numeric, encoded.Numeric != nil
}

func encodeValue(value interface{}) (encodedValue, error) {
	result := encodedValue{ValueType: valueType(value)}
	result.Numeric, result.NumericValue, result.IntegerValue, result.SpecialValue = numericParts(value)
	raw, err := json.Marshal(value)
	if err != nil {
		raw, err = json.Marshal(jsonSafeValue(reflect.ValueOf(value)))
	}
	if err != nil {
		return encodedValue{}, err
	}
	result.JSON = string(raw)
	result.Identity = result.ValueType + "|" + result.JSON
	if result.SpecialValue != nil {
		result.Identity += "|" + fmt.Sprint(result.SpecialValue)
	}
	return result, nil
}

func valueType(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case int:
		return "int"
	case int8:
		return "int8"
	case int16:
		return "int16"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case uint:
		return "uint"
	case uint8:
		return "uint8"
	case uint16:
		return "uint16"
	case uint32:
		return "uint32"
	case uint64:
		return "uint64"
	case float32:
		return "float32"
	case float64:
		return "float64"
	case json.Number:
		return "number"
	default:
		return "json"
	}
}

func numericParts(value interface{}) (*big.Rat, interface{}, interface{}, interface{}) {
	const largestExactFloatInteger = uint64(1 << 53)
	switch typed := value.(type) {
	case int:
		return integerParts(strconv.FormatInt(int64(typed), 10), int64(typed) >= -(1<<53) && int64(typed) <= 1<<53)
	case int8:
		return integerParts(strconv.FormatInt(int64(typed), 10), true)
	case int16:
		return integerParts(strconv.FormatInt(int64(typed), 10), true)
	case int32:
		return integerParts(strconv.FormatInt(int64(typed), 10), true)
	case int64:
		return integerParts(strconv.FormatInt(typed, 10), typed >= -(1<<53) && typed <= 1<<53)
	case uint:
		return unsignedIntegerParts(strconv.FormatUint(uint64(typed), 10), uint64(typed) <= largestExactFloatInteger)
	case uint8:
		return unsignedIntegerParts(strconv.FormatUint(uint64(typed), 10), true)
	case uint16:
		return unsignedIntegerParts(strconv.FormatUint(uint64(typed), 10), true)
	case uint32:
		return unsignedIntegerParts(strconv.FormatUint(uint64(typed), 10), true)
	case uint64:
		return unsignedIntegerParts(strconv.FormatUint(typed, 10), typed <= largestExactFloatInteger)
	case float32:
		return floatParts(float64(typed))
	case float64:
		return floatParts(typed)
	case json.Number:
		text := typed.String()
		rat := decimalRat(text)
		if !strings.ContainsAny(text, ".eE") {
			return rat, nil, text, nil
		}
		if numeric, err := typed.Float64(); err == nil && !math.IsNaN(numeric) && !math.IsInf(numeric, 0) {
			return rat, numeric, nil, nil
		}
		return rat, nil, nil, nil
	default:
		return nil, nil, nil, nil
	}
}

func integerParts(text string, safe bool) (*big.Rat, interface{}, interface{}, interface{}) {
	integer, _ := new(big.Int).SetString(text, 10)
	rat := new(big.Rat).SetInt(integer)
	if safe {
		numeric, _ := strconv.ParseFloat(text, 64)
		return rat, numeric, text, nil
	}
	return rat, nil, text, nil
}

func unsignedIntegerParts(text string, safe bool) (*big.Rat, interface{}, interface{}, interface{}) {
	return integerParts(text, safe)
}

func floatParts(value float64) (*big.Rat, interface{}, interface{}, interface{}) {
	if math.IsNaN(value) {
		return nil, nil, nil, "NaN"
	}
	if math.IsInf(value, 1) {
		return nil, nil, nil, "+Inf"
	}
	if math.IsInf(value, -1) {
		return nil, nil, nil, "-Inf"
	}
	return new(big.Rat).SetFloat64(value), value, nil, nil
}

func decimalRat(text string) *big.Rat {
	if rat, ok := new(big.Rat).SetString(text); ok {
		return rat
	}
	value, _, err := big.ParseFloat(text, 10, 256, big.ToNearestEven)
	if err != nil {
		return nil
	}
	rat, _ := value.Rat(nil)
	return rat
}

func jsonSafeValue(value reflect.Value) interface{} {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Float32, reflect.Float64:
		floating := value.Float()
		if math.IsNaN(floating) {
			return map[string]string{"$float": "NaN"}
		}
		if math.IsInf(floating, 1) {
			return map[string]string{"$float": "+Inf"}
		}
		if math.IsInf(floating, -1) {
			return map[string]string{"$float": "-Inf"}
		}
		return floating
	case reflect.Slice, reflect.Array:
		values := make([]interface{}, value.Len())
		for index := range values {
			values[index] = jsonSafeValue(value.Index(index))
		}
		return values
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return fmt.Sprint(value.Interface())
		}
		values := make(map[string]interface{}, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			values[iterator.Key().String()] = jsonSafeValue(iterator.Value())
		}
		return values
	default:
		return value.Interface()
	}
}

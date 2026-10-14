// Package valueformat prepares measurements for display and external transport.
// It must not be used to round inputs to alarms, expressions or protocol writes.
package valueformat

import (
	"math"
	"strconv"
	"strings"
)

// Number removes only near-machine-precision noise which
// admits a substantially shorter decimal representation. Unlike fixed decimal
// rounding, it retains ordinary high-precision measurements and tiny values.
// There is no way to distinguish all intentional near-decimal values from
// arithmetic noise; any correction is bounded to two adjacent float64 steps.
func Number(value float64) float64 {
	if value == 0 {
		return 0 // Avoid emitting negative zero.
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) == value {
		return value
	}
	mantissa := strings.SplitN(strconv.FormatFloat(value, 'e', -1, 64), "e", 2)[0]
	digits := len(strings.ReplaceAll(strings.TrimPrefix(mantissa, "-"), ".", ""))
	step := math.Max(math.Abs(math.Nextafter(value, math.Inf(1))-value), math.Abs(math.Nextafter(value, math.Inf(-1))-value))
	// Require at least four redundant significant digits; do not simply cap
	// every measurement at an arbitrary number of significant/decimal places.
	for precision := 1; precision <= digits-4; precision++ {
		candidate, err := strconv.ParseFloat(strconv.FormatFloat(value, 'g', precision, 64), 64)
		if err == nil && candidate != 0 && math.Abs(candidate-value) <= 2*step {
			return candidate
		}
	}
	return value
}

// Always copy collections: raw readings are shared with history, alarms and
// protocol forwarding, and must not be changed while preparing MQTT reports.
func Value(value interface{}) interface{} {
	switch typed := value.(type) {
	case float64:
		return Number(typed)
	case map[string]interface{}:
		if typed == nil {
			return typed
		}
		copy := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			copy[key] = Value(item)
		}
		return copy
	case []interface{}:
		if typed == nil {
			return typed
		}
		copy := make([]interface{}, len(typed))
		for index, item := range typed {
			copy[index] = Value(item)
		}
		return copy
	case []float64:
		if typed == nil {
			return typed
		}
		copy := make([]float64, len(typed))
		for index, item := range typed {
			copy[index] = Number(item)
		}
		return copy
	default:
		// Integer width, float32 serialization, booleans and text are unchanged.
		return value
	}
}

func Groups(grouped map[string]map[string]interface{}) map[string]map[string]interface{} {
	if grouped == nil {
		return nil
	}
	copy := make(map[string]map[string]interface{}, len(grouped))
	for key, metrics := range grouped {
		copy[key] = Value(metrics).(map[string]interface{})
	}
	return copy
}

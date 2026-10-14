package cloud

import "weikong-iot-platform/apps/gateway-go/internal/valueformat"

func normalizeTelemetryNumber(value float64) float64        { return valueformat.Number(value) }
func normalizeTelemetryValue(value interface{}) interface{} { return valueformat.Value(value) }
func normalizeTelemetryGroups(grouped map[string]map[string]interface{}) map[string]map[string]interface{} {
	return valueformat.Groups(grouped)
}

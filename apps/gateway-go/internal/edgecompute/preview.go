package edgecompute

import (
	"fmt"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

// Preview validates the complete dependency graph and evaluates it against the
// current in-memory values without changing gateway state.
func Preview(cfg config.Config, store *state.Store, groupKey, metric string) (interface{}, map[string]interface{}, error) {
	program, err := Compile(cfg)
	if err != nil {
		return nil, nil, err
	}
	targetKey := sourceKey(groupKey, metric)
	if _, ok := program.ByKey[targetKey]; !ok {
		return nil, nil, fmt.Errorf("未找到派生点 %s", targetKey)
	}

	valuesByKey := map[string]interface{}{}
	statusByKey := map[string]state.PointStatus{}
	for _, status := range store.PointStatuses() {
		key := sourceKey(status.DeviceKey, status.Metric)
		statusByKey[key] = status
		if status.UpdatedAt != nil && status.Error == "" {
			valuesByKey[key] = status.Value
		}
	}
	var targetInputs map[string]interface{}
	for _, point := range program.Points {
		activation := map[string]interface{}{}
		for _, input := range point.Config.Inputs {
			key := sourceKey(input.SourceDeviceKey, input.SourceMetric)
			value, ok := valuesByKey[key]
			if !ok {
				if status, exists := statusByKey[key]; exists && status.Error != "" {
					return nil, nil, fmt.Errorf("输入 %s 异常: %s", key, status.Error)
				}
				return nil, nil, fmt.Errorf("输入 %s 尚无有效实时值", key)
			}
			normalized, err := NormalizeInput(point.InputTypes[input.Alias], value)
			if err != nil {
				return nil, nil, fmt.Errorf("输入 %s: %w", key, err)
			}
			activation[input.Alias] = normalized
		}
		out, _, err := point.Program.Eval(activation)
		if err != nil {
			return nil, nil, fmt.Errorf("派生点 %s 执行失败: %w", point.Key, err)
		}
		value, err := ConvertOutput(point.Config.DataType, out.Value())
		if err != nil {
			return nil, nil, fmt.Errorf("派生点 %s: %w", point.Key, err)
		}
		valuesByKey[point.Key] = value
		if point.Key == targetKey {
			targetInputs = activation
			return value, targetInputs, nil
		}
	}
	return nil, nil, fmt.Errorf("派生点 %s 未启用", targetKey)
}

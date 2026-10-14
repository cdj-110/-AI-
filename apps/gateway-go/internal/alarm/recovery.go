package alarm

import (
	"fmt"
	"math"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func usableAlarmValue(rule config.AlarmRuleConfig, point state.PointStatus) bool {
	quality := strings.ToUpper(string(point.Quality))
	if point.Error != "" || point.Stale || (quality != "" && quality != "GOOD") {
		return false
	}
	if rule.Type == "boolean" {
		_, ok := boolValue(point.Value)
		return ok
	}
	value, ok := numberValue(point.Value)
	return ok && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func activeRecoveryHint(rule config.AlarmRuleConfig) string {
	condition := "告警条件仍成立，条件解除后自动恢复"
	if rule.Type == "threshold" && rule.Threshold != nil {
		operator, boundary := "", *rule.Threshold
		switch rule.Operator {
		case ">":
			operator, boundary = "≤", boundary-rule.Hysteresis
		case ">=":
			operator, boundary = "<", boundary-rule.Hysteresis
			if rule.Hysteresis > 0 {
				operator = "≤"
			}
		case "<":
			operator, boundary = "≥", boundary+rule.Hysteresis
		case "<=":
			operator, boundary = ">", boundary+rule.Hysteresis
			if rule.Hysteresis > 0 {
				operator = "≥"
			}
		}
		condition = fmt.Sprintf("恢复需有效值 %s %g（回差 %g）", operator, boundary, rule.Hysteresis)
	}
	if rule.RecoveryDelaySeconds > 0 {
		condition += fmt.Sprintf("，持续 %d 秒", rule.RecoveryDelaySeconds)
	}
	return condition + "；确认仅表示已知晓，不会清除仍成立的告警"
}

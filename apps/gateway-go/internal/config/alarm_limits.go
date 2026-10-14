package config

import (
	"fmt"
	"math"
	"strings"
)

type AlarmLimitConfig struct {
	Enabled   *bool    `json:"enabled,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Level     string   `json:"level,omitempty"`
}
type AlarmLimitsConfig struct {
	LowLow   *AlarmLimitConfig `json:"lowLow,omitempty"`
	Low      *AlarmLimitConfig `json:"low,omitempty"`
	High     *AlarmLimitConfig `json:"high,omitempty"`
	HighHigh *AlarmLimitConfig `json:"highHigh,omitempty"`
}
type alarmLimitEntry struct {
	key, name, operator, level string
	value                      *AlarmLimitConfig
}

func (l AlarmLimitsConfig) entries() []alarmLimitEntry {
	return []alarmLimitEntry{{"lowLow", "下下限", "<", "CRITICAL", l.LowLow}, {"low", "下限", "<", "WARNING", l.Low}, {"high", "上限", ">", "WARNING", l.High}, {"highHigh", "上上限", ">", "CRITICAL", l.HighHigh}}
}
func (l *AlarmLimitConfig) IsEnabled() bool { return l != nil && (l.Enabled == nil || *l.Enabled) }
func (l *AlarmLimitsConfig) ApplyDefaults() {
	for _, entry := range l.entries() {
		if entry.value != nil {
			entry.value.Level = strings.ToUpper(strings.TrimSpace(entry.value.Level))
			if entry.value.Level == "" {
				entry.value.Level = entry.level
			}
		}
	}
}
func (l AlarmLimitsConfig) Validate() error {
	var last *alarmLimitEntry
	for _, entry := range l.entries() {
		if !entry.value.IsEnabled() {
			continue
		}
		value := entry.value
		if value.Threshold == nil || math.IsNaN(*value.Threshold) || math.IsInf(*value.Threshold, 0) {
			return fmt.Errorf("请填写%s的有效阈值", entry.name)
		}
		if value.Level != "INFO" && value.Level != "WARNING" && value.Level != "CRITICAL" {
			return fmt.Errorf("%s的告警等级无效", entry.name)
		}
		if last != nil && *last.value.Threshold >= *value.Threshold {
			return fmt.Errorf("%s必须小于%s（下下限 < 下限 < 上限 < 上上限）", last.name, entry.name)
		}
		copy := entry
		last = &copy
	}
	if last == nil {
		return fmt.Errorf("四限告警请至少启用一个限值")
	}
	return nil
}

// Four limits use the existing independent debounce, recovery and audit engine.
// Only runtime rules are expanded; the saved configuration remains one parent rule.
func (a AlarmConfig) RuntimeRules() []AlarmRuleConfig {
	var result []AlarmRuleConfig
	for _, rule := range a.Rules {
		if rule.Type != "threshold" || rule.Limits == nil {
			result = append(result, rule)
			continue
		}
		for _, entry := range rule.Limits.entries() {
			if !entry.value.IsEnabled() {
				continue
			}
			child := rule
			child.ParentRuleID, child.ParentRuleName = rule.RuleID, rule.Name
			child.LimitKey, child.LimitName = entry.key, entry.name
			child.Limits = nil
			child.RuleID = rule.RuleID + "::" + entry.key
			child.Name = rule.Name + " · " + entry.name
			child.Operator = entry.operator
			child.Threshold = entry.value.Threshold
			child.Level = entry.value.Level
			if child.Message != "" {
				child.Message = entry.name + "：" + child.Message
			}
			result = append(result, child)
		}
	}
	return result
}
func (a AlarmConfig) ValidateRuntimeRuleIDs() error {
	seen := map[string]bool{}
	for _, rule := range a.RuntimeRules() {
		key := strings.ToLower(strings.TrimSpace(rule.RuleID))
		if seen[key] {
			return fmt.Errorf("告警规则标识 %s 与四限子规则冲突，请修改标识", rule.RuleID)
		}
		seen[key] = true
	}
	return nil
}

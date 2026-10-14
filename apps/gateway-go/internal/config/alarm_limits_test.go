package config

import (
	"encoding/json"
	"math"
	"testing"
)

func limitAt(value float64) *AlarmLimitConfig { return &AlarmLimitConfig{Threshold: &value} }
func TestAlarmLimitsDefaultsValidationAndExpansion(t *testing.T) {
	limits := &AlarmLimitsConfig{LowLow: limitAt(-20), Low: limitAt(-10), High: limitAt(10), HighHigh: limitAt(20)}
	cfg := AlarmConfig{Rules: []AlarmRuleConfig{{RuleID: "temperature", Name: "温度", Type: "threshold", Limits: limits, AllowManualRecovery: true, Hysteresis: 2, TriggerDelaySeconds: 3, RecoveryDelaySeconds: 4}}}
	cfg.ApplyDefaults()
	if err := limits.Validate(); err != nil {
		t.Fatal(err)
	}
	rules := cfg.RuntimeRules()
	if len(rules) != 4 {
		t.Fatal("four runtime rules required")
	}
	for i, rule := range rules {
		if rule.ParentRuleID != "temperature" || rule.ParentRuleName != "温度" || rule.LimitKey == "" || rule.LimitName == "" {
			t.Fatalf("missing parent identity: %+v", rule)
		}
		if rule.Limits != nil || !rule.AllowManualRecovery || rule.Hysteresis != 2 || rule.TriggerDelaySeconds != 3 || rule.RecoveryDelaySeconds != 4 {
			t.Fatalf("lost parent behavior: %+v", rule)
		}
		if i < 2 && rule.Operator != "<" || i >= 2 && rule.Operator != ">" {
			t.Fatal("wrong limit direction")
		}
	}
	if rules[0].Level != "CRITICAL" || rules[1].Level != "WARNING" || rules[2].Level != "WARNING" || rules[3].Level != "CRITICAL" {
		t.Fatal("default severity wrong")
	}
	raw, _ := json.Marshal(cfg)
	var restored AlarmConfig
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.ApplyDefaults()
	if len(restored.Rules) != 1 || len(restored.RuntimeRules()) != 4 {
		t.Fatal("runtime expansion leaked into stored config")
	}
	cfg.Rules = append(cfg.Rules, AlarmRuleConfig{RuleID: "temperature::high", Type: "threshold"})
	if cfg.ValidateRuntimeRuleIDs() == nil {
		t.Fatal("derived ID collision accepted")
	}
	legacy := AlarmConfig{Rules: []AlarmRuleConfig{{RuleID: "old", Type: "threshold", Threshold: limitAt(0).Threshold, Operator: ">="}}}
	legacy.ApplyDefaults()
	if legacy.RuntimeRules()[0].ParentRuleID != "" {
		t.Fatal("single threshold cannot inherit a different rule")
	}
	if legacy.RuntimeRules()[0].RuleID != "old" || legacy.RuntimeRules()[0].Operator != ">=" || legacy.Rules[0].Limits != nil {
		t.Fatal("legacy threshold changed")
	}
}

func TestAlarmLimitsRejectInvalidRanges(t *testing.T) {
	disabled := false
	for name, limits := range map[string]AlarmLimitsConfig{
		"empty": {}, "missing": {High: &AlarmLimitConfig{}}, "equal": {Low: limitAt(0), High: limitAt(0)}, "inverted": {LowLow: limitAt(20), HighHigh: limitAt(10)}, "NaN": {High: limitAt(math.NaN())}, "all-disabled": {High: &AlarmLimitConfig{Enabled: &disabled}},
	} {
		t.Run(name, func(t *testing.T) {
			limits.ApplyDefaults()
			if limits.Validate() == nil {
				t.Fatal("invalid limits accepted")
			}
		})
	}
	limits := AlarmLimitsConfig{High: limitAt(0), Low: &AlarmLimitConfig{Enabled: &disabled}}
	limits.ApplyDefaults()
	if err := limits.Validate(); err != nil {
		t.Fatal("zero threshold or disabled empty threshold rejected", err)
	}
}

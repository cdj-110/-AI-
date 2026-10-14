package alarm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

const evaluationInterval = time.Second

type Event struct {
	ParentRuleID            string      `json:"parentRuleId,omitempty"`
	ParentRuleName          string      `json:"parentRuleName,omitempty"`
	LimitKey                string      `json:"limitKey,omitempty"`
	LimitName               string      `json:"limitName,omitempty"`
	EventID                 string      `json:"eventId"`
	RuleID                  string      `json:"ruleId"`
	RuleName                string      `json:"ruleName"`
	DeviceKey               string      `json:"deviceKey"`
	Metric                  string      `json:"metric"`
	PointName               string      `json:"pointName,omitempty"`
	Level                   string      `json:"level"`
	State                   string      `json:"state"`
	Acknowledged            bool        `json:"acknowledged"`
	AcknowledgedAt          *time.Time  `json:"acknowledgedAt,omitempty"`
	Value                   interface{} `json:"value,omitempty"`
	ValueText               string      `json:"valueText,omitempty"`
	Threshold               *float64    `json:"threshold,omitempty"`
	Quality                 string      `json:"quality,omitempty"`
	Message                 string      `json:"message"`
	OccurredAt              time.Time   `json:"occurredAt"`
	UpdatedAt               time.Time   `json:"updatedAt"`
	RecoveredAt             *time.Time  `json:"recoveredAt,omitempty"`
	Unit                    string      `json:"unit,omitempty"`
	RecoveryStatus          string      `json:"recoveryStatus,omitempty"`
	RecoveryHint            string      `json:"recoveryHint,omitempty"`
	AllowManualRecovery     bool        `json:"allowManualRecovery,omitempty"`
	PopupEnabled            bool        `json:"popupEnabled"`
	SoundWhenPopupDisabled  bool        `json:"soundWhenPopupDisabled"`
	RetainAfterAcknowledged bool        `json:"retainAfterAcknowledged"`
}

type Snapshot struct {
	PopupDurationSeconds int     `json:"popupDurationSeconds"`
	Active               []Event `json:"active"`
	History              []Event `json:"history"`
	ActiveCount          int     `json:"activeCount"`
	CriticalCount        int     `json:"criticalCount"`
	UnackedCount         int     `json:"unackedCount"`
}

type ruleRuntime struct {
	ConditionSince time.Time
	RecoverySince  time.Time
	Active         *Event
	ManualReset    *Event
}

type Manager struct {
	store       *state.Store
	mu          sync.RWMutex
	config      config.AlarmConfig
	runtimes    map[string]*ruleRuntime
	history     []Event
	storagePath string
	storage     *sqliteHistoryStore
	wake        chan struct{}
	eventSink   func(Event) error
}

func NewManager(cfg config.Config, store *state.Store, configPath string) *Manager {
	manager := &Manager{store: store, runtimes: map[string]*ruleRuntime{}, wake: make(chan struct{}, 1)}
	manager.updateConfigLocked(cfg, configPath)
	manager.restoreManualResetsLocked()
	return manager
}

func (m *Manager) Start(ctx context.Context) {
	if m == nil || m.store == nil {
		return
	}
	updates, unsubscribe := m.store.Subscribe()
	defer unsubscribe()
	defer m.closeHistory()
	ticker := time.NewTicker(evaluationInterval)
	defer ticker.Stop()
	m.signal()
	for {
		select {
		case <-ctx.Done():
			return
		case <-updates:
			m.evaluate(time.Now())
		case <-ticker.C:
			m.evaluate(time.Now())
		case <-m.wake:
			m.evaluate(time.Now())
		}
	}
}

// SetEventSink registers the durable cloud-export handoff. It is called for
// lifecycle transitions only (trigger, acknowledge and recovery), never for
// the one-second live-value refreshes of an active alarm.
func (m *Manager) SetEventSink(sink func(Event) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.eventSink = sink
	m.mu.Unlock()
}

func (m *Manager) UpdateConfig(cfg config.Config, configPath string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	previous := m.config
	m.updateConfigLocked(cfg, configPath)
	valid := map[string]config.AlarmRuleConfig{}
	for _, rule := range m.config.Rules {
		valid[rule.RuleID] = rule
	}
	now := time.Now()
	for ruleID, runtime := range m.runtimes {
		next, exists := valid[ruleID]
		old, wasPresent := ruleByID(previous.Rules, ruleID)
		if exists && wasPresent && ruleSignature(next) == ruleSignature(old) {
			if runtime.Active != nil {
				runtime.Active.PopupEnabled = next.IsPopupEnabled()
				runtime.Active.SoundWhenPopupDisabled = next.SoundWhenPopupDisabled
				runtime.Active.RetainAfterAcknowledged = next.RetainAfterAcknowledged
			}
			continue
		}
		if runtime.Active != nil {
			m.resolveLocked(runtime, now, "规则已修改、停用或删除，当前告警已结束")
		}
		if m.storage != nil {
			if err := m.storage.deleteManualReset(ruleID); err != nil {
				log.Printf("remove changed alarm manual reset: %v", err)
			}
		}
		delete(m.runtimes, ruleID)
	}
	m.mu.Unlock()
	m.signal()
}

func (m *Manager) Snapshot() Snapshot {
	return m.snapshot(true)
}

// ActiveSnapshot is used by lightweight global UI notifications. It avoids
// copying and serializing alarm history on every short polling interval.
func (m *Manager) ActiveSnapshot() Snapshot {
	return m.snapshot(false)
}

func (m *Manager) History(query HistoryQuery) (HistoryPage, error) {
	if m == nil {
		return HistoryPage{Items: []Event{}, Page: 1, PageSize: 50}, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.storage != nil {
		return m.storage.Query(query)
	}
	return memoryHistoryPage(m.history, query), nil
}

func (m *Manager) snapshot(includeHistory bool) Snapshot {
	if m == nil {
		return Snapshot{Active: []Event{}, History: []Event{}}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	active := make([]Event, 0, len(m.runtimes))
	critical, unacked := 0, 0
	for _, runtime := range m.runtimes {
		if runtime.Active == nil {
			continue
		}
		event := *runtime.Active
		active = append(active, event)
		if event.Level == "CRITICAL" {
			critical++
		}
		if !event.Acknowledged {
			unacked++
		}
	}
	// A collection cycle can activate several rules at exactly the same time.
	// Map iteration is unordered, so timestamp-only sorting lets tied rows jump.
	sort.Slice(active, func(i, j int) bool {
		if !active[i].OccurredAt.Equal(active[j].OccurredAt) {
			return active[i].OccurredAt.After(active[j].OccurredAt)
		}
		if active[i].RuleID != active[j].RuleID {
			return active[i].RuleID < active[j].RuleID
		}
		return active[i].EventID < active[j].EventID
	})
	history := []Event{}
	if includeHistory {
		history = append(history, m.history...)
	}
	return Snapshot{Active: active, History: history, ActiveCount: len(active), CriticalCount: critical, UnackedCount: unacked, PopupDurationSeconds: m.config.PopupDuration()}
}

func (m *Manager) Acknowledge(eventID string) (Event, error) {
	if m == nil {
		return Event{}, fmt.Errorf("告警服务不可用")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, runtime := range m.runtimes {
		if runtime.Active == nil || runtime.Active.EventID != eventID {
			continue
		}
		if runtime.Active.Acknowledged {
			return *runtime.Active, nil
		}
		now := time.Now()
		runtime.Active.Acknowledged = true
		runtime.Active.AcknowledgedAt = &now
		runtime.Active.UpdatedAt = now
		record := *runtime.Active
		record.State = "ACKNOWLEDGED"
		m.recordLocked(record)
		return *runtime.Active, nil
	}
	return Event{}, fmt.Errorf("当前告警 %s 不存在或已经恢复", eventID)
}

func (m *Manager) evaluate(now time.Time) {
	statuses := m.store.PointStatuses()
	byPoint := make(map[string]state.PointStatus, len(statuses))
	for _, status := range statuses {
		byPoint[pointKey(status.DeviceKey, status.Metric)] = status
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rule := range m.config.Rules {
		if !rule.IsEnabled() {
			continue
		}
		runtime := m.runtimes[rule.RuleID]
		if runtime == nil {
			runtime = &ruleRuntime{}
			m.runtimes[rule.RuleID] = runtime
		}
		point, exists := byPoint[pointKey(rule.DeviceKey, rule.Metric)]
		if runtime.Active != nil && exists {
			// Update only the in-memory active snapshot. History keeps the value
			// captured at each transition, rather than one row per evaluation.
			runtime.Active.Value = point.Value
			runtime.Active.ValueText = point.ValueText
			runtime.Active.Unit = point.Unit
			runtime.Active.Quality = string(point.Quality)
			if point.UpdatedAt != nil {
				runtime.Active.UpdatedAt = *point.UpdatedAt
			}
		}
		if exists && (rule.Type == "threshold" || rule.Type == "boolean") && !usableAlarmValue(rule, point) {
			runtime.ConditionSince = time.Time{}
			runtime.RecoverySince = time.Time{}
			if runtime.Active != nil {
				runtime.Active.RecoveryStatus = "awaiting_data"
				runtime.Active.RecoveryHint = "采集数据无效或已陈旧，暂不能确认恢复；请先检查点位采集"
			}
			continue
		}
		condition := false
		if exists {
			condition = matches(rule, point, runtime.Active != nil || runtime.ManualReset != nil)
		}
		if runtime.ManualReset != nil {
			// Missing measurements cannot prove that the original fault cleared.
			if !exists {
				runtime.RecoverySince = time.Time{}
				continue
			}
			if condition {
				runtime.RecoverySince = time.Time{}
				continue
			}
			if runtime.RecoverySince.IsZero() {
				runtime.RecoverySince = now
			}
			if now.Sub(runtime.RecoverySince) < time.Duration(rule.RecoveryDelaySeconds)*time.Second {
				continue
			}
			if m.storage != nil {
				if err := m.storage.deleteManualReset(rule.RuleID); err != nil {
					log.Printf("clear alarm manual reset: %v", err)
					continue
				}
			}
			runtime.ManualReset = nil
			runtime.ConditionSince = time.Time{}
			runtime.RecoverySince = time.Time{}
			continue
		}
		if runtime.Active == nil {
			runtime.RecoverySince = time.Time{}
			if !condition {
				runtime.ConditionSince = time.Time{}
				continue
			}
			if runtime.ConditionSince.IsZero() {
				runtime.ConditionSince = now
			}
			if now.Sub(runtime.ConditionSince) >= time.Duration(rule.TriggerDelaySeconds)*time.Second {
				m.activateLocked(rule, point, now, runtime)
			}
			continue
		}
		runtime.ConditionSince = time.Time{}
		if condition {
			runtime.RecoverySince = time.Time{}
			runtime.Active.RecoveryStatus = "condition_active"
			runtime.Active.RecoveryHint = activeRecoveryHint(rule)
			continue
		}
		if runtime.RecoverySince.IsZero() {
			runtime.RecoverySince = now
		}
		if now.Sub(runtime.RecoverySince) >= time.Duration(rule.RecoveryDelaySeconds)*time.Second {
			message := recoveryMessage(rule, point)
			if !exists {
				message = "关联点位已移除或停用，当前告警已结束（非测量值恢复）"
			}
			m.resolveLocked(runtime, now, message)
		} else {
			runtime.Active.RecoveryStatus = "recovering"
			remaining := int(math.Ceil((time.Duration(rule.RecoveryDelaySeconds)*time.Second - now.Sub(runtime.RecoverySince)).Seconds()))
			runtime.Active.RecoveryHint = fmt.Sprintf("已满足恢复条件，需继续保持 %d 秒；确认告警不会跳过恢复延时", remaining)
		}
	}
}

func (m *Manager) activateLocked(rule config.AlarmRuleConfig, point state.PointStatus, now time.Time, runtime *ruleRuntime) {
	message := strings.TrimSpace(rule.Message)
	if message == "" {
		message = defaultMessage(rule, point)
	}
	event := Event{EventID: newEventID(), RuleID: rule.RuleID, RuleName: rule.Name, DeviceKey: rule.DeviceKey, Metric: rule.Metric, PointName: point.Name, Level: rule.Level, State: "ACTIVE", Value: point.Value, ValueText: point.ValueText, Threshold: rule.Threshold, Quality: string(point.Quality), Message: message, OccurredAt: now, UpdatedAt: now}
	event.Unit = point.Unit
	event.ParentRuleID, event.ParentRuleName = rule.ParentRuleID, rule.ParentRuleName
	event.LimitKey, event.LimitName = rule.LimitKey, rule.LimitName
	event.RecoveryStatus = "condition_active"
	event.RecoveryHint = activeRecoveryHint(rule)
	event.AllowManualRecovery = rule.AllowManualRecovery
	event.PopupEnabled = rule.IsPopupEnabled()
	event.SoundWhenPopupDisabled = rule.SoundWhenPopupDisabled
	event.RetainAfterAcknowledged = rule.RetainAfterAcknowledged
	runtime.Active = &event
	runtime.ConditionSince = time.Time{}
	m.recordLocked(event)
}

func (m *Manager) resolveLocked(runtime *ruleRuntime, now time.Time, message string) {
	if runtime.Active == nil {
		return
	}
	record := *runtime.Active
	record.State = "RESOLVED"
	record.RecoveryStatus = ""
	record.RecoveryHint = ""
	record.RecoveredAt = &now
	record.UpdatedAt = now
	if strings.TrimSpace(message) != "" {
		record.Message = message
	}
	m.recordLocked(record)
	runtime.Active = nil
	runtime.RecoverySince = time.Time{}
}

func (m *Manager) recordLocked(event Event) {
	m.history = append([]Event{event}, m.history...)
	if len(m.history) > m.config.HistoryLimit {
		m.history = m.history[:m.config.HistoryLimit]
	}
	if m.storage != nil {
		if err := m.storage.Append(event, m.config.RetentionLimit); err != nil {
			log.Printf("append alarm history failed: %v", err)
		}
	}
	m.emitEventLocked(event)
}

func (m *Manager) emitEventLocked(event Event) {
	if m.eventSink == nil {
		return
	}
	if err := m.eventSink(event); err != nil {
		log.Printf("queue alarm cloud event failed event=%s state=%s: %v", event.EventID, event.State, err)
	}
}

func (m *Manager) updateConfigLocked(cfg config.Config, configPath string) {
	previousPath := m.storagePath
	m.config = cfg.Alarms
	m.config.Rules = cfg.Alarms.RuntimeRules()
	path := strings.TrimSpace(cfg.Alarms.StoragePath)
	if path != "" && !filepath.IsAbs(path) {
		if configPath == "" {
			path = ""
		} else {
			path = filepath.Join(filepath.Dir(configPath), path)
		}
	}
	m.storagePath = path
	if previousPath == path && (m.storage != nil || path == "") {
		if m.storage != nil {
			if err := m.storage.Prune(m.config.RetentionLimit); err != nil {
				log.Printf("prune alarm history failed: %v", err)
			}
		}
		if len(m.history) > m.config.HistoryLimit {
			m.history = m.history[:m.config.HistoryLimit]
		}
		return
	}
	if m.storage != nil {
		_ = m.storage.Close()
		m.storage = nil
	}
	m.history = []Event{}
	if path == "" {
		return
	}
	legacyPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".jsonl"
	storage, history, err := openSQLiteHistory(path, legacyPath, m.config.RetentionLimit, m.config.HistoryLimit)
	if err != nil {
		log.Printf("open alarm SQLite history failed path=%s: %v", path, err)
		return
	}
	m.storage = storage
	m.history = history
}

func (m *Manager) closeHistory() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storage != nil {
		_ = m.storage.Close()
		m.storage = nil
	}
}

func matches(rule config.AlarmRuleConfig, point state.PointStatus, active bool) bool {
	switch rule.Type {
	case "boolean":
		value, ok := boolValue(point.Value)
		return ok && rule.BooleanValue != nil && value == *rule.BooleanValue
	case "quality":
		return strings.EqualFold(string(point.Quality), rule.Quality) || rule.Quality == "BAD" && point.Error != ""
	case "stale":
		return point.Stale
	case "threshold":
		value, ok := numberValue(point.Value)
		if !ok || rule.Threshold == nil {
			return false
		}
		threshold := *rule.Threshold
		if active && rule.Hysteresis > 0 {
			switch rule.Operator {
			case ">", ">=":
				return value > threshold-rule.Hysteresis
			case "<", "<=":
				return value < threshold+rule.Hysteresis
			}
		}
		switch rule.Operator {
		case ">":
			return value > threshold
		case ">=":
			return value >= threshold
		case "<":
			return value < threshold
		case "<=":
			return value <= threshold
		}
	}
	return false
}

func numberValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func boolValue(value interface{}) (bool, bool) {
	if typed, ok := value.(bool); ok {
		return typed, true
	}
	if typed, ok := value.(string); ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed, err == nil
	}
	return false, false
}

func defaultMessage(rule config.AlarmRuleConfig, point state.PointStatus) string {
	switch rule.Type {
	case "threshold":
		return fmt.Sprintf("%s 当前值 %v %s 阈值 %v", pointLabel(point, rule), point.Value, rule.Operator, valueOrDash(rule.Threshold))
	case "boolean":
		return fmt.Sprintf("%s 当前状态为 %t", pointLabel(point, rule), valueOrFalse(rule.BooleanValue))
	case "quality":
		return fmt.Sprintf("%s 数据质量为 %s", pointLabel(point, rule), point.Quality)
	case "stale":
		return fmt.Sprintf("%s 数据长时间未更新", pointLabel(point, rule))
	default:
		return rule.Name
	}
}

func recoveryMessage(rule config.AlarmRuleConfig, point state.PointStatus) string {
	return fmt.Sprintf("%s 已恢复正常，当前值 %v", pointLabel(point, rule), point.Value)
}

func pointLabel(point state.PointStatus, rule config.AlarmRuleConfig) string {
	if point.Name != "" {
		return point.Name
	}
	return rule.DeviceKey + "/" + rule.Metric
}

func valueOrDash(value *float64) interface{} {
	if value == nil {
		return "-"
	}
	return *value
}

func valueOrFalse(value *bool) bool {
	return value != nil && *value
}

func ruleByID(rules []config.AlarmRuleConfig, ruleID string) (config.AlarmRuleConfig, bool) {
	for _, rule := range rules {
		if rule.RuleID == ruleID {
			return rule, true
		}
	}
	return config.AlarmRuleConfig{}, false
}

func ruleSignature(rule config.AlarmRuleConfig) string {
	// Presentation-only changes must not end/retrigger an episode or clear
	// persisted manual recovery suppression. Nil keeps old signatures compatible.
	rule.PopupEnabled = nil
	rule.SoundWhenPopupDisabled = false
	rule.RetainAfterAcknowledged = false
	raw, _ := json.Marshal(rule)
	return string(raw)
}

func pointKey(deviceKey, metric string) string {
	return strings.ToLower(strings.TrimSpace(deviceKey)) + "::" + strings.ToLower(strings.TrimSpace(metric))
}

func newEventID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err == nil {
		return hex.EncodeToString(raw)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

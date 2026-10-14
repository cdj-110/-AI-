package alarm

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

var ErrManualRecoveryDisabled = errors.New("该规则不允许手动恢复，请等待告警条件解除后自动恢复")
var ErrAlarmNotActive = errors.New("告警已结束或不存在，请刷新列表")

func (m *Manager) ManualRecover(eventID, username string) (Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for ruleID, runtime := range m.runtimes {
		if runtime.ManualReset != nil && runtime.ManualReset.EventID == eventID {
			return *runtime.ManualReset, nil
		}
		if runtime.Active == nil || runtime.Active.EventID != eventID {
			continue
		}
		rule, ok := ruleByID(m.config.Rules, ruleID)
		if !ok || !rule.IsEnabled() || !rule.AllowManualRecovery {
			return Event{}, ErrManualRecoveryDisabled
		}
		record := *runtime.Active
		now := time.Now()
		record.State, record.UpdatedAt, record.RecoveredAt = "MANUAL_RESOLVED", now, &now
		record.RecoveryStatus, record.RecoveryHint = "", ""
		record.Message = fmt.Sprintf("由 %s 手动恢复本次告警（不代表测量值正常）；同一异常持续期间不重复触发，条件解除后重新布防", username)
		if m.storage == nil && m.storagePath != "" {
			return Event{}, errors.New("告警历史存储不可用，未执行手动恢复")
		}
		if m.storage != nil {
			if err := m.storage.saveManualReset(record, ruleSignature(rule)); err != nil {
				return Event{}, fmt.Errorf("保存手动恢复记录失败，告警仍保留：%w", err)
			}
		}
		m.history = append([]Event{record}, m.history...)
		if len(m.history) > m.config.HistoryLimit {
			m.history = m.history[:m.config.HistoryLimit]
		}
		if m.storage != nil {
			if err := m.storage.Prune(m.config.RetentionLimit); err != nil {
				log.Printf("prune manually resolved alarm history: %v", err)
			}
		}
		m.emitEventLocked(record)
		runtime.Active, runtime.ManualReset = nil, &record
		runtime.ConditionSince, runtime.RecoverySince = time.Time{}, time.Time{}
		return record, nil
	}
	return Event{}, ErrAlarmNotActive
}

func (s *sqliteHistoryStore) saveManualReset(event Event, signature string) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = insertEvent(tx, event); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT OR REPLACE INTO alarm_manual_resets(rule_id,signature,event_json) VALUES(?,?,?)`, event.RuleID, signature, string(raw)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *sqliteHistoryStore) deleteManualReset(ruleID string) error {
	_, err := s.db.Exec(`DELETE FROM alarm_manual_resets WHERE rule_id=?`, ruleID)
	return err
}

func (m *Manager) restoreManualResetsLocked() {
	if m.storage == nil {
		return
	}
	rows, err := m.storage.db.Query(`SELECT rule_id,signature,event_json FROM alarm_manual_resets`)
	if err != nil {
		log.Printf("load manually resolved alarms: %v", err)
		return
	}
	var obsolete []string
	for rows.Next() {
		var ruleID, signature, raw string
		if err := rows.Scan(&ruleID, &signature, &raw); err != nil {
			log.Printf("read manually resolved alarm: %v", err)
			continue
		}
		rule, ok := ruleByID(m.config.Rules, ruleID)
		var event Event
		if !ok || !rule.IsEnabled() || !rule.AllowManualRecovery || ruleSignature(rule) != signature || json.Unmarshal([]byte(raw), &event) != nil {
			obsolete = append(obsolete, ruleID)
			continue
		}
		m.runtimes[ruleID] = &ruleRuntime{ManualReset: &event}
	}
	if err := rows.Err(); err != nil {
		log.Printf("load manually resolved alarms: %v", err)
	}
	rows.Close() // Single SQLite connection: close before issuing deletes.
	for _, ruleID := range obsolete {
		if err := m.storage.deleteManualReset(ruleID); err != nil {
			log.Printf("remove obsolete manual reset: %v", err)
		}
	}
}

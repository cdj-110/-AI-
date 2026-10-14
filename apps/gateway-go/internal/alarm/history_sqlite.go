package alarm

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type HistoryQuery struct {
	Page       int
	PageSize   int
	Level      string
	State      string
	DeviceKey  string
	Metric     string
	Keyword    string
	StartedAt  *time.Time
	FinishedAt *time.Time
}

type HistoryPage struct {
	Items      []Event `json:"items"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	Total      int     `json:"total"`
	TotalPages int     `json:"totalPages"`
}

type sqliteHistoryStore struct {
	db   *sql.DB
	path string
}

const alarmHistorySchema = `
CREATE TABLE IF NOT EXISTS alarm_manual_resets (
  rule_id TEXT PRIMARY KEY,
  signature TEXT NOT NULL,
  event_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS alarm_history (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  event_id TEXT NOT NULL,
  rule_id TEXT NOT NULL,
  rule_name TEXT NOT NULL,
  device_key TEXT NOT NULL,
  metric TEXT NOT NULL,
  point_name TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL,
  state TEXT NOT NULL,
  acknowledged INTEGER NOT NULL DEFAULT 0,
  acknowledged_at TEXT,
  value_json TEXT NOT NULL DEFAULT 'null',
  value_text TEXT NOT NULL DEFAULT '',
  threshold REAL,
  quality TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  recovered_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS alarm_history_revision ON alarm_history(event_id, state, updated_at);
CREATE INDEX IF NOT EXISTS alarm_history_updated ON alarm_history(updated_at DESC, seq DESC);
CREATE INDEX IF NOT EXISTS alarm_history_level_updated ON alarm_history(level, updated_at DESC);
CREATE INDEX IF NOT EXISTS alarm_history_device_metric_updated ON alarm_history(device_key, metric, updated_at DESC);
CREATE INDEX IF NOT EXISTS alarm_history_state_updated ON alarm_history(state, updated_at DESC);
`

func openSQLiteHistory(path, legacyPath string, retentionLimit, recentLimit int) (*sqliteHistoryStore, []Event, error) {
	if strings.TrimSpace(path) == "" {
		return nil, []Event{}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &sqliteHistoryStore{db: db, path: path}
	if _, err = db.Exec(alarmHistorySchema); err != nil {
		db.Close()
		return nil, nil, err
	}
	if err = store.migrateJSONL(legacyPath); err != nil {
		db.Close()
		return nil, nil, err
	}
	if err = store.Prune(retentionLimit); err != nil {
		db.Close()
		return nil, nil, err
	}
	page, err := store.Query(HistoryQuery{Page: 1, PageSize: recentLimit})
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return store, page.Items, nil
}

func (s *sqliteHistoryStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sqliteHistoryStore) Append(event Event, limit int) error {
	if s == nil || s.db == nil {
		return nil
	}
	if _, err := insertEvent(s.db, event); err != nil {
		return err
	}
	return s.Prune(limit)
}

func (s *sqliteHistoryStore) Prune(limit int) error {
	if s == nil || s.db == nil || limit <= 0 {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM alarm_history WHERE seq <= COALESCE((SELECT seq FROM alarm_history ORDER BY seq DESC LIMIT 1 OFFSET ?), 0)`, limit)
	return err
}

func (s *sqliteHistoryStore) Query(query HistoryQuery) (HistoryPage, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 50
	}
	if query.PageSize > 10000 {
		query.PageSize = 10000
	}
	where, args := historyWhere(query)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM alarm_history`+where, args...).Scan(&total); err != nil {
		return HistoryPage{}, err
	}
	rows, err := s.db.Query(`SELECT event_id, rule_id, rule_name, device_key, metric, point_name, level, state,
acknowledged, acknowledged_at, value_json, value_text, threshold, quality, message, occurred_at, updated_at, recovered_at
FROM alarm_history`+where+` ORDER BY updated_at DESC, seq DESC LIMIT ? OFFSET ?`, append(args, query.PageSize, (query.Page-1)*query.PageSize)...)
	if err != nil {
		return HistoryPage{}, err
	}
	defer rows.Close()
	items := make([]Event, 0, query.PageSize)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return HistoryPage{}, err
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return HistoryPage{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	return HistoryPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total, TotalPages: totalPages}, nil
}

func historyWhere(query HistoryQuery) (string, []interface{}) {
	clauses := make([]string, 0, 8)
	args := make([]interface{}, 0, 8)
	addEqual := func(column, value string) {
		if value = strings.TrimSpace(value); value != "" {
			clauses = append(clauses, column+" = ?")
			args = append(args, value)
		}
	}
	addEqual("level", strings.ToUpper(query.Level))
	addEqual("state", strings.ToUpper(query.State))
	addEqual("device_key", query.DeviceKey)
	addEqual("metric", query.Metric)
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		clauses = append(clauses, `(rule_name LIKE ? OR device_key LIKE ? OR metric LIKE ? OR point_name LIKE ? OR message LIKE ?)`)
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}
	if query.StartedAt != nil {
		clauses = append(clauses, "updated_at >= ?")
		args = append(args, formatSQLiteTime(*query.StartedAt))
	}
	if query.FinishedAt != nil {
		clauses = append(clauses, "updated_at <= ?")
		args = append(args, formatSQLiteTime(*query.FinishedAt))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

type eventScanner interface {
	Scan(dest ...interface{}) error
}

func scanEvent(scanner eventScanner) (Event, error) {
	var event Event
	var acknowledged int
	var acknowledgedAt, recoveredAt sql.NullString
	var valueJSON, occurredAt, updatedAt string
	var threshold sql.NullFloat64
	if err := scanner.Scan(&event.EventID, &event.RuleID, &event.RuleName, &event.DeviceKey, &event.Metric, &event.PointName,
		&event.Level, &event.State, &acknowledged, &acknowledgedAt, &valueJSON, &event.ValueText, &threshold,
		&event.Quality, &event.Message, &occurredAt, &updatedAt, &recoveredAt); err != nil {
		return Event{}, err
	}
	event.Acknowledged = acknowledged != 0
	if threshold.Valid {
		event.Threshold = &threshold.Float64
	}
	if acknowledgedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, acknowledgedAt.String)
		if err != nil {
			return Event{}, err
		}
		event.AcknowledgedAt = &parsed
	}
	if recoveredAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, recoveredAt.String)
		if err != nil {
			return Event{}, err
		}
		event.RecoveredAt = &parsed
	}
	var value interface{}
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err == nil {
		event.Value = value
	}
	var err error
	if event.OccurredAt, err = time.Parse(time.RFC3339Nano, occurredAt); err != nil {
		return Event{}, err
	}
	if event.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		return Event{}, err
	}
	return event, nil
}

type sqlExecutor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func insertEvent(executor sqlExecutor, event Event) (sql.Result, error) {
	valueJSON, err := json.Marshal(event.Value)
	if err != nil {
		valueJSON = []byte("null")
	}
	return executor.Exec(`INSERT OR IGNORE INTO alarm_history (
event_id, rule_id, rule_name, device_key, metric, point_name, level, state, acknowledged, acknowledged_at,
value_json, value_text, threshold, quality, message, occurred_at, updated_at, recovered_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.EventID, event.RuleID, event.RuleName, event.DeviceKey, event.Metric, event.PointName, event.Level, event.State,
		boolInt(event.Acknowledged), nullableTime(event.AcknowledgedAt), string(valueJSON), event.ValueText, event.Threshold,
		event.Quality, event.Message, formatSQLiteTime(event.OccurredAt), formatSQLiteTime(event.UpdatedAt), nullableTime(event.RecoveredAt))
}

func (s *sqliteHistoryStore) migrateJSONL(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		_ = file.Close()
		return err
	}
	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event Event
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.EventID == "" {
			continue
		}
		if _, err := insertEvent(tx, event); err != nil {
			tx.Rollback()
			_ = file.Close()
			return err
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		tx.Rollback()
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	backupPath := path + ".migrated"
	if _, err := os.Stat(backupPath); err == nil {
		backupPath += "." + time.Now().UTC().Format("20060102T150405Z")
	}
	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("SQLite migration completed but preserving JSONL backup failed: %w", err)
	}
	return nil
}

func formatSQLiteTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableTime(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return formatSQLiteTime(*value)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func memoryHistoryPage(history []Event, query HistoryQuery) HistoryPage {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 50
	}
	if query.PageSize > 10000 {
		query.PageSize = 10000
	}
	filtered := make([]Event, 0, len(history))
	for _, event := range history {
		if query.Level != "" && !strings.EqualFold(event.Level, query.Level) || query.State != "" && !strings.EqualFold(event.State, query.State) || query.DeviceKey != "" && event.DeviceKey != query.DeviceKey || query.Metric != "" && event.Metric != query.Metric {
			continue
		}
		if query.StartedAt != nil && event.UpdatedAt.Before(*query.StartedAt) || query.FinishedAt != nil && event.UpdatedAt.After(*query.FinishedAt) {
			continue
		}
		if keyword := strings.ToLower(strings.TrimSpace(query.Keyword)); keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{event.RuleName, event.DeviceKey, event.Metric, event.PointName, event.Message}, " "))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		filtered = append(filtered, event)
	}
	start := (query.Page - 1) * query.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	totalPages := 0
	if len(filtered) > 0 {
		totalPages = (len(filtered) + query.PageSize - 1) / query.PageSize
	}
	return HistoryPage{Items: append([]Event{}, filtered[start:end]...), Page: query.Page, PageSize: query.PageSize, Total: len(filtered), TotalPages: totalPages}
}

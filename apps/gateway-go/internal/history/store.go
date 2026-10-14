package history

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

const databaseFileName = "telemetry-history.db"

var integrityChecked sync.Map

var (
	ErrDisabled = errors.New("history storage is disabled")
	ErrLowDisk  = errors.New("历史存储剩余空间不足，写入已暂停")
)

type DiskSpaceFunc func() (totalBytes uint64, freeBytes uint64, err error)

type Store struct {
	queuePending  atomic.Int64
	queueDropped  atomic.Uint64
	mu            sync.Mutex
	dir           string
	maxBytes      int64
	enabled       bool
	available     func() bool
	diskSpace     DiskSpaceFunc
	minFree       uint64
	retention     time.Duration
	aggregation   bool
	lastAggregate time.Time
	db            *sql.DB
	lastWrite     time.Time
	lastError     string
	detached      bool
}

type Status struct {
	PendingSamples   int64  `json:"pendingSamples"`
	DroppedSamples   uint64 `json:"droppedSamples"`
	Enabled          bool   `json:"enabled"`
	Path             string `json:"path,omitempty"`
	DatabasePath     string `json:"databasePath,omitempty"`
	Bytes            int64  `json:"bytes"`
	MaxBytes         int64  `json:"maxBytes"`
	Files            int    `json:"files"`
	Samples          int64  `json:"samples"`
	LegacyBytes      int64  `json:"legacyBytes"`
	LegacyFiles      int    `json:"legacyFiles"`
	Writable         bool   `json:"writable"`
	Paused           bool   `json:"paused"`
	PauseReason      string `json:"pauseReason,omitempty"`
	TotalBytes       uint64 `json:"totalBytes,omitempty"`
	FreeBytes        uint64 `json:"freeBytes,omitempty"`
	MinFreeBytes     uint64 `json:"minFreeBytes,omitempty"`
	LastWriteAt      string `json:"lastWriteAt,omitempty"`
	LastError        string `json:"lastError,omitempty"`
	RecoveryCount    int    `json:"recoveryCount"`
	LastRecoveryPath string `json:"lastRecoveryPath,omitempty"`
}

type QueryOptions struct {
	TaskKey    string
	DeviceKey  string
	Metric     string
	Start      time.Time
	End        time.Time
	Offset     int
	Limit      int
	Resolution string
}

type TaskStat struct {
	TaskKey        string `json:"taskKey"`
	Samples        int64  `json:"samples"`
	EstimatedBytes int64  `json:"estimatedBytes"`
	LastWriteAt    string `json:"lastWriteAt,omitempty"`
}

type Record struct {
	ID               int64       `json:"id"`
	Time             string      `json:"time"`
	DeviceKey        string      `json:"deviceKey"`
	Metric           string      `json:"metric"`
	Value            interface{} `json:"value"`
	ValueText        string      `json:"valueText"`
	ActualDataType   string      `json:"actualDataType,omitempty"`
	Quality          string      `json:"quality,omitempty"`
	RawQuality       int64       `json:"rawQuality,omitempty"`
	SourceTimestamp  string      `json:"sourceTimestamp,omitempty"`
	ReceiveTimestamp string      `json:"receiveTimestamp,omitempty"`
	CollectionMode   string      `json:"collectionMode,omitempty"`
	DiagnosticCode   int64       `json:"diagnosticCode,omitempty"`
	Diagnostic       string      `json:"diagnostic,omitempty"`
	Resolution       string      `json:"resolution,omitempty"`
	SampleCount      int64       `json:"sampleCount,omitempty"`
	Minimum          *float64    `json:"minimum,omitempty"`
	Maximum          *float64    `json:"maximum,omitempty"`
	Average          *float64    `json:"average,omitempty"`
}

type QueryResult struct {
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
	Total      int64    `json:"total"`
	Items      []Record `json:"items"`
	Resolution string   `json:"resolution,omitempty"`
}

const schema = `
CREATE TABLE IF NOT EXISTS history_samples (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp_ms INTEGER NOT NULL,
  device_key TEXT NOT NULL,
  metric TEXT NOT NULL,
  value_type TEXT NOT NULL,
  numeric_value REAL,
  value_json TEXT NOT NULL,
  integer_value TEXT,
  special_value TEXT,
  actual_data_type TEXT,
  quality TEXT,
  raw_quality INTEGER,
  source_timestamp_ms INTEGER,
  receive_timestamp_ms INTEGER,
  collection_mode TEXT,
  diagnostic_code INTEGER,
  diagnostic TEXT
);
CREATE INDEX IF NOT EXISTS history_point_time ON history_samples(device_key, metric, timestamp_ms DESC, id DESC);
CREATE INDEX IF NOT EXISTS history_time ON history_samples(timestamp_ms DESC, id DESC);
CREATE TABLE IF NOT EXISTS history_sample_tasks (
  sample_id INTEGER NOT NULL,
  task_key TEXT NOT NULL,
  PRIMARY KEY(sample_id, task_key),
  FOREIGN KEY(sample_id) REFERENCES history_samples(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS history_task_sample ON history_sample_tasks(task_key, sample_id DESC);
CREATE TABLE IF NOT EXISTS history_aggregates (
  period TEXT NOT NULL,
  bucket_start_ms INTEGER NOT NULL,
  task_key TEXT NOT NULL DEFAULT '',
  device_key TEXT NOT NULL,
  metric TEXT NOT NULL,
  sample_count INTEGER NOT NULL,
  minimum REAL NOT NULL,
  maximum REAL NOT NULL,
  average REAL NOT NULL,
  first_value REAL NOT NULL,
  last_value REAL NOT NULL,
  good_count INTEGER NOT NULL,
  bad_count INTEGER NOT NULL,
  uncertain_count INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL,
  PRIMARY KEY(period, bucket_start_ms, task_key, device_key, metric)
);
CREATE INDEX IF NOT EXISTS history_aggregate_point_time ON history_aggregates(period, task_key, device_key, metric, bucket_start_ms DESC);
CREATE TABLE IF NOT EXISTS history_aggregate_progress (
  period TEXT PRIMARY KEY,
  covered_until_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL
);
`

func NewGuarded(dir string, maxBytes int64, available func() bool) *Store {
	return &Store{dir: dir, maxBytes: maxBytes, enabled: dir != "", available: available}
}

func (s *Store) SetDiskGuard(space DiskSpaceFunc, minFreeBytes uint64) *Store {
	s.mu.Lock()
	s.diskSpace = space
	s.minFree = minFreeBytes
	s.mu.Unlock()
	return s
}

func (s *Store) SetRetentionDays(days int) *Store {
	s.mu.Lock()
	if days > 0 {
		s.retention = time.Duration(days) * 24 * time.Hour
	} else {
		s.retention = 0
	}
	s.mu.Unlock()
	return s
}

func (s *Store) SetAggregationEnabled(enabled bool) *Store {
	s.mu.Lock()
	s.aggregation = enabled
	s.mu.Unlock()
	return s
}

func Disabled() *Store { return &Store{} }

func Path(mountPath string) string { return filepath.Join(mountPath, ".weikong", "history") }

func DatabasePath(dir string) string { return filepath.Join(dir, databaseFileName) }

func (s *Store) Append(reading model.Reading) error {
	if len(reading.Metrics) == 0 {
		return nil
	}
	if reading.Time.IsZero() {
		reading.Time = time.Now()
	}
	samples := make([]Sample, 0, len(reading.Metrics))
	for metric, value := range reading.Metrics {
		samples = append(samples, Sample{
			Time:             reading.Time,
			DeviceKey:        reading.DeviceKey,
			Metric:           metric,
			Value:            value,
			Quality:          model.QualityGood,
			ReceiveTimestamp: reading.Time,
			CollectionMode:   "legacy",
		})
	}
	return s.AppendBatch(samples)
}

// AppendBatch persists samples from any number of devices in one transaction.
// Capacity pruning is intentionally separate so callers can keep it off the
// collection path and run it from a background maintenance loop.
func (s *Store) AppendBatch(samples []Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return ErrDisabled
	}
	if len(samples) == 0 {
		return nil
	}
	if err := s.checkWritableLocked(); err != nil {
		s.lastError = err.Error()
		return err
	}
	if err := s.ensureDatabaseLocked(true); err != nil {
		s.lastError = err.Error()
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO history_samples(
		timestamp_ms, device_key, metric, value_type, numeric_value, value_json,
		integer_value, special_value, actual_data_type, quality, raw_quality,
		source_timestamp_ms, receive_timestamp_ms, collection_mode,
		diagnostic_code, diagnostic
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	taskStatement, err := tx.Prepare(`INSERT OR IGNORE INTO history_sample_tasks(sample_id, task_key) VALUES(?, ?)`)
	if err != nil {
		_ = statement.Close()
		_ = tx.Rollback()
		return err
	}
	sort.SliceStable(samples, func(left, right int) bool {
		if samples[left].DeviceKey == samples[right].DeviceKey {
			return samples[left].Metric < samples[right].Metric
		}
		return samples[left].DeviceKey < samples[right].DeviceKey
	})
	for _, sample := range samples {
		if sample.Time.IsZero() {
			sample.Time = time.Now()
		}
		encoded, encodeErr := encodeValue(sample.Value)
		if encodeErr != nil {
			_ = taskStatement.Close()
			_ = statement.Close()
			_ = tx.Rollback()
			return encodeErr
		}
		actualDataType := sample.ActualDataType
		if actualDataType == "" {
			actualDataType = encoded.ValueType
		}
		quality := sample.Quality
		if quality == "" {
			quality = model.QualityGood
		}
		insertResult, insertErr := statement.Exec(
			sample.Time.UnixMilli(), sample.DeviceKey, sample.Metric,
			encoded.ValueType, encoded.NumericValue, encoded.JSON,
			encoded.IntegerValue, encoded.SpecialValue, actualDataType,
			string(quality), int64(sample.RawQuality), nullableTime(sample.SourceTimestamp),
			nullableTime(sample.ReceiveTimestamp), sample.CollectionMode,
			sample.DiagnosticCode, sample.Diagnostic,
		)
		if insertErr != nil {
			err = insertErr
			_ = taskStatement.Close()
			_ = statement.Close()
			_ = tx.Rollback()
			return err
		}
		sampleID, idErr := insertResult.LastInsertId()
		if idErr != nil {
			_ = taskStatement.Close()
			_ = statement.Close()
			_ = tx.Rollback()
			return idErr
		}
		for _, taskKey := range uniqueStrings(sample.TaskKeys) {
			if _, err = taskStatement.Exec(sampleID, taskKey); err != nil {
				_ = taskStatement.Close()
				_ = statement.Close()
				_ = tx.Rollback()
				return err
			}
		}
	}
	if err = taskStatement.Close(); err != nil {
		_ = statement.Close()
		_ = tx.Rollback()
		return err
	}
	if err = statement.Close(); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		s.lastError = err.Error()
		return err
	}
	s.lastWrite = time.Now()
	s.lastError = ""
	return nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func nullableTime(value time.Time) interface{} {
	if value.IsZero() {
		return nil
	}
	return value.UnixMilli()
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *Store) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := Status{Enabled: s.isAvailable(), Path: s.dir, DatabasePath: DatabasePath(s.dir), MaxBytes: s.maxBytes, MinFreeBytes: s.minFree, LastError: s.lastError}
	status.PendingSamples, status.DroppedSamples = s.queuePending.Load(), s.queueDropped.Load()
	status.Writable = status.Enabled
	if !status.Enabled {
		// Do not open or even inspect database files under a lost mount. That
		// directory can now be the root filesystem or a different USB partition.
		if s.enabled {
			status.Paused = true
			status.PauseReason = s.lastError
		}
		return status
	}
	if status.Enabled {
		if err := s.checkWritableLocked(); err != nil {
			status.Writable = false
			status.Paused = true
			status.PauseReason = err.Error()
		}
	}
	if s.diskSpace != nil {
		status.TotalBytes, status.FreeBytes, _ = s.diskSpace()
	}
	legacy, _ := s.historyFilesLocked()
	status.LegacyFiles = len(legacy)
	for _, path := range legacy {
		if info, err := os.Stat(path); err == nil {
			status.LegacyBytes += info.Size()
		}
	}
	status.Bytes = status.LegacyBytes + s.databaseBytesLocked()
	status.Files = status.LegacyFiles
	if _, err := os.Stat(DatabasePath(s.dir)); err == nil {
		status.Files++
		if err := s.ensureDatabaseLocked(false); err != nil {
			s.lastError = err.Error()
			status.Writable = false
			status.Paused = true
			status.PauseReason = err.Error()
		} else if s.db != nil {
			var lastWrite sql.NullInt64
			_ = s.db.QueryRow(`SELECT COUNT(*), MAX(timestamp_ms) FROM history_samples`).Scan(&status.Samples, &lastWrite)
			if lastWrite.Valid {
				status.LastWriteAt = time.UnixMilli(lastWrite.Int64).Format(time.RFC3339Nano)
			}
		}
	}
	if !s.lastWrite.IsZero() {
		status.LastWriteAt = s.lastWrite.Format(time.RFC3339Nano)
	}
	status.LastError = s.lastError
	status.RecoveryCount, status.LastRecoveryPath = s.recoveryStatusLocked()
	return status
}

func (s *Store) Query(options QueryOptions) (QueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := QueryResult{Offset: options.Offset, Limit: options.Limit, Items: []Record{}}
	if !s.isAvailable() {
		return result, ErrDisabled
	}
	if options.Offset < 0 {
		options.Offset = 0
		result.Offset = 0
	}
	if options.Limit < 1 {
		options.Limit = 100
	}
	if options.Limit > 500 {
		options.Limit = 500
	}
	result.Limit = options.Limit
	if err := s.ensureDatabaseLocked(false); err != nil {
		return result, err
	}
	if s.db == nil {
		return result, nil
	}
	resolution := chooseResolution(options, s.aggregation)
	if resolution != "raw" {
		aggregated, err := s.queryAggregateLocked(options, resolution)
		if err != nil {
			return result, err
		}
		if aggregated.Total > 0 {
			return aggregated, nil
		}
	}
	result.Resolution = "raw"
	from := " FROM history_samples"
	if options.TaskKey != "" {
		from += " JOIN history_sample_tasks ON history_sample_tasks.sample_id = history_samples.id"
	}
	where, args := historyQueryWhere(options)
	if err := s.db.QueryRow(`SELECT COUNT(*)`+from+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	queryArgs := append(append([]interface{}{}, args...), options.Limit, options.Offset)
	rows, err := s.db.Query(`SELECT id, timestamp_ms, device_key, metric, value_json,
		special_value, integer_value, actual_data_type, quality, raw_quality,
		source_timestamp_ms, receive_timestamp_ms, collection_mode,
		diagnostic_code, diagnostic`+from+where+`
		ORDER BY timestamp_ms DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var record Record
		var timestamp int64
		var valueJSON string
		var specialValue, integerValue, actualType, quality, mode, diagnostic sql.NullString
		var rawQuality, sourceTimestamp, receiveTimestamp, diagnosticCode sql.NullInt64
		if err := rows.Scan(
			&record.ID, &timestamp, &record.DeviceKey, &record.Metric, &valueJSON,
			&specialValue, &integerValue, &actualType, &quality, &rawQuality,
			&sourceTimestamp, &receiveTimestamp, &mode, &diagnosticCode, &diagnostic,
		); err != nil {
			return result, err
		}
		record.Time = time.UnixMilli(timestamp).Format(time.RFC3339Nano)
		record.Value, record.ValueText = decodeHistoryValue(valueJSON, specialValue, integerValue)
		record.ActualDataType, record.Quality, record.CollectionMode, record.Diagnostic = actualType.String, quality.String, mode.String, diagnostic.String
		if rawQuality.Valid {
			record.RawQuality = rawQuality.Int64
		}
		if diagnosticCode.Valid {
			record.DiagnosticCode = diagnosticCode.Int64
		}
		if sourceTimestamp.Valid {
			record.SourceTimestamp = time.UnixMilli(sourceTimestamp.Int64).Format(time.RFC3339Nano)
		}
		if receiveTimestamp.Valid {
			record.ReceiveTimestamp = time.UnixMilli(receiveTimestamp.Int64).Format(time.RFC3339Nano)
		}
		result.Items = append(result.Items, record)
	}
	return result, rows.Err()
}

func (s *Store) TaskStats() ([]TaskStat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []TaskStat{}
	if !s.isAvailable() {
		return result, ErrDisabled
	}
	if err := s.ensureDatabaseLocked(false); err != nil {
		return result, err
	}
	if s.db == nil {
		return result, nil
	}
	rows, err := s.db.Query(`SELECT links.task_key, COUNT(*),
		COALESCE(SUM(LENGTH(samples.value_json)+LENGTH(samples.device_key)+LENGTH(samples.metric)+96),0),
		MAX(samples.timestamp_ms)
		FROM history_sample_tasks links
		JOIN history_samples samples ON samples.id=links.sample_id
		GROUP BY links.task_key ORDER BY links.task_key`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item TaskStat
		var lastWrite sql.NullInt64
		if err := rows.Scan(&item.TaskKey, &item.Samples, &item.EstimatedBytes, &lastWrite); err != nil {
			return result, err
		}
		if lastWrite.Valid {
			item.LastWriteAt = time.UnixMilli(lastWrite.Int64).Format(time.RFC3339Nano)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func historyQueryWhere(options QueryOptions) (string, []interface{}) {
	conditions := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)
	if options.TaskKey != "" {
		conditions = append(conditions, "history_sample_tasks.task_key = ?")
		args = append(args, options.TaskKey)
	}
	if options.DeviceKey != "" {
		conditions = append(conditions, "device_key = ?")
		args = append(args, options.DeviceKey)
	}
	if options.Metric != "" {
		conditions = append(conditions, "metric = ?")
		args = append(args, options.Metric)
	}
	if !options.Start.IsZero() {
		conditions = append(conditions, "timestamp_ms >= ?")
		args = append(args, options.Start.UnixMilli())
	}
	if !options.End.IsZero() {
		conditions = append(conditions, "timestamp_ms <= ?")
		args = append(args, options.End.UnixMilli())
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func decodeHistoryValue(valueJSON string, specialValue, integerValue sql.NullString) (interface{}, string) {
	if specialValue.Valid {
		return specialValue.String, specialValue.String
	}
	text := csvValue(valueJSON)
	if integerValue.Valid {
		text = integerValue.String
	}
	var value interface{}
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return valueJSON, text
	}
	return value, text
}

func (s *Store) ExportCSV(writer io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return ErrDisabled
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{
		"time", "deviceKey", "metric", "value", "actualDataType", "quality",
		"rawQuality", "sourceTime", "receiveTime", "collectionMode",
		"diagnosticCode", "diagnostic",
	}); err != nil {
		return err
	}
	legacy, err := s.historyFilesLocked()
	if err != nil {
		return err
	}
	for _, path := range legacy {
		if err := exportLegacyFileCSV(csvWriter, path); err != nil {
			return err
		}
	}
	if _, err := os.Stat(DatabasePath(s.dir)); err == nil {
		if err := s.ensureDatabaseLocked(false); err != nil {
			return err
		}
		rows, err := s.db.Query(`SELECT
			timestamp_ms, device_key, metric, value_json, special_value,
			actual_data_type, quality, raw_quality, source_timestamp_ms,
			receive_timestamp_ms, collection_mode, diagnostic_code, diagnostic
			FROM history_samples ORDER BY timestamp_ms, id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var deviceKey, metric, valueJSON string
			var special, actualType, quality, mode, diagnostic sql.NullString
			var rawQuality, sourceTimestamp, receiveTimestamp, diagnosticCode sql.NullInt64
			if err := rows.Scan(
				&timestamp, &deviceKey, &metric, &valueJSON, &special,
				&actualType, &quality, &rawQuality, &sourceTimestamp,
				&receiveTimestamp, &mode, &diagnosticCode, &diagnostic,
			); err != nil {
				return err
			}
			valueText := csvValue(valueJSON)
			if special.Valid {
				valueText = special.String
			}
			if err := csvWriter.Write([]string{
				time.UnixMilli(timestamp).Format(time.RFC3339Nano), deviceKey, metric, valueText,
				actualType.String, quality.String, nullableIntegerText(rawQuality),
				nullableTimestampText(sourceTimestamp), nullableTimestampText(receiveTimestamp),
				mode.String, nullableIntegerText(diagnosticCode), diagnostic.String,
			}); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func (s *Store) ExportFilteredCSV(writer io.Writer, options QueryOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return ErrDisabled
	}
	if err := s.ensureDatabaseLocked(false); err != nil {
		return err
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"time", "deviceKey", "metric", "value", "actualDataType", "quality", "rawQuality", "sourceTime", "receiveTime", "collectionMode", "diagnosticCode", "diagnostic"}); err != nil {
		return err
	}
	if s.db != nil {
		from := " FROM history_samples"
		if options.TaskKey != "" {
			from += " JOIN history_sample_tasks ON history_sample_tasks.sample_id = history_samples.id"
		}
		where, args := historyQueryWhere(options)
		rows, err := s.db.Query(`SELECT timestamp_ms, device_key, metric, value_json, special_value,
			actual_data_type, quality, raw_quality, source_timestamp_ms,
			receive_timestamp_ms, collection_mode, diagnostic_code, diagnostic`+from+where+` ORDER BY timestamp_ms, history_samples.id`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var timestamp int64
			var deviceKey, metric, valueJSON string
			var special, actualType, quality, mode, diagnostic sql.NullString
			var rawQuality, sourceTimestamp, receiveTimestamp, diagnosticCode sql.NullInt64
			if err := rows.Scan(&timestamp, &deviceKey, &metric, &valueJSON, &special, &actualType, &quality, &rawQuality, &sourceTimestamp, &receiveTimestamp, &mode, &diagnosticCode, &diagnostic); err != nil {
				return err
			}
			valueText := csvValue(valueJSON)
			if special.Valid {
				valueText = special.String
			}
			if err := csvWriter.Write([]string{time.UnixMilli(timestamp).Format(time.RFC3339Nano), deviceKey, metric, valueText, actualType.String, quality.String, nullableIntegerText(rawQuality), nullableTimestampText(sourceTimestamp), nullableTimestampText(receiveTimestamp), mode.String, nullableIntegerText(diagnosticCode), diagnostic.String}); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func (s *Store) BackupTo(destinationDir string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return "", ErrDisabled
	}
	if filepath.Clean(destinationDir) == filepath.Clean(s.dir) {
		return "", errors.New("source and destination history directories are the same")
	}
	if err := s.ensureDatabaseLocked(true); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destinationDir, 0700); err != nil {
		return "", err
	}
	temporary := DatabasePath(destinationDir) + ".migrate-tmp"
	_ = os.Remove(temporary)
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(PASSIVE);`); err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(temporary, "'", "''")
	if _, err := s.db.Exec(`VACUUM INTO '` + escaped + `'`); err != nil {
		_ = os.Remove(temporary)
		return "", err
	}
	check, err := sql.Open("sqlite3", temporary+"?_busy_timeout=5000")
	if err != nil {
		_ = os.Remove(temporary)
		return "", err
	}
	var integrity string
	err = check.QueryRow(`PRAGMA quick_check(1);`).Scan(&integrity)
	_ = check.Close()
	if err != nil {
		_ = os.Remove(temporary)
		return "", fmt.Errorf("migrated history database integrity check failed: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(integrity), "ok") {
		_ = os.Remove(temporary)
		return "", fmt.Errorf("migrated history database integrity check failed: %s", integrity)
	}
	destination := DatabasePath(destinationDir)
	var previous string
	if _, err := os.Stat(destination); err == nil {
		previous = destination + ".before-migrate-" + time.Now().Format("20060102-150405")
		if err := os.Rename(destination, previous); err != nil {
			_ = os.Remove(temporary)
			return "", err
		}
	}
	// A detached WAL/SHM from a previous database must never be paired with the
	// newly migrated main file.
	_ = os.Remove(destination + "-wal")
	_ = os.Remove(destination + "-shm")
	if err := os.Rename(temporary, destination); err != nil {
		if previous != "" {
			_ = os.Rename(previous, destination)
		}
		return "", err
	}
	return destination, nil
}

func nullableIntegerText(value sql.NullInt64) string {
	if !value.Valid {
		return ""
	}
	return strconv.FormatInt(value.Int64, 10)
}

func nullableTimestampText(value sql.NullInt64) string {
	if !value.Valid {
		return ""
	}
	return time.UnixMilli(value.Int64).Format(time.RFC3339Nano)
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return nil
	}
	// Clear through SQLite instead of unlinking the database. The running
	// gateway keeps its own connection open; deleting the file would leave
	// that connection writing to an unlinked inode until the next reload.
	if err := s.ensureDatabaseLocked(false); err != nil {
		return err
	}
	if s.db != nil {
		if _, err := s.db.Exec(`DELETE FROM history_samples`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`PRAGMA incremental_vacuum;`); err != nil {
			return err
		}
	}
	legacy, err := s.historyFilesLocked()
	if err != nil {
		return err
	}
	for _, path := range legacy {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Store) ensureDatabaseLocked(create bool) error {
	if !s.isAvailable() {
		return ErrDisabled
	}
	if s.db != nil {
		return nil
	}
	path := DatabasePath(s.dir)
	existed := false
	if !create {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		} else {
			existed = true
		}
	} else if _, err := os.Stat(path); err == nil {
		existed = true
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	db, err := s.openDatabaseLocked(path, existed)
	if err == nil {
		s.db = db
		return nil
	}
	if !existed || !isSQLiteCorruption(err) {
		return err
	}
	if !s.isAvailable() {
		return ErrDisabled
	}
	if db != nil {
		_ = db.Close()
	}
	quarantined, quarantineErr := s.quarantineDatabaseLocked(path)
	if quarantineErr != nil {
		return fmt.Errorf("history database is corrupt (%v) and could not be quarantined: %w", err, quarantineErr)
	}
	s.lastError = "SQLite 数据库损坏，已隔离到 " + quarantined
	db, err = s.openDatabaseLocked(path, false)
	if err != nil {
		return fmt.Errorf("create history database after recovery: %w", err)
	}
	s.db = db
	return nil
}

func (s *Store) openDatabaseLocked(path string, checkIntegrity bool) (*sql.DB, error) {
	journal, synchronous := "WAL", "NORMAL"
	if s.available != nil {
		// Removable media must not leave SQLite using a file-backed WAL index
		// mapping after surprise removal. Rollback journaling uses ordinary I/O.
		journal, synchronous = "DELETE", "FULL"
	}
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode="+journal+"&_synchronous="+synchronous+"&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA mmap_size=0;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	_, alreadyChecked := integrityChecked.Load(path)
	if checkIntegrity && !alreadyChecked {
		var result string
		if err = db.QueryRow(`PRAGMA quick_check(1);`).Scan(&result); err != nil {
			_ = db.Close()
			return nil, err
		}
		if !strings.EqualFold(strings.TrimSpace(result), "ok") {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite quick_check failed: %s", result)
		}
	}
	if _, err = db.Exec(`PRAGMA auto_vacuum=INCREMENTAL;`); err == nil {
		_, err = db.Exec(schema)
		if err == nil {
			err = migrateHistorySchema(db)
		}
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	integrityChecked.Store(path, struct{}{})
	return db, nil
}

func isSQLiteCorruption(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{"quick_check failed", "database disk image is malformed", "file is not a database", "database corruption", "database malformed"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (s *Store) quarantineDatabaseLocked(path string) (string, error) {
	integrityChecked.Delete(path)
	stamp := time.Now().Format("20060102-150405.000")
	quarantinePath := path + ".corrupt-" + stamp
	if err := os.Rename(path, quarantinePath); err != nil {
		return "", err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Rename(path+suffix, quarantinePath+suffix); err != nil && !os.IsNotExist(err) {
			return quarantinePath, err
		}
	}
	return quarantinePath, nil
}

func (s *Store) recoveryStatusLocked() (int, string) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, ""
	}
	prefix := databaseFileName + ".corrupt-"
	count, latest := 0, ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || strings.HasSuffix(entry.Name(), "-wal") || strings.HasSuffix(entry.Name(), "-shm") {
			continue
		}
		count++
		if entry.Name() > filepath.Base(latest) {
			latest = filepath.Join(s.dir, entry.Name())
		}
	}
	return count, latest
}

func (s *Store) checkWritableLocked() error {
	if !s.isAvailable() {
		return ErrDisabled
	}
	if s.diskSpace == nil || s.minFree == 0 {
		return nil
	}
	_, freeBytes, err := s.diskSpace()
	if err != nil {
		return fmt.Errorf("inspect history storage space: %w", err)
	}
	if freeBytes < s.minFree {
		return fmt.Errorf("%w：当前可用 %d 字节，最低要求 %d 字节", ErrLowDisk, freeBytes, s.minFree)
	}
	return nil
}

func migrateHistorySchema(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(history_samples)`)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		existing[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	columns := []struct{ name, definition string }{
		{"integer_value", "TEXT"}, {"special_value", "TEXT"},
		{"actual_data_type", "TEXT"}, {"quality", "TEXT"},
		{"raw_quality", "INTEGER"}, {"source_timestamp_ms", "INTEGER"},
		{"receive_timestamp_ms", "INTEGER"}, {"collection_mode", "TEXT"},
		{"diagnostic_code", "INTEGER"}, {"diagnostic", "TEXT"},
	}
	for _, column := range columns {
		if existing[column.name] {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE history_samples ADD COLUMN ` + column.name + ` ` + column.definition); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Prune() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return ErrDisabled
	}
	if err := s.ensureDatabaseLocked(false); err != nil {
		return err
	}
	if s.db == nil {
		return nil
	}
	return s.pruneLocked()
}

func (s *Store) pruneLocked() error {
	if s.aggregation && (s.lastAggregate.IsZero() || time.Since(s.lastAggregate) >= time.Minute) {
		if err := s.aggregateLocked(time.Now()); err != nil {
			return err
		}
		s.lastAggregate = time.Now()
	}
	if s.retention > 0 {
		cutoff := time.Now().Add(-s.retention).UnixMilli()
		if s.aggregation {
			var covered sql.NullInt64
			if err := s.db.QueryRow(`SELECT covered_until_ms FROM history_aggregate_progress WHERE period='hour'`).Scan(&covered); err != nil && err != sql.ErrNoRows {
				return err
			}
			// Never prune raw data beyond the completed aggregation watermark.
			// This lets an existing database backfill progressively without losing
			// the older samples needed to build hourly/daily summaries.
			if !covered.Valid {
				return nil
			}
			if covered.Int64 < cutoff {
				cutoff = covered.Int64
			}
		}
		if _, err := s.db.Exec(`DELETE FROM history_samples WHERE timestamp_ms < ?`, cutoff); err != nil {
			return err
		}
		if _, err := s.db.Exec(`PRAGMA wal_checkpoint(PASSIVE);`); err != nil {
			return err
		}
	}
	if s.maxBytes <= 0 || s.databaseBytesLocked() <= s.maxBytes {
		return nil
	}
	target := s.maxBytes * 9 / 10
	for attempt := 0; attempt < 100 && s.databaseBytesLocked() > target; attempt++ {
		result, err := s.db.Exec(`DELETE FROM history_samples WHERE id IN (SELECT id FROM history_samples ORDER BY timestamp_ms, id LIMIT 5000)`)
		if err != nil {
			return err
		}
		removed, _ := result.RowsAffected()
		if _, err = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
			return err
		}
		if _, err = s.db.Exec(`PRAGMA incremental_vacuum(5000);`); err != nil {
			return err
		}
		if removed == 0 {
			break
		}
	}
	// Incremental auto-vacuum is not guaranteed to release pages when indexes
	// leave non-tail free pages. A full VACUUM is expensive, so it is only used
	// as the final fallback after batched deletion. Prune is called exclusively
	// by the background writer in production and never by a collector goroutine.
	if s.databaseBytesLocked() > s.maxBytes {
		if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`VACUUM;`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) databaseBytesLocked() int64 {
	if s.dir == "" {
		return 0
	}
	var total int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if info, err := os.Stat(DatabasePath(s.dir) + suffix); err == nil {
			total += info.Size()
		}
	}
	return total
}

func csvValue(valueJSON string) string {
	var value interface{}
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return valueJSON
	}
	if text, ok := value.(string); ok {
		return text
	}
	if number, ok := value.(json.Number); ok {
		return number.String()
	}
	if value == nil {
		return ""
	}
	if boolean, ok := value.(bool); ok {
		return strconv.FormatBool(boolean)
	}
	return valueJSON
}

func exportLegacyFileCSV(writer *csv.Writer, path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 2*1024*1024)
	for scanner.Scan() {
		var reading model.Reading
		if err := json.Unmarshal(scanner.Bytes(), &reading); err != nil {
			continue
		}
		keys := make([]string, 0, len(reading.Metrics))
		for key := range reading.Metrics {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := writer.Write([]string{
				reading.Time.Format(time.RFC3339Nano), reading.DeviceKey, key,
				fmt.Sprint(reading.Metrics[key]), "", "", "", "", "", "legacy", "", "",
			}); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (s *Store) historyFilesLocked() ([]string, error) {
	if s.dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		files = append(files, filepath.Join(s.dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func (s *Store) isAvailable() bool {
	if !s.enabled || s.detached {
		return false
	}
	if s.available != nil && !s.available() {
		// A store that has never opened a connection can retry safely. Once
		// opened, its old file handles must never survive a mount loss.
		s.detached = s.db != nil
		s.lastError = "历史存储已断开、只读或挂载已变化，已暂停访问；请重新连接原分区后重新启用历史存储"
		return false
	}
	return true
}

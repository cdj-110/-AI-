package history

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type AggregateStatus struct {
	Enabled      bool   `json:"enabled"`
	HourlyRows   int64  `json:"hourlyRows"`
	DailyRows    int64  `json:"dailyRows"`
	LatestHourly string `json:"latestHourly,omitempty"`
	LatestDaily  string `json:"latestDaily,omitempty"`
	LastRunAt    string `json:"lastRunAt,omitempty"`
}

func (s *Store) Aggregate(now time.Time) error {
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
	if now.IsZero() {
		now = time.Now()
	}
	if err := s.aggregateLocked(now); err != nil {
		return err
	}
	s.lastAggregate = now
	return nil
}

func (s *Store) AggregateStatus() (AggregateStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := AggregateStatus{Enabled: s.aggregation}
	if !s.isAvailable() {
		return result, ErrDisabled
	}
	if err := s.ensureDatabaseLocked(false); err != nil {
		return result, err
	}
	if s.db == nil {
		return result, nil
	}
	var hourly, daily sql.NullInt64
	if err := s.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN period='hour' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN period='day' THEN 1 ELSE 0 END),0),
		MAX(CASE WHEN period='hour' THEN bucket_start_ms END),
		MAX(CASE WHEN period='day' THEN bucket_start_ms END)
		FROM history_aggregates`).Scan(&result.HourlyRows, &result.DailyRows, &hourly, &daily); err != nil {
		return result, err
	}
	if hourly.Valid {
		result.LatestHourly = time.UnixMilli(hourly.Int64).Format(time.RFC3339Nano)
	}
	if daily.Valid {
		result.LatestDaily = time.UnixMilli(daily.Int64).Format(time.RFC3339Nano)
	}
	if !s.lastAggregate.IsZero() {
		result.LastRunAt = s.lastAggregate.Format(time.RFC3339Nano)
	}
	return result, nil
}

func (s *Store) aggregateLocked(now time.Time) error {
	if !s.aggregation || s.db == nil {
		return nil
	}
	if err := s.aggregatePeriodLocked("hour", time.Hour, 48, now); err != nil {
		return err
	}
	return s.aggregatePeriodLocked("day", 24*time.Hour, 14, now)
}

func (s *Store) aggregatePeriodLocked(period string, width time.Duration, maxBuckets int, now time.Time) error {
	end := now.UTC().Truncate(width)
	var firstSample, covered sql.NullInt64
	if err := s.db.QueryRow(`SELECT MIN(timestamp_ms) FROM history_samples WHERE numeric_value IS NOT NULL`).Scan(&firstSample); err != nil || !firstSample.Valid {
		// Even a string-only database needs a progress marker so retention can
		// safely prune it after the empty aggregation windows are processed.
		if err != nil {
			return err
		}
		if err := s.db.QueryRow(`SELECT MIN(timestamp_ms) FROM history_samples`).Scan(&firstSample); err != nil || !firstSample.Valid {
			return err
		}
	}
	if err := s.db.QueryRow(`SELECT covered_until_ms FROM history_aggregate_progress WHERE period=?`, period).Scan(&covered); err != nil && err != sql.ErrNoRows {
		return err
	}
	start := time.UnixMilli(firstSample.Int64).UTC().Truncate(width)
	if covered.Valid {
		// Rebuild the latest completed bucket so delayed samples are included.
		start = time.UnixMilli(covered.Int64).UTC().Add(-width)
	}
	if !start.Before(end) {
		return nil
	}
	limit := start.Add(time.Duration(maxBuckets) * width)
	if limit.Before(end) {
		end = limit
	}
	for bucket := start; bucket.Before(end); bucket = bucket.Add(width) {
		if err := s.aggregateBucketLocked(period, bucket, bucket.Add(width)); err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`INSERT INTO history_aggregate_progress(period,covered_until_ms,updated_at_ms) VALUES(?,?,?)
		ON CONFLICT(period) DO UPDATE SET covered_until_ms=excluded.covered_until_ms,updated_at_ms=excluded.updated_at_ms`, period, end.UnixMilli(), time.Now().UnixMilli())
	return err
}

func (s *Store) aggregateBucketLocked(period string, start, end time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM history_aggregates WHERE period=? AND bucket_start_ms=?`, period, start.UnixMilli()); err != nil {
		return err
	}
	base := `INSERT INTO history_aggregates(
		period,bucket_start_ms,task_key,device_key,metric,sample_count,minimum,maximum,average,
		first_value,last_value,good_count,bad_count,uncertain_count,updated_at_ms)
		SELECT ?,?,'',device_key,metric,COUNT(*),MIN(numeric_value),MAX(numeric_value),AVG(numeric_value),
		AVG(numeric_value),AVG(numeric_value),
		SUM(CASE WHEN quality='GOOD' THEN 1 ELSE 0 END),
		SUM(CASE WHEN quality='BAD' THEN 1 ELSE 0 END),
		SUM(CASE WHEN quality='UNCERTAIN' THEN 1 ELSE 0 END),?
		FROM history_samples WHERE timestamp_ms>=? AND timestamp_ms<? AND numeric_value IS NOT NULL
		GROUP BY device_key,metric`
	if _, err := s.db.Exec(base, period, start.UnixMilli(), time.Now().UnixMilli(), start.UnixMilli(), end.UnixMilli()); err != nil {
		return err
	}
	byTask := `INSERT INTO history_aggregates(
		period,bucket_start_ms,task_key,device_key,metric,sample_count,minimum,maximum,average,
		first_value,last_value,good_count,bad_count,uncertain_count,updated_at_ms)
		SELECT ?,?,links.task_key,samples.device_key,samples.metric,COUNT(*),MIN(samples.numeric_value),MAX(samples.numeric_value),AVG(samples.numeric_value),
		AVG(samples.numeric_value),AVG(samples.numeric_value),
		SUM(CASE WHEN samples.quality='GOOD' THEN 1 ELSE 0 END),
		SUM(CASE WHEN samples.quality='BAD' THEN 1 ELSE 0 END),
		SUM(CASE WHEN samples.quality='UNCERTAIN' THEN 1 ELSE 0 END),?
		FROM history_samples samples JOIN history_sample_tasks links ON links.sample_id=samples.id
		WHERE samples.timestamp_ms>=? AND samples.timestamp_ms<? AND samples.numeric_value IS NOT NULL
		GROUP BY links.task_key,samples.device_key,samples.metric`
	_, err := s.db.Exec(byTask, period, start.UnixMilli(), time.Now().UnixMilli(), start.UnixMilli(), end.UnixMilli())
	return err
}

func chooseResolution(options QueryOptions, aggregationEnabled bool) string {
	requested := strings.ToLower(strings.TrimSpace(options.Resolution))
	if requested == "raw" || !aggregationEnabled {
		return "raw"
	}
	if requested == "hour" || requested == "day" {
		return requested
	}
	if options.Start.IsZero() || options.End.IsZero() {
		return "raw"
	}
	duration := options.End.Sub(options.Start)
	if duration >= 90*24*time.Hour {
		return "day"
	}
	if duration >= 48*time.Hour {
		return "hour"
	}
	return "raw"
}

func (s *Store) queryAggregateLocked(options QueryOptions, period string) (QueryResult, error) {
	result := QueryResult{Offset: options.Offset, Limit: options.Limit, Items: []Record{}, Resolution: period}
	conditions := []string{"period=?", "task_key=?"}
	args := []interface{}{period, options.TaskKey}
	if options.DeviceKey != "" {
		conditions = append(conditions, "device_key=?")
		args = append(args, options.DeviceKey)
	}
	if options.Metric != "" {
		conditions = append(conditions, "metric=?")
		args = append(args, options.Metric)
	}
	if !options.Start.IsZero() {
		conditions = append(conditions, "bucket_start_ms>=?")
		args = append(args, options.Start.UnixMilli())
	}
	if !options.End.IsZero() {
		conditions = append(conditions, "bucket_start_ms<=?")
		args = append(args, options.End.UnixMilli())
	}
	where := " WHERE " + strings.Join(conditions, " AND ")
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM history_aggregates`+where, args...).Scan(&result.Total); err != nil || result.Total == 0 {
		return result, err
	}
	queryArgs := append(append([]interface{}{}, args...), options.Limit, options.Offset)
	rows, err := s.db.Query(`SELECT bucket_start_ms,device_key,metric,sample_count,minimum,maximum,average,good_count,bad_count,uncertain_count
		FROM history_aggregates`+where+` ORDER BY bucket_start_ms DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp, good, bad, uncertain int64
		var record Record
		var minimum, maximum, average float64
		if err := rows.Scan(&timestamp, &record.DeviceKey, &record.Metric, &record.SampleCount, &minimum, &maximum, &average, &good, &bad, &uncertain); err != nil {
			return result, err
		}
		record.Time = time.UnixMilli(timestamp).Format(time.RFC3339Nano)
		record.Value, record.ValueText = average, fmt.Sprintf("%g", average)
		record.Minimum, record.Maximum, record.Average = &minimum, &maximum, &average
		record.Resolution, record.ActualDataType, record.CollectionMode = period, "float64", "aggregate_"+period
		record.Quality = "GOOD"
		if bad > 0 {
			record.Quality = "BAD"
		} else if uncertain > 0 || good == 0 {
			record.Quality = "UNCERTAIN"
		}
		result.Items = append(result.Items, record)
	}
	return result, rows.Err()
}

package history

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

func TestStorePausesOnLowDiskAndResumesAfterSpaceRecovers(t *testing.T) {
	dir := t.TempDir()
	freeBytes := uint64(3 * 1024 * 1024)
	store := NewGuarded(dir, 16*1024*1024, nil).SetDiskGuard(func() (uint64, uint64, error) {
		return 128 * 1024 * 1024, freeBytes, nil
	}, 4*1024*1024)
	defer store.Close()
	err := store.Append(model.Reading{DeviceKey: "device", Time: time.Now(), Metrics: map[string]interface{}{"value": 1}})
	if !errors.Is(err, ErrLowDisk) {
		t.Fatalf("low disk append error = %v, want ErrLowDisk", err)
	}
	if status := store.Status(); !status.Paused || status.Writable || status.FreeBytes != freeBytes {
		t.Fatalf("low disk status not exposed: %#v", status)
	}
	freeBytes = 8 * 1024 * 1024
	if err := store.Append(model.Reading{DeviceKey: "device", Time: time.Now(), Metrics: map[string]interface{}{"value": 2}}); err != nil {
		t.Fatalf("append did not resume after space recovered: %v", err)
	}
	if status := store.Status(); status.Paused || !status.Writable || status.Samples != 1 {
		t.Fatalf("recovered disk status is wrong: %#v", status)
	}
}

func TestStoreQuarantinesCorruptSQLiteAndCreatesFreshDatabase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(DatabasePath(dir), []byte("this is not sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewGuarded(dir, 4*1024*1024, nil)
	defer store.Close()
	if err := store.Append(model.Reading{DeviceKey: "device", Time: time.Now(), Metrics: map[string]interface{}{"value": 7}}); err != nil {
		t.Fatalf("corrupt database was not recovered: %v", err)
	}
	status := store.Status()
	if status.RecoveryCount != 1 || status.LastRecoveryPath == "" || status.Samples != 1 {
		t.Fatalf("recovery status is incomplete: %#v", status)
	}
	if _, err := os.Stat(status.LastRecoveryPath); err != nil {
		t.Fatalf("quarantined database is missing: %v", err)
	}
}

func TestStoreQueryFiltersPaginatesAndPreservesMetadata(t *testing.T) {
	dir := t.TempDir()
	store := NewGuarded(dir, 4*1024*1024, nil)
	defer store.Close()
	base := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	for index := 0; index < 5; index++ {
		at := base.Add(time.Duration(index) * time.Minute)
		if err := store.AppendBatch([]Sample{{Time: at, DeviceKey: "device", Metric: "temperature", Value: index, ActualDataType: "int32", Quality: model.QualityGood, SourceTimestamp: at.Add(-time.Second), ReceiveTimestamp: at, CollectionMode: "poll"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Append(model.Reading{DeviceKey: "other", Time: base, Metrics: map[string]interface{}{"temperature": 99}}); err != nil {
		t.Fatal(err)
	}
	result, err := store.Query(QueryOptions{DeviceKey: "device", Metric: "temperature", Start: base.Add(time.Minute), End: base.Add(4 * time.Minute), Offset: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Items) != 2 || result.Items[0].ValueText != "3" || result.Items[1].ValueText != "2" {
		t.Fatalf("unexpected query page: %#v", result)
	}
	if result.Items[0].ActualDataType != "int32" || result.Items[0].CollectionMode != "poll" || result.Items[0].SourceTimestamp == "" {
		t.Fatalf("query metadata was lost: %#v", result.Items[0])
	}
}

func TestStoreWritesSQLiteAndExportsCSV(t *testing.T) {
	dir := t.TempDir()
	store := NewGuarded(dir, 4*1024*1024, nil)
	defer store.Close()
	at := time.Date(2026, 9, 2, 10, 11, 12, 345000000, time.UTC)
	if err := store.Append(model.Reading{DeviceKey: "device-1", Time: at, Metrics: map[string]interface{}{"temperature": 23.5, "state": "运行"}}); err != nil {
		t.Fatal(err)
	}
	status := store.Status()
	if status.Samples != 2 || status.Files != 1 {
		t.Fatalf("unexpected SQLite status: %#v", status)
	}
	if _, err := os.Stat(DatabasePath(dir)); err != nil {
		t.Fatalf("SQLite database was not created: %v", err)
	}
	var output bytes.Buffer
	if err := store.ExportCSV(&output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "device-1,temperature,23.5") || !strings.Contains(text, "device-1,state,运行") {
		t.Fatalf("unexpected CSV export: %s", text)
	}
}

func TestStoreExportsLegacyJSONLAlongsideSQLite(t *testing.T) {
	dir := t.TempDir()
	legacy := `{"deviceKey":"legacy-device","time":"2026-09-01T00:00:00Z","metrics":{"old":7}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "history-20260901.jsonl"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewGuarded(dir, 4*1024*1024, nil)
	defer store.Close()
	if err := store.Append(model.Reading{DeviceKey: "new-device", Time: time.Now(), Metrics: map[string]interface{}{"new": 8}}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := store.ExportCSV(&output); err != nil {
		t.Fatal(err)
	}
	if text := output.String(); !strings.Contains(text, "legacy-device,old,7") || !strings.Contains(text, "new-device,new,8") {
		t.Fatalf("combined export omitted data: %s", text)
	}
}

func TestStorePrunesOldestSamplesAtCapacity(t *testing.T) {
	dir := t.TempDir()
	const maxBytes = 160 * 1024
	store := NewGuarded(dir, maxBytes, nil)
	defer store.Close()
	payload := strings.Repeat("x", 2048)
	for index := 0; index < 200; index++ {
		if err := store.Append(model.Reading{DeviceKey: "device", Time: time.Unix(int64(index), 0), Metrics: map[string]interface{}{"value": payload + string(rune(index))}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Prune(); err != nil {
		t.Fatal(err)
	}
	status := store.Status()
	if status.Samples >= 200 {
		t.Fatalf("capacity did not prune old samples: %#v", status)
	}
	if status.Bytes > maxBytes {
		t.Fatalf("history exceeded capacity: bytes=%d max=%d", status.Bytes, maxBytes)
	}
}

func TestStorePreservesMetadataSpecialFloatsAndWideIntegers(t *testing.T) {
	dir := t.TempDir()
	store := NewGuarded(dir, 4*1024*1024, nil)
	defer store.Close()
	source := time.Date(2026, 9, 2, 10, 11, 12, 123000000, time.UTC)
	received := source.Add(45 * time.Millisecond)
	samples := []Sample{
		{Time: received, DeviceKey: "modbus", Metric: "counter", Value: uint64(math.MaxUint64), ActualDataType: "uint64", Quality: model.QualityGood, SourceTimestamp: source, ReceiveTimestamp: received, CollectionMode: "poll"},
		{Time: received, DeviceKey: "opcua", Metric: "nan", Value: math.NaN(), ActualDataType: "float64", Quality: model.QualityUncertain, RawQuality: 0x44, SourceTimestamp: source, ReceiveTimestamp: received, CollectionMode: "subscription"},
		{Time: received, DeviceKey: "opcda", Metric: "array", Value: []float64{1, math.Inf(1)}, ActualDataType: "float64[]", Quality: model.QualityGood, SourceTimestamp: source, ReceiveTimestamp: received, CollectionMode: "async_subscription"},
	}
	if err := store.AppendBatch(samples); err != nil {
		t.Fatal(err)
	}
	var integerValue, valueJSON, actualType, quality, mode string
	var numericValue interface{}
	var sourceMillis, receiveMillis int64
	if err := store.db.QueryRow(`SELECT integer_value, numeric_value, value_json, actual_data_type, quality, source_timestamp_ms, receive_timestamp_ms, collection_mode FROM history_samples WHERE device_key='modbus'`).Scan(
		&integerValue, &numericValue, &valueJSON, &actualType, &quality, &sourceMillis, &receiveMillis, &mode,
	); err != nil {
		t.Fatal(err)
	}
	if integerValue != "18446744073709551615" || numericValue != nil || valueJSON != "18446744073709551615" {
		t.Fatalf("uint64 lost precision: integer=%q numeric=%v json=%q", integerValue, numericValue, valueJSON)
	}
	if actualType != "uint64" || quality != "GOOD" || sourceMillis != source.UnixMilli() || receiveMillis != received.UnixMilli() || mode != "poll" {
		t.Fatalf("metadata mismatch: type=%s quality=%s source=%d receive=%d mode=%s", actualType, quality, sourceMillis, receiveMillis, mode)
	}
	var special string
	if err := store.db.QueryRow(`SELECT special_value, value_json FROM history_samples WHERE device_key='opcua'`).Scan(&special, &valueJSON); err != nil {
		t.Fatal(err)
	}
	if special != "NaN" || !strings.Contains(valueJSON, "NaN") {
		t.Fatalf("NaN was not preserved: special=%q json=%q", special, valueJSON)
	}
	if err := store.db.QueryRow(`SELECT value_json FROM history_samples WHERE device_key='opcda'`).Scan(&valueJSON); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(valueJSON, `"$float":"+Inf"`) {
		t.Fatalf("array infinity was not preserved: %q", valueJSON)
	}
	var exported bytes.Buffer
	if err := store.ExportCSV(&exported); err != nil {
		t.Fatal(err)
	}
	if text := exported.String(); !strings.Contains(text, "actualDataType,quality,rawQuality,sourceTime,receiveTime,collectionMode") || !strings.Contains(text, "uint64,GOOD,0,") {
		t.Fatalf("metadata missing from CSV export: %s", text)
	}
}

func TestAsyncWriterPreservesChangesWithoutBlockingProducer(t *testing.T) {
	store := NewGuarded(t.TempDir(), 4*1024*1024, nil)
	writer := NewAsyncWriter(store, func(err error) { t.Errorf("async writer: %v", err) })
	started := time.Now()
	for index := 0; index < 1000; index++ {
		writer.Enqueue([]Sample{{Time: time.Now(), DeviceKey: "device", Metric: "value", Value: index}})
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("producer was blocked by history I/O: %s", elapsed)
	}
	if status := writer.Status(); status.Pending > 1000 || status.Dropped != 0 {
		t.Fatalf("sample queue lost data: %#v", status)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	checker := NewGuarded(store.dir, 4*1024*1024, nil)
	defer checker.Close()
	if status := checker.Status(); status.Samples != 1000 {
		t.Fatalf("persisted %d samples, want 1000", status.Samples)
	}
}

func TestAsyncWriterRetriesLatestValuesAfterStorageReturns(t *testing.T) {
	var available atomic.Bool
	store := NewGuarded(t.TempDir(), 4*1024*1024, available.Load)
	writer := NewAsyncWriter(store, func(err error) { t.Errorf("async writer: %v", err) })
	writer.Enqueue([]Sample{{Time: time.Now(), DeviceKey: "device", Metric: "value", Value: 7}})
	time.Sleep(defaultFlushInterval + 100*time.Millisecond)
	if status := writer.Status(); status.Pending != 1 {
		t.Fatalf("unavailable storage did not retain latest sample: %#v", status)
	}
	available.Store(true)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	checker := NewGuarded(store.dir, 4*1024*1024, nil)
	defer checker.Close()
	if status := checker.Status(); status.Samples != 1 {
		t.Fatalf("retained sample was not written after storage returned: %#v", status)
	}
}

func TestStoreMigratesExistingSQLiteSchema(t *testing.T) {
	dir := t.TempDir()
	database, err := sql.Open("sqlite3", DatabasePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`CREATE TABLE history_samples (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp_ms INTEGER NOT NULL,
		device_key TEXT NOT NULL,
		metric TEXT NOT NULL,
		value_type TEXT NOT NULL,
		numeric_value REAL,
		value_json TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	store := NewGuarded(dir, 4*1024*1024, nil)
	defer store.Close()
	if err := store.AppendBatch([]Sample{{Time: time.Now(), DeviceKey: "device", Metric: "value", Value: 1, Quality: model.QualityGood}}); err != nil {
		t.Fatalf("append after migration failed: %v", err)
	}
	var quality string
	if err := store.db.QueryRow(`SELECT quality FROM history_samples LIMIT 1`).Scan(&quality); err != nil || quality != "GOOD" {
		t.Fatalf("new metadata column unavailable after migration: quality=%q err=%v", quality, err)
	}
}

func TestStoreClearKeepsOpenDatabaseUsable(t *testing.T) {
	dir := t.TempDir()
	running := NewGuarded(dir, 4*1024*1024, nil)
	defer running.Close()
	if err := running.Append(model.Reading{DeviceKey: "device", Time: time.Now(), Metrics: map[string]interface{}{"before": 1}}); err != nil {
		t.Fatal(err)
	}
	clearer := NewGuarded(dir, 4*1024*1024, nil)
	if err := clearer.Clear(); err != nil {
		t.Fatal(err)
	}
	_ = clearer.Close()
	if status := running.Status(); status.Samples != 0 {
		t.Fatalf("clear left samples behind: %#v", status)
	}
	if err := running.Append(model.Reading{DeviceKey: "device", Time: time.Now(), Metrics: map[string]interface{}{"after": 2}}); err != nil {
		t.Fatalf("running connection was unusable after clear: %v", err)
	}
	if status := running.Status(); status.Samples != 1 {
		t.Fatalf("new sample was not persisted after clear: %#v", status)
	}
}

func TestStoreRetentionPrunesExpiredSamples(t *testing.T) {
	dir := t.TempDir()
	store := NewGuarded(dir, 4*1024*1024, nil).SetRetentionDays(7)
	defer store.Close()
	now := time.Now()
	if err := store.AppendBatch([]Sample{
		{Time: now.Add(-8 * 24 * time.Hour), DeviceKey: "device", Metric: "expired", Value: 1},
		{Time: now.Add(-6 * 24 * time.Hour), DeviceKey: "device", Metric: "retained", Value: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Prune(); err != nil {
		t.Fatal(err)
	}
	result, err := store.Query(QueryOptions{DeviceKey: "device", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].Metric != "retained" {
		t.Fatalf("retention did not remove only expired samples: %#v", result)
	}
}

func TestStoreTracksTaskStatsAndFiltersExports(t *testing.T) {
	dir := t.TempDir()
	store := NewGuarded(dir, 4*1024*1024, nil)
	defer store.Close()
	at := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	if err := store.AppendBatch([]Sample{
		{Time: at, DeviceKey: "device", Metric: "one", Value: 1, TaskKeys: []string{"task-a", "task-b"}},
		{Time: at.Add(time.Second), DeviceKey: "device", Metric: "two", Value: 2, TaskKeys: []string{"task-b"}},
	}); err != nil {
		t.Fatal(err)
	}
	stats, err := store.TaskStats()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, stat := range stats {
		counts[stat.TaskKey] = stat.Samples
	}
	if counts["task-a"] != 1 || counts["task-b"] != 2 {
		t.Fatalf("unexpected task statistics: %#v", stats)
	}
	result, err := store.Query(QueryOptions{TaskKey: "task-a", Limit: 10})
	if err != nil || result.Total != 1 || result.Items[0].Metric != "one" {
		t.Fatalf("task query mismatch: result=%#v err=%v", result, err)
	}
	var output bytes.Buffer
	if err := store.ExportFilteredCSV(&output, QueryOptions{TaskKey: "task-a"}); err != nil {
		t.Fatal(err)
	}
	if text := output.String(); !strings.Contains(text, ",one,") || strings.Contains(text, ",two,") {
		t.Fatalf("task export was not filtered: %s", text)
	}
}

func TestStoreBackupToCreatesVerifiedSQLiteCopy(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	store := NewGuarded(sourceDir, 4*1024*1024, nil)
	if err := store.AppendBatch([]Sample{{Time: time.Now(), DeviceKey: "device", Metric: "value", Value: 9, TaskKeys: []string{"task"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BackupTo(targetDir); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	copyStore := NewGuarded(targetDir, 4*1024*1024, nil)
	defer copyStore.Close()
	result, err := copyStore.Query(QueryOptions{TaskKey: "task", Limit: 10})
	if err != nil || result.Total != 1 || result.Items[0].ValueText != "9" {
		t.Fatalf("migrated SQLite is incomplete: result=%#v err=%v", result, err)
	}
}

func TestEncryptedBackupCanBeListedVerifiedAndRestored(t *testing.T) {
	source := NewGuarded(t.TempDir(), 4*1024*1024, nil)
	defer source.Close()
	at := time.Now().Add(-time.Hour)
	if err := source.AppendBatch([]Sample{{Time: at, DeviceKey: "device", Metric: "value", Value: 42}}); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	backupDir := t.TempDir()
	created, err := source.CreateBackup(backupDir, 3, key)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Encrypted || created.SHA256 == "" {
		t.Fatalf("encrypted backup metadata is incomplete: %#v", created)
	}
	items, err := ListBackups(backupDir)
	if err != nil || len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("backup listing mismatch: items=%#v err=%v", items, err)
	}
	target := NewGuarded(t.TempDir(), 4*1024*1024, nil)
	defer target.Close()
	if _, err := target.RestoreBackup(backupDir, created.ID, key); err != nil {
		t.Fatal(err)
	}
	result, err := target.Query(QueryOptions{DeviceKey: "device", Metric: "value", Limit: 10})
	if err != nil || result.Total != 1 || result.Items[0].ValueText != "42" {
		t.Fatalf("restored backup is incomplete: result=%#v err=%v", result, err)
	}
}

func TestHourlyAndDailyAggregationSupportsAdaptiveQuery(t *testing.T) {
	store := NewGuarded(t.TempDir(), 8*1024*1024, nil).SetAggregationEnabled(true)
	defer store.Close()
	now := time.Now().UTC().Truncate(time.Hour)
	at := now.Add(-72 * time.Hour).Add(10 * time.Minute)
	if err := store.AppendBatch([]Sample{
		{Time: at, DeviceKey: "device", Metric: "temperature", Value: 10.0, Quality: model.QualityGood, TaskKeys: []string{"task"}},
		{Time: at.Add(10 * time.Minute), DeviceKey: "device", Metric: "temperature", Value: 20.0, Quality: model.QualityGood, TaskKeys: []string{"task"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Aggregate(now); err != nil {
		t.Fatal(err)
	}
	result, err := store.Query(QueryOptions{TaskKey: "task", DeviceKey: "device", Metric: "temperature", Start: now.Add(-96 * time.Hour), End: now, Limit: 100, Resolution: "hour"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Resolution != "hour" || result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("hour aggregate query mismatch: %#v", result)
	}
	item := result.Items[0]
	if item.Average == nil || *item.Average != 15 || item.Minimum == nil || *item.Minimum != 10 || item.Maximum == nil || *item.Maximum != 20 || item.SampleCount != 2 {
		t.Fatalf("aggregate values are wrong: %#v", item)
	}
	status, err := store.AggregateStatus()
	if err != nil || status.HourlyRows == 0 || status.DailyRows == 0 {
		t.Fatalf("aggregate status is incomplete: %#v err=%v", status, err)
	}
}

func TestRestoreRollbackRecoversPreRestoreDatabase(t *testing.T) {
	backupSource := NewGuarded(t.TempDir(), 4*1024*1024, nil)
	if err := backupSource.Append(model.Reading{DeviceKey: "device", Time: time.Now(), Metrics: map[string]interface{}{"value": 42}}); err != nil {
		t.Fatal(err)
	}
	backupDir := t.TempDir()
	item, err := backupSource.CreateBackup(backupDir, 3, nil)
	_ = backupSource.Close()
	if err != nil {
		t.Fatal(err)
	}
	target := NewGuarded(t.TempDir(), 4*1024*1024, nil)
	defer target.Close()
	if err := target.Append(model.Reading{DeviceKey: "device", Time: time.Now(), Metrics: map[string]interface{}{"value": 99}}); err != nil {
		t.Fatal(err)
	}
	rollback, err := target.RestoreBackup(backupDir, item.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.RollbackRestore(rollback); err != nil {
		t.Fatal(err)
	}
	result, err := target.Query(QueryOptions{DeviceKey: "device", Metric: "value", Limit: 10})
	if err != nil || result.Total != 1 || result.Items[0].ValueText != "99" {
		t.Fatalf("restore rollback did not recover previous database: result=%#v err=%v", result, err)
	}
}

package config

import (
	"testing"
	"time"
)

func TestHistoryStorageSelectionDefaultsToAll(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway-test","historyStorage":{"enabled":false,"maxSizeMB":256}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistoryStorage.SelectionMode != "all" {
		t.Fatalf("selection mode = %q, want all", cfg.HistoryStorage.SelectionMode)
	}
	if cfg.HistoryStorage.SaveMode != "interval" || cfg.HistoryStorage.SaveIntervalSeconds != 1 {
		t.Fatalf("unexpected default save policy: %#v", cfg.HistoryStorage)
	}
}

func TestHistoryStorageDefaultsToLocalEG100Storage(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway-test","historyStorage":{"enabled":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistoryStorage.StoragePath != DefaultHistoryStoragePath || !cfg.HistoryStorage.UsesLocalStorage() {
		t.Fatalf("history storage path = %q, want local %q", cfg.HistoryStorage.StoragePath, DefaultHistoryStoragePath)
	}
	if cfg.HistoryStorage.MaxSizeMB != 16 {
		t.Fatalf("local history default capacity = %d MB, want 16 MB", cfg.HistoryStorage.MaxSizeMB)
	}
}

func TestHistoryStorageMigratesLegacyEmptyPathAndDefaultCapacity(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway-test","historyStorage":{"enabled":false,"maxSizeMB":256}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistoryStorage.StoragePath != DefaultHistoryStoragePath || cfg.HistoryStorage.MaxSizeMB != 16 {
		t.Fatalf("legacy empty history storage was not migrated safely: %#v", cfg.HistoryStorage)
	}
}

func TestHistoryStoragePreservesExplicitRemovableStorage(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway-test","historyStorage":{"enabled":false,"maxSizeMB":256,"storagePath":"/media/udisk1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistoryStorage.StoragePath != "/media/udisk1" || cfg.HistoryStorage.UsesLocalStorage() {
		t.Fatalf("explicit removable storage was changed: %#v", cfg.HistoryStorage)
	}
}

func TestHistoryStorageMigratesLegacyPolicyToLongRunningTask(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway-test","historyStorage":{"enabled":true,"maxSizeMB":256,"selectionMode":"selected","points":[{"deviceKey":"device-1","metric":"temperature"}],"saveMode":"change","changeDeadband":0.5}}`))
	if err != nil {
		t.Fatal(err)
	}
	tasks := cfg.HistoryStorage.EffectiveTasks()
	if len(tasks) != 1 || tasks[0].TaskKey != "legacy-default" || tasks[0].Name != "长期历史任务" || tasks[0].SaveMode != "change" || len(tasks[0].Points) != 1 {
		t.Fatalf("legacy history task was not preserved: %#v", tasks)
	}
}

func TestHistoryTaskStatusUsesStartAndEndTime(t *testing.T) {
	now := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	start, end := now.Add(time.Hour), now.Add(2*time.Hour)
	task := HistoryStorageTask{StartAt: &start, EndAt: &end}
	if got := task.StatusAt(now); got != "pending" {
		t.Fatalf("status before start = %q", got)
	}
	if got := task.StatusAt(start); got != "running" {
		t.Fatalf("status at start = %q", got)
	}
	if got := task.StatusAt(end); got != "completed" {
		t.Fatalf("status at end = %q", got)
	}
}

func TestHistoryStorageSelectionModeIsNormalized(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway-test","historyStorage":{"enabled":false,"maxSizeMB":256,"selectionMode":" SELECTED ","points":[{"deviceKey":"device-1","metric":"temperature"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistoryStorage.SelectionMode != "selected" {
		t.Fatalf("selection mode = %q, want selected", cfg.HistoryStorage.SelectionMode)
	}
}

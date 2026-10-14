package alarm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteHistoryMigratesJSONLAndQueriesPages(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "alarm-events.db")
	legacyPath := filepath.Join(directory, "alarm-events.jsonl")
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	legacy := []Event{
		historyEvent("event-1", "ACTIVE", "WARNING", "device-1", "temperature", base),
		historyEvent("event-1", "RESOLVED", "WARNING", "device-1", "temperature", base.Add(time.Minute)),
		historyEvent("event-2", "ACTIVE", "CRITICAL", "device-2", "pressure", base.Add(2*time.Minute)),
	}
	file, err := os.Create(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range legacy {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, recent, err := openSQLiteHistory(databasePath, legacyPath, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if len(recent) != 3 || recent[0].EventID != "event-2" {
		t.Fatalf("unexpected migrated history: %#v", recent)
	}
	if _, err := os.Stat(legacyPath + ".migrated"); err != nil {
		t.Fatalf("JSONL migration backup missing: %v", err)
	}
	page, err := store.Query(HistoryQuery{Page: 1, PageSize: 1, Level: "CRITICAL"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.TotalPages != 1 || len(page.Items) != 1 || page.Items[0].Metric != "pressure" {
		t.Fatalf("unexpected filtered page: %#v", page)
	}
	page, err = store.Query(HistoryQuery{Page: 1, PageSize: 2, Keyword: "temperature"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected keyword page: %#v", page)
	}
}

func TestSQLiteHistoryRetentionLimit(t *testing.T) {
	store, _, err := openSQLiteHistory(filepath.Join(t.TempDir(), "alarm-events.db"), "", 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	base := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	for index := 0; index < 5; index++ {
		event := historyEvent(string(rune('a'+index)), "ACTIVE", "INFO", "device", "metric", base.Add(time.Duration(index)*time.Second))
		if err := store.Append(event, 3); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.Query(HistoryQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 3 || page.Items[0].EventID != "e" || page.Items[2].EventID != "c" {
		t.Fatalf("retention limit was not enforced: %#v", page)
	}
}

func historyEvent(id, state, level, deviceKey, metric string, updatedAt time.Time) Event {
	return Event{EventID: id, RuleID: "rule-" + id, RuleName: metric + " alarm", DeviceKey: deviceKey, Metric: metric, Level: level, State: state, Value: 42, ValueText: "42", Quality: "GOOD", Message: metric, OccurredAt: updatedAt, UpdatedAt: updatedAt}
}

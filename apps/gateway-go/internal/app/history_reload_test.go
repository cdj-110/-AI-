package app

import (
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestOrdinaryConfigEditKeepsHistoryWriter(t *testing.T) {
	cfg, err := config.Parse([]byte(`{"gatewayKey":"test","historyStorage":{"enabled":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = a.historyWriter.Close()
		if a.collectionIssues != nil {
			_ = a.collectionIssues.Close()
		}
	}()
	writer, store := a.historyWriter, a.history
	a.historyLastSaved["test"] = historySavedSample{At: time.Now()}
	cfg.CollectIntervalMilliseconds = 2000
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if a.historyWriter != writer || a.history != store || len(a.historyLastSaved) != 1 {
		t.Fatal("ordinary save reset history")
	}
	cfg.HistoryStorage.MaxSizeMB++
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if a.historyWriter == writer || a.history == store || len(a.historyLastSaved) != 0 {
		t.Fatal("storage config did not replace history")
	}
}

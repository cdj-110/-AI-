package history

import (
	"errors"
	"os"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

func TestRemovableUsesRollbackJournalAndNoMmap(t *testing.T) {
	s := NewGuarded(t.TempDir(), 4<<20, func() bool { return true })
	defer s.Close()
	if err := s.Append(model.Reading{Time: time.Now(), Metrics: map[string]interface{}{"x": 1}}); err != nil {
		t.Fatal(err)
	}
	var mode string
	var mmap int64
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("mode=%s err=%v", mode, err)
	}
	if err := s.db.QueryRow("PRAGMA mmap_size").Scan(&mmap); err != nil || mmap != 0 {
		t.Fatalf("mmap=%d err=%v", mmap, err)
	}
	if _, err := os.Stat(DatabasePath(s.dir) + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("shared-memory file exists: %v", err)
	}
}

func TestLostMountBlocksStatusClearAndReuse(t *testing.T) {
	available := true
	s := NewGuarded(t.TempDir(), 4<<20, func() bool { return available })
	defer s.Close()
	reading := model.Reading{Time: time.Now(), Metrics: map[string]interface{}{"x": 1}}
	if err := s.Append(reading); err != nil {
		t.Fatal(err)
	}
	available = false
	status := s.Status()
	if !status.Paused || status.Writable || status.PauseReason == "" {
		t.Fatalf("bad status: %+v", status)
	}
	if err := s.Clear(); !errors.Is(err, ErrDisabled) {
		t.Fatalf("clear on lost mount: %v", err)
	}
	available = true
	if err := s.Append(reading); !errors.Is(err, ErrDisabled) {
		t.Fatalf("stale connection reused: %v", err)
	}
}

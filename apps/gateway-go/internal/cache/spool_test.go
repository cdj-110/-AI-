package cache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

func TestDrainLimitKeepsUnsentReadings(t *testing.T) {
	spool := New(filepath.Join(t.TempDir(), "spool.jsonl"))
	for index := 0; index < 4; index++ {
		if err := spool.Append(model.Reading{DeviceKey: "device", Time: time.Unix(int64(index), 0), Metrics: map[string]interface{}{"value": index}}); err != nil {
			t.Fatal(err)
		}
	}

	published := 0
	if err := spool.DrainLimit(func(model.Reading) error {
		published++
		return nil
	}, 2); err != nil {
		t.Fatal(err)
	}
	if published != 2 {
		t.Fatalf("published = %d, want 2", published)
	}

	remaining := 0
	if err := spool.Drain(func(model.Reading) error {
		remaining++
		return errors.New("keep")
	}); err != nil {
		t.Fatal(err)
	}
	if remaining != 2 {
		t.Fatalf("remaining = %d, want 2", remaining)
	}
}

func TestLimitedSpoolNeverExceedsConfiguredSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spool.jsonl")
	spool := NewLimited(path, 1024)
	for index := 0; index < 100; index++ {
		if err := spool.Append(model.Reading{
			DeviceKey: "device",
			Time:      time.Unix(int64(index), 0),
			Metrics:   map[string]interface{}{"value": index, "padding": "abcdefghijklmnopqrstuvwxyz"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 1024 {
		t.Fatalf("spool size = %d, want <= 1024", info.Size())
	}
}

func TestDisabledSpoolDoesNotWrite(t *testing.T) {
	spool := Disabled()
	err := spool.Append(model.Reading{DeviceKey: "device", Time: time.Now()})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("Append error = %v, want ErrDisabled", err)
	}
}

func TestGuardedSpoolDoesNotWriteAfterStorageDisappears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-mount", "spool.jsonl")
	spool := NewGuarded(path, 1024, func() bool { return false })
	err := spool.Append(model.Reading{DeviceKey: "device", Time: time.Now()})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("Append error = %v, want ErrDisabled", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("guarded spool created a file after storage disappeared")
	}
}

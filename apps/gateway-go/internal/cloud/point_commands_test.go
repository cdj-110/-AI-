package cloud

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testPointCommand() PointCommand {
	now := time.Now()
	return PointCommand{CommandID: "19a46a95-d770-4547-b469-b724b7018075", DeviceKey: "child-001", Metric: "setpoint", Value: 45.15, IssuedAt: now, ExpiresAt: now.Add(30 * time.Second)}
}

func TestPointCommandSuccessAndRestartDeduplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.json")
	command := testPointCommand()
	count := 0
	write := func() error { count++; return nil }
	first := NewPointCommandExecutor(path).Execute(command, write)
	second := NewPointCommandExecutor(path).Execute(command, write)
	if first.Status != "SUCCEEDED" || second.Status != "SUCCEEDED" || count != 1 {
		t.Fatalf("first=%+v second=%+v count=%d", first, second, count)
	}
	command.DeviceKey = "other-child"
	if result := NewPointCommandExecutor(path).Execute(command, write); result.Status != "FAILED" || count != 1 {
		t.Fatal(result, count)
	}
}

func TestPointCommandFailureIsNotRetried(t *testing.T) {
	executor := NewPointCommandExecutor(filepath.Join(t.TempDir(), "commands.json"))
	count := 0
	write := func() error { count++; return fmt.Errorf("device rejected write") }
	command := testPointCommand()
	for index := 0; index < 2; index++ {
		result := executor.Execute(command, write)
		if result.Status != "FAILED" || result.Message != "device rejected write" {
			t.Fatal(result)
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
}

func TestPointCommandExpiredInvalidAndUnavailableJournal(t *testing.T) {
	for _, kind := range []string{"expired", "future", "long", "missing", "invalid", "null-journal", "corrupt-journal", "unavailable"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "commands.json")
			command := testPointCommand()
			switch kind {
			case "expired":
				command.ExpiresAt = time.Now().Add(-time.Second)
			case "future":
				command.IssuedAt = time.Now().Add(10 * time.Second)
				command.ExpiresAt = command.IssuedAt.Add(20 * time.Second)
			case "long":
				command.ExpiresAt = command.IssuedAt.Add(time.Minute)
			case "missing":
				command.DeviceKey = ""
			case "invalid":
				command.CommandID = "invalid"
			case "null-journal":
				if err := os.WriteFile(path, []byte("null"), 0600); err != nil {
					t.Fatal(err)
				}
			case "corrupt-journal":
				if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unavailable":
				path = filepath.Join(path, "missing", "commands.json")
			}
			result := NewPointCommandExecutor(path).Execute(command, func() error { t.Fatal("must not touch physical equipment"); return nil })
			if result.Status != "FAILED" {
				t.Fatal(result)
			}
		})
	}
}

func TestPointCommandCrashReservationDoesNotWriteAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.json")
	executor := NewPointCommandExecutor(path)
	command := testPointCommand()
	result := PointCommandResult{CommandID: command.CommandID, DeviceKey: command.DeviceKey, Metric: command.Metric, Status: "UNKNOWN"}
	if err := executor.save(map[string]commandEntry{command.CommandID: {Result: result, ExpiresAt: command.ExpiresAt}}); err != nil {
		t.Fatal(err)
	}
	actual := NewPointCommandExecutor(path).Execute(command, func() error { t.Fatal("crash-reserved command cannot be replayed"); return nil })
	if actual.Status != "UNKNOWN" {
		t.Fatal(actual)
	}
}

func TestConcurrentDuplicateCommandsExecuteOnce(t *testing.T) {
	executor := NewPointCommandExecutor(filepath.Join(t.TempDir(), "commands.json"))
	command := testPointCommand()
	var count atomic.Int32
	var group sync.WaitGroup
	for index := 0; index < 12; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result := executor.Execute(command, func() error { count.Add(1); return nil })
			if result.Status != "SUCCEEDED" {
				t.Error(result)
			}
		}()
	}
	group.Wait()
	if count.Load() != 1 {
		t.Fatal(count.Load())
	}
}

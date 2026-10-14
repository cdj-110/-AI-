package cloud

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func testMaintenanceCommand() MaintenanceCommand {
	now := time.Now()
	return MaintenanceCommand{
		TaskID: "11111111-1111-4111-8111-111111111111", HardwareID: "ABC123", Action: MaintenanceServiceRestart,
		IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}
}

func TestMaintenanceExecutorIsDurablyIdempotent(t *testing.T) {
	executor := NewMaintenanceCommandExecutor(filepath.Join(t.TempDir(), "maintenance.json"))
	command := testMaintenanceCommand()
	var calls atomic.Int32
	schedule := func(action string) error {
		calls.Add(1)
		if action != MaintenanceServiceRestart {
			t.Fatal(action)
		}
		return nil
	}
	first := executor.Execute(command, schedule)
	second := executor.Execute(command, schedule)
	if first.Status != "ACCEPTED" || second != first || calls.Load() != 1 {
		t.Fatalf("first=%#v second=%#v calls=%d", first, second, calls.Load())
	}
	reloaded := NewMaintenanceCommandExecutor(executor.path).Execute(command, schedule)
	if reloaded != first || calls.Load() != 1 {
		t.Fatalf("persistent replay executed again: %#v calls=%d", reloaded, calls.Load())
	}
}

func TestMaintenanceExecutorRejectsExpiredAndInvalidCommands(t *testing.T) {
	executor := NewMaintenanceCommandExecutor(filepath.Join(t.TempDir(), "maintenance.json"))
	var calls atomic.Int32
	schedule := func(string) error { calls.Add(1); return nil }
	command := testMaintenanceCommand()
	command.ExpiresAt = time.Now().Add(-time.Second)
	if result := executor.Execute(command, schedule); result.Status != "FAILED" {
		t.Fatal(result)
	}
	command = testMaintenanceCommand()
	command.Action = "SHELL"
	if result := executor.Execute(command, schedule); result.Status != "FAILED" {
		t.Fatal(result)
	}
	if calls.Load() != 0 {
		t.Fatal("rejected maintenance command scheduled an action")
	}
}

func TestMaintenanceExecutorPersistsFailedSchedulingAndDoesNotRetry(t *testing.T) {
	executor := NewMaintenanceCommandExecutor(filepath.Join(t.TempDir(), "maintenance.json"))
	command := testMaintenanceCommand()
	var calls atomic.Int32
	failure := executor.Execute(command, func(string) error {
		calls.Add(1)
		return errors.New("restart unavailable")
	})
	if failure.Status != "FAILED" || failure.Message != "restart unavailable" {
		t.Fatal(failure)
	}
	replayed := NewMaintenanceCommandExecutor(executor.path).Execute(command, func(string) error {
		calls.Add(1)
		return nil
	})
	if replayed != failure || calls.Load() != 1 {
		t.Fatalf("failed task was scheduled again: %#v calls=%d", replayed, calls.Load())
	}
}

func TestMaintenanceMQTTRejectsRetainedAndPersistsReplyUntilPublished(t *testing.T) {
	transport := &commandTestMQTT{published: make(chan commandTestPublish, 1)}
	resultPath := filepath.Join(t.TempDir(), "maintenance-result.json")
	client := &Client{hardwareID: "ABC123", mqtt: transport}
	var calls atomic.Int32
	if err := client.SubscribeMaintenance(func(command MaintenanceCommand) MaintenanceResult {
		calls.Add(1)
		return MaintenanceResult{TaskID: command.TaskID, HardwareID: command.HardwareID, Action: command.Action, Status: "ACCEPTED"}
	}, resultPath); err != nil {
		t.Fatal(err)
	}
	if transport.subscribed != "weikong/factory/ABC123/maintenance/set" {
		t.Fatal(transport.subscribed)
	}
	raw, _ := json.Marshal(testMaintenanceCommand())
	transport.handler(transport, commandTestMessage{retained: true, raw: raw})
	transport.handler(transport, commandTestMessage{raw: []byte("broken")})
	if calls.Load() != 0 {
		t.Fatal("unsafe messages executed maintenance")
	}
	transport.handler(transport, commandTestMessage{raw: raw})
	select {
	case published := <-transport.published:
		if published.topic != "weikong/factory/ABC123/maintenance/reply" || published.qos != 1 || published.retained {
			t.Fatal(published)
		}
		var result MaintenanceResult
		if err := json.Unmarshal(published.payload.([]byte), &result); err != nil || result.Status != "ACCEPTED" {
			t.Fatal(err, result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing maintenance receipt")
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
}

package datamanager

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestManagerBroadcastsOnlyChangedPoint(t *testing.T) {
	store := state.New(config.Config{Points: []config.PointConfig{
		{DeviceKey: "d1", Metric: "p1"},
		{DeviceKey: "d1", Metric: "p2"},
	}})
	manager := New(store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)
	updates, unsubscribe := manager.Subscribe(16)
	defer unsubscribe()

	store.SetPointValue("d1", "p2", 42)
	message := waitForMessage(t, updates, "points.diff")
	var diff PointDiff
	if err := json.Unmarshal(message.Payload, &diff); err != nil {
		t.Fatal(err)
	}
	if diff.Replace || len(diff.Points) != 1 || diff.Points[0].Metric != "p2" {
		t.Fatalf("unexpected point diff: %#v", diff)
	}
}

func TestManagerPublishesDeviceAndLogDiffs(t *testing.T) {
	store := state.New(config.Config{Points: []config.PointConfig{{DeviceKey: "d1", Metric: "p1"}}})
	manager := New(store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)
	updates, unsubscribe := manager.Subscribe(16)
	defer unsubscribe()

	store.SetPointValue("d1", "p1", 1)
	deviceMessage := waitForMessage(t, updates, "devices.diff")
	var devices DeviceDiff
	if err := json.Unmarshal(deviceMessage.Payload, &devices); err != nil {
		t.Fatal(err)
	}
	if len(devices.Devices) != 1 || devices.Devices[0].Status != "online" {
		t.Fatalf("unexpected device diff: %#v", devices)
	}

	manager.PublishLog("INFO", "command complete")
	logMessage := waitForMessage(t, updates, "logs.diff")
	var logs LogDiff
	if err := json.Unmarshal(logMessage.Payload, &logs); err != nil {
		t.Fatal(err)
	}
	if len(logs.Entries) != 1 || logs.Entries[0].Message != "command complete" {
		t.Fatalf("unexpected log diff: %#v", logs)
	}
}

func waitForMessage(t *testing.T, updates <-chan Message, kind string) Message {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case message, open := <-updates:
			if !open {
				t.Fatal("data manager subscription closed")
			}
			if message.Type == kind {
				return message
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for %s", kind)
		}
	}
}

package app

import (
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestApplyMaintenanceChecksIdentityAndSchedulesKnownAction(t *testing.T) {
	original := scheduleRemoteMaintenance
	t.Cleanup(func() { scheduleRemoteMaintenance = original })
	called := ""
	scheduleRemoteMaintenance = func(action string) error { called = action; return nil }
	app := &App{
		cfg:                 config.Config{Activation: config.ActivationConfig{HardwareID: "ABC123"}},
		maintenanceCommands: cloud.NewMaintenanceCommandExecutor(filepath.Join(t.TempDir(), "commands.json")),
	}
	now := time.Now()
	command := cloud.MaintenanceCommand{TaskID: "11111111-1111-4111-8111-111111111111", HardwareID: "ABC123", Action: cloud.MaintenanceGatewayReboot, IssuedAt: now, ExpiresAt: now.Add(time.Minute)}
	if result := app.applyMaintenance(command); result.Status != "ACCEPTED" || called != cloud.MaintenanceGatewayReboot {
		t.Fatal(result, called)
	}
	command.TaskID = "22222222-2222-4222-8222-222222222222"
	command.HardwareID = "OTHER"
	called = ""
	if result := app.applyMaintenance(command); result.Status != "FAILED" || called != "" {
		t.Fatal(result, called)
	}
}

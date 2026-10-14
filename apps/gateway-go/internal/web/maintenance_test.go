package web

import (
	"os"
	"path/filepath"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestFactoryDefaultConfigPreservesAccessAndClearsProject(t *testing.T) {
	enabled := true
	current := config.Config{
		GatewayKey: "gw-01",
		Web:        config.ListenerConfig{Enabled: true, Listen: "127.0.0.1:18088"},
		Security: config.SecurityConfig{
			Users: []config.UserConfig{{Username: "admin", RoleKey: "admin", Enabled: true}},
			Roles: []config.RoleConfig{{RoleKey: "admin", Permissions: []string{"*"}}},
		},
		MQTT:         config.MQTTConfig{Enabled: &enabled, Broker: "tcp://127.0.0.1:1883", ClientID: "client", Username: "user"},
		MQTTChannels: []config.MQTTConfig{{Name: "cloud", Enabled: &enabled, Broker: "tcp://127.0.0.1:1883", ClientID: "client", Username: "user"}},
		Devices: []config.DeviceConfig{{
			DeviceKey: "dev-01",
			Name:      "Device",
			Protocol:  "modbus-tcp",
			Address:   "127.0.0.1:502",
			Points:    []config.PointConfig{{Name: "P1", Metric: "p1", Function: 3, Register: 1, DataType: "uint16"}},
		}},
	}

	next := factoryDefaultConfig(current)

	if next.GatewayKey != "gw-01" {
		t.Fatalf("gatewayKey = %q", next.GatewayKey)
	}
	if next.Web.Listen != "127.0.0.1:18088" || !next.Web.Enabled {
		t.Fatalf("web config = %#v", next.Web)
	}
	if len(next.Security.Users) != 1 || len(next.Security.Roles) != 1 {
		t.Fatalf("security was not preserved: %#v", next.Security)
	}
	if len(next.Devices) != 0 || len(next.Points) != 0 || len(next.MQTTChannels) != 0 {
		t.Fatalf("project data was not cleared: devices=%d points=%d mqttChannels=%d", len(next.Devices), len(next.Points), len(next.MQTTChannels))
	}
	if next.MQTT.IsEnabled() || next.Activation.IsEnabled() || next.OfflineCache.Enabled || next.WiFi.Enabled || next.Cellular.IsEnabled() || next.ForwardSlave.Enabled {
		t.Fatalf("factory features should be disabled: %#v", next)
	}
}

func TestVerifySuperAdminPasswordUsesEnabledSuperAdminOnly(t *testing.T) {
	adminHash, err := hashPassword("admin-secret")
	if err != nil {
		t.Fatal(err)
	}
	operatorHash, err := hashPassword("operator-secret")
	if err != nil {
		t.Fatal(err)
	}
	fullHash, err := hashPassword("full-secret")
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	cfg := config.Config{
		GatewayKey: "gw-01",
		MQTT:       config.MQTTConfig{Enabled: boolPtr(false)},
		Security: config.SecurityConfig{
			Users: []config.UserConfig{
				{Username: "admin", PasswordHash: adminHash, RoleKey: "admin", Enabled: true},
				{Username: "operator", PasswordHash: operatorHash, RoleKey: "operator", Enabled: true},
				{Username: "full", PasswordHash: fullHash, RoleKey: "full", Enabled: true},
			},
			Roles: []config.RoleConfig{
				{RoleKey: "admin", Permissions: []string{"*"}},
				{RoleKey: "operator", Permissions: []string{"maintenance.run", "config.manage"}},
				{RoleKey: "full", Permissions: []string{"*"}},
			},
		},
	}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	server := &Server{configPath: configPath}

	if !server.verifySuperAdminPassword("admin-secret") {
		t.Fatal("admin password should be accepted")
	}
	if server.verifySuperAdminPassword("operator-secret") {
		t.Fatal("operator password should not be accepted as super admin")
	}
	if server.verifySuperAdminPassword("full-secret") {
		t.Fatal("non-admin wildcard user password should not be accepted as super admin")
	}
	if server.verifySuperAdminPassword("wrong") {
		t.Fatal("wrong password should not be accepted")
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatal(err)
	}
}

func TestIsSuperAdminUser(t *testing.T) {
	if !isSuperAdminUser(currentUserInfo{Username: "admin", RoleKey: "viewer"}) {
		t.Fatal("admin username should be super admin")
	}
	if isSuperAdminUser(currentUserInfo{Username: "operator", RoleKey: "admin"}) {
		t.Fatal("admin role alone should not be super admin")
	}
	if isSuperAdminUser(currentUserInfo{Username: "full", RoleKey: "custom", Permissions: []string{"*"}}) {
		t.Fatal("wildcard permission alone should not be super admin")
	}
	if isSuperAdminUser(currentUserInfo{RoleKey: "operator", Permissions: []string{"maintenance.run", "config.manage"}}) {
		t.Fatal("operator should not be super admin")
	}
}

func boolPtr(value bool) *bool {
	return &value
}

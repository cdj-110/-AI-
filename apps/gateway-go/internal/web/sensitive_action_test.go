package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestSensitiveActionRequiresCurrentPasswordAndConsumesTOTP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	hash, err := hashPassword("operator-pass")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{GatewayKey: "test", Security: config.SecurityConfig{
		Users: []config.UserConfig{{Username: "operator", PasswordHash: hash, RoleKey: "operator", Enabled: true, MFAEnabled: true, TOTPSecret: secret}},
	}}
	cfg.ApplyDefaults()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	server := New(configForAuthTest(), state.New(cfg), nil, path, nil)
	auth := server.authState()
	auth.mu.Lock()
	auth.sessions["sensitive-test"] = sessionInfo{ExpiresAt: time.Now().Add(time.Hour), Username: "operator", RoleKey: "operator", Permissions: []string{"maintenance.restart"}, MFAEnabled: true}
	auth.mu.Unlock()
	request := httptest.NewRequest(http.MethodPost, "/api/maintenance/restart", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "sensitive-test"})

	if err := server.verifySensitiveAction(request, sensitiveActionRequest{}); err != errSensitivePasswordRequired {
		t.Fatalf("missing password error = %v", err)
	}
	if err := server.verifySensitiveAction(request, sensitiveActionRequest{CurrentPassword: "wrong"}); err != errSensitivePasswordInvalid {
		t.Fatalf("wrong password error = %v", err)
	}
	code, err := totpCode(secret, time.Now().Unix()/30)
	if err != nil {
		t.Fatal(err)
	}
	body := sensitiveActionRequest{CurrentPassword: "operator-pass", TOTPCode: code}
	if err := server.verifySensitiveAction(request, body); err != nil {
		t.Fatalf("valid verification failed: %v", err)
	}
	if err := server.verifySensitiveAction(request, body); err != errSensitiveTOTPInvalid {
		t.Fatalf("replayed TOTP error = %v", err)
	}
}

func TestLegacyMaintenancePermissionImpliesGranularPermissions(t *testing.T) {
	for _, permission := range legacyMaintenancePermissionsForTest() {
		if !permissionGranted([]string{"maintenance.run"}, permission) {
			t.Fatalf("maintenance.run no longer implies %q", permission)
		}
	}
	if permissionGranted([]string{"maintenance.diagnose"}, "maintenance.factory_reset") {
		t.Fatal("diagnostic permission must not imply factory reset")
	}
}

func legacyMaintenancePermissionsForTest() []string {
	return []string{"maintenance.view", "maintenance.diagnose", "maintenance.time", "maintenance.remote", "maintenance.restart", "maintenance.factory_reset", "maintenance.audit"}
}

func TestSaveAndApplyConfigRestoresPreviousConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	previous := config.Config{GatewayKey: "previous"}
	previous.ApplyDefaults()
	if err := config.Save(path, previous); err != nil {
		t.Fatal(err)
	}
	server := New(configForAuthTest(), state.New(previous), nil, path, func(next config.Config) error {
		if next.GatewayKey == "next" {
			return errText("simulated apply failure")
		}
		return nil
	})
	next := previous
	next.GatewayKey = "next"
	err := server.saveAndApplyConfig(next)
	if err == nil || !strings.Contains(err.Error(), "previous configuration restored") {
		t.Fatalf("transaction error = %v", err)
	}
	restored, loadErr := config.Load(path)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if restored.GatewayKey != previous.GatewayKey {
		t.Fatalf("restored gateway key = %q", restored.GatewayKey)
	}
}

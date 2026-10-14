package web

import (
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestPermissionCatalogContainsCurrentGatewayModules(t *testing.T) {
	wanted := []string{
		"collection.run", "collection_incidents.view", "collection_incidents.manage", "points.write", "packet.view", "edge.view", "edge.manage",
		"alarms.view", "alarms.manage", "history.view", "history.manage", "offline.manage",
	}
	available := make(map[string]bool, len(permissionCatalog))
	for _, item := range permissionCatalog {
		available[item.Key] = true
	}
	for _, key := range wanted {
		if !available[key] {
			t.Errorf("permission catalog is missing %q", key)
		}
	}
}

func TestPermissionGrantedSupportsGranularAndLegacyRoles(t *testing.T) {
	if !permissionGranted([]string{"alarms.manage"}, "alarms.view") {
		t.Fatal("alarm manage should include alarm view")
	}
	if !permissionGranted([]string{"status.view"}, "collection_incidents.view") {
		t.Fatal("legacy status viewers should retain access to collection incidents")
	}
	if !permissionGranted([]string{"collection_incidents.manage"}, "collection_incidents.view") {
		t.Fatal("collection incident manage should include view")
	}
	if permissionGranted([]string{"alarms.view"}, "alarms.manage") {
		t.Fatal("alarm view must not include alarm manage")
	}
	for _, key := range []string{"collection.run", "points.write", "packet.view", "edge.manage", "alarms.manage", "history.manage", "offline.manage"} {
		if !permissionGranted([]string{"config.manage"}, key) {
			t.Errorf("legacy config.manage should still grant %q", key)
		}
	}
	if !permissionGranted([]string{"*"}, "future.permission") {
		t.Fatal("wildcard should include future permissions")
	}
}

func TestNormalizeSecurityUsersKeepsAdminAccount(t *testing.T) {
	adminHash, err := hashPassword("admin-secret")
	if err != nil {
		t.Fatal(err)
	}
	operatorHash, err := hashPassword("operator-secret")
	if err != nil {
		t.Fatal(err)
	}
	existingUsers := map[string]config.UserConfig{
		"admin":    {Username: "admin", PasswordHash: adminHash, MFAEnabled: true, TOTPSecret: "SECRET", RecoveryCodeHashes: []string{"hash"}, TOTPLastCounter: 42},
		"operator": {Username: "operator", PasswordHash: operatorHash},
	}

	if _, err := normalizeSecurityUsers([]securityUserPayload{
		{Username: "operator", RoleKey: "operator", Enabled: true},
	}, existingUsers); err == nil {
		t.Fatal("removing admin should be rejected")
	}

	users, err := normalizeSecurityUsers([]securityUserPayload{
		{Username: "admin", RoleKey: "viewer", Enabled: false},
		{Username: "operator", RoleKey: "operator", Enabled: true},
	}, existingUsers)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if user.Username == "admin" {
			if user.RoleKey != "admin" || !user.Enabled {
				t.Fatalf("admin should stay enabled super admin: %#v", user)
			}
			if !user.MFAEnabled || user.TOTPSecret != "SECRET" || len(user.RecoveryCodeHashes) != 1 || user.TOTPLastCounter != 42 {
				t.Fatalf("saving user management must preserve private MFA state: %#v", user)
			}
			return
		}
	}
	t.Fatal("admin user missing")
}

func TestSecurityChangesRevokeAffectedSessionsOnly(t *testing.T) {
	server := newAuthTestServer()
	auth := server.authState()
	auth.mu.Lock()
	auth.sessions["admin-session"] = sessionInfo{Username: "admin", RoleKey: "admin"}
	auth.sessions["operator-session"] = sessionInfo{Username: "operator", RoleKey: "operator"}
	auth.mu.Unlock()
	previous := config.SecurityConfig{
		Users: []config.UserConfig{{Username: "admin", PasswordHash: "a", RoleKey: "admin", Enabled: true}, {Username: "operator", PasswordHash: "b", RoleKey: "operator", Enabled: true}},
	}
	next := previous
	next.Users = append([]config.UserConfig{}, previous.Users...)
	next.Users[1].Enabled = false
	server.revokeChangedSecuritySessions(previous, next)
	auth.mu.Lock()
	defer auth.mu.Unlock()
	if _, ok := auth.sessions["operator-session"]; ok {
		t.Fatal("disabled user's session was not revoked")
	}
	if _, ok := auth.sessions["admin-session"]; !ok {
		t.Fatal("unaffected administrator session was revoked")
	}
}

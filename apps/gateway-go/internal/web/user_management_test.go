package web

import "testing"

func TestNormalizeSecurityUsersKeepsAdminAccount(t *testing.T) {
	adminHash, err := hashPassword("admin-secret")
	if err != nil {
		t.Fatal(err)
	}
	operatorHash, err := hashPassword("operator-secret")
	if err != nil {
		t.Fatal(err)
	}
	existingHashes := map[string]string{
		"admin":    adminHash,
		"operator": operatorHash,
	}

	if _, err := normalizeSecurityUsers([]securityUserPayload{
		{Username: "operator", RoleKey: "operator", Enabled: true},
	}, existingHashes); err == nil {
		t.Fatal("removing admin should be rejected")
	}

	users, err := normalizeSecurityUsers([]securityUserPayload{
		{Username: "admin", RoleKey: "viewer", Enabled: false},
		{Username: "operator", RoleKey: "operator", Enabled: true},
	}, existingHashes)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if user.Username == "admin" {
			if user.RoleKey != "admin" || !user.Enabled {
				t.Fatalf("admin should stay enabled super admin: %#v", user)
			}
			return
		}
	}
	t.Fatal("admin user missing")
}

package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func authorizeConfigTestRequest(s *Server, r *http.Request, permissions ...string) {
	s.authState().sessions["config-test"] = sessionInfo{Username: "test", Permissions: permissions, ExpiresAt: time.Now().Add(time.Hour)}
	r.AddCookie(&http.Cookie{Name: sessionCookieNameForRequest(r), Value: "config-test"})
}

func TestHistoryViewerCanLoadCatalogueWithoutConfigPermission(t *testing.T) {
	cfg, err := config.Parse([]byte(`{"gatewayKey":"test","devices":[{"deviceKey":"meter","name":"电表","protocol":"dlt645","password":"private-device-secret"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	s := &Server{configPath: path}
	r := httptest.NewRequest(http.MethodGet, "/api/history-storage", nil)
	authorizeConfigTestRequest(s, r, "history.view")
	w := httptest.NewRecorder()
	s.historyStorageStatus(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		PointGroups []historyPointGroup `json:"pointGroups"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.PointGroups) != 1 || result.PointGroups[0].DeviceKey != "meter" {
		t.Fatalf("unexpected catalogue: %+v", result)
	}
	if strings.Contains(w.Body.String(), "private-device-secret") {
		t.Fatal("device credential exposed")
	}
}

func TestConfigDocumentPermissions(t *testing.T) {
	for _, route := range []string{"full", "compact", "import"} {
		for _, section := range []string{"security", "mqtt", "edge"} {
			t.Run(route+"/"+section, func(t *testing.T) {
				cfg, err := config.Parse([]byte(`{"gatewayKey":"test","mqtt":{"enabled":false,"broker":"tcp://127.0.0.1:1883","clientId":"test","username":"test"}}`))
				if err != nil {
					t.Fatal(err)
				}
				cfg.Security.Users = []config.UserConfig{{Username: "test", RoleKey: "limited", Enabled: true}}
				cfg.Security.Roles = []config.RoleConfig{{RoleKey: "limited", Name: "Limited", Permissions: []string{"config.manage"}}}
				path := filepath.Join(t.TempDir(), "config.json")
				if err = config.Save(path, cfg); err != nil {
					t.Fatal(err)
				}
				next, err := config.Load(path)
				if err != nil {
					t.Fatal(err)
				}
				switch section {
				case "security":
					next.Security.Roles[0].Permissions = []string{"*"}
				case "mqtt":
					next.MQTT.Password = "changed"
				case "edge":
					next.EdgeComputing.Enabled = !next.EdgeComputing.Enabled
				}
				raw, _ := json.Marshal(next)
				req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(raw))
				req.Header.Set("Content-Type", "application/vnd.weikong.config+json")
				s := &Server{configPath: path, onConfig: func(config.Config) error { return nil }}
				authorizeConfigTestRequest(s, req, "config.manage")
				w := httptest.NewRecorder()
				switch route {
				case "full":
					s.configFile(w, req)
				case "compact":
					s.saveCompactConfig(w, req)
				case "import":
					req.Method = http.MethodPost
					s.importProject(w, req)
				}
				if section == "edge" {
					if w.Code != 200 {
						t.Fatalf("allowed edit failed: %d %s", w.Code, w.Body.String())
					}
					return
				}
				if w.Code != 403 {
					t.Fatalf("sensitive edit accepted: %d %s", w.Code, w.Body.String())
				}
				saved, _ := config.Load(path)
				if saved.Security.Roles[0].Permissions[0] == "*" || saved.MQTT.Password == "changed" {
					t.Fatal("forbidden change reached disk")
				}
			})
		}
	}
}

func TestConfigRollbackReportsRuntimeFailure(t *testing.T) {
	cfg, err := config.Parse([]byte(`{"gatewayKey":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err = config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	s := &Server{configPath: path, onConfig: func(config.Config) error { return errors.New("simulated apply failure") }}
	err = s.saveAndApplyConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "rollback runtime failed") || strings.Contains(err.Error(), "previous configuration restored") {
		t.Fatalf("misleading result: %v", err)
	}
}

func TestEdgeOnlyPermissionUsesScopedSave(t *testing.T) {
	cfg, err := config.Parse([]byte(`{"gatewayKey":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err = config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	s := &Server{configPath: path, onConfig: func(config.Config) error { return nil }}
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"edgeComputing":{"enabled":true,"groups":[]}}`, 200},
		{`{"edgeComputing":{"enabled":false},"security":{"users":[]}}`, 400},
		{`{}`, 400},
	} {
		r := httptest.NewRequest(http.MethodPut, "/api/edge-compute/config", strings.NewReader(test.body))
		authorizeConfigTestRequest(s, r, "edge.manage")
		w := httptest.NewRecorder()
		s.saveEdgeConfig(w, r)
		if w.Code != test.status {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	saved, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.GatewayKey != "test" || !saved.EdgeComputing.Enabled {
		t.Fatal("scoped save changed unrelated config or accepted invalid body")
	}
}

func TestModuleSavePreservesLatestSecurity(t *testing.T) {
	cfg, err := config.Parse([]byte(`{"gatewayKey":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	stale := cfg
	cfg.Security.Users = []config.UserConfig{{Username: "user", RoleKey: "viewer", Enabled: true, MFAEnabled: true, TOTPSecret: "test-only", TOTPLastCounter: 123}}
	if err = config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	s := &Server{configPath: path, onConfig: func(config.Config) error { return nil }}
	if err = s.saveAndApplyConfig(stale); err != nil {
		t.Fatal(err)
	}
	current, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Security.Users) != 1 || current.Security.Users[0].TOTPLastCounter != 123 {
		t.Fatal("module save reverted security")
	}
}

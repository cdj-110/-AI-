package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestLicenseDetailsRequiresPermissionAndOnlyExposesSummary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"activation":{"file":"license.json"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "license.json"), []byte(`{"sn":"WK-DETAILS","hardwareId":"HW","chipUsid":"CHIP","licenseVersion":2,"keyId":"production-v2","licenseSequence":3,"expiresAt":"2030-01-01T00:00:00Z","features":["collection"],"signature":"secret-signature","deviceSecret":"secret-password","broker":"tcp://private-broker:1883","cloudApi":"https://private-cloud","remoteAccessEndpoint":"wss://private-tunnel"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{configPath: path, store: state.New(config.Config{GatewayKey: "WK-DETAILS", License: config.LicenseStatus{Valid: true}})}
	for _, tc := range []struct {
		name        string
		permissions []string
		code        int
		replace     bool
	}{
		{"anonymous", nil, 401, false}, {"no permission", []string{"status.view"}, 403, false},
		{"maintenance viewer", []string{"maintenance.view"}, 200, false}, {"legacy maintenance viewer", []string{"maintenance.run"}, 200, false},
		{"license manager", []string{"cloud.manage"}, 200, true}, {"administrator", []string{"*"}, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/license/details", nil)
			if tc.permissions != nil {
				authorizeConfigTestRequest(s, r, tc.permissions...)
			}
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Fatalf("status: %d %s", w.Code, w.Body.String())
			}
			if tc.code != 200 {
				return
			}
			for _, secret := range []string{"secret-signature", "secret-password", "private-broker", "private-cloud", "private-tunnel", dir, "\"content\"", "\"file\""} {
				if strings.Contains(w.Body.String(), secret) {
					t.Fatalf("summary exposed %s", secret)
				}
			}
			var body struct {
				CanReplace    bool `json:"canReplace"`
				Authorization struct {
					SN       string `json:"sn"`
					Sequence int    `json:"licenseSequence"`
				} `json:"authorization"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.CanReplace != tc.replace || body.Authorization.SN != "WK-DETAILS" || body.Authorization.Sequence != 3 {
				t.Fatalf("incorrect summary: %+v", body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("summary was cacheable")
			}
			// Read access never grants replacement permissions.
			if !tc.replace {
				replace := httptest.NewRequest(http.MethodPost, "/api/activation", strings.NewReader(`{"content":"{}"}`))
				authorizeConfigTestRequest(s, replace, tc.permissions...)
				denied := httptest.NewRecorder()
				s.routes().ServeHTTP(denied, replace)
				if denied.Code != 403 {
					t.Fatal("viewer could replace license")
				}
			}
		})
	}
}

func TestLicenseDetailsFailsClosedWithoutReadableLicense(t *testing.T) {
	for _, name := range []string{"missing", "corrupt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			if err := os.WriteFile(path, []byte(`{"activation":{"file":"license.json"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			if name == "corrupt" {
				if err := os.WriteFile(filepath.Join(dir, "license.json"), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			s := &Server{configPath: path, store: state.New(config.Config{License: config.LicenseStatus{Valid: true}})}
			r := httptest.NewRequest(http.MethodGet, "/api/license/details", nil)
			authorizeConfigTestRequest(s, r, "cloud.manage")
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			if w.Code != 503 || strings.Contains(w.Body.String(), dir) {
				t.Fatalf("unsafe failed summary: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

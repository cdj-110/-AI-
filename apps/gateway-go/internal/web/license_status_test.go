package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestLicenseStatusPublicWhileRestrictedAndNoBusinessData(t *testing.T) {
	cfg := config.Config{GatewayKey: "WK-TEST", Activation: config.ActivationConfig{DeviceSecret: "never-expose-this"}, License: config.LicenseStatus{Restricted: true, Reason: "awaiting signed license"}}
	s := &Server{store: state.New(cfg)}
	handler := s.routes()
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/api/license/status", nil))
	if unauth.Code != http.StatusOK {
		t.Fatalf("anonymous status: %d", unauth.Code)
	}
	if strings.Contains(unauth.Body.String(), "never-expose-this") || strings.Contains(unauth.Body.String(), "awaiting signed license") || strings.Contains(unauth.Body.String(), "points") {
		t.Fatal("public activation status leaked credentials or internal diagnostics")
	}
	if !strings.Contains(unauth.Body.String(), `"authenticated":false`) {
		t.Fatal("anonymous identity incorrectly marked authenticated")
	}
	request := httptest.NewRequest(http.MethodGet, "/api/license/status", nil)
	authorizeConfigTestRequest(s, request)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("logged in without business permissions: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		License config.LicenseStatus `json:"license"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.License.Restricted || body.License.Valid {
		t.Fatalf("incorrect state: %+v", body.License)
	}
	if strings.Contains(response.Body.String(), "never-expose-this") || strings.Contains(response.Body.String(), "points") {
		t.Fatal("authorization status exposed business data or credentials")
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("license status must not be cached")
	}
}

func TestLicenseEntryKeepsActivatedAndBusinessRoutesProtected(t *testing.T) {
	for _, restricted := range []bool{false, true} {
		s := &Server{store: state.New(config.Config{GatewayKey: "WK-TEST", License: config.LicenseStatus{Valid: !restricted, Restricted: restricted}})}
		handler := s.routes()
		for _, path := range []string{"/api/config", "/api/status", "/api/session", "/api/activation"} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("restricted=%v %s bypassed authentication: %d", restricted, path, w.Code)
			}
		}
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(method, "/api/activation", strings.NewReader(`{"content":"test"}`)))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("anonymous import bypassed authentication: %d", w.Code)
			}
		}
		root := httptest.NewRecorder()
		handler.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
		if restricted && root.Code != 200 {
			t.Fatalf("restricted entry: %d", root.Code)
		}
		if !restricted && root.Code != http.StatusSeeOther {
			t.Fatalf("activated entry no longer requires login: %d", root.Code)
		}
		status := httptest.NewRecorder()
		handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/license/status", nil))
		if !restricted && status.Code != http.StatusUnauthorized {
			t.Fatal("activated license status exposed publicly")
		}
	}
}

func TestLicenseStatusRejectsMutation(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	s.licenseStatus(w, httptest.NewRequest(http.MethodPost, "/api/license/status", nil))
	if w.Code != 405 {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestRestrictedLogoutReturnsToPublicActivationEntry(t *testing.T) {
	s := &Server{store: state.New(config.Config{License: config.LicenseStatus{Restricted: true}})}
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	authorizeConfigTestRequest(s, r)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("logout did not return to activation entry: %d %s", w.Code, w.Header().Get("Location"))
	}
	if s.authenticated(r) {
		t.Fatal("logout did not revoke administrator session")
	}
}

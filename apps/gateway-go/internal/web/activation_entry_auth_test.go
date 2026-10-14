package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestActivationEntryLoginMatchesLicenseState(t *testing.T) {
	for _, restricted := range []bool{true, false} {
		s := &Server{store: state.New(config.Config{License: config.LicenseStatus{Valid: !restricted, Restricted: restricted}})}
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/login", nil))
		if restricted {
			if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
				t.Fatalf("unlicensed login entry: %d", w.Code)
			}
		} else if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "login-card") {
			t.Fatalf("activated login entry: %d", w.Code)
		}
	}
}

func TestActivationInlineAuthenticationPreservesRateLimits(t *testing.T) {
	t.Setenv("GATEWAY_ADMIN_USERNAME", "admin")
	t.Setenv("GATEWAY_ADMIN_PASSWORD", "TestOnlyPass9")
	s := newAuthTestServer()
	for attempt := 0; attempt <= maxLoginFailures; attempt++ {
		r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(url.Values{"username": {"admin"}, "password": {"wrong"}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		expected := http.StatusUnauthorized
		if attempt == maxLoginFailures {
			expected = http.StatusTooManyRequests
		}
		if w.Code != expected || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || len(w.Result().Cookies()) > 0 {
			t.Fatalf("inline failed login %d: %d", attempt, w.Code)
		}
	}
	s = newAuthTestServer()
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(url.Values{"username": {"admin"}, "password": {"TestOnlyPass9"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Location") != "" || !strings.Contains(w.Body.String(), `"next":"/"`) || len(w.Result().Cookies()) != 1 {
		t.Fatalf("inline successful authentication: %d %s", w.Code, w.Body.String())
	}
}

func TestActivationInlineAuthenticationRequiresMFA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	hash, err := hashPassword("TestOnlyPass9")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{GatewayKey: "test-gateway", Security: config.SecurityConfig{Users: []config.UserConfig{{Username: "admin", PasswordHash: hash, RoleKey: "admin", Enabled: true, MFAEnabled: true, TOTPSecret: secret}}}}
	cfg.ApplyDefaults()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	s := New(configForAuthTest(), state.New(cfg), nil, path, nil)
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(url.Values{"username": {"admin"}, "password": {"TestOnlyPass9"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mfaRequired":true`) {
		t.Fatal("MFA not required")
	}
	var challenge *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == sessionCookieNameForRequest(r) {
			t.Fatal("session issued before MFA")
		}
		if cookie.Path == "/login/verify" {
			challenge = cookie
		}
	}
	if challenge == nil {
		t.Fatal("MFA cookie missing")
	}
	code, err := totpCode(secret, time.Now().Unix()/30)
	if err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest(http.MethodPost, "/login/verify", strings.NewReader(url.Values{"code": {code}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	r.AddCookie(challenge)
	w = httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Location") != "" || !strings.Contains(w.Body.String(), `"next":"/"`) {
		t.Fatalf("MFA completion: %d %s", w.Code, w.Body.String())
	}
	found := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == sessionCookieNameForRequest(r) && cookie.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("session not issued after MFA")
	}
}

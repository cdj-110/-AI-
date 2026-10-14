package web

import (
	"encoding/json"
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

func TestProtectedRoutesRequireLogin(t *testing.T) {
	server := newAuthTestServer()

	pageResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(pageResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if pageResponse.Code != http.StatusSeeOther || !strings.HasPrefix(pageResponse.Header().Get("Location"), "/login") {
		t.Fatalf("page response = %d %q", pageResponse.Code, pageResponse.Header().Get("Location"))
	}

	apiResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if apiResponse.Code != http.StatusUnauthorized {
		t.Fatalf("protected API status = %d, want %d", apiResponse.Code, http.StatusUnauthorized)
	}
}

func TestSessionActivityExtendsIdleExpiryWithoutExtendingAbsoluteExpiry(t *testing.T) {
	server := newAuthTestServer()
	auth := server.authState()
	absolute := time.Now().Add(2 * time.Hour)
	auth.sessions["idle-session"] = sessionInfo{
		ExpiresAt: time.Now().Add(time.Minute), AbsoluteExpiresAt: absolute,
		Username: "admin", RoleKey: "admin", Permissions: []string{"*"},
	}
	request := httptest.NewRequest(http.MethodPost, "/api/session/activity", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "idle-session"})
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("activity status = %d body=%s", response.Code, response.Body.String())
	}
	auth.mu.Lock()
	updated := auth.sessions["idle-session"]
	auth.mu.Unlock()
	if time.Until(updated.ExpiresAt) < 59*time.Minute || !updated.AbsoluteExpiresAt.Equal(absolute) {
		t.Fatalf("updated session = %#v", updated)
	}
}

func TestExpiredIdleSessionIsRejected(t *testing.T) {
	server := newAuthTestServer()
	server.authState().sessions["expired-session"] = sessionInfo{
		ExpiresAt: time.Now().Add(-time.Second), AbsoluteExpiresAt: time.Now().Add(time.Hour),
		Username: "admin", RoleKey: "admin", Permissions: []string{"*"},
	}
	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired-session"})
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expired session status = %d", response.Code)
	}
}

func TestStatusRequiresLogin(t *testing.T) {
	server := newAuthTestServer()

	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status response = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestPointStatusReturnsLatestInternalValue(t *testing.T) {
	cfg := config.Config{GatewayKey: "test-gateway", Points: []config.PointConfig{{DeviceKey: "d1", Metric: "p1"}}}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	store.SetPointValue("d1", "p1", 42.5)
	server := New(configForAuthTest(), store, nil, "", nil)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/point-status", nil)
	authenticateTestRequest(server, request)
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("point status response = %d, want %d", response.Code, http.StatusOK)
	}
	var points []state.PointStatus
	if err := json.NewDecoder(response.Body).Decode(&points); err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Value != 42.5 {
		t.Fatalf("point status = %#v", points)
	}
}

func TestPointStatusFiltersCurrentDevice(t *testing.T) {
	cfg := config.Config{GatewayKey: "test-gateway", Points: []config.PointConfig{
		{DeviceKey: "d1", Metric: "p1"},
		{DeviceKey: "d2", Metric: "p2"},
	}}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	store.SetPointValue("d1", "p1", 1)
	store.SetPointValue("d2", "p2", 2)
	server := New(configForAuthTest(), store, nil, "", nil)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/point-status?deviceKey=d2", nil)
	authenticateTestRequest(server, request)
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("point status response = %d, want %d", response.Code, http.StatusOK)
	}
	var points []state.PointStatus
	if err := json.NewDecoder(response.Body).Decode(&points); err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].DeviceKey != "d2" || points[0].Value != float64(2) {
		t.Fatalf("filtered point status = %#v", points)
	}
}

func TestDefaultCredentialsCreateSession(t *testing.T) {
	server := newAuthTestServer()
	form := url.Values{"username": {"admin"}, "password": {"123456"}, "next": {"/"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	result := response.Result()
	if len(result.Cookies()) != 1 || result.Cookies()[0].Name != sessionCookieName || !result.Cookies()[0].HttpOnly {
		t.Fatalf("unexpected login cookies: %#v", result.Cookies())
	}

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	authenticatedRequest.AddCookie(result.Cookies()[0])
	authenticatedResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(authenticatedResponse, authenticatedRequest)
	if authenticatedResponse.Code != http.StatusSeeOther || !strings.HasPrefix(authenticatedResponse.Header().Get("Location"), "/change-password") {
		t.Fatalf("authenticated page should require password change, status = %d location = %q", authenticatedResponse.Code, authenticatedResponse.Header().Get("Location"))
	}
}

func TestInvalidCredentialsAreRejected(t *testing.T) {
	server := newAuthTestServer()
	form := url.Values{"username": {"admin"}, "password": {"wrong"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestChangedPasswordLoginDoesNotReturnToStalePasswordPage(t *testing.T) {
	server := newAuthTestServer()
	server.authState().password = "ChangedPassword!9"
	form := url.Values{
		"username": {"admin"}, "password": {"ChangedPassword!9"},
		"next": {"/change-password?next=%2F"},
	}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("login response = %d location=%q", response.Code, response.Header().Get("Location"))
	}
}

func TestPasswordPageAlwaysShowsSkipButton(t *testing.T) {
	server := newAuthTestServer()
	request := httptest.NewRequest(http.MethodGet, "/change-password?next=/", nil)
	authenticateTestRequest(server, request)
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "暂时跳过") {
		t.Fatalf("password page status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestNewPasswordHashesUseBcrypt(t *testing.T) {
	hash, err := hashPassword("123456")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$2") || !verifyPasswordHash(hash, "123456") || verifyPasswordHash(hash, "wrong") {
		t.Fatalf("unexpected bcrypt verification result for %q", hash)
	}
}

func TestPasswordPolicy(t *testing.T) {
	for _, item := range []struct {
		name, username, password string
		valid                    bool
	}{
		{name: "strong", username: "operator", password: "Secure-Edge-2026!", valid: true},
		{name: "simple digits", username: "operator", password: "246810", valid: true},
		{name: "simple letters", username: "operator", password: "edgegw", valid: true},
		{name: "simple unicode", username: "operator", password: "现场密码测试", valid: true},
		{name: "too short", username: "operator", password: "12345"},
		{name: "common", username: "operator", password: "password123"},
		{name: "contains username", username: "operator", password: "Operator!2026"},
		{name: "spaces only", username: "operator", password: "      "},
	} {
		t.Run(item.name, func(t *testing.T) {
			err := validatePassword(item.username, item.password)
			if (err == nil) != item.valid {
				t.Fatalf("validatePassword() error = %v, valid = %v", err, item.valid)
			}
		})
	}
}

func TestForcedPasswordChangeRevokesOldSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	hash, err := hashPassword("TempPass!9")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{GatewayKey: "test-gateway", Security: config.SecurityConfig{
		Users: []config.UserConfig{{Username: "operator", PasswordHash: hash, RoleKey: "operator", Enabled: true, MustChangePassword: true}},
	}}
	cfg.ApplyDefaults()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	server := New(configForAuthTest(), state.New(cfg), nil, path, nil)
	loginForm := url.Values{"username": {"operator"}, "password": {"TempPass!9"}, "next": {"/"}}
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginForm.Encode()))
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(loginResponse, loginRequest)
	oldCookie := loginResponse.Result().Cookies()[0]
	if !strings.HasPrefix(loginResponse.Header().Get("Location"), "/change-password") {
		t.Fatalf("login location = %q", loginResponse.Header().Get("Location"))
	}

	form := url.Values{"current_password": {"TempPass!9"}, "new_password": {"NewSecure!2026"}, "confirm_password": {"NewSecure!2026"}, "next": {"/"}}
	request := httptest.NewRequest(http.MethodPost, "/change-password", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(oldCookie)
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("change response = %d %q body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	newCookies := response.Result().Cookies()
	if len(newCookies) != 1 {
		t.Fatalf("new cookies = %#v", newCookies)
	}
	oldRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	oldRequest.AddCookie(oldCookie)
	oldResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(oldResponse, oldRequest)
	if oldResponse.Code != http.StatusUnauthorized {
		t.Fatalf("old session status = %d", oldResponse.Code)
	}
	newRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	newRequest.AddCookie(newCookies[0])
	newResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(newResponse, newRequest)
	if newResponse.Code != http.StatusOK {
		t.Fatalf("new session status = %d body=%s", newResponse.Code, newResponse.Body.String())
	}
	updated, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Security.Users[0].MustChangePassword || !verifyPasswordHash(updated.Security.Users[0].PasswordHash, "NewSecure!2026") {
		t.Fatalf("updated account = %#v", updated.Security.Users[0])
	}
}

func TestSkipPasswordChangeOnlyUnlocksCurrentSession(t *testing.T) {
	server := newAuthTestServer()
	auth := server.authState()
	auth.mu.Lock()
	auth.sessions["forced-session"] = sessionInfo{
		ExpiresAt: time.Now().Add(time.Hour), Username: "admin", RoleKey: "admin",
		Permissions: []string{"*"}, PasswordChangeRequired: true,
	}
	auth.sessions["other-forced-session"] = sessionInfo{
		ExpiresAt: time.Now().Add(time.Hour), Username: "admin", RoleKey: "admin",
		Permissions: []string{"*"}, PasswordChangeRequired: true,
	}
	auth.mu.Unlock()

	form := url.Values{"next": {"/api/session"}}
	request := httptest.NewRequest(http.MethodPost, "/change-password/skip", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "forced-session"})
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/api/session" {
		t.Fatalf("skip response = %d %q", response.Code, response.Header().Get("Location"))
	}

	currentRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	currentRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "forced-session"})
	currentResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(currentResponse, currentRequest)
	if currentResponse.Code != http.StatusOK {
		t.Fatalf("current session remains locked: %d", currentResponse.Code)
	}
	otherRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	otherRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "other-forced-session"})
	otherResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(otherResponse, otherRequest)
	if otherResponse.Code != http.StatusPreconditionRequired {
		t.Fatalf("other session status = %d, want %d", otherResponse.Code, http.StatusPreconditionRequired)
	}
}

func TestLoginIsRateLimitedAfterRepeatedFailures(t *testing.T) {
	server := newAuthTestServer()
	for attempt := 1; attempt <= maxLoginFailures+1; attempt++ {
		form := url.Values{"username": {"admin"}, "password": {"wrong"}}
		request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		server.routes().ServeHTTP(response, request)
		if attempt <= maxLoginFailures && response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", attempt, response.Code)
		}
		if attempt > maxLoginFailures && response.Code != http.StatusTooManyRequests {
			t.Fatalf("rate-limited attempt status = %d, want %d", response.Code, http.StatusTooManyRequests)
		}
	}
}

func configForAuthTest() config.ListenerConfig {
	return config.ListenerConfig{Enabled: true, Listen: "127.0.0.1:8088"}
}

func newAuthTestServer() *Server {
	cfg := config.Config{GatewayKey: "test-gateway"}
	cfg.ApplyDefaults()
	return New(configForAuthTest(), state.New(cfg), nil, "", nil)
}

func authenticateTestRequest(server *Server, request *http.Request) {
	auth := server.authState()
	auth.mu.Lock()
	auth.sessions["test-session"] = sessionInfo{ExpiresAt: time.Now().Add(time.Hour), Username: "admin", RoleKey: "admin", Permissions: []string{"*"}}
	auth.mu.Unlock()
	request.AddCookie(&http.Cookie{Name: sessionCookieNameForRequest(request), Value: "test-session"})
}

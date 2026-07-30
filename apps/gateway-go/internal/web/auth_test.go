package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if authenticatedResponse.Code != http.StatusOK {
		t.Fatalf("authenticated page status = %d", authenticatedResponse.Code)
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

func TestNewPasswordHashesUseBcrypt(t *testing.T) {
	hash, err := hashPassword("123456")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$2") || !verifyPasswordHash(hash, "123456") || verifyPasswordHash(hash, "wrong") {
		t.Fatalf("unexpected bcrypt verification result for %q", hash)
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
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
}

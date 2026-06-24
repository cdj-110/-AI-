package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestProtectedRoutesRequireLogin(t *testing.T) {
	server := New(configForAuthTest(), nil, nil, "", nil)

	pageResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(pageResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if pageResponse.Code != http.StatusSeeOther || !strings.HasPrefix(pageResponse.Header().Get("Location"), "/login") {
		t.Fatalf("page response = %d %q", pageResponse.Code, pageResponse.Header().Get("Location"))
	}

	apiResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if apiResponse.Code != http.StatusUnauthorized {
		t.Fatalf("API status = %d, want %d", apiResponse.Code, http.StatusUnauthorized)
	}
}

func TestDefaultCredentialsCreateSession(t *testing.T) {
	server := New(configForAuthTest(), nil, nil, "", nil)
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
	server := New(configForAuthTest(), nil, nil, "", nil)
	form := url.Values{"username": {"admin"}, "password": {"wrong"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func configForAuthTest() config.ListenerConfig {
	return config.ListenerConfig{Enabled: true, Listen: "127.0.0.1:8088"}
}

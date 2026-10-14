package web

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Model one browser retaining its old cookies across HTTPS -> HTTP -> HTTPS.
// Cookie jars do not implement every browser anti-overlay rule, so explicitly
// require new names distinct from the legacy Secure cookies as well.
func TestLoginAcrossProtocolSwitchesWithLegacyCookies(t *testing.T) {
	server := newAuthTestServer()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	secureURL, _ := url.Parse("https://gateway.test/login/verify")
	jar.SetCookies(secureURL, []*http.Cookie{
		{Name: "weikong_gateway_session", Value: "legacy-session", Path: "/", Secure: true, HttpOnly: true},
		{Name: "weikong_gateway_mfa", Value: "legacy-mfa", Path: "/login/verify", Secure: true, HttpOnly: true},
	})
	for _, scheme := range []string{"https", "http", "https"} {
		baseURL := scheme + "://gateway.test"
		send := func(method, path string, form url.Values) *httptest.ResponseRecorder {
			request := httptest.NewRequest(method, baseURL+path, strings.NewReader(form.Encode()))
			if method == http.MethodPost {
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			for _, cookie := range jar.Cookies(request.URL) {
				request.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			server.routes().ServeHTTP(response, request)
			jar.SetCookies(request.URL, response.Result().Cookies())
			return response
		}
		login := send(http.MethodPost, "/login", url.Values{"username": {"admin"}, "password": {"123456"}})
		if login.Code != http.StatusSeeOther {
			t.Fatalf("%s login failed: %d", scheme, login.Code)
		}
		cookies := login.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != "weikong_gateway_session_"+scheme || cookies[0].Secure != (scheme == "https") || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" {
			t.Fatalf("%s login did not issue the protocol-specific protected cookie", scheme)
		}
		skip := send(http.MethodPost, "/change-password/skip", url.Values{"next": {"/"}})
		if skip.Code != http.StatusSeeOther || skip.Header().Get("Location") != "/" {
			t.Fatalf("%s skip could not find current session", scheme)
		}
		if response := send(http.MethodGet, "/api/session", nil); response.Code != http.StatusOK {
			t.Fatalf("%s session rejected after login: %d", scheme, response.Code)
		}
		// A service restart invalidates in-memory sessions, but leaves browser
		// cookies untouched, as when the gateway changes its listener protocol.
		server.auth = newAuthManager()
	}
}

func TestProtocolSessionIsolationAndLogout(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			server := newAuthTestServer()
			baseURL := scheme + "://gateway.test"
			request := httptest.NewRequest(http.MethodPost, baseURL+"/login", nil)
			response := httptest.NewRecorder()
			user := currentUserInfo{Username: "admin", Permissions: []string{"*"}}
			if err := server.issueSession(response, request, user, false); err != nil {
				t.Fatal(err)
			}
			cookie := response.Result().Cookies()[0]
			otherName := "weikong_gateway_session_https"
			if scheme == "https" {
				otherName = "weikong_gateway_session_http"
			}
			for _, name := range []string{"weikong_gateway_session", otherName} {
				probe := httptest.NewRequest(http.MethodGet, baseURL+"/api/session", nil)
				probe.AddCookie(&http.Cookie{Name: name, Value: cookie.Value})
				if _, ok := server.currentUser(probe); ok {
					t.Fatalf("accepted legacy/wrong-protocol cookie %s", name)
				}
			}
			logout := httptest.NewRequest(http.MethodPost, baseURL+"/logout", nil)
			logout.AddCookie(cookie)
			result := httptest.NewRecorder()
			server.routes().ServeHTTP(result, logout)
			if result.Code != http.StatusSeeOther {
				t.Fatalf("logout status = %d", result.Code)
			}
			cleared := result.Result().Cookies()
			if len(cleared) != 1 || cleared[0].Name != cookie.Name || cleared[0].Path != cookie.Path || cleared[0].Secure != cookie.Secure || cleared[0].MaxAge >= 0 {
				t.Fatal("logout must delete the cookie for the current protocol")
			}
			if _, ok := server.currentUser(logout); ok {
				t.Fatal("logout did not revoke session on the server")
			}
		})
	}
}

func TestExpiredMFAChallengeClearsCurrentProtocolCookie(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			server := newAuthTestServer()
			request := httptest.NewRequest(http.MethodGet, scheme+"://gateway.test/login/verify", nil)
			name := "weikong_gateway_mfa_" + scheme
			request.AddCookie(&http.Cookie{Name: name, Value: "expired-challenge"})
			result := httptest.NewRecorder()
			server.routes().ServeHTTP(result, request)
			cookies := result.Result().Cookies()
			if result.Code != http.StatusSeeOther || result.Header().Get("Location") != "/login" || len(cookies) != 1 {
				t.Fatal("expired challenge must redirect to login and clear its cookie")
			}
			if cookies[0].Name != name || cookies[0].MaxAge >= 0 || cookies[0].Path != "/login/verify" || cookies[0].Secure != (scheme == "https") {
				t.Fatal("expired challenge cleared the wrong cookie")
			}
		})
	}
}

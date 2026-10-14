package web

import (
	"bytes"
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

func TestTOTPMatchesRFC6238SHA1Vector(t *testing.T) {
	secret := base32Secret([]byte("12345678901234567890"))
	code, err := totpCode(secret, 59/30)
	if err != nil {
		t.Fatal(err)
	}
	if code != "287082" {
		t.Fatalf("TOTP code = %q, want 287082", code)
	}
	if counter, ok := verifyTOTP(secret, code, time.Unix(59, 0), -1); !ok || counter != 1 {
		t.Fatalf("verifyTOTP() = %d, %v", counter, ok)
	}
	if _, ok := verifyTOTP(secret, code, time.Unix(59, 0), 1); ok {
		t.Fatal("a previously accepted TOTP counter must not be accepted again")
	}
}

func base32Secret(raw []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	var result strings.Builder
	var bits uint64
	var count uint
	for _, value := range raw {
		bits = bits<<8 | uint64(value)
		count += 8
		for count >= 5 {
			count -= 5
			result.WriteByte(alphabet[(bits>>count)&31])
		}
	}
	if count > 0 {
		result.WriteByte(alphabet[(bits<<(5-count))&31])
	}
	return result.String()
}

func TestMFAEnabledLoginRequiresAndCompletesSecondFactor(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) { testMFALoginForProtocol(t, scheme) })
	}
}

func testMFALoginForProtocol(t *testing.T, scheme string) {
	baseURL := scheme + "://gateway.test"
	wantSession := "weikong_gateway_session_" + scheme
	wantMFA := "weikong_gateway_mfa_" + scheme
	path := filepath.Join(t.TempDir(), "config.json")
	hash, err := hashPassword("SecurePass9")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{GatewayKey: "test-gateway", Security: config.SecurityConfig{Users: []config.UserConfig{{Username: "operator", PasswordHash: hash, RoleKey: "operator", Enabled: true, MFAEnabled: true, TOTPSecret: secret}}}}
	cfg.ApplyDefaults()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	server := New(configForAuthTest(), state.New(cfg), nil, path, nil)
	form := url.Values{"username": {"operator"}, "password": {"SecurePass9"}, "next": {"/"}}
	request := httptest.NewRequest(http.MethodPost, baseURL+"/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login/verify" {
		t.Fatalf("first factor response = %d location=%q body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	var challengeCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == wantSession {
			t.Fatal("full session must not be issued before the second factor")
		}
		if cookie.Name == wantMFA {
			challengeCookie = cookie
		}
	}
	if challengeCookie == nil {
		t.Fatal("missing MFA challenge cookie")
	}
	if challengeCookie.Secure != (scheme == "https") || !challengeCookie.HttpOnly || challengeCookie.Path != "/login/verify" || challengeCookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("incorrect MFA cookie security attributes")
	}
	otherScheme := "https"
	if scheme == "https" {
		otherScheme = "http"
	}
	for _, ignoredName := range []string{"weikong_gateway_mfa", "weikong_gateway_mfa_" + otherScheme} {
		ignoredRequest := httptest.NewRequest(http.MethodGet, baseURL+"/login/verify", nil)
		ignoredRequest.AddCookie(&http.Cookie{Name: ignoredName, Value: challengeCookie.Value})
		ignoredResponse := httptest.NewRecorder()
		server.routes().ServeHTTP(ignoredResponse, ignoredRequest)
		if ignoredResponse.Code != http.StatusSeeOther || ignoredResponse.Header().Get("Location") != "/login" {
			t.Fatalf("accepted wrong-protocol or legacy challenge %s", ignoredName)
		}
	}
	code, err := totpCode(secret, time.Now().Unix()/30)
	if err != nil {
		t.Fatal(err)
	}
	verifyForm := url.Values{"code": {code}}
	verifyRequest := httptest.NewRequest(http.MethodPost, baseURL+"/login/verify", strings.NewReader(verifyForm.Encode()))
	verifyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	verifyRequest.AddCookie(challengeCookie)
	verifyResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(verifyResponse, verifyRequest)
	if verifyResponse.Code != http.StatusSeeOther || verifyResponse.Header().Get("Location") != "/" {
		t.Fatalf("second factor response = %d location=%q body=%s", verifyResponse.Code, verifyResponse.Header().Get("Location"), verifyResponse.Body.String())
	}
	foundSession := false
	clearedChallenge := false
	for _, cookie := range verifyResponse.Result().Cookies() {
		if cookie.Name == wantSession {
			foundSession = true
			if cookie.Secure != (scheme == "https") || !cookie.HttpOnly {
				t.Fatal("incorrect session security attributes after MFA")
			}
			sessionRequest := httptest.NewRequest(http.MethodGet, baseURL+"/api/session", nil)
			sessionRequest.AddCookie(cookie)
			sessionResponse := httptest.NewRecorder()
			server.routes().ServeHTTP(sessionResponse, sessionRequest)
			if sessionResponse.Code != http.StatusOK {
				t.Fatalf("MFA session rejected: %d", sessionResponse.Code)
			}
		}
		if cookie.Name == wantMFA && cookie.MaxAge < 0 && cookie.Path == challengeCookie.Path && cookie.Secure == challengeCookie.Secure {
			clearedChallenge = true
		}
	}
	if !foundSession || !clearedChallenge {
		t.Fatal("successful second factor must issue a full session and clear its challenge cookie")
	}
}

func TestRecoveryCodeCanOnlyBeConsumedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	code := "ABCDE-23456"
	cfg := config.Config{GatewayKey: "test-gateway", Security: config.SecurityConfig{Users: []config.UserConfig{{Username: "admin", PasswordHash: "unused", RoleKey: "admin", Enabled: true, MFAEnabled: true, TOTPSecret: "JBSWY3DPEHPK3PXP", RecoveryCodeHashes: []string{recoveryCodeHash(code)}}}}}
	cfg.ApplyDefaults()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	server := New(configForAuthTest(), state.New(cfg), nil, path, nil)
	if ok, err := server.consumeMFA("admin", code); err != nil || !ok {
		t.Fatalf("first recovery use = %v, %v", ok, err)
	}
	if ok, err := server.consumeMFA("admin", code); err != nil || ok {
		t.Fatalf("second recovery use = %v, %v", ok, err)
	}
}

func TestMFAEnrollmentPersistsFallbackAdministrator(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{GatewayKey: "test-gateway"}
	cfg.ApplyDefaults()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	server := New(configForAuthTest(), state.New(cfg), nil, path, nil)

	setupBody, _ := json.Marshal(map[string]string{"password": "123456"})
	setupRequest := httptest.NewRequest(http.MethodPost, "/api/security/mfa/setup", bytes.NewReader(setupBody))
	setupRequest.Header.Set("Content-Type", "application/json")
	authenticateTestRequest(server, setupRequest)
	setupResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(setupResponse, setupRequest)
	if setupResponse.Code != http.StatusOK {
		t.Fatalf("setup status=%d body=%s", setupResponse.Code, setupResponse.Body.String())
	}
	server.authState().mu.Lock()
	setup := server.authState().mfaSetups["admin"]
	server.authState().mu.Unlock()
	if setup.Secret == "" {
		t.Fatal("setup secret was not retained for confirmation")
	}
	code, err := totpCode(setup.Secret, time.Now().Unix()/30)
	if err != nil {
		t.Fatal(err)
	}
	enableBody, _ := json.Marshal(map[string]string{"code": code})
	enableRequest := httptest.NewRequest(http.MethodPost, "/api/security/mfa/enable", bytes.NewReader(enableBody))
	enableRequest.Header.Set("Content-Type", "application/json")
	authenticateTestRequest(server, enableRequest)
	enableResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(enableResponse, enableRequest)
	if enableResponse.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", enableResponse.Code, enableResponse.Body.String())
	}
	stored, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Security.Users) != 1 || stored.Security.Users[0].Username != "admin" || !stored.Security.Users[0].MFAEnabled || stored.Security.Users[0].TOTPSecret == "" || len(stored.Security.Users[0].RecoveryCodeHashes) != 8 {
		t.Fatalf("MFA enrollment was not persisted safely: %#v", stored.Security.Users)
	}
	if strings.Contains(enableResponse.Body.String(), stored.Security.Users[0].TOTPSecret) {
		t.Fatal("enable response leaked the persisted TOTP secret")
	}
}

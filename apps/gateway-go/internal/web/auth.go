package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const (
	sessionCookieName       = "weikong_gateway_session_http"
	sessionHTTPSCookieName  = "weikong_gateway_session_https"
	mfaCookieName           = "weikong_gateway_mfa_http"
	mfaHTTPSCookieName      = "weikong_gateway_mfa_https"
	sessionIdleLifetime     = 60 * time.Minute
	sessionAbsoluteLifetime = 12 * time.Hour
	loginWindow             = 5 * time.Minute
	loginBlockTime          = 5 * time.Minute
	maxLoginFailures        = 5
)

var permissionImplications = map[string]map[string]bool{
	// Keep existing read-only roles able to open the collection diagnostics
	// module after it was split out of the status page.
	"status.view":                 {"alarms.view": true, "collection_incidents.view": true},
	"config.view":                 {"alarms.view": true, "edge.view": true},
	"config.manage":               {"config.view": true, "collection.run": true, "collection_incidents.view": true, "collection_incidents.manage": true, "points.write": true, "packet.view": true, "edge.view": true, "edge.manage": true, "alarms.view": true, "alarms.manage": true, "history.view": true, "history.manage": true, "offline.manage": true},
	"collection_incidents.manage": {"collection_incidents.view": true},
	"edge.view":                   {"config.view": true},
	"edge.manage":                 {"edge.view": true, "config.view": true},
	"alarms.manage":               {"alarms.view": true},
	"history.manage":              {"history.view": true},
	"cloud.manage":                {"offline.manage": true},
	// Keep roles saved by older gateway versions working after the former
	// maintenance.run umbrella permission was split into least-privilege items.
	"maintenance.run": {
		"maintenance.view": true, "maintenance.diagnose": true, "maintenance.time": true,
		"maintenance.remote": true, "maintenance.restart": true,
		"maintenance.factory_reset": true, "maintenance.audit": true,
	},
}

type authManager struct {
	mu            sync.Mutex
	sessions      map[string]sessionInfo
	username      string
	password      string
	attempts      map[string]loginAttempt
	mfaChallenges map[string]mfaLoginChallenge
	mfaSetups     map[string]mfaSetup
}

type loginAttempt struct {
	Failures     []time.Time
	BlockedUntil time.Time
}

type sessionInfo struct {
	ExpiresAt              time.Time
	AbsoluteExpiresAt      time.Time
	Username               string
	DisplayName            string
	RoleKey                string
	Permissions            []string
	PasswordChangeRequired bool
	MFAEnabled             bool
}

type currentUserInfo struct {
	Username               string   `json:"username"`
	DisplayName            string   `json:"displayName,omitempty"`
	RoleKey                string   `json:"roleKey"`
	Permissions            []string `json:"permissions"`
	PasswordChangeRequired bool     `json:"passwordChangeRequired,omitempty"`
	MFAEnabled             bool     `json:"mfaEnabled,omitempty"`
}

func newAuthManager() *authManager {
	username := os.Getenv("GATEWAY_ADMIN_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("GATEWAY_ADMIN_PASSWORD")
	if password == "" {
		password = "123456"
	}
	return &authManager{sessions: make(map[string]sessionInfo), attempts: make(map[string]loginAttempt), mfaChallenges: make(map[string]mfaLoginChallenge), mfaSetups: make(map[string]mfaSetup), username: username, password: password}
}

func (s *Server) authState() *authManager {
	if s.auth == nil {
		s.auth = newAuthManager()
	}
	return s.auth
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, ok := s.currentUser(request)
		if !ok {
			if strings.HasPrefix(request.URL.Path, "/api/") {
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(writer, request, "/login?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		if user.PasswordChangeRequired {
			if strings.HasPrefix(request.URL.Path, "/api/") {
				http.Error(writer, "password change required", http.StatusPreconditionRequired)
				return
			}
			http.Redirect(writer, request, "/change-password?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) authenticated(request *http.Request) bool {
	_, ok := s.currentUser(request)
	return ok
}

// Use fresh names for both protocols: a legacy Secure cookie cannot be replaced
// or deleted over HTTP. Never fall back to that cookie after a protocol switch.
// Match the actual connection, not the saved TLS setting (which awaits restart).
func sessionCookieNameForRequest(request *http.Request) string {
	if request.TLS != nil {
		return sessionHTTPSCookieName
	}
	return sessionCookieName
}

func mfaCookieNameForRequest(request *http.Request) string {
	if request.TLS != nil {
		return mfaHTTPSCookieName
	}
	return mfaCookieName
}

func (s *Server) currentUser(request *http.Request) (currentUserInfo, bool) {
	cookie, err := request.Cookie(sessionCookieNameForRequest(request))
	if err != nil || cookie.Value == "" {
		return currentUserInfo{}, false
	}
	auth := s.authState()
	auth.mu.Lock()
	defer auth.mu.Unlock()
	session, ok := auth.sessions[cookie.Value]
	now := time.Now()
	if !ok || now.After(session.ExpiresAt) || (!session.AbsoluteExpiresAt.IsZero() && now.After(session.AbsoluteExpiresAt)) {
		delete(auth.sessions, cookie.Value)
		return currentUserInfo{}, false
	}
	return currentUserInfo{
		Username:               session.Username,
		DisplayName:            session.DisplayName,
		RoleKey:                session.RoleKey,
		Permissions:            session.Permissions,
		PasswordChangeRequired: session.PasswordChangeRequired,
		MFAEnabled:             session.MFAEnabled,
	}, true
}

func (s *Server) hasPermission(request *http.Request, permission string) bool {
	user, ok := s.currentUser(request)
	if !ok {
		return false
	}
	return permissionGranted(user.Permissions, permission)
}

// permissionGranted keeps roles created before granular feature permissions
// were introduced working after an upgrade. New roles can grant only the
// required feature, while the former umbrella permissions retain their old
// effective access.
func permissionGranted(granted []string, permission string) bool {
	for _, item := range granted {
		if item == "*" || item == permission {
			return true
		}
		if permissionImplications[item][permission] {
			return true
		}
	}
	return false
}

func (s *Server) login(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		if s.store != nil && s.store.CompactSnapshot().License.Restricted {
			http.Redirect(writer, request, "/", http.StatusSeeOther)
			return
		}
		if s.authenticated(request) {
			http.Redirect(writer, request, "/", http.StatusSeeOther)
			return
		}
		s.renderLogin(writer, request.URL.Query().Get("next"), "", http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	if err := request.ParseForm(); err != nil {
		s.renderLoginResponse(writer, request, "/", "请求格式错误", http.StatusBadRequest)
		return
	}
	username := request.FormValue("gateway_user")
	if username == "" {
		username = request.FormValue("username")
	}
	password := request.FormValue("gateway_pass")
	if password == "" {
		password = request.FormValue("password")
	}
	if retryAfter, allowed := s.allowLogin(request, username); !allowed {
		writer.Header().Set("Retry-After", strconv.Itoa(int(time.Until(retryAfter).Seconds())+1))
		s.auditLogin(request, username, false, "rate limited")
		s.renderLoginResponse(writer, request, request.FormValue("next"), "登录失败次数过多，请稍后重试", http.StatusTooManyRequests)
		return
	}
	loginUser, ok := s.verifyCredentials(username, password)
	if !ok {
		s.recordLoginResult(request, username, false)
		s.auditLogin(request, username, false, "invalid credentials")
		s.renderLoginResponse(writer, request, request.FormValue("next"), "账号或密码错误", http.StatusUnauthorized)
		return
	}
	s.recordLoginResult(request, username, true)
	if loginUser.MFAEnabled {
		if err := s.beginMFALogin(writer, request, loginUser, safeNext(request.FormValue("next"))); err != nil {
			http.Error(writer, "create verification session failed", http.StatusInternalServerError)
			return
		}
		authenticationResponse(writer, request, "/login/verify")
		return
	}
	s.auditLogin(request, username, true, "")
	if err := s.issueSession(writer, request, loginUser, false); err != nil {
		http.Error(writer, "create session failed", http.StatusInternalServerError)
		return
	}
	next := safeNext(request.FormValue("next"))
	if loginUser.PasswordChangeRequired {
		next = "/change-password?next=" + url.QueryEscape(next)
	} else if isPasswordChangePath(next) {
		// A gateway restart can invalidate the session while the browser is on
		// the password page. Do not carry that stale return address through the
		// next successful login after the password has already been changed.
		next = "/"
	}
	authenticationResponse(writer, request, next)
}

// Inline activation verification reuses the normal login, MFA and rate limits.
func authenticationResponse(writer http.ResponseWriter, request *http.Request, next string) {
	if strings.Contains(request.Header.Get("Accept"), "application/json") {
		writer.Header().Set("Cache-Control", "no-store")
		writeJSON(writer, map[string]interface{}{"next": next, "mfaRequired": next == "/login/verify", "passwordChangeRequired": isPasswordChangePath(next)})
		return
	}
	http.Redirect(writer, request, next, http.StatusSeeOther)
}

func (s *Server) renderLoginResponse(writer http.ResponseWriter, request *http.Request, next, message string, status int) {
	if strings.Contains(request.Header.Get("Accept"), "application/json") {
		writer.Header().Set("Cache-Control", "no-store")
		writeJSONStatus(writer, status, map[string]string{"message": message})
		return
	}
	s.renderLogin(writer, next, message, status)
}

func isPasswordChangePath(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Path == "/change-password" || parsed.Path == "/change-password/skip")
}

func (s *Server) issueSession(writer http.ResponseWriter, request *http.Request, user currentUserInfo, replaceUserSessions bool) error {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	now := time.Now()
	auth := s.authState()
	auth.mu.Lock()
	for key, session := range auth.sessions {
		if now.After(session.ExpiresAt) || (replaceUserSessions && session.Username == user.Username) {
			delete(auth.sessions, key)
		}
	}
	auth.sessions[token] = sessionInfo{
		ExpiresAt: now.Add(sessionIdleLifetime), AbsoluteExpiresAt: now.Add(sessionAbsoluteLifetime), Username: user.Username, DisplayName: user.DisplayName,
		RoleKey: user.RoleKey, Permissions: uniqueStrings(user.Permissions), PasswordChangeRequired: user.PasswordChangeRequired, MFAEnabled: user.MFAEnabled,
	}
	auth.mu.Unlock()
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieNameForRequest(request), Value: token, Path: "/", MaxAge: int(sessionAbsoluteLifetime.Seconds()), Expires: now.Add(sessionAbsoluteLifetime),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: request.TLS != nil,
	})
	return nil
}

func (s *Server) sessionActivity(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := request.Cookie(sessionCookieNameForRequest(request))
	if err != nil || cookie.Value == "" {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	now := time.Now()
	auth := s.authState()
	auth.mu.Lock()
	session, ok := auth.sessions[cookie.Value]
	if ok && (now.After(session.ExpiresAt) || (!session.AbsoluteExpiresAt.IsZero() && now.After(session.AbsoluteExpiresAt))) {
		delete(auth.sessions, cookie.Value)
		ok = false
	}
	if ok {
		if session.AbsoluteExpiresAt.IsZero() {
			session.AbsoluteExpiresAt = now.Add(sessionAbsoluteLifetime)
		}
		nextExpiry := now.Add(sessionIdleLifetime)
		if nextExpiry.After(session.AbsoluteExpiresAt) {
			nextExpiry = session.AbsoluteExpiresAt
		}
		session.ExpiresAt = nextExpiry
		auth.sessions[cookie.Value] = session
	}
	auth.mu.Unlock()
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"idleTimeoutSeconds": int(sessionIdleLifetime.Seconds()),
		"absoluteExpiresAt":  session.AbsoluteExpiresAt.Format(time.RFC3339),
	})
}

func (s *Server) verifyCredentials(username string, password string) (currentUserInfo, bool) {
	username = strings.TrimSpace(username)
	if username == "" {
		return currentUserInfo{}, false
	}
	security, err := s.loadSecurity()
	if err == nil && len(security.Users) > 0 {
		rolePermissions := map[string][]string{}
		for _, role := range defaultedRoles(security.Roles) {
			rolePermissions[role.RoleKey] = uniqueStrings(role.Permissions)
		}
		for _, user := range security.Users {
			if !user.Enabled || user.Username != username {
				continue
			}
			if !verifyPasswordHash(user.PasswordHash, password) {
				return currentUserInfo{}, false
			}
			roleKey := valueOrDefault(user.RoleKey, "viewer")
			return currentUserInfo{
				Username: user.Username, DisplayName: user.DisplayName, RoleKey: roleKey,
				Permissions: rolePermissions[roleKey], PasswordChangeRequired: user.MustChangePassword || verifyPasswordHash(user.PasswordHash, "123456"), MFAEnabled: user.MFAEnabled,
			}, true
		}
		return currentUserInfo{}, false
	}

	auth := s.authState()
	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(auth.username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(auth.password)) == 1
	if usernameOK && passwordOK {
		return currentUserInfo{Username: auth.username, RoleKey: "admin", Permissions: []string{"*"}, PasswordChangeRequired: auth.password == "123456"}, true
	}
	return currentUserInfo{}, false
}

func (s *Server) loadSecurity() (config.SecurityConfig, error) {
	if s.configPath == "" {
		return config.SecurityConfig{}, os.ErrNotExist
	}
	raw, err := os.ReadFile(s.configPath)
	if err != nil {
		return config.SecurityConfig{}, err
	}
	var cfg struct {
		Security config.SecurityConfig `json:"security"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return config.SecurityConfig{}, err
	}
	return cfg.Security, nil
}

func hashPassword(password string) (string, error) {
	raw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(raw), err
}

func validatePassword(username, password string) error {
	length := utf8.RuneCountInString(password)
	if length < 6 {
		return errText("密码至少需要 6 个字符")
	}
	if length > 128 {
		return errText("密码不能超过 128 个字符")
	}
	if strings.TrimSpace(password) == "" {
		return errText("密码不能全部为空格")
	}
	lower := strings.ToLower(password)
	common := map[string]bool{"123456": true, "12345678": true, "123456789": true, "password": true, "password123": true, "admin123": true, "qwerty123": true}
	if common[lower] || strings.Contains(lower, "123456") || strings.Contains(lower, "abcdef") {
		return errText("密码过于简单，请避免常见密码和连续字符")
	}
	trimmedUser := strings.ToLower(strings.TrimSpace(username))
	if utf8.RuneCountInString(trimmedUser) >= 3 && strings.Contains(lower, trimmedUser) {
		return errText("密码不能包含用户账号")
	}
	return nil
}

func verifyPasswordHash(hash string, password string) bool {
	if strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") || strings.HasPrefix(hash, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	}
	// Keep legacy hashes readable during rolling upgrades. They are replaced by
	// bcrypt the next time an administrator changes the password.
	parts := strings.Split(hash, ":")
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	sum := sha256.Sum256([]byte(parts[1] + ":" + password))
	expected := parts[2]
	actual := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func (s *Server) allowLogin(request *http.Request, username string) (time.Time, bool) {
	auth := s.authState()
	key := loginAttemptKey(request, username)
	now := time.Now()
	auth.mu.Lock()
	defer auth.mu.Unlock()
	attempt := auth.attempts[key]
	if attempt.BlockedUntil.After(now) {
		return attempt.BlockedUntil, false
	}
	attempt.Failures = recentFailures(attempt.Failures, now)
	auth.attempts[key] = attempt
	return time.Time{}, true
}

func (s *Server) recordLoginResult(request *http.Request, username string, success bool) {
	auth := s.authState()
	key := loginAttemptKey(request, username)
	now := time.Now()
	auth.mu.Lock()
	defer auth.mu.Unlock()
	if success {
		delete(auth.attempts, key)
		return
	}
	attempt := auth.attempts[key]
	attempt.Failures = append(recentFailures(attempt.Failures, now), now)
	if len(attempt.Failures) >= maxLoginFailures {
		attempt.BlockedUntil = now.Add(loginBlockTime)
		attempt.Failures = nil
	}
	auth.attempts[key] = attempt
}

func recentFailures(items []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-loginWindow)
	kept := items[:0]
	for _, item := range items {
		if item.After(cutoff) {
			kept = append(kept, item)
		}
	}
	return kept
}

func loginAttemptKey(request *http.Request, username string) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	return strings.ToLower(strings.TrimSpace(username)) + "@" + host
}

func (s *Server) auditLogin(request *http.Request, username string, success bool, reason string) {
	result := "success"
	if !success {
		result = "failed"
	}
	message := "login " + result + " user=" + strings.TrimSpace(username) + " remote=" + request.RemoteAddr
	if reason != "" {
		message += " reason=" + reason
	}
	log.Print(message)
	if s.data != nil {
		level := "INFO"
		if !success {
			level = "WARN"
		}
		s.data.PublishLog(level, message)
	}
	status := http.StatusOK
	if !success {
		status = http.StatusUnauthorized
	}
	s.appendAudit(auditEvent{Username: strings.TrimSpace(username), RemoteIP: requestRemoteIP(request), Method: request.Method, Path: "/login", Status: status, Result: result, Detail: reason})
}

func (s *Server) logout(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := request.Cookie(sessionCookieNameForRequest(request)); err == nil {
		auth := s.authState()
		auth.mu.Lock()
		delete(auth.sessions, cookie.Value)
		auth.mu.Unlock()
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieNameForRequest(request), Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: request.TLS != nil})
	if s.store != nil && s.store.CompactSnapshot().License.Restricted {
		http.Redirect(writer, request, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(writer, request, "/login", http.StatusSeeOther)
}

func (s *Server) changePassword(writer http.ResponseWriter, request *http.Request) {
	user, ok := s.currentUser(request)
	if !ok {
		http.Redirect(writer, request, "/login?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
		return
	}
	next := safeNext(request.URL.Query().Get("next"))
	if request.Method == http.MethodGet {
		s.renderChangePassword(writer, user, next, "", http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	if err := request.ParseForm(); err != nil {
		s.renderChangePassword(writer, user, next, "请求格式错误", http.StatusBadRequest)
		return
	}
	next = safeNext(request.FormValue("next"))
	currentPassword := request.FormValue("current_password")
	newPassword := request.FormValue("new_password")
	if newPassword != request.FormValue("confirm_password") {
		s.renderChangePassword(writer, user, next, "两次输入的新密码不一致", http.StatusBadRequest)
		return
	}
	updated, err := s.updateOwnPassword(user.Username, currentPassword, newPassword)
	if err != nil {
		s.appendAudit(auditEvent{Username: user.Username, RemoteIP: requestRemoteIP(request), Method: request.Method, Path: "/change-password", Status: http.StatusBadRequest, Result: "failed", Detail: err.Error()})
		s.renderChangePassword(writer, user, next, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.issueSession(writer, request, updated, true); err != nil {
		http.Error(writer, "create session failed", http.StatusInternalServerError)
		return
	}
	s.appendAudit(auditEvent{Username: user.Username, RemoteIP: requestRemoteIP(request), Method: request.Method, Path: "/change-password", Status: http.StatusOK, Result: "success", Detail: "password changed and other sessions revoked"})
	http.Redirect(writer, request, next, http.StatusSeeOther)
}

func (s *Server) skipPasswordChange(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := s.currentUser(request)
	if !ok {
		http.Redirect(writer, request, "/login", http.StatusSeeOther)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "请求格式错误", http.StatusBadRequest)
		return
	}
	cookie, err := request.Cookie(sessionCookieNameForRequest(request))
	if err != nil || cookie.Value == "" {
		http.Redirect(writer, request, "/login", http.StatusSeeOther)
		return
	}
	auth := s.authState()
	auth.mu.Lock()
	session, exists := auth.sessions[cookie.Value]
	active := exists && time.Now().Before(session.ExpiresAt)
	if active {
		session.PasswordChangeRequired = false
		auth.sessions[cookie.Value] = session
	} else if exists {
		delete(auth.sessions, cookie.Value)
	}
	auth.mu.Unlock()
	if !active {
		http.Redirect(writer, request, "/login", http.StatusSeeOther)
		return
	}
	s.appendAudit(auditEvent{Username: user.Username, RemoteIP: requestRemoteIP(request), Method: request.Method, Path: "/change-password/skip", Status: http.StatusOK, Result: "success", Detail: "password change postponed for current session"})
	http.Redirect(writer, request, safeNext(request.FormValue("next")), http.StatusSeeOther)
}

func (s *Server) updateOwnPassword(username, currentPassword, newPassword string) (currentUserInfo, error) {
	if newPassword == currentPassword {
		return currentUserInfo{}, errText("新密码不能与当前密码相同")
	}
	if err := validatePassword(username, newPassword); err != nil {
		return currentUserInfo{}, err
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return currentUserInfo{}, errText("无法读取安全配置")
	}
	roles := defaultedRoles(cfg.Security.Roles)
	if len(cfg.Security.Users) == 0 {
		auth := s.authState()
		if username != auth.username || subtle.ConstantTimeCompare([]byte(currentPassword), []byte(auth.password)) != 1 {
			return currentUserInfo{}, errText("当前密码错误")
		}
		hash, hashErr := hashPassword(newPassword)
		if hashErr != nil {
			return currentUserInfo{}, hashErr
		}
		cfg.Security.Users = []config.UserConfig{{Username: auth.username, DisplayName: "系统管理员", PasswordHash: hash, RoleKey: "admin", Enabled: true}}
		cfg.Security.Roles = roles
		if err := config.Save(s.configPath, cfg); err != nil {
			return currentUserInfo{}, err
		}
		return currentUserInfo{Username: auth.username, DisplayName: "系统管理员", RoleKey: "admin", Permissions: []string{"*"}}, nil
	}
	rolePermissions := map[string][]string{}
	for _, role := range roles {
		rolePermissions[role.RoleKey] = uniqueStrings(role.Permissions)
	}
	for index := range cfg.Security.Users {
		account := &cfg.Security.Users[index]
		if account.Username != username || !account.Enabled || !verifyPasswordHash(account.PasswordHash, currentPassword) {
			continue
		}
		hash, hashErr := hashPassword(newPassword)
		if hashErr != nil {
			return currentUserInfo{}, hashErr
		}
		account.PasswordHash = hash
		account.MustChangePassword = false
		if err := config.Save(s.configPath, cfg); err != nil {
			return currentUserInfo{}, err
		}
		roleKey := valueOrDefault(account.RoleKey, "viewer")
		return currentUserInfo{Username: account.Username, DisplayName: account.DisplayName, RoleKey: roleKey, Permissions: rolePermissions[roleKey], MFAEnabled: account.MFAEnabled}, nil
	}
	return currentUserInfo{}, errText("当前密码错误")
}

func (s *Server) renderChangePassword(writer http.ResponseWriter, user currentUserInfo, next, message string, status int) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = changePasswordTemplate.Execute(writer, struct {
		User     currentUserInfo
		Next     string
		Message  string
		Required bool
	}{User: user, Next: safeNext(next), Message: message, Required: user.PasswordChangeRequired})
}

func (s *Server) renderLogin(writer http.ResponseWriter, next, message string, status int) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	_ = loginTemplate.Execute(writer, struct {
		Next    string
		Message string
	}{Next: safeNext(next), Message: message})
}

func safeNext(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/"
	}
	return value
}

var loginTemplate = template.Must(template.New("login").Parse(loginHTML))
var changePasswordTemplate = template.Must(template.New("change-password").Parse(changePasswordHTML))

const loginHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="icon" type="image/png" href="/brand-logo.png?v=20260922" />
  <link rel="apple-touch-icon" href="/brand-logo.png?v=20260922" />
  <title>微控网关 - 登录</title>
  <style>
    * { box-sizing: border-box; }
    body { display:grid; min-height:100vh; margin:0; padding:24px; place-items:center; color:#0f172a; background:radial-gradient(circle at 20% 10%,#dbeafe 0,transparent 32%),linear-gradient(135deg,#f8fbff,#eef4fb); font-family:Inter,"Microsoft YaHei",sans-serif; }
    .login-card { width:min(100%,420px); padding:38px; border:1px solid #e2e8f0; border-radius:22px; background:rgba(255,255,255,.94); box-shadow:0 24px 70px rgba(15,23,42,.12); }
    .brand { display:flex; gap:12px; align-items:center; margin-bottom:30px; }
    .mark { position:relative; width:42px; height:42px; border-radius:13px; background:linear-gradient(135deg,#1677ff,#52c41a); box-shadow:0 10px 24px rgba(22,119,255,.22); }
    .mark::after { content:""; position:absolute; inset:11px; border:2px solid rgba(255,255,255,.88); border-radius:6px; }
    h1 { margin:0; font-size:21px; }
    .subtitle { margin:4px 0 0; color:#94a3b8; font-size:12px; }
    form { display:grid; gap:18px; }
    label { display:grid; gap:7px; color:#475569; font-size:13px; font-weight:600; }
    input { width:100%; height:44px; padding:0 13px; border:1px solid #dbe3ee; border-radius:10px; outline:none; font:inherit; }
    input:focus { border-color:#1677ff; box-shadow:0 0 0 3px rgba(22,119,255,.1); }
    button { height:44px; border:0; border-radius:10px; color:#fff; background:#1677ff; font:inherit; font-weight:700; cursor:pointer; }
    button:hover { background:#0958d9; }
    .error { margin:-4px 0 0; color:#dc2626; font-size:13px; }
    .hint { margin:20px 0 0; color:#94a3b8; text-align:center; font-size:12px; }
  </style>
</head>
<body>
  <main class="login-card">
    <div class="brand"><span class="mark"></span><div><h1>微控网关</h1><p class="subtitle">Edge Gateway Management</p></div></div>
    <form method="post" action="/login" autocomplete="off">
      <input type="hidden" name="next" value="{{.Next}}" />
      <label>账号<input name="gateway_user" autocomplete="off" autocapitalize="none" spellcheck="false" autofocus required /></label>
      <label>密码<input name="gateway_pass" type="password" autocomplete="new-password" required /></label>
      {{if .Message}}<p class="error">{{.Message}}</p>{{end}}
      <button type="submit">登录</button>
    </form>
    <p class="hint">请输入网关管理员账号</p>
  </main>
</body>
</html>`

const changePasswordHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="icon" type="image/png" href="/brand-logo.png?v=20260922" />
  <link rel="apple-touch-icon" href="/brand-logo.png?v=20260922" />
  <title>微控网关 - 修改密码</title>
  <style>
    * { box-sizing:border-box; }
    body { display:grid; min-height:100vh; margin:0; padding:24px; place-items:center; color:#0f172a; background:radial-gradient(circle at 20% 10%,#dbeafe 0,transparent 32%),linear-gradient(135deg,#f8fbff,#eef4fb); font-family:Inter,"Microsoft YaHei",sans-serif; }
    .card { width:min(100%,460px); padding:34px 38px; border:1px solid #e2e8f0; border-radius:22px; background:rgba(255,255,255,.96); box-shadow:0 24px 70px rgba(15,23,42,.12); }
    h1 { margin:0 0 6px; font-size:22px; } .subtitle,.hint { color:#64748b; font-size:13px; line-height:1.7; } .subtitle { margin:0 0 22px; }
    .required { margin:0 0 20px; padding:10px 12px; color:#9a3412; background:#fff7ed; border:1px solid #fed7aa; border-radius:9px; font-size:13px; }
    form { display:grid; gap:16px; } label { display:grid; gap:7px; color:#475569; font-size:13px; font-weight:600; } b { color:#dc2626; }
    input { width:100%; height:44px; padding:0 13px; border:1px solid #dbe3ee; border-radius:10px; outline:none; font:inherit; }
    input:focus { border-color:#1677ff; box-shadow:0 0 0 3px rgba(22,119,255,.1); }
    button { min-width:112px; height:40px; padding:0 17px; border:1px solid #1677ff; border-radius:9px; color:#fff; background:#1677ff; font:inherit; font-weight:600; cursor:pointer; transition:border-color .16s,background .16s,box-shadow .16s; }
    button:hover { border-color:#0958d9; background:#0958d9; box-shadow:0 4px 12px rgba(22,119,255,.16); }
    .actions { display:flex; justify-content:flex-end; align-items:center; gap:10px; padding-top:2px; }
    .secondary { border-color:#d5dbe5; color:#64748b; background:#eef1f5; box-shadow:none; }
    .secondary:hover { border-color:#c6ced9; color:#475569; background:#e2e7ed; box-shadow:none; }
    .error { margin:0; color:#dc2626; font-size:13px; } .hint { margin:2px 0 0; }
  </style>
</head>
<body><main class="card">
  <h1>修改登录密码</h1>
  <p class="subtitle">当前账号：{{.User.Username}}{{if .User.DisplayName}}（{{.User.DisplayName}}）{{end}}</p>
  {{if .Required}}<p class="required">当前使用的是初始或临时密码，为保障网关安全，修改密码后才能继续使用。</p>{{end}}
  <form method="post" action="/change-password" autocomplete="off">
    <input type="hidden" name="next" value="{{.Next}}" />
    <label>当前密码 <b>*</b><input name="current_password" type="password" autocomplete="current-password" autofocus required /></label>
    <label>新密码 <b>*</b><input name="new_password" type="password" autocomplete="new-password" minlength="6" maxlength="128" required /></label>
    <label>确认新密码 <b>*</b><input name="confirm_password" type="password" autocomplete="new-password" minlength="6" maxlength="128" required /></label>
    <p class="hint">至少 6 位，可使用纯数字、字母或中文，无需组合；不能包含账号或使用常见弱密码。</p>
    {{if .Message}}<p class="error" role="alert">{{.Message}}</p>{{end}}
    <div class="actions">
      <button class="secondary" type="submit" formaction="/change-password/skip" formnovalidate>暂时跳过</button>
      <button type="submit">保存新密码</button>
    </div>
  </form>
</main></body></html>`

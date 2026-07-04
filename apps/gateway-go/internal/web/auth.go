package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const (
	sessionCookieName = "weikong_gateway_session"
	sessionLifetime   = 24 * time.Hour
)

type authManager struct {
	mu       sync.Mutex
	sessions map[string]sessionInfo
	username string
	password string
}

type sessionInfo struct {
	ExpiresAt   time.Time
	Username    string
	DisplayName string
	RoleKey     string
	Permissions []string
}

type currentUserInfo struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName,omitempty"`
	RoleKey     string   `json:"roleKey"`
	Permissions []string `json:"permissions"`
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
	return &authManager{sessions: make(map[string]sessionInfo), username: username, password: password}
}

func (s *Server) authState() *authManager {
	if s.auth == nil {
		s.auth = newAuthManager()
	}
	return s.auth
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if s.authenticated(request) {
			next.ServeHTTP(writer, request)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/api/") {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(writer, request, "/login?next="+url.QueryEscape(request.URL.RequestURI()), http.StatusSeeOther)
	})
}

func (s *Server) authenticated(request *http.Request) bool {
	_, ok := s.currentUser(request)
	return ok
}

func (s *Server) currentUser(request *http.Request) (currentUserInfo, bool) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return currentUserInfo{}, false
	}
	auth := s.authState()
	auth.mu.Lock()
	defer auth.mu.Unlock()
	session, ok := auth.sessions[cookie.Value]
	if !ok || time.Now().After(session.ExpiresAt) {
		delete(auth.sessions, cookie.Value)
		return currentUserInfo{}, false
	}
	return currentUserInfo{
		Username:    session.Username,
		DisplayName: session.DisplayName,
		RoleKey:     session.RoleKey,
		Permissions: session.Permissions,
	}, true
}

func (s *Server) hasPermission(request *http.Request, permission string) bool {
	user, ok := s.currentUser(request)
	if !ok {
		return false
	}
	for _, item := range user.Permissions {
		if item == "*" || item == permission {
			return true
		}
	}
	return false
}

func (s *Server) login(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
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
		s.renderLogin(writer, "/", "请求格式错误", http.StatusBadRequest)
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
	loginUser, ok := s.verifyCredentials(username, password)
	if !ok {
		s.renderLogin(writer, request.FormValue("next"), "账号或密码错误", http.StatusUnauthorized)
		return
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(writer, "create session failed", http.StatusInternalServerError)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	auth := s.authState()
	auth.mu.Lock()
	auth.sessions[token] = sessionInfo{
		ExpiresAt:   time.Now().Add(sessionLifetime),
		Username:    loginUser.Username,
		DisplayName: loginUser.DisplayName,
		RoleKey:     loginUser.RoleKey,
		Permissions: uniqueStrings(loginUser.Permissions),
	}
	auth.mu.Unlock()
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/", MaxAge: int(sessionLifetime.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: request.TLS != nil,
	})
	http.Redirect(writer, request, safeNext(request.FormValue("next")), http.StatusSeeOther)
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
				Username:    user.Username,
				DisplayName: user.DisplayName,
				RoleKey:     roleKey,
				Permissions: rolePermissions[roleKey],
			}, true
		}
		return currentUserInfo{}, false
	}

	auth := s.authState()
	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(auth.username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(auth.password)) == 1
	if usernameOK && passwordOK {
		return currentUserInfo{Username: auth.username, RoleKey: "admin", Permissions: []string{"*"}}, true
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
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", err
	}
	salt := hex.EncodeToString(saltBytes)
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return "sha256:" + salt + ":" + hex.EncodeToString(sum[:]), nil
}

func verifyPasswordHash(hash string, password string) bool {
	parts := strings.Split(hash, ":")
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	sum := sha256.Sum256([]byte(parts[1] + ":" + password))
	expected := parts[2]
	actual := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func (s *Server) logout(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := request.Cookie(sessionCookieName); err == nil {
		auth := s.authState()
		auth.mu.Lock()
		delete(auth.sessions, cookie.Value)
		auth.mu.Unlock()
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(writer, request, "/login", http.StatusSeeOther)
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

const loginHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
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

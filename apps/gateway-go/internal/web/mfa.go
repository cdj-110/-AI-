package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const (
	mfaChallengeLifetime = 5 * time.Minute
	mfaSetupLifetime     = 10 * time.Minute
	maxMFAFailures       = 5
)

type mfaLoginChallenge struct {
	User      currentUserInfo
	Next      string
	ExpiresAt time.Time
	Failures  int
}

type mfaSetup struct {
	Secret       string
	PasswordHash string
	User         currentUserInfo
	ExpiresAt    time.Time
}

type mfaStatusResponse struct {
	Enabled           bool   `json:"enabled"`
	Username          string `json:"username"`
	RecoveryRemaining int    `json:"recoveryRemaining"`
}

func randomToken(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func generateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func totpCode(secret string, counter int64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", err
	}
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1000000), nil
}

func verifyTOTP(secret, code string, now time.Time, lastCounter int64) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return 0, false
	}
	base := now.Unix() / 30
	for _, candidate := range []int64{base, base - 1, base + 1} {
		if candidate <= lastCounter {
			continue
		}
		expected, err := totpCode(secret, candidate)
		if err == nil && subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return candidate, true
		}
	}
	return 0, false
}

func normalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}

func recoveryCodeHash(code string) string {
	sum := sha256.Sum256([]byte(normalizeRecoveryCode(code)))
	return hex.EncodeToString(sum[:])
}

func generateRecoveryCodes() ([]string, []string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codes := make([]string, 8)
	hashes := make([]string, 8)
	for i := range codes {
		raw := make([]byte, 10)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		for j := range raw {
			raw[j] = alphabet[int(raw[j])%len(alphabet)]
		}
		codes[i] = string(raw[:5]) + "-" + string(raw[5:])
		hashes[i] = recoveryCodeHash(codes[i])
	}
	return codes, hashes, nil
}

func (s *Server) beginMFALogin(writer http.ResponseWriter, request *http.Request, user currentUserInfo, next string) error {
	token, err := randomToken(32)
	if err != nil {
		return err
	}
	now := time.Now()
	auth := s.authState()
	auth.mu.Lock()
	for key, challenge := range auth.mfaChallenges {
		if now.After(challenge.ExpiresAt) {
			delete(auth.mfaChallenges, key)
		}
	}
	auth.mfaChallenges[token] = mfaLoginChallenge{User: user, Next: safeNext(next), ExpiresAt: now.Add(mfaChallengeLifetime)}
	auth.mu.Unlock()
	http.SetCookie(writer, &http.Cookie{Name: mfaCookieNameForRequest(request), Value: token, Path: "/login/verify", MaxAge: int(mfaChallengeLifetime.Seconds()), Expires: now.Add(mfaChallengeLifetime), HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: request.TLS != nil})
	return nil
}

func (s *Server) loginMFA(writer http.ResponseWriter, request *http.Request) {
	cookie, err := request.Cookie(mfaCookieNameForRequest(request))
	if err != nil || cookie.Value == "" {
		if strings.Contains(request.Header.Get("Accept"), "application/json") {
			s.renderLoginResponse(writer, request, "/", "验证已失效，请重新验证管理员身份", http.StatusUnauthorized)
		} else {
			http.Redirect(writer, request, "/login", http.StatusSeeOther)
		}
		return
	}
	auth := s.authState()
	auth.mu.Lock()
	challenge, ok := auth.mfaChallenges[cookie.Value]
	if !ok || time.Now().After(challenge.ExpiresAt) {
		delete(auth.mfaChallenges, cookie.Value)
		ok = false
	}
	auth.mu.Unlock()
	if !ok {
		s.clearMFACookie(writer, request)
		if strings.Contains(request.Header.Get("Accept"), "application/json") {
			s.renderLoginResponse(writer, request, "/", "验证已失效，请重新验证管理员身份", http.StatusUnauthorized)
		} else {
			http.Redirect(writer, request, "/login", http.StatusSeeOther)
		}
		return
	}
	if request.Method == http.MethodGet {
		s.renderMFALogin(writer, challenge.User, "", http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 8*1024)
	if err := request.ParseForm(); err != nil {
		if strings.Contains(request.Header.Get("Accept"), "application/json") {
			s.renderLoginResponse(writer, request, "/", "请求格式错误", http.StatusBadRequest)
		} else {
			s.renderMFALogin(writer, challenge.User, "请求格式错误", http.StatusBadRequest)
		}
		return
	}
	verified, verifyErr := s.consumeMFA(challenge.User.Username, request.FormValue("code"))
	if verifyErr != nil {
		http.Error(writer, "verification unavailable", http.StatusInternalServerError)
		return
	}
	if !verified {
		auth.mu.Lock()
		current, exists := auth.mfaChallenges[cookie.Value]
		if exists {
			current.Failures++
			if current.Failures >= maxMFAFailures {
				delete(auth.mfaChallenges, cookie.Value)
			} else {
				auth.mfaChallenges[cookie.Value] = current
			}
		}
		auth.mu.Unlock()
		s.auditLogin(request, challenge.User.Username, false, "invalid second factor")
		if exists && current.Failures < maxMFAFailures {
			if strings.Contains(request.Header.Get("Accept"), "application/json") {
				s.renderLoginResponse(writer, request, "/", "动态验证码或恢复码错误，请重试", http.StatusUnauthorized)
			} else {
				s.renderMFALogin(writer, challenge.User, "动态验证码或恢复码错误，请重试", http.StatusUnauthorized)
			}
			return
		}
		s.clearMFACookie(writer, request)
		s.renderLoginResponse(writer, request, challenge.Next, "二次验证失败次数过多，请重新登录", http.StatusTooManyRequests)
		return
	}
	auth.mu.Lock()
	delete(auth.mfaChallenges, cookie.Value)
	auth.mu.Unlock()
	s.clearMFACookie(writer, request)
	if err := s.issueSession(writer, request, challenge.User, false); err != nil {
		http.Error(writer, "create session failed", http.StatusInternalServerError)
		return
	}
	s.auditLogin(request, challenge.User.Username, true, "mfa verified")
	next := challenge.Next
	if challenge.User.PasswordChangeRequired {
		next = "/change-password?next=" + url.QueryEscape(next)
	} else if isPasswordChangePath(next) {
		next = "/"
	}
	authenticationResponse(writer, request, next)
}

func (s *Server) clearMFACookie(writer http.ResponseWriter, request *http.Request) {
	http.SetCookie(writer, &http.Cookie{Name: mfaCookieNameForRequest(request), Path: "/login/verify", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: request.TLS != nil})
}

func (s *Server) consumeMFA(username, code string) (bool, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return false, err
	}
	for index := range cfg.Security.Users {
		user := &cfg.Security.Users[index]
		if user.Username != username || !user.Enabled || !user.MFAEnabled || user.TOTPSecret == "" {
			continue
		}
		if counter, ok := verifyTOTP(user.TOTPSecret, code, time.Now(), user.TOTPLastCounter); ok {
			user.TOTPLastCounter = counter
			return true, config.Save(s.configPath, cfg)
		}
		providedHash := recoveryCodeHash(code)
		for i, storedHash := range user.RecoveryCodeHashes {
			if subtle.ConstantTimeCompare([]byte(providedHash), []byte(storedHash)) == 1 {
				user.RecoveryCodeHashes = append(user.RecoveryCodeHashes[:i], user.RecoveryCodeHashes[i+1:]...)
				return true, config.Save(s.configPath, cfg)
			}
		}
		return false, nil
	}
	return false, nil
}

func (s *Server) mfaStatus(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	current, ok := s.currentUser(request)
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	security, err := s.loadSecurity()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	response := mfaStatusResponse{Username: current.Username}
	for _, user := range security.Users {
		if user.Username == current.Username {
			response.Enabled = user.MFAEnabled
			response.RecoveryRemaining = len(user.RecoveryCodeHashes)
			break
		}
	}
	writeJSON(writer, response)
}

func (s *Server) mfaSetupBegin(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	current, ok := s.currentUser(request)
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8*1024)).Decode(&body); err != nil {
		http.Error(writer, "请求格式错误", http.StatusBadRequest)
		return
	}
	verified, ok := s.verifyCredentials(current.Username, body.Password)
	if !ok || verified.Username != current.Username {
		http.Error(writer, "当前密码错误", http.StatusUnauthorized)
		return
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		http.Error(writer, "生成身份验证密钥失败", http.StatusInternalServerError)
		return
	}
	passwordHash, err := hashPassword(body.Password)
	if err != nil {
		http.Error(writer, "准备账号安全配置失败", http.StatusInternalServerError)
		return
	}
	issuer := "Weikong Edge Gateway"
	label := issuer + ":" + current.Username
	otpauth := "otpauth://totp/" + url.PathEscape(label) + "?secret=" + url.QueryEscape(secret) + "&issuer=" + url.QueryEscape(issuer) + "&algorithm=SHA1&digits=6&period=30"
	png, err := qrcode.Encode(otpauth, qrcode.Medium, 256)
	if err != nil {
		http.Error(writer, "生成二维码失败", http.StatusInternalServerError)
		return
	}
	auth := s.authState()
	auth.mu.Lock()
	auth.mfaSetups[current.Username] = mfaSetup{Secret: secret, PasswordHash: passwordHash, User: current, ExpiresAt: time.Now().Add(mfaSetupLifetime)}
	auth.mu.Unlock()
	writeJSON(writer, map[string]interface{}{"secret": secret, "otpauthUri": otpauth, "qrCodeDataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), "expiresIn": int(mfaSetupLifetime.Seconds())})
}

func (s *Server) mfaEnable(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	current, ok := s.currentUser(request)
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8*1024)).Decode(&body); err != nil {
		http.Error(writer, "请求格式错误", http.StatusBadRequest)
		return
	}
	auth := s.authState()
	auth.mu.Lock()
	setup, exists := auth.mfaSetups[current.Username]
	auth.mu.Unlock()
	if !exists || time.Now().After(setup.ExpiresAt) {
		http.Error(writer, "绑定信息已过期，请重新开始", http.StatusBadRequest)
		return
	}
	counter, valid := verifyTOTP(setup.Secret, body.Code, time.Now(), -1)
	if !valid {
		http.Error(writer, "动态验证码错误，请确认网关和手机时间一致", http.StatusBadRequest)
		return
	}
	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		http.Error(writer, "生成恢复码失败", http.StatusInternalServerError)
		return
	}
	s.configMu.Lock()
	cfg, err := config.Load(s.configPath)
	if err == nil {
		found := false
		for index := range cfg.Security.Users {
			if cfg.Security.Users[index].Username == current.Username {
				cfg.Security.Users[index].MFAEnabled = true
				cfg.Security.Users[index].TOTPSecret = setup.Secret
				cfg.Security.Users[index].RecoveryCodeHashes = hashes
				cfg.Security.Users[index].TOTPLastCounter = counter
				found = true
				break
			}
		}
		if !found {
			cfg.Security.Users = append(cfg.Security.Users, config.UserConfig{
				Username: setup.User.Username, DisplayName: setup.User.DisplayName, PasswordHash: setup.PasswordHash,
				RoleKey: valueOrDefault(setup.User.RoleKey, "admin"), Enabled: true, MFAEnabled: true,
				TOTPSecret: setup.Secret, RecoveryCodeHashes: hashes, TOTPLastCounter: counter,
			})
			cfg.Security.Roles = defaultedRoles(cfg.Security.Roles)
			err = config.Save(s.configPath, cfg)
		} else {
			err = config.Save(s.configPath, cfg)
		}
	}
	s.configMu.Unlock()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	auth.mu.Lock()
	delete(auth.mfaSetups, current.Username)
	for token, session := range auth.sessions {
		if session.Username == current.Username {
			session.MFAEnabled = true
			auth.sessions[token] = session
		}
	}
	auth.mu.Unlock()
	s.appendAudit(auditEvent{Username: current.Username, RemoteIP: requestRemoteIP(request), Method: request.Method, Path: request.URL.Path, Status: http.StatusOK, Result: "success", Detail: "authenticator MFA enabled"})
	writeJSON(writer, map[string]interface{}{"ok": true, "recoveryCodes": codes})
}

func (s *Server) mfaDisable(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	current, ok := s.currentUser(request)
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8*1024)).Decode(&body); err != nil {
		http.Error(writer, "请求格式错误", http.StatusBadRequest)
		return
	}
	verified, ok := s.verifyCredentials(current.Username, body.Password)
	if !ok || verified.Username != current.Username {
		http.Error(writer, "当前密码错误", http.StatusUnauthorized)
		return
	}
	accepted, err := s.consumeMFA(current.Username, body.Code)
	if err != nil || !accepted {
		http.Error(writer, "动态验证码或恢复码错误", http.StatusBadRequest)
		return
	}
	s.configMu.Lock()
	cfg, err := config.Load(s.configPath)
	if err == nil {
		for index := range cfg.Security.Users {
			if cfg.Security.Users[index].Username == current.Username {
				cfg.Security.Users[index].MFAEnabled = false
				cfg.Security.Users[index].TOTPSecret = ""
				cfg.Security.Users[index].RecoveryCodeHashes = nil
				cfg.Security.Users[index].TOTPLastCounter = 0
				break
			}
		}
		err = config.Save(s.configPath, cfg)
	}
	s.configMu.Unlock()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	auth := s.authState()
	auth.mu.Lock()
	for token, session := range auth.sessions {
		if session.Username == current.Username {
			session.MFAEnabled = false
			auth.sessions[token] = session
		}
	}
	auth.mu.Unlock()
	s.appendAudit(auditEvent{Username: current.Username, RemoteIP: requestRemoteIP(request), Method: request.Method, Path: request.URL.Path, Status: http.StatusOK, Result: "success", Detail: "authenticator MFA disabled"})
	writeJSON(writer, map[string]bool{"ok": true})
}

func (s *Server) renderMFALogin(writer http.ResponseWriter, user currentUserInfo, message string, status int) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(mfaLoginHTML, "{{USER}}", templateEscape(user.Username)), "{{MESSAGE}}", templateEscape(message)), "{{ERROR_CLASS}}", map[bool]string{true: "error", false: "error hidden"}[message != ""])))
}

func templateEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&#34;", "'", "&#39;").Replace(value)
}

const mfaLoginHTML = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="icon" type="image/png" href="/brand-logo.png?v=20260922"><link rel="apple-touch-icon" href="/brand-logo.png?v=20260922"><title>微控网关 - 二次验证</title><style>
*{box-sizing:border-box}body{display:grid;min-height:100vh;margin:0;padding:24px;place-items:center;color:#0f172a;background:radial-gradient(circle at 20% 10%,#dbeafe 0,transparent 32%),linear-gradient(135deg,#f8fbff,#eef4fb);font-family:Inter,"Microsoft YaHei",sans-serif}.card{width:min(100%,420px);padding:36px;border:1px solid #e2e8f0;border-radius:22px;background:rgba(255,255,255,.96);box-shadow:0 24px 70px rgba(15,23,42,.12)}.icon{display:grid;width:46px;height:46px;margin-bottom:18px;place-items:center;border-radius:14px;color:#fff;background:#1677ff;font-size:23px}h1{margin:0 0 7px;font-size:21px}.subtitle{margin:0 0 24px;color:#64748b;font-size:13px;line-height:1.6}label{display:grid;gap:7px;color:#475569;font-size:13px;font-weight:600}input{width:100%;height:48px;padding:0 13px;border:1px solid #dbe3ee;border-radius:10px;outline:none;font:700 20px/1 Consolas,monospace;letter-spacing:3px}input:focus{border-color:#1677ff;box-shadow:0 0 0 3px rgba(22,119,255,.1)}button{width:100%;height:44px;margin-top:17px;border:0;border-radius:10px;color:#fff;background:#1677ff;font:inherit;font-weight:700;cursor:pointer}.error{margin:13px 0 0;color:#dc2626;font-size:13px}.hidden{display:none}.back{display:block;margin-top:16px;color:#64748b;text-align:center;text-decoration:none;font-size:12px}
</style></head><body><main class="card"><div class="icon">✓</div><h1>二次登录验证</h1><p class="subtitle">账号 {{USER}} 已启用身份验证器。请输入应用中显示的 6 位动态验证码，也可以使用一条恢复码。</p><form method="post" action="/login/verify" autocomplete="off"><label>动态验证码或恢复码<input name="code" autocomplete="one-time-code" autofocus required></label><p class="{{ERROR_CLASS}}" role="alert">{{MESSAGE}}</p><button type="submit">验证并登录</button></form><a class="back" href="/login">返回账号密码登录</a></main></body></html>`

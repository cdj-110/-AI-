package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type sensitiveActionRequest struct {
	CurrentPassword string `json:"currentPassword"`
	TOTPCode        string `json:"totpCode,omitempty"`
}

var (
	errSensitiveSessionExpired   = errors.New("登录状态已失效，请重新登录")
	errSensitivePasswordRequired = errors.New("请输入当前账号密码")
	errSensitivePasswordInvalid  = errors.New("当前账号密码错误")
	errSensitiveTOTPRequired     = errors.New("请输入身份验证器动态验证码")
	errSensitiveTOTPInvalid      = errors.New("动态验证码错误或已使用，请输入最新验证码")
)

// verifySensitiveAction performs a fresh password check and, when configured,
// consumes a TOTP counter so the same code cannot authorize two operations.
func (s *Server) verifySensitiveAction(request *http.Request, body sensitiveActionRequest) error {
	current, ok := s.currentUser(request)
	if !ok {
		return errSensitiveSessionExpired
	}
	if body.CurrentPassword == "" {
		return errSensitivePasswordRequired
	}
	verified, ok := s.verifyCredentials(current.Username, body.CurrentPassword)
	if !ok || verified.Username != current.Username {
		return errSensitivePasswordInvalid
	}
	if !current.MFAEnabled {
		return nil
	}
	if strings.TrimSpace(body.TOTPCode) == "" {
		return errSensitiveTOTPRequired
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return errors.New("无法读取身份验证配置")
	}
	for index := range cfg.Security.Users {
		user := &cfg.Security.Users[index]
		if user.Username != current.Username || !user.Enabled || !user.MFAEnabled || user.TOTPSecret == "" {
			continue
		}
		counter, accepted := verifyTOTP(user.TOTPSecret, body.TOTPCode, time.Now(), user.TOTPLastCounter)
		if !accepted {
			return errSensitiveTOTPInvalid
		}
		user.TOTPLastCounter = counter
		if err := config.Save(s.configPath, cfg); err != nil {
			return errors.New("无法保存身份验证状态")
		}
		return nil
	}
	return errSensitiveTOTPInvalid
}

func writeSensitiveActionError(writer http.ResponseWriter, err error) {
	status := http.StatusForbidden
	if errors.Is(err, errSensitiveSessionExpired) {
		status = http.StatusUnauthorized
	}
	if err != nil && (strings.Contains(err.Error(), "无法读取") || strings.Contains(err.Error(), "无法保存")) {
		status = http.StatusInternalServerError
	}
	http.Error(writer, err.Error(), status)
}

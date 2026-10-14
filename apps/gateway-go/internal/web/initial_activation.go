package web

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// First activation authorizes installation by possession of a signed,
// device-bound package. All other activation operations remain authenticated.
func (s *Server) initialActivation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "首次激活需要 JSON 授权请求", http.StatusUnsupportedMediaType)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.saveActivationImport(w, r, true)
}

// Caller holds configMu. Fail closed on any existing (even invalid/expired)
// license evidence, symlink, unreadable config, or filesystem lookup error.
func (s *Server) initialActivationAllowed(cfg config.Config) bool {
	if strings.TrimSpace(s.configPath) == "" || s.store == nil {
		return false
	}
	license := s.store.CompactSnapshot().License
	if license.Valid || !license.Restricted {
		return false
	}
	a := cfg.Activation
	if a.Signature != "" || a.DeviceSecret != "" || a.SN != "" || a.HardwareID != "" || a.ChipUSID != "" || a.LicenseSequence != 0 || a.LicenseVersion != 0 || a.KeyID != "" || a.Broker != "" || a.CloudAPI != "" || a.RemoteAccessEndpoint != "" || a.IssuedAt != "" || a.ExpiresAt != "" || a.Nonce != "" || len(a.Features) != 0 {
		return false
	}
	dir := filepath.Dir(s.configPath)
	activationFile := a.File
	if strings.TrimSpace(activationFile) == "" {
		activationFile = "activation.local.json"
	}
	for _, path := range []string{s.activationPath(activationFile), filepath.Join(dir, "activation.local.json"), filepath.Join(dir, ".license-state.json")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return false
		}
	}
	files, err := filepath.Glob(filepath.Join(dir, "activation-*.json"))
	return err == nil && len(files) == 0
}

func (s *Server) initialActivationAvailable() bool {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := s.readRawConfig()
	return err == nil && s.initialActivationAllowed(cfg)
}

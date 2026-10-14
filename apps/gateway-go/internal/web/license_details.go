package web

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

// This authenticated summary deliberately excludes credentials, signatures,
// cloud endpoints, and local filesystem paths. Replacement uses the existing
// cloud.manage-protected activation preflight/import transaction.
func (s *Server) licenseDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	canReplace := s.hasPermission(r, "cloud.manage")
	if !canReplace && !s.hasPermission(r, "maintenance.view") {
		http.Error(w, "当前账号没有设备授权查看权限", http.StatusForbidden)
		return
	}
	if s.store == nil {
		http.Error(w, "设备授权状态暂时不可用", http.StatusServiceUnavailable)
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := s.readRawConfig()
	if err != nil {
		http.Error(w, "设备授权信息读取失败，请重新检查", http.StatusServiceUnavailable)
		return
	}
	activation := cfg.Activation
	if activation.File != "" {
		file, err := os.Open(s.activationPath(activation.File))
		if err != nil {
			http.Error(w, "授权文件暂时无法读取，请联系管理员检查", http.StatusServiceUnavailable)
			return
		}
		defer file.Close()
		if err := json.NewDecoder(io.LimitReader(file, 1024*1024)).Decode(&activation); err != nil {
			http.Error(w, "授权文件格式无效，请联系管理员检查", http.StatusServiceUnavailable)
			return
		}
	}
	snapshot := s.store.CompactSnapshot()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"license": snapshot.License, "gatewayKey": snapshot.GatewayKey, "canReplace": canReplace,
		"authorization": map[string]interface{}{
			"sn": activation.SN, "hardwareId": activation.HardwareID, "chipUsid": activation.ChipUSID,
			"licenseVersion": activation.LicenseVersion, "keyId": activation.KeyID,
			"licenseSequence": activation.LicenseSequence, "issuedAt": activation.IssuedAt,
			"expiresAt": activation.ExpiresAt, "features": activation.Features, "signed": activation.Signature != "",
		},
	})
}

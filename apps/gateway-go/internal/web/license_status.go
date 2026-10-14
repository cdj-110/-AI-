package web

import (
	"encoding/json"
	"net/http"
)

// Only restricted gateways expose activation identity without a session.
func (s *Server) licenseEntryStatus(w http.ResponseWriter, r *http.Request) {
	if s.store == nil || !s.store.CompactSnapshot().License.Restricted {
		s.requireAuth(http.HandlerFunc(s.licenseStatus)).ServeHTTP(w, r)
		return
	}
	s.licenseStatus(w, r)
}

func (s *Server) licenseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	if s.store == nil {
		http.Error(w, "authorization status unavailable", 503)
		return
	}
	snapshot := s.store.CompactSnapshot()
	user, authenticated := s.currentUser(r)
	authenticated = authenticated && !user.PasswordChangeRequired
	license := snapshot.License
	chip := snapshot.SecureChip
	if !authenticated && license.Restricted {
		// Internal filesystem paths and validation details are not public data.
		license.Reason = "尚未导入有效设备授权，请联系管理员获取授权文件。"
		if !chip.Available {
			chip.Message = "芯片身份不可用，请联系管理员检查设备。"
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"license": license, "gatewayKey": snapshot.GatewayKey, "authenticated": authenticated,
		"initialActivationAllowed": s.initialActivationAvailable(),
		"hardwareIdentity":         snapshot.HardwareID, "secureChipIdentity": chip,
	})
}

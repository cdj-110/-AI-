package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
)

func (s *Server) saveEdgeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !s.hasPermission(r, "edge.manage") {
		http.Error(w, "forbidden", 403)
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	revision, err := configFileRevision(s.configPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if expected := strings.Trim(r.Header.Get("If-Match"), "\""); expected != "" && expected != revision {
		http.Error(w, "配置已被其他会话修改，请重新加载后再保存", 409)
		return
	}
	var body struct {
		EdgeComputing *config.EdgeComputingConfig `json:"edgeComputing"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024*1024))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.EdgeComputing == nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "需要有效的 edgeComputing 配置对象", http.StatusBadRequest)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	previous := cfg
	cfg.EdgeComputing = *body.EdgeComputing
	cfg.ApplyDefaults()
	if err = cfg.ValidateNewGlobalIdentifierDuplicates(previous); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err = edgecompute.Validate(cfg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err = s.saveAndApplyConfigLocked(cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	revision, _ = configFileRevision(s.configPath)
	w.Header().Set("ETag", quoteETag(revision))
	writeJSON(w, map[string]bool{"ok": true})
}

package web

import (
	"encoding/json"
	"net/http"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func (s *Server) alarmSettings(w http.ResponseWriter, r *http.Request) {
	permission := "alarms.view"
	if r.Method == http.MethodPut {
		permission = "alarms.manage"
	} else if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.hasPermission(r, permission) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPut {
		var body struct {
			PopupDurationSeconds *int `json:"popupDurationSeconds"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil || body.PopupDurationSeconds == nil {
			http.Error(w, "请填写弹窗显示时长（整数秒）", http.StatusBadRequest)
			return
		}
		cfg.Alarms.PopupDurationSeconds = body.PopupDurationSeconds
		if err := cfg.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.saveAndApplyConfig(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"popupDurationSeconds": cfg.Alarms.PopupDuration()})
}

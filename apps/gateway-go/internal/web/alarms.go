package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func (s *Server) alarmEvents(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.alarms == nil {
		http.Error(writer, "告警服务不可用", http.StatusServiceUnavailable)
		return
	}
	if request.URL.Query().Get("activeOnly") == "1" {
		writeJSON(writer, s.alarms.ActiveSnapshot())
		return
	}
	writeJSON(writer, s.alarms.Snapshot())
}

func (s *Server) alarmHistory(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.alarms == nil {
		http.Error(writer, "告警服务不可用", http.StatusServiceUnavailable)
		return
	}
	query := alarm.HistoryQuery{
		Page:      alarmQueryInt(request, "page", 1),
		PageSize:  alarmQueryInt(request, "pageSize", 50),
		Level:     strings.TrimSpace(request.URL.Query().Get("level")),
		State:     strings.TrimSpace(request.URL.Query().Get("state")),
		DeviceKey: strings.TrimSpace(request.URL.Query().Get("deviceKey")),
		Metric:    strings.TrimSpace(request.URL.Query().Get("metric")),
		Keyword:   strings.TrimSpace(request.URL.Query().Get("keyword")),
	}
	if query.PageSize > 200 {
		http.Error(writer, "pageSize must be between 1 and 200", http.StatusBadRequest)
		return
	}
	var err error
	if value := strings.TrimSpace(request.URL.Query().Get("startedAt")); value != "" {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			http.Error(writer, "startedAt must be RFC3339", http.StatusBadRequest)
			return
		}
		query.StartedAt = &parsed
	}
	if value := strings.TrimSpace(request.URL.Query().Get("finishedAt")); value != "" {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			http.Error(writer, "finishedAt must be RFC3339", http.StatusBadRequest)
			return
		}
		query.FinishedAt = &parsed
	}
	page, err := s.alarms.History(query)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(writer, page)
}

func alarmQueryInt(request *http.Request, name string, fallback int) int {
	value := strings.TrimSpace(request.URL.Query().Get(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func (s *Server) manualRecoverAlarm(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.alarms == nil {
		http.Error(writer, "告警服务不可用", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		EventID string `json:"eventId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024)).Decode(&body); err != nil || strings.TrimSpace(body.EventID) == "" {
		http.Error(writer, "eventId is required", http.StatusBadRequest)
		return
	}
	user, ok := s.currentUser(request)
	if !ok {
		http.Error(writer, "登录已失效", http.StatusUnauthorized)
		return
	}
	event, err := s.alarms.ManualRecover(strings.TrimSpace(body.EventID), user.Username)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, alarm.ErrManualRecoveryDisabled) {
			status = http.StatusForbidden
		}
		if errors.Is(err, alarm.ErrAlarmNotActive) {
			status = http.StatusConflict
		}
		http.Error(writer, err.Error(), status)
		return
	}
	writeJSON(writer, map[string]interface{}{"ok": true, "alarm": event})
}

func (s *Server) acknowledgeAlarm(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.alarms == nil {
		http.Error(writer, "告警服务不可用", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		EventID string `json:"eventId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024)).Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.EventID) == "" {
		http.Error(writer, "eventId is required", http.StatusBadRequest)
		return
	}
	event, err := s.alarms.Acknowledge(strings.TrimSpace(body.EventID))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(writer, map[string]interface{}{"ok": true, "alarm": event})
}

func (s *Server) alarmRules(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		if !s.hasPermission(request, "alarms.view") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		cfg, err := config.Load(s.configPath)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]interface{}{"rules": cfg.Alarms.Rules, "historyLimit": cfg.Alarms.HistoryLimit, "retentionLimit": cfg.Alarms.RetentionLimit})
	case http.MethodPut:
		if !s.hasPermission(request, "alarms.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		var body struct {
			Rules []config.AlarmRuleConfig `json:"rules"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4*1024*1024)).Decode(&body); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		cfg, err := config.Load(s.configPath)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		cfg.Alarms.Rules = body.Rules
		cfg.ApplyDefaults()
		if err := cfg.Validate(); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.saveAndApplyConfig(cfg); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]interface{}{"ok": true, "rules": cfg.Alarms.Rules})
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

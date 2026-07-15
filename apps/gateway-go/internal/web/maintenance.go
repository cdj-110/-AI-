package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const restartDelayEnv = "GATEWAY_RESTART_DELAY_MS"

type pingRequest struct {
	Target string `json:"target"`
	Count  int    `json:"count"`
}

type factoryResetRequest struct {
	AdminPassword string `json:"adminPassword"`
}

func (s *Server) pingDiagnostic(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body pingRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	target := strings.TrimSpace(body.Target)
	if target == "" || strings.ContainsAny(target, " \t\r\n") {
		http.Error(writer, "invalid ping target", http.StatusBadRequest)
		return
	}
	count := body.Count
	if count <= 0 || count > 10 {
		count = 4
	}

	ctx, cancel := context.WithTimeout(request.Context(), time.Duration(count*3+3)*time.Second)
	defer cancel()
	args := pingArgs(target, count)
	output, err := exec.CommandContext(ctx, "ping", args...).CombinedOutput()
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok":     err == nil,
		"target": target,
		"output": string(output),
	})
}

func pingArgs(target string, count int) []string {
	if runtime.GOOS == "windows" {
		return []string{"-n", strconv.Itoa(count), target}
	}
	return []string{"-c", strconv.Itoa(count), "-W", "3", target}
}

func (s *Server) restartService(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := scheduleSelfRestart(); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "message": "gateway service is restarting"})
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
}

func (s *Server) rebootGateway(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if runtime.GOOS == "windows" {
		http.Error(writer, "gateway reboot is disabled on Windows local test builds", http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "message": "gateway is rebooting"})
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = exec.Command("reboot").Start()
	}()
}

func (s *Server) factoryReset(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.hasPermission(request, "config.manage") {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	var body factoryResetRequest
	if request.Body != nil {
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
		if err := decoder.Decode(&body); err != nil && err.Error() != "EOF" {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	}
	user, ok := s.currentUser(request)
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !isSuperAdminUser(user) {
		if strings.TrimSpace(body.AdminPassword) == "" {
			http.Error(writer, "super admin password is required", http.StatusForbidden)
			return
		}
		if !s.verifySuperAdminPassword(body.AdminPassword) {
			http.Error(writer, "super admin password is incorrect", http.StatusForbidden)
			return
		}
	}
	current, err := s.readRawConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	next := factoryDefaultConfig(current)
	if err := config.Save(s.configPath, next); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	s.applyConfig(next)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok":              true,
		"gatewayKey":      next.GatewayKey,
		"restartRequired": false,
	})
}

func factoryDefaultConfig(current config.Config) config.Config {
	disabled := false
	gatewayKey := strings.TrimSpace(current.GatewayKey)
	if gatewayKey == "" {
		gatewayKey = "gateway"
	}
	next := config.Config{
		GatewayKey:             gatewayKey,
		CollectIntervalSeconds: 5,
		CacheFile:              ".runtime/gateway-spool.jsonl",
		OfflineCache: config.OfflineCacheConfig{
			Enabled:   false,
			MaxSizeMB: 16,
		},
		HistoryStorage: config.HistoryStorageConfig{
			Enabled:   false,
			MaxSizeMB: 256,
		},
		Activation: config.ActivationConfig{
			Enabled: &disabled,
		},
		MQTT: config.MQTTConfig{
			Enabled: &disabled,
		},
		WiFi: config.WiFiConfig{
			Enabled: false,
		},
		Cellular: config.CellularConfig{
			Enabled: &disabled,
		},
		ForwardSlave: config.FeatureConfig{
			Enabled: false,
			Listen:  "0.0.0.0:1502",
		},
		Web:      current.Web,
		Security: current.Security,
	}
	if next.Web.Listen == "" {
		next.Web = config.ListenerConfig{Enabled: true, Listen: "0.0.0.0:8088"}
	}
	next.ApplyDefaults()
	next.Devices = nil
	next.Points = nil
	return next
}

func isSuperAdminUser(user currentUserInfo) bool {
	return user.Username == "admin"
}

func (s *Server) verifySuperAdminPassword(password string) bool {
	password = strings.TrimSpace(password)
	if password == "" {
		return false
	}
	security, err := s.loadSecurity()
	if err == nil && len(security.Users) > 0 {
		for _, user := range security.Users {
			if user.Username == "admin" && user.Enabled && verifyPasswordHash(user.PasswordHash, password) {
				return true
			}
		}
		return false
	}
	return s.authState().password == password
}

func scheduleSelfRestart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory failed: %w", err)
	}
	args := append([]string(nil), os.Args[1:]...)
	command := exec.Command(exe, args...)
	command.Dir = cwd
	command.Env = restartEnv(os.Environ())
	if err := command.Start(); err != nil {
		return fmt.Errorf("start replacement process failed: %w", err)
	}
	return command.Process.Release()
}

func restartEnv(env []string) []string {
	filtered := make([]string, 0, len(env)+1)
	prefix := restartDelayEnv + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, restartDelayEnv+"=1200")
}

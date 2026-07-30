package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	chronyConfigPath = "/etc/chrony.conf"
	timezoneFilePath = "/etc/timezone"
	zoneinfoRoot     = "/usr/share/zoneinfo"
)

type timeSyncStatus struct {
	Config              timeSyncConfig `json:"config"`
	SystemTime          time.Time      `json:"systemTime"`
	Synchronized        bool           `json:"synchronized"`
	Supported           bool           `json:"supported"`
	Stratum             int            `json:"stratum,omitempty"`
	Reference           string         `json:"reference,omitempty"`
	LastOffsetSeconds   float64        `json:"lastOffsetSeconds,omitempty"`
	SystemOffsetSeconds float64        `json:"systemOffsetSeconds,omitempty"`
	LeapStatus          string         `json:"leapStatus,omitempty"`
	Message             string         `json:"message,omitempty"`
}

type timeSyncConfig struct {
	Enabled         *bool  `json:"enabled,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	PrimaryServer   string `json:"primaryServer,omitempty"`
	SecondaryServer string `json:"secondaryServer,omitempty"`
}

func (t *timeSyncConfig) applyDefaults() {
	if t.Enabled == nil {
		enabled := true
		t.Enabled = &enabled
	}
	if strings.TrimSpace(t.Timezone) == "" {
		t.Timezone = "Asia/Shanghai"
	}
	if strings.TrimSpace(t.PrimaryServer) == "" {
		t.PrimaryServer = "pool.ntp.org"
	}
}

func (t timeSyncConfig) isEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

type timeSyncActionRequest struct {
	Action          string `json:"action"`
	PrimaryServer   string `json:"primaryServer,omitempty"`
	SecondaryServer string `json:"secondaryServer,omitempty"`
}

func (s *Server) timeSync(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		cfg, err := loadTimeSyncConfig(s.timeSyncConfigPath())
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, readTimeSyncStatus(cfg))
	case http.MethodPut:
		var next timeSyncConfig
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&next); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		next.applyDefaults()
		if err := validateTimeSyncConfig(next); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if err := applyTimeSyncConfig(request.Context(), next); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := saveTimeSyncConfig(s.timeSyncConfigPath(), next); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, readTimeSyncStatus(next))
	case http.MethodPost:
		var body timeSyncActionRequest
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		var err error
		switch strings.TrimSpace(body.Action) {
		case "test":
			err = testNTPServers(request.Context(), body.PrimaryServer, body.SecondaryServer)
		case "sync":
			err = syncSystemClock(request.Context())
		default:
			http.Error(writer, "unsupported time synchronization action", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
			return
		}
		cfg, loadErr := loadTimeSyncConfig(s.timeSyncConfigPath())
		if loadErr != nil {
			http.Error(writer, loadErr.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, readTimeSyncStatus(cfg))
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) timeSyncConfigPath() string {
	return filepath.Join(filepath.Dir(s.configPath), "time-sync.json")
}

func loadTimeSyncConfig(path string) (timeSyncConfig, error) {
	var cfg timeSyncConfig
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return cfg, err
	}
	if err == nil {
		if err := json.Unmarshal(content, &cfg); err != nil {
			return cfg, fmt.Errorf("parse time synchronization configuration: %w", err)
		}
	}
	cfg.applyDefaults()
	return cfg, nil
}

func saveTimeSyncConfig(path string, cfg timeSyncConfig) error {
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return atomicWriteFile(path, append(content, '\n'), 0644)
}

func writeJSON(writer http.ResponseWriter, value interface{}) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(value)
}

func validateTimeSyncConfig(cfg timeSyncConfig) error {
	timezone := strings.TrimSpace(cfg.Timezone)
	if timezone == "" || filepath.IsAbs(timezone) || strings.Contains(timezone, "..") {
		return errors.New("invalid timezone")
	}
	for _, character := range timezone {
		if !(unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("_+-/", character)) {
			return errors.New("invalid timezone")
		}
	}
	if cfg.isEnabled() && strings.TrimSpace(cfg.PrimaryServer) == "" {
		return errors.New("primary NTP server is required when automatic synchronization is enabled")
	}
	for _, server := range []string{cfg.PrimaryServer, cfg.SecondaryServer} {
		if strings.TrimSpace(server) != "" && !validNTPServer(server) {
			return fmt.Errorf("invalid NTP server %q", server)
		}
	}
	return nil
}

func validNTPServer(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 253 || strings.ContainsAny(value, " \t\r\n/:?#[]@") {
		return net.ParseIP(value) != nil
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-') {
				return false
			}
		}
	}
	return true
}

func applyTimeSyncConfig(ctx context.Context, cfg timeSyncConfig) error {
	if runtime.GOOS == "windows" {
		return errors.New("time synchronization is available only on the Linux gateway")
	}
	zonePath := filepath.Join(zoneinfoRoot, filepath.FromSlash(cfg.Timezone))
	if info, err := os.Stat(zonePath); err != nil || info.IsDir() {
		return fmt.Errorf("timezone %q is not installed", cfg.Timezone)
	}
	content, err := os.ReadFile(chronyConfigPath)
	if err != nil {
		return fmt.Errorf("read chrony configuration: %w", err)
	}
	next := renderChronyConfig(string(content), cfg)
	if err := atomicWriteFile(chronyConfigPath, []byte(next), 0644); err != nil {
		return fmt.Errorf("write chrony configuration: %w", err)
	}
	if err := applyTimezone(zonePath, cfg.Timezone); err != nil {
		return err
	}
	restartCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	output, err := exec.CommandContext(restartCtx, "/etc/init.d/S49chronyd", "restart").CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart chrony failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func renderChronyConfig(existing string, cfg timeSyncConfig) string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(existing))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "server ") || strings.HasPrefix(trimmed, "pool ") || strings.HasPrefix(trimmed, "peer ") {
			continue
		}
		if trimmed == "# BEGIN WEIKONG MANAGED NTP" || trimmed == "# END WEIKONG MANAGED NTP" || trimmed == "# NTP synchronization disabled" {
			continue
		}
		lines = append(lines, line)
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	lines = append(lines, "", "# BEGIN WEIKONG MANAGED NTP")
	if cfg.isEnabled() {
		for _, server := range []string{cfg.PrimaryServer, cfg.SecondaryServer} {
			if server = strings.TrimSpace(server); server != "" {
				lines = append(lines, "server "+server+" iburst")
			}
		}
	} else {
		lines = append(lines, "# NTP synchronization disabled")
	}
	lines = append(lines, "# END WEIKONG MANAGED NTP", "")
	return strings.Join(lines, "\n")
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if _, err = file.Write(content); err == nil {
		err = file.Chmod(mode)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func applyTimezone(zonePath, timezone string) error {
	tempLink := filepath.Join(filepath.Dir("/etc/localtime"), ".localtime.weikong")
	_ = os.Remove(tempLink)
	if err := os.Symlink(zonePath, tempLink); err != nil {
		return fmt.Errorf("prepare timezone link: %w", err)
	}
	if err := os.Rename(tempLink, "/etc/localtime"); err != nil {
		_ = os.Remove(tempLink)
		return fmt.Errorf("apply timezone: %w", err)
	}
	if err := atomicWriteFile(timezoneFilePath, []byte(timezone+"\n"), 0644); err != nil {
		return fmt.Errorf("write timezone: %w", err)
	}
	return nil
}

func readTimeSyncStatus(cfg timeSyncConfig) timeSyncStatus {
	cfg.applyDefaults()
	status := timeSyncStatus{Config: cfg, SystemTime: time.Now(), Supported: runtime.GOOS != "windows"}
	if !status.Supported {
		status.Message = "当前系统不支持网关时间同步"
		return status
	}
	output, err := exec.Command("chronyc", "-c", "tracking").Output()
	if err != nil {
		status.Message = "暂时无法读取时间同步服务状态"
		return status
	}
	fields := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(fields) < 14 {
		status.Message = "时间同步服务返回了无法识别的状态"
		return status
	}
	status.Reference = fields[1]
	status.Stratum, _ = strconv.Atoi(fields[2])
	status.SystemOffsetSeconds, _ = strconv.ParseFloat(fields[4], 64)
	status.LastOffsetSeconds, _ = strconv.ParseFloat(fields[5], 64)
	status.LeapStatus = fields[13]
	status.Synchronized = fields[0] != "00000000" && status.Stratum > 0 && !strings.EqualFold(status.LeapStatus, "Not synchronised")
	if status.Synchronized {
		status.Message = "NTP 时间已同步"
	} else if cfg.isEnabled() {
		status.Message = "NTP 已启用，但当前没有可用的时间源"
	} else {
		status.Message = "自动时间同步已停用"
	}
	return status
}

func testNTPServers(ctx context.Context, primary, secondary string) error {
	var servers []string
	for _, server := range []string{primary, secondary} {
		if server = strings.TrimSpace(server); server != "" {
			if !validNTPServer(server) {
				return fmt.Errorf("invalid NTP server %q", server)
			}
			servers = append(servers, server)
		}
	}
	if len(servers) == 0 {
		return errors.New("at least one NTP server is required")
	}
	var failures []string
	for _, server := range servers {
		if net.ParseIP(server) == nil {
			resolveCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			_, resolveErr := net.DefaultResolver.LookupHost(resolveCtx, server)
			cancel()
			if resolveErr != nil {
				failures = append(failures, fmt.Sprintf("无法解析 NTP 服务器 %s，请检查网关 DNS 配置，或填写可访问的局域网 NTP 服务器 IP", server))
				continue
			}
		}
		testCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
		output, err := exec.CommandContext(testCtx, "chronyd", "-Q", "-t", "5", "server "+server+" iburst maxsamples 1").CombinedOutput()
		cancel()
		if err == nil {
			return nil
		}
		failures = append(failures, friendlyNTPProbeError(server, err, string(output)))
	}
	return errors.New(strings.Join(failures, "；"))
}

func friendlyNTPProbeError(server string, err error, output string) string {
	message := strings.ToLower(output)
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "timeout reached"):
		return fmt.Sprintf("NTP 服务器 %s 未响应，请检查网络连通性以及 UDP 123 端口", server)
	case strings.Contains(message, "could not resolve") || strings.Contains(message, "name or service not known") || strings.Contains(message, "unknown address"):
		return fmt.Sprintf("无法解析 NTP 服务器 %s，请检查网关 DNS 配置，或填写可访问的局域网 NTP 服务器 IP", server)
	default:
		return fmt.Sprintf("无法连接 NTP 服务器 %s，请检查服务器地址、网络路由及 UDP 123 端口", server)
	}
}

func syncSystemClock(ctx context.Context) error {
	if runtime.GOOS == "windows" {
		return errors.New("time synchronization is available only on the Linux gateway")
	}
	for _, args := range [][]string{{"online"}, {"burst", "4/4"}} {
		commandCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		output, err := exec.CommandContext(commandCtx, "chronyc", args...).CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("chronyc %s failed: %s", args[0], strings.TrimSpace(string(output)))
		}
	}
	waitCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	output, err := exec.CommandContext(waitCtx, "chronyc", "waitsync", "10", "0.5", "0", "1").CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("no usable NTP response was received: %s", strings.TrimSpace(string(output)))
	}
	stepCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	output, err = exec.CommandContext(stepCtx, "chronyc", "makestep").CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("apply synchronized time failed: %s", strings.TrimSpace(string(output)))
	}
	_ = exec.Command("hwclock", "-w").Run()
	return nil
}

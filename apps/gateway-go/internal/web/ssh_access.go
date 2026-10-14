package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type sshAccessStatus struct {
	Supported  bool   `json:"supported"`
	Enabled    bool   `json:"enabled"`
	Running    bool   `json:"running"`
	Service    string `json:"service,omitempty"`
	InitScript string `json:"initScript,omitempty"`
	Port       int    `json:"port"`
	Message    string `json:"message,omitempty"`
}

var sshInitScriptCandidates = []string{
	"/etc/init.d/S50dropbear",
	"/etc/init.d/S50sshd",
	"/etc/init.d/S40sshd",
	"/etc/init.d/dropbear",
	"/etc/init.d/sshd",
}

func (s *Server) sshAccess(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(writer, readSSHAccessStatus(sshInitScriptCandidates, sshProcessRunning))
	case http.MethodPut:
		var body struct {
			Enabled         bool   `json:"enabled"`
			CurrentPassword string `json:"currentPassword,omitempty"`
			TOTPCode        string `json:"totpCode,omitempty"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		status := readSSHAccessStatus(sshInitScriptCandidates, sshProcessRunning)
		if !status.Supported {
			http.Error(writer, status.Message, http.StatusBadRequest)
			return
		}
		if err := s.verifySensitiveAction(request, sensitiveActionRequest{CurrentPassword: body.CurrentPassword, TOTPCode: body.TOTPCode}); err != nil {
			writeSensitiveActionError(writer, err)
			return
		}
		if err := applySSHAccess(request.Context(), status, body.Enabled, runSSHInitScript); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		result := readSSHAccessStatus(sshInitScriptCandidates, sshProcessRunning)
		s.appendAudit(auditEvent{Username: currentUsername(s, request), RemoteIP: requestRemoteIP(request), Method: request.Method, Path: request.URL.Path, Status: http.StatusOK, Result: "success", Detail: "ssh enabled=" + boolText(body.Enabled)})
		writeJSON(writer, result)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func readSSHAccessStatus(candidates []string, processRunning func(string) bool) sshAccessStatus {
	status := sshAccessStatus{Port: 22}
	if runtime.GOOS == "windows" {
		status.Message = "SSH 服务管理仅支持 Linux 网关"
		return status
	}
	script := findSSHInitScript(candidates)
	if script == "" {
		status.Message = "未检测到 OpenSSH 或 Dropbear 启动脚本"
		return status
	}
	info, err := os.Stat(script)
	if err != nil {
		status.Message = "SSH 启动脚本状态读取失败"
		return status
	}
	process := sshProcessName(script)
	status.Supported = true
	status.InitScript = script
	status.Service = sshServiceName(script)
	status.Enabled = info.Mode().Perm()&0111 != 0
	status.Running = processRunning(process)
	switch {
	case status.Enabled && status.Running:
		status.Message = "SSH 正在运行，并已设置为开机启动"
	case status.Enabled:
		status.Message = "SSH 已设置为开机启动，但当前服务未运行"
	case status.Running:
		status.Message = "SSH 当前正在运行，但未设置为开机启动"
	default:
		status.Message = "SSH 已关闭"
	}
	return status
}

func findSSHInitScript(candidates []string) string {
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	matches, _ := filepath.Glob("/etc/init.d/S*")
	sort.Strings(matches)
	for _, candidate := range matches {
		name := strings.ToLower(filepath.Base(candidate))
		if !strings.Contains(name, "dropbear") && !strings.Contains(name, "sshd") {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func sshServiceName(script string) string {
	if strings.Contains(strings.ToLower(filepath.Base(script)), "dropbear") {
		return "Dropbear SSH"
	}
	return "OpenSSH"
}

func sshProcessName(script string) string {
	if strings.Contains(strings.ToLower(filepath.Base(script)), "dropbear") {
		return "dropbear"
	}
	return "sshd"
}

func sshProcessRunning(name string) bool {
	if output, err := exec.Command("pidof", name).Output(); err == nil && strings.TrimSpace(string(output)) != "" {
		return true
	}
	entries, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, entry := range entries {
		content, err := os.ReadFile(entry)
		if err == nil && strings.TrimSpace(string(content)) == name {
			return true
		}
	}
	return false
}

type sshInitRunner func(context.Context, string, string) (string, error)

func applySSHAccess(ctx context.Context, status sshAccessStatus, enabled bool, runner sshInitRunner) error {
	if !status.Supported || strings.TrimSpace(status.InitScript) == "" {
		return errors.New("SSH service management is not supported")
	}
	info, err := os.Stat(status.InitScript)
	if err != nil {
		return fmt.Errorf("read SSH init script: %w", err)
	}
	originalMode := info.Mode().Perm()
	if enabled {
		if err := os.Chmod(status.InitScript, originalMode|0111); err != nil {
			return fmt.Errorf("enable SSH at boot: %w", err)
		}
		if !status.Running {
			if output, runErr := runner(ctx, status.InitScript, "start"); runErr != nil {
				_ = os.Chmod(status.InitScript, originalMode)
				return fmt.Errorf("start SSH service failed: %s", strings.TrimSpace(output))
			}
		}
		return nil
	}
	if status.Running {
		if output, runErr := runner(ctx, status.InitScript, "stop"); runErr != nil {
			return fmt.Errorf("stop SSH service failed: %s", strings.TrimSpace(output))
		}
	}
	if err := os.Chmod(status.InitScript, originalMode&^0111); err != nil {
		return fmt.Errorf("disable SSH at boot: %w", err)
	}
	return nil
}

func runSSHInitScript(ctx context.Context, script, action string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, "sh", script, action).CombinedOutput()
	return string(output), err
}

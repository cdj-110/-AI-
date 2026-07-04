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
)

const restartDelayEnv = "GATEWAY_RESTART_DELAY_MS"

type pingRequest struct {
	Target string `json:"target"`
	Count  int    `json:"count"`
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

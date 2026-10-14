package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/remotestore"
	"weikong-iot-platform/apps/gateway-go/internal/storage"
)

type remoteStorageRuntime interface {
	Statuses() []remotestore.Status
	ValidateChange(config.RemoteStorageConfig) error
}

func (s *Server) SetRemoteStorage(manager remoteStorageRuntime) { s.remoteStorage = manager }
func (s *Server) remoteStorageStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	statuses := []remotestore.Status{}
	if s.remoteStorage != nil {
		statuses = s.remoteStorage.Statuses()
	}
	writeJSON(w, map[string]any{"statuses": statuses})
}
func (s *Server) remoteStorageConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", 405)
		return
	}
	permission := "history.view"
	if r.Method == http.MethodPut {
		permission = "history.manage"
	}
	if !s.hasPermission(r, permission) {
		http.Error(w, "forbidden", 403)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if r.Method == http.MethodPut {
		revision, err := configFileRevision(s.configPath)
		if err != nil {
			http.Error(w, "读取配置版本失败", 500)
			return
		}
		if strings.Trim(r.Header.Get("If-Match"), "\"") != revision {
			http.Error(w, "配置已变更或缺少版本，请重新加载后再保存", 409)
			return
		}
		var next config.RemoteStorageConfig
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
		d.DisallowUnknownFields()
		if err := d.Decode(&next); err != nil || d.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "远程存储配置格式无效", 400)
			return
		}
		next.ApplyDefaults()
		cfg, err := config.Load(s.configPath)
		if err != nil {
			http.Error(w, "读取配置失败", 500)
			return
		}
		config.PreserveRemoteStorageSecrets(cfg.RemoteStorage, &next)
		if next.StoragePath != config.DefaultHistoryStoragePath {
			device, ok := storage.Resolve(next.StoragePath, next.StorageUUID)
			if !ok || device.ReadOnly || device.UUID == "" {
				http.Error(w, "请选择已挂载且可写的存储卷", 400)
				return
			}
			next.StoragePath = device.MountPath
			next.StorageUUID = device.UUID
		} else {
			next.StorageUUID = ""
		}
		if err = next.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		valid := map[string]bool{}
		for _, p := range append(cfg.FlattenPoints(), cfg.ComputedStatusPoints()...) {
			valid[p.DeviceKey+"\x00"+p.Metric] = true
		}
		for _, t := range next.Targets {
			for _, p := range t.Points {
				if !valid[p.DeviceKey+"\x00"+p.Metric] {
					http.Error(w, "所选点位不存在，请刷新点位列表", 400)
					return
				}
			}
		}
		if s.remoteStorage != nil {
			if err = s.remoteStorage.ValidateChange(next); err != nil {
				http.Error(w, err.Error(), 409)
				return
			}
		}
		cfg.RemoteStorage = next
		if err = s.saveAndApplyConfigLocked(cfg); err != nil {
			http.Error(w, "远程存储配置保存失败，已尝试恢复原配置", 500)
			return
		}
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(w, "读取配置失败", 500)
		return
	}
	redacted := config.RedactSecrets(cfg)
	statuses := []remotestore.Status{}
	if s.remoteStorage != nil {
		statuses = s.remoteStorage.Statuses()
	}
	devices, _ := storage.Detect()
	// Passwords are always masked. Read-only history viewers get no credentials.
	if !s.hasPermission(r, "history.manage") {
		for i := range redacted.RemoteStorage.Targets {
			redacted.RemoteStorage.Targets[i].Username = ""
			redacted.RemoteStorage.Targets[i].Password = ""
			redacted.RemoteStorage.Targets[i].CAFile = ""
		}
	}
	revision, _ := configFileRevision(s.configPath)
	w.Header().Set("ETag", quoteETag(revision))
	writeJSON(w, map[string]any{"config": redacted.RemoteStorage, "revision": revision, "statuses": statuses, "devices": devices, "pointGroups": historyPointGroups(cfg)})
}
func (s *Server) testRemoteStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var target config.RemoteStorageTarget
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(&target); err != nil || d.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "连接配置格式无效", 400)
		return
	}
	target.ApplyDefaults()
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(w, "读取配置失败", 500)
		return
	}
	wrapper := config.RemoteStorageConfig{Targets: []config.RemoteStorageTarget{target}}
	config.PreserveRemoteStorageSecrets(cfg.RemoteStorage, &wrapper)
	target = wrapper.Targets[0]
	if err = target.ValidateConnection(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// Connection test is strictly read-only: never auto-create a table here.
	target.CreateTable = false
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(target.TimeoutSeconds)*time.Second)
	defer cancel()
	adapter, err := remotestore.Open(target)
	if err == nil {
		defer adapter.Close()
		err = adapter.Prepare(ctx)
	}
	message := "连接和专用表结构验证通过（测试不写入数据）"
	if err != nil {
		message = fmt.Sprintf("连接或表结构检查失败：%s；若为首次使用，可启用自动建表后保存任务", remotestore.SafeError(err, target))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"ok": err == nil, "message": message, "durationMs": time.Since(started).Milliseconds()})
}

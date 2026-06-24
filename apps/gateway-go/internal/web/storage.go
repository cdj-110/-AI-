package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	removablestorage "weikong-iot-platform/apps/gateway-go/internal/storage"
)

func (s *Server) storageStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	devices, err := removablestorage.Detect()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	cacheBytes := int64(0)
	cachePath := ""
	active := false
	if cfg.OfflineCache.Enabled {
		if device, ok := removablestorage.Find(cfg.OfflineCache.StoragePath); ok {
			active = true
			cachePath = filepath.Join(device.MountPath, ".weikong", "gateway-spool.jsonl")
			if info, statErr := os.Stat(cachePath); statErr == nil {
				cacheBytes = info.Size()
			}
		}
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"devices":     devices,
		"configured":  cfg.OfflineCache.Enabled,
		"active":      active,
		"storagePath": cfg.OfflineCache.StoragePath,
		"maxSizeMB":   cfg.OfflineCache.MaxSizeMB,
		"cachePath":   cachePath,
		"cacheBytes":  cacheBytes,
	})
}

func validateOfflineCache(cfg config.Config) error {
	if !cfg.OfflineCache.Enabled {
		return nil
	}
	if cfg.OfflineCache.MaxSizeMB < 12 || cfg.OfflineCache.MaxSizeMB > 16 {
		return fmt.Errorf("离线缓存容量必须在 12–16 MB 之间")
	}
	device, ok := removablestorage.Find(cfg.OfflineCache.StoragePath)
	if !ok {
		return fmt.Errorf("未检测到已挂载的 TF 卡或 U 盘，无法开启离线缓存")
	}
	required := uint64(cfg.OfflineCache.MaxSizeMB*2+4) * 1024 * 1024
	if device.FreeBytes < required {
		return fmt.Errorf("可移动存储剩余空间不足，至少需要 %d MB", cfg.OfflineCache.MaxSizeMB*2+4)
	}
	return nil
}

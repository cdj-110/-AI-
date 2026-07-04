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
	switch request.Method {
	case http.MethodGet:
		s.writeStorageStatus(writer)
	case http.MethodPut:
		s.updateOfflineCache(writer, request)
	case http.MethodDelete:
		s.clearOfflineCache(writer)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) writeStorageStatus(writer http.ResponseWriter) {
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
			cachePath = offlineCachePath(device.MountPath)
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

func (s *Server) updateOfflineCache(writer http.ResponseWriter, request *http.Request) {
	var offline config.OfflineCacheConfig
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&offline); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if offline.MaxSizeMB == 0 {
		offline.MaxSizeMB = 16
	}
	cfg := s.runtime.Config()
	cfg.OfflineCache = offline
	if err := validateOfflineCache(cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.onConfig != nil {
		s.onConfig(cfg)
	} else {
		s.runtime.UpdateConfig(cfg)
	}
	s.writeStorageStatus(writer)
}

func (s *Server) clearOfflineCache(writer http.ResponseWriter) {
	cfg := s.runtime.Config()
	if cfg.OfflineCache.Enabled {
		if device, ok := removablestorage.Find(cfg.OfflineCache.StoragePath); ok {
			cachePath := offlineCachePath(device.MountPath)
			for _, path := range []string{cachePath, cachePath + ".drain", cachePath + ".trim"} {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	}
	s.writeStorageStatus(writer)
}

func offlineCachePath(mountPath string) string {
	return filepath.Join(mountPath, ".weikong", "gateway-spool.jsonl")
}

func validateOfflineCache(cfg config.Config) error {
	if !cfg.OfflineCache.Enabled {
		return nil
	}
	if cfg.OfflineCache.MaxSizeMB < 12 || cfg.OfflineCache.MaxSizeMB > 16 {
		return fmt.Errorf("\u79bb\u7ebf\u7f13\u5b58\u5bb9\u91cf\u5fc5\u987b\u5728 12-16 MB \u4e4b\u95f4")
	}
	device, ok := removablestorage.Find(cfg.OfflineCache.StoragePath)
	if !ok {
		return fmt.Errorf("\u672a\u68c0\u6d4b\u5230\u5df2\u6302\u8f7d\u7684 TF \u5361\u6216 U \u76d8\uff0c\u65e0\u6cd5\u5f00\u542f\u79bb\u7ebf\u7f13\u5b58")
	}
	required := uint64(cfg.OfflineCache.MaxSizeMB*2+4) * 1024 * 1024
	if device.FreeBytes < required {
		return fmt.Errorf("\u53ef\u79fb\u52a8\u5b58\u50a8\u5269\u4f59\u7a7a\u95f4\u4e0d\u8db3\uff0c\u81f3\u5c11\u9700\u8981 %d MB", cfg.OfflineCache.MaxSizeMB*2+4)
	}
	return nil
}

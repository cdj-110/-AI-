package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/history"
	removablestorage "weikong-iot-platform/apps/gateway-go/internal/storage"
)

func (s *Server) historyStorageStatus(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.writeHistoryStorageStatus(writer)
	case http.MethodPut:
		s.updateHistoryStorage(writer, request)
	case http.MethodDelete:
		s.clearHistoryStorage(writer)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) writeHistoryStorageStatus(writer http.ResponseWriter) {
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
	status := history.Status{MaxBytes: int64(cfg.HistoryStorage.MaxSizeMB) * 1024 * 1024}
	active := false
	if cfg.HistoryStorage.Enabled {
		if device, ok := removablestorage.Find(cfg.HistoryStorage.StoragePath); ok {
			active = true
			store := history.NewGuarded(history.Path(device.MountPath), status.MaxBytes, nil)
			status = store.Status()
		}
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"devices":     devices,
		"configured":  cfg.HistoryStorage.Enabled,
		"active":      active,
		"storagePath": cfg.HistoryStorage.StoragePath,
		"maxSizeMB":   cfg.HistoryStorage.MaxSizeMB,
		"historyPath": status.Path,
		"bytes":       status.Bytes,
		"maxBytes":    status.MaxBytes,
		"files":       status.Files,
	})
}

func (s *Server) updateHistoryStorage(writer http.ResponseWriter, request *http.Request) {
	var historyConfig config.HistoryStorageConfig
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&historyConfig); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if historyConfig.MaxSizeMB == 0 {
		historyConfig.MaxSizeMB = 256
	}
	cfg := s.runtime.Config()
	cfg.HistoryStorage = historyConfig
	if err := validateHistoryStorage(cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.saveAndApplyConfig(cfg); err != nil {
		http.Error(writer, "apply history storage configuration failed; previous configuration was restored: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeHistoryStorageStatus(writer)
}

func (s *Server) clearHistoryStorage(writer http.ResponseWriter) {
	cfg := s.runtime.Config()
	if cfg.HistoryStorage.Enabled {
		if device, ok := removablestorage.Find(cfg.HistoryStorage.StoragePath); ok {
			if err := history.NewGuarded(history.Path(device.MountPath), 0, nil).Clear(); err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	s.writeHistoryStorageStatus(writer)
}

func (s *Server) exportHistoryStorage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := s.runtime.Config()
	if !cfg.HistoryStorage.Enabled {
		http.Error(writer, "history storage is disabled", http.StatusBadRequest)
		return
	}
	device, ok := removablestorage.Find(cfg.HistoryStorage.StoragePath)
	if !ok {
		http.Error(writer, "history storage device is not mounted", http.StatusBadRequest)
		return
	}
	fileName := "gateway-history-" + time.Now().Format("20060102-150405") + ".csv"
	writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	writer.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	if err := history.NewGuarded(history.Path(device.MountPath), int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, nil).ExportCSV(writer); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func validateHistoryStorage(cfg config.Config) error {
	if !cfg.HistoryStorage.Enabled {
		return nil
	}
	if cfg.HistoryStorage.MaxSizeMB < 16 || cfg.HistoryStorage.MaxSizeMB > 32768 {
		return fmt.Errorf("历史数据容量必须在 16-32768 MB 之间")
	}
	device, ok := removablestorage.Find(cfg.HistoryStorage.StoragePath)
	if !ok {
		return fmt.Errorf("未检测到已挂载的 TF 卡或 U 盘，无法开启历史数据保存")
	}
	required := uint64(cfg.HistoryStorage.MaxSizeMB+16) * 1024 * 1024
	if device.FreeBytes < required {
		return fmt.Errorf("可移动存储剩余空间不足，至少需要 %d MB", cfg.HistoryStorage.MaxSizeMB+16)
	}
	if err := os.MkdirAll(history.Path(device.MountPath), 0700); err != nil {
		return err
	}
	return nil
}

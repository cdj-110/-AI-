package web

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/history"
	removablestorage "weikong-iot-platform/apps/gateway-go/internal/storage"
)

func (s *Server) historyMaintenance(writer http.ResponseWriter, request *http.Request) {
	s.historyMaintenanceMu.Lock()
	defer s.historyMaintenanceMu.Unlock()
	if request.Method == http.MethodGet {
		s.writeHistoryMaintenance(writer)
		return
	}
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Action string `json:"action"`
		ID     string `json:"id"`
		Full   bool   `json:"full"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, path, available, backupDir, err := s.historyMaintenanceContext()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	store := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	defer store.Close()
	switch strings.ToLower(strings.TrimSpace(body.Action)) {
	case "check":
		result, err := store.CheckIntegrity(body.Full)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, result)
	case "backup":
		item, err := s.createHistoryBackup(cfg, store, backupDir)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, item)
	case "aggregate":
		if err := store.Aggregate(time.Now()); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		status, _ := store.AggregateStatus()
		writeJSON(writer, status)
	case "vacuum":
		if err := store.Vacuum(); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]interface{}{"ok": true})
	case "delete_backup":
		if err := history.DeleteBackup(backupDir, body.ID); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]interface{}{"ok": true})
	case "restore":
		_ = store.Close()
		if err := s.restoreHistoryBackup(cfg, path, available, backupDir, body.ID); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]interface{}{"ok": true, "message": "数据库恢复完成，历史写入已自动恢复"})
	default:
		http.Error(writer, "unsupported maintenance action", http.StatusBadRequest)
	}
}

func (s *Server) writeHistoryMaintenance(writer http.ResponseWriter) {
	cfg, path, available, backupDir, err := s.historyMaintenanceContext()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	store := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	defer store.Close()
	integrity, integrityErr := store.CheckIntegrity(false)
	backups, backupErr := history.ListBackups(backupDir)
	aggregates, aggregateErr := store.AggregateStatus()
	status := store.Status()
	writeJSON(writer, map[string]interface{}{
		"database": status, "integrity": integrity, "backups": backups, "backupPath": backupDir,
		"backupEnabled": cfg.HistoryStorage.BackupEnabled, "backupIntervalHours": cfg.HistoryStorage.BackupIntervalHours,
		"backupKeep": cfg.HistoryStorage.BackupKeep, "encryptBackups": cfg.HistoryStorage.EncryptBackups,
		"aggregates": aggregates, "integrityError": errorText(integrityErr), "backupError": errorText(backupErr), "aggregateError": errorText(aggregateErr),
	})
}

func (s *Server) downloadHistoryBackup(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, _, _, backupDir, err := s.historyMaintenanceContext()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(request.URL.Query().Get("id"))
	items, err := history.ListBackups(backupDir)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		writer.Header().Set("Content-Disposition", `attachment; filename="`+item.FileName+`"`)
		writer.Header().Set("X-Content-SHA256", item.SHA256)
		http.ServeFile(writer, request, item.Path)
		return
	}
	http.Error(writer, "backup not found", http.StatusNotFound)
}

func (s *Server) historyMaintenanceContext() (config.Config, string, func() bool, string, error) {
	cfg, err := s.runtimeConfig()
	if err != nil {
		return cfg, "", nil, "", err
	}
	if !cfg.HistoryStorage.Enabled {
		return cfg, "", nil, "", errors.New("历史数据保存未启用")
	}
	path, _, available, ok := resolveHistoryStorage(cfg.HistoryStorage)
	if !ok {
		return cfg, "", nil, "", errors.New("历史存储不可用")
	}
	backupDir, err := resolveHistoryBackupDir(cfg.HistoryStorage, path)
	return cfg, path, available, backupDir, err
}

func resolveHistoryBackupDir(historyConfig config.HistoryStorageConfig, historyPath string) (string, error) {
	configured := strings.TrimSpace(historyConfig.BackupPath)
	if configured == "" || strings.EqualFold(configured, "current") {
		return filepath.Join(historyPath, "backups"), nil
	}
	if filepath.Clean(configured) == filepath.Clean(config.DefaultHistoryStoragePath) {
		return filepath.Join(history.Path(config.DefaultHistoryStoragePath), "backups"), nil
	}
	device, ok := removablestorage.Find(configured)
	if !ok {
		return "", errors.New("备份存储未挂载")
	}
	return filepath.Join(history.Path(device.MountPath), "backups"), nil
}

func (s *Server) createHistoryBackup(cfg config.Config, store *history.Store, backupDir string) (history.BackupInfo, error) {
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return history.BackupInfo{}, err
	}
	status := store.Status()
	_, freeBytes, spaceErr := removablestorage.Space(backupDir)
	if spaceErr == nil {
		required := uint64(status.Bytes) + 4*1024*1024
		if cfg.HistoryStorage.EncryptBackups {
			required += uint64(status.Bytes)
		}
		if freeBytes < required {
			return history.BackupInfo{}, errors.New("备份目标空间不足，无法安全创建数据库快照")
		}
	}
	var key []byte
	var err error
	if cfg.HistoryStorage.EncryptBackups {
		key, err = historyBackupKey()
		if err != nil {
			return history.BackupInfo{}, err
		}
	}
	return store.CreateBackup(backupDir, cfg.HistoryStorage.BackupKeep, key)
}

func (s *Server) restoreHistoryBackup(cfg config.Config, path string, available func() bool, backupDir, id string) error {
	paused := cfg
	paused.HistoryStorage.Enabled = false
	if err := s.saveAndApplyConfig(paused); err != nil {
		return err
	}
	store := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	var key []byte
	var err error
	if key, err = historyBackupKeyIfExists(); err != nil {
		_ = store.Close()
		_ = s.saveAndApplyConfig(cfg)
		return err
	}
	rollbackPath, restoreErr := store.RestoreBackup(backupDir, id, key)
	_ = store.Close()
	if restoreErr != nil {
		_ = s.saveAndApplyConfig(cfg)
		return restoreErr
	}
	if applyErr := s.saveAndApplyConfig(cfg); applyErr != nil {
		rollbackStore := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
		rollbackErr := rollbackStore.RollbackRestore(rollbackPath)
		_ = rollbackStore.Close()
		_ = s.saveAndApplyConfig(cfg)
		if rollbackErr != nil {
			return errors.New(applyErr.Error() + "; rollback failed: " + rollbackErr.Error())
		}
		return applyErr
	}
	_ = os.Remove(rollbackPath)
	return nil
}

func (s *Server) historyMaintenanceLoop(ctx context.Context) {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.runScheduledHistoryBackup()
			timer.Reset(time.Minute)
		}
	}
}

func (s *Server) runScheduledHistoryBackup() {
	s.historyMaintenanceMu.Lock()
	defer s.historyMaintenanceMu.Unlock()
	cfg, path, available, backupDir, err := s.historyMaintenanceContext()
	if err != nil || !cfg.HistoryStorage.BackupEnabled {
		return
	}
	items, _ := history.ListBackups(backupDir)
	if len(items) > 0 {
		last, _ := time.Parse(time.RFC3339Nano, items[0].CreatedAt)
		if time.Since(last) < time.Duration(cfg.HistoryStorage.BackupIntervalHours)*time.Hour {
			return
		}
	}
	store := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	defer store.Close()
	if _, err := s.createHistoryBackup(cfg, store, backupDir); err != nil {
		log.Printf("scheduled history backup failed: %v", err)
	}
}

func historyBackupKey() ([]byte, error) {
	path := filepath.Join(config.DefaultHistoryStoragePath, ".history-backup.key")
	if key, err := os.ReadFile(path); err == nil && len(key) == 32 {
		return key, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}

func historyBackupKeyIfExists() ([]byte, error) {
	path := filepath.Join(config.DefaultHistoryStoragePath, ".history-backup.key")
	key, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("历史备份密钥格式无效")
	}
	return key, nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

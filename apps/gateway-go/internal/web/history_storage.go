package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/history"
	removablestorage "weikong-iot-platform/apps/gateway-go/internal/storage"
)

func (s *Server) historyStorageStatus(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		if !s.hasPermission(request, "history.view") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		s.writeHistoryStorageStatus(writer)
	case http.MethodPut:
		if !s.hasPermission(request, "history.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		s.updateHistoryStorage(writer, request)
	case http.MethodDelete:
		if !s.hasPermission(request, "history.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
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
	taskStats := []history.TaskStat{}
	aggregates := history.AggregateStatus{Enabled: cfg.HistoryStorage.AggregationEnabled}
	storageType := "local"
	storageError := ""
	if cfg.HistoryStorage.Enabled {
		if path, kind, available, ok := resolveHistoryStorage(cfg.HistoryStorage); ok {
			storageType = kind
			if err := os.MkdirAll(path, 0700); err != nil {
				storageError = err.Error()
			} else {
				store := newHistoryStore(path, status.MaxBytes, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
				defer store.Close()
				status = store.Status()
				taskStats, _ = store.TaskStats()
				aggregates, _ = store.AggregateStatus()
				active = status.Enabled
			}
		} else {
			storageType = "removable"
			storageError = "未检测到已挂载的 U 盘或 TF 卡"
		}
	} else if !cfg.HistoryStorage.UsesLocalStorage() {
		storageType = "removable"
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if s.historyWriterStatus != nil {
		queue := s.historyWriterStatus()
		status.PendingSamples, status.DroppedSamples = int64(queue.Pending), queue.Dropped
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"devices":             devices,
		"pointGroups":         historyPointGroups(cfg),
		"configured":          cfg.HistoryStorage.Enabled,
		"active":              active,
		"storageType":         storageType,
		"storageError":        storageError,
		"storagePath":         cfg.HistoryStorage.StoragePath,
		"storageUUID":         cfg.HistoryStorage.StorageUUID,
		"maxSizeMB":           cfg.HistoryStorage.MaxSizeMB,
		"retentionDays":       cfg.HistoryStorage.RetentionDays,
		"backupEnabled":       cfg.HistoryStorage.BackupEnabled,
		"backupIntervalHours": cfg.HistoryStorage.BackupIntervalHours,
		"backupKeep":          cfg.HistoryStorage.BackupKeep,
		"backupPath":          cfg.HistoryStorage.BackupPath,
		"encryptBackups":      cfg.HistoryStorage.EncryptBackups,
		"aggregationEnabled":  cfg.HistoryStorage.AggregationEnabled,
		"aggregates":          aggregates,
		"tasks":               cfg.HistoryStorage.EffectiveTasks(),
		"taskStats":           taskStats,
		"selectionMode":       cfg.HistoryStorage.SelectionMode,
		"points":              cfg.HistoryStorage.Points,
		"saveMode":            cfg.HistoryStorage.SaveMode,
		"saveIntervalSeconds": cfg.HistoryStorage.SaveIntervalSeconds,
		"changeDeadband":      cfg.HistoryStorage.ChangeDeadband,
		"historyPath":         status.Path,
		"bytes":               status.Bytes,
		"maxBytes":            status.MaxBytes,
		"files":               status.Files,
		"databasePath":        status.DatabasePath,
		"samples":             status.Samples,
		"legacyBytes":         status.LegacyBytes,
		"legacyFiles":         status.LegacyFiles,
		"writable":            status.Writable,
		"paused":              status.Paused,
		"pauseReason":         status.PauseReason,
		"totalBytes":          status.TotalBytes,
		"freeBytes":           status.FreeBytes,
		"minFreeBytes":        status.MinFreeBytes,
		"lastWriteAt":         status.LastWriteAt,
		"lastError":           status.LastError,
		"pendingSamples":      status.PendingSamples,
		"droppedSamples":      status.DroppedSamples,
		"recoveryCount":       status.RecoveryCount,
		"lastRecoveryPath":    status.LastRecoveryPath,
	})
}

type historyPointGroup struct {
	DeviceKey string `json:"deviceKey"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Count     int    `json:"count"`
}

// Only expose the catalogue needed by history users, never device credentials.
func historyPointGroups(cfg config.Config) []historyPointGroup {
	groups := make([]historyPointGroup, 0, len(cfg.Devices)+len(cfg.EdgeComputing.Groups))
	for _, device := range cfg.Devices {
		groups = append(groups, historyPointGroup{device.DeviceKey, device.Name, device.Protocol, len(device.Points)})
	}
	for _, group := range cfg.EdgeComputing.Groups {
		groups = append(groups, historyPointGroup{group.GroupKey, group.Name, "edge-compute", len(group.Points)})
	}
	return groups
}

type historyStoragePointOption struct {
	DeviceKey string `json:"deviceKey"`
	Metric    string `json:"metric"`
	Name      string `json:"name"`
	DataType  string `json:"dataType,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Unit      string `json:"unit,omitempty"`
}

func (s *Server) historyStoragePoints(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	deviceKey := strings.TrimSpace(request.URL.Query().Get("deviceKey"))
	search := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("search")))
	offset := queryNonNegativeInt(request, "offset", 0)
	limit := queryNonNegativeInt(request, "limit", 200)
	if limit < 1 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	all := append(cfg.FlattenPoints(), cfg.ComputedStatusPoints()...)
	items := make([]historyStoragePointOption, 0, limit)
	total := 0
	seen := make(map[string]struct{}, len(all))
	for _, point := range all {
		key := point.DeviceKey + "\x00" + point.Metric
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if deviceKey != "" && point.DeviceKey != deviceKey {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(point.Name+" "+point.Metric+" "+point.ItemID), search) {
			continue
		}
		if total >= offset && len(items) < limit {
			items = append(items, historyStoragePointOption{DeviceKey: point.DeviceKey, Metric: point.Metric, Name: point.Name, DataType: point.DataType, Protocol: point.Protocol, Unit: point.Unit})
		}
		total++
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"offset": offset, "limit": limit, "total": total, "points": items})
}

func (s *Server) updateHistoryStorage(writer http.ResponseWriter, request *http.Request) {
	var historyConfig config.HistoryStorageConfig
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&historyConfig); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	historyConfig.ApplyDefaults()
	historyConfig.SelectionMode = strings.ToLower(strings.TrimSpace(historyConfig.SelectionMode))
	if historyConfig.SelectionMode == "" {
		historyConfig.SelectionMode = "all"
	}
	historyConfig.SaveMode = strings.ToLower(strings.TrimSpace(historyConfig.SaveMode))
	if historyConfig.SaveMode == "" {
		historyConfig.SaveMode = "interval"
	}
	if historyConfig.SaveIntervalSeconds <= 0 {
		historyConfig.SaveIntervalSeconds = 1
	}
	points := make([]config.HistoryPointRef, 0, len(historyConfig.Points))
	seen := make(map[string]struct{}, len(historyConfig.Points))
	for _, point := range historyConfig.Points {
		point.DeviceKey = strings.TrimSpace(point.DeviceKey)
		point.Metric = strings.TrimSpace(point.Metric)
		if point.DeviceKey == "" || point.Metric == "" {
			continue
		}
		key := point.DeviceKey + "\x00" + point.Metric
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		points = append(points, point)
	}
	historyConfig.Points = points
	for index := range historyConfig.Tasks {
		historyConfig.Tasks[index].Points = normalizeHistoryPoints(historyConfig.Tasks[index].Points)
	}
	cfg := s.runtime.Config()
	bindHistoryVolume(&historyConfig, cfg.HistoryStorage)
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

func normalizeHistoryPoints(input []config.HistoryPointRef) []config.HistoryPointRef {
	points := make([]config.HistoryPointRef, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, point := range input {
		point.DeviceKey = strings.TrimSpace(point.DeviceKey)
		point.Metric = strings.TrimSpace(point.Metric)
		if point.DeviceKey == "" || point.Metric == "" {
			continue
		}
		key := point.DeviceKey + "\x00" + point.Metric
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		points = append(points, point)
	}
	return points
}

func (s *Server) clearHistoryStorage(writer http.ResponseWriter) {
	cfg, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if cfg.HistoryStorage.Enabled {
		if path, _, available, ok := resolveHistoryStorage(cfg.HistoryStorage); ok {
			store := newHistoryStore(path, 0, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
			defer store.Close()
			if err := store.Clear(); err != nil {
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
	path, _, available, ok := resolveHistoryStorage(cfg.HistoryStorage)
	if !ok {
		http.Error(writer, "history storage device is not mounted", http.StatusBadRequest)
		return
	}
	fileName := "gateway-history-" + time.Now().Format("20060102-150405") + ".csv"
	writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	writer.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	store := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	defer store.Close()
	if err := store.ExportCSV(writer); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) queryHistoryStorage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if !cfg.HistoryStorage.Enabled {
		http.Error(writer, "历史数据保存未启用", http.StatusBadRequest)
		return
	}
	deviceKey := strings.TrimSpace(request.URL.Query().Get("deviceKey"))
	metric := strings.TrimSpace(request.URL.Query().Get("metric"))
	if deviceKey == "" || metric == "" {
		http.Error(writer, "必须指定设备和点位", http.StatusBadRequest)
		return
	}
	start, err := parseHistoryQueryTime(request.URL.Query().Get("start"))
	if err != nil {
		http.Error(writer, "开始时间格式无效", http.StatusBadRequest)
		return
	}
	end, err := parseHistoryQueryTime(request.URL.Query().Get("end"))
	if err != nil {
		http.Error(writer, "结束时间格式无效", http.StatusBadRequest)
		return
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		http.Error(writer, "结束时间不能早于开始时间", http.StatusBadRequest)
		return
	}
	path, _, available, ok := resolveHistoryStorage(cfg.HistoryStorage)
	if !ok {
		http.Error(writer, "历史存储不可用", http.StatusServiceUnavailable)
		return
	}
	store := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	defer store.Close()
	result, err := store.Query(history.QueryOptions{
		TaskKey: strings.TrimSpace(request.URL.Query().Get("taskKey")), DeviceKey: deviceKey, Metric: metric, Start: start, End: end,
		Offset: queryNonNegativeInt(request, "offset", 0), Limit: queryNonNegativeInt(request, "limit", 100), Resolution: strings.TrimSpace(request.URL.Query().Get("resolution")),
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(result)
}

func (s *Server) migrateHistoryStorage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		TargetPath string `json:"targetPath"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	bodyTarget := strings.TrimSpace(body.TargetPath)
	if bodyTarget == "" {
		http.Error(writer, "请选择迁移目标存储", http.StatusBadRequest)
		return
	}
	cfg, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if !cfg.HistoryStorage.Enabled {
		http.Error(writer, "请先启用历史数据保存", http.StatusBadRequest)
		return
	}
	if filepath.Clean(bodyTarget) == filepath.Clean(cfg.HistoryStorage.StoragePath) {
		http.Error(writer, "迁移目标与当前存储相同", http.StatusBadRequest)
		return
	}
	targetConfig := cfg
	targetConfig.HistoryStorage.StoragePath = bodyTarget
	targetConfig.HistoryStorage.StorageUUID = ""
	bindHistoryVolume(&targetConfig.HistoryStorage, config.HistoryStorageConfig{})
	if err := validateHistoryStorage(targetConfig); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	sourcePath, _, sourceAvailable, ok := resolveHistoryStorage(cfg.HistoryStorage)
	if !ok {
		http.Error(writer, "当前历史存储不可用", http.StatusServiceUnavailable)
		return
	}
	targetPath, _, _, ok := resolveHistoryStorage(targetConfig.HistoryStorage)
	if !ok {
		http.Error(writer, "迁移目标存储不可用", http.StatusServiceUnavailable)
		return
	}
	// Stop the history writer before taking the SQLite snapshot. Without this
	// short pause, samples written after VACUUM INTO but before the config switch
	// would remain only in the source database.
	pausedConfig := cfg
	pausedConfig.HistoryStorage.Enabled = false
	if err := s.saveAndApplyConfig(pausedConfig); err != nil {
		http.Error(writer, "暂停历史写入失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	source := newHistoryStore(sourcePath, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, sourceAvailable, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	destination, err := source.BackupTo(targetPath)
	_ = source.Close()
	if err != nil {
		_ = s.saveAndApplyConfig(cfg)
		http.Error(writer, "历史数据库迁移失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.saveAndApplyConfig(targetConfig); err != nil {
		_ = s.saveAndApplyConfig(cfg)
		http.Error(writer, "数据库已复制但配置切换失败，原配置已恢复: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok": true, "sourcePath": sourcePath, "targetPath": targetPath, "databasePath": destination,
		"message": "历史数据库已完成一致性复制并切换到目标存储，源数据库保留作为迁移备份",
	})
}

func parseHistoryQueryTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func newHistoryStore(path string, maxBytes int64, available func() bool, retentionDays int, aggregationEnabled ...bool) *history.Store {
	const minHistoryFreeBytes = 4 * 1024 * 1024
	store := history.NewGuarded(path, maxBytes, available).SetRetentionDays(retentionDays)
	if len(aggregationEnabled) > 0 {
		store.SetAggregationEnabled(aggregationEnabled[0])
	}
	return store.SetDiskGuard(func() (uint64, uint64, error) {
		total, free, err := removablestorage.Space(path)
		if errors.Is(err, removablestorage.ErrSpaceUnsupported) {
			return 0, ^uint64(0), nil
		}
		return total, free, err
	}, minHistoryFreeBytes)
}

func validateHistoryStorage(cfg config.Config) error {
	if !cfg.HistoryStorage.Enabled {
		return nil
	}
	tasks := cfg.HistoryStorage.EffectiveTasks()
	if len(tasks) == 0 {
		return fmt.Errorf("历史数据保存启用时，至少需要创建一个任务")
	}
	seenTasks := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.TaskKey == "" || task.Name == "" {
			return fmt.Errorf("历史任务必须填写任务名称")
		}
		if _, exists := seenTasks[task.TaskKey]; exists {
			return fmt.Errorf("历史任务标识 %q 重复", task.TaskKey)
		}
		seenTasks[task.TaskKey] = struct{}{}
		if task.SelectionMode != "all" && task.SelectionMode != "selected" {
			return fmt.Errorf("历史任务 %q 的保存范围必须为全部点位或指定点位", task.Name)
		}
		if task.SelectionMode == "selected" && len(task.Points) == 0 {
			return fmt.Errorf("历史任务 %q 至少需要选择一个点位", task.Name)
		}
		if task.SaveMode != "interval" && task.SaveMode != "change" {
			return fmt.Errorf("历史任务 %q 的保存策略必须为按周期保存或变化时保存", task.Name)
		}
		if task.SaveIntervalSeconds < 1 || task.SaveIntervalSeconds > 86400 {
			return fmt.Errorf("历史任务 %q 的保存周期必须在 1～86400 秒之间", task.Name)
		}
		if task.ChangeDeadband < 0 {
			return fmt.Errorf("历史任务 %q 的变化死区不能小于 0", task.Name)
		}
		if task.StartAt != nil && task.EndAt != nil && !task.EndAt.After(*task.StartAt) {
			return fmt.Errorf("历史任务 %q 的截止时间必须晚于开始时间", task.Name)
		}
	}
	if cfg.HistoryStorage.MaxSizeMB < 16 || cfg.HistoryStorage.MaxSizeMB > 32768 {
		return fmt.Errorf("历史数据容量必须在 16-32768 MB 之间")
	}
	if retention := cfg.HistoryStorage.RetentionDays; retention != 0 && retention != 7 && retention != 30 && retention != 90 {
		return fmt.Errorf("历史数据保存期限必须为仅按容量、7天、30天或90天")
	}
	if cfg.HistoryStorage.BackupIntervalHours < 1 || cfg.HistoryStorage.BackupIntervalHours > 168 {
		return fmt.Errorf("数据库备份周期必须在 1～168 小时之间")
	}
	if cfg.HistoryStorage.BackupKeep < 1 || cfg.HistoryStorage.BackupKeep > 30 {
		return fmt.Errorf("数据库备份保留数量必须在 1～30 份之间")
	}
	path, storageType, _, ok := resolveHistoryStorage(cfg.HistoryStorage)
	if !ok {
		return fmt.Errorf("未检测到已挂载的 TF 卡或 U 盘，无法使用所选外部存储")
	}
	if storageType == "removable" {
		device, _ := removablestorage.Resolve(cfg.HistoryStorage.StoragePath, cfg.HistoryStorage.StorageUUID)
		if device.ReadOnly {
			return fmt.Errorf("所选分区已只读，历史写入已暂停")
		}
		required := uint64(cfg.HistoryStorage.MaxSizeMB+16) * 1024 * 1024
		if device.FreeBytes < required {
			return fmt.Errorf("可移动存储剩余空间不足，至少需要 %d MB", cfg.HistoryStorage.MaxSizeMB+16)
		}
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	if storageType == "local" {
		_, freeBytes, err := removablestorage.Space(path)
		if err != nil && !errors.Is(err, removablestorage.ErrSpaceUnsupported) {
			return fmt.Errorf("无法读取本地存储剩余空间: %w", err)
		}
		const localReserveMB = 4
		required := uint64(cfg.HistoryStorage.MaxSizeMB+localReserveMB) * 1024 * 1024
		if err == nil && freeBytes < required {
			return fmt.Errorf("本地存储剩余空间不足，容量上限为 %d MB 时至少需要保留 %d MB 可用空间", cfg.HistoryStorage.MaxSizeMB, cfg.HistoryStorage.MaxSizeMB+localReserveMB)
		}
	}
	if cfg.HistoryStorage.BackupEnabled {
		backupDir, err := resolveHistoryBackupDir(cfg.HistoryStorage, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(backupDir, 0700); err != nil {
			return fmt.Errorf("无法创建数据库备份目录: %w", err)
		}
	}
	return nil
}

// resolveHistoryStorage keeps local and removable storage behavior consistent
// across status, validation, clear and export operations.
func resolveHistoryStorage(historyConfig config.HistoryStorageConfig) (path string, storageType string, available func() bool, ok bool) {
	if historyConfig.UsesLocalStorage() {
		return history.Path(config.DefaultHistoryStoragePath), "local", nil, true
	}
	device, mounted := removablestorage.Resolve(historyConfig.StoragePath, historyConfig.StorageUUID)
	if !mounted {
		return "", "removable", nil, false
	}
	return history.Path(device.MountPath), "removable", removablestorage.Guard(device), true
}

func bindHistoryVolume(next *config.HistoryStorageConfig, previous config.HistoryStorageConfig) {
	if next.UsesLocalStorage() {
		next.StorageUUID = ""
		return
	}
	if next.StoragePath == previous.StoragePath && previous.StorageUUID != "" {
		next.StorageUUID = previous.StorageUUID
		return
	}
	if device, ok := removablestorage.Resolve(next.StoragePath, next.StorageUUID); ok {
		next.StorageUUID = device.UUID
	}
}

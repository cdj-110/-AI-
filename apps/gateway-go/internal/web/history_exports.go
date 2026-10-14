package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/history"
)

type historyExportJob struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	TaskKey     string `json:"taskKey,omitempty"`
	DeviceKey   string `json:"deviceKey,omitempty"`
	Metric      string `json:"metric,omitempty"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	Error       string `json:"error,omitempty"`
	CreatedAt   string `json:"createdAt"`
	CompletedAt string `json:"completedAt,omitempty"`
	filePath    string
}

type historyExportManager struct {
	mu      sync.Mutex
	jobs    map[string]*historyExportJob
	counter atomic.Uint64
}

func newHistoryExportManager() *historyExportManager {
	return &historyExportManager{jobs: map[string]*historyExportJob{}}
}

func (s *Server) historyExportManager() *historyExportManager {
	if s.exportJobs == nil {
		s.exportJobs = newHistoryExportManager()
	}
	return s.exportJobs
}

func (s *Server) historyStorageExports(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		if !s.hasPermission(request, "history.view") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		jobs := s.historyExportManager().list()
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"jobs": jobs})
	case http.MethodPost:
		if !s.hasPermission(request, "history.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		s.createHistoryExport(writer, request)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) createHistoryExport(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		TaskKey   string `json:"taskKey"`
		DeviceKey string `json:"deviceKey"`
		Metric    string `json:"metric"`
		Start     string `json:"start"`
		End       string `json:"end"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	body.TaskKey, body.DeviceKey, body.Metric = strings.TrimSpace(body.TaskKey), strings.TrimSpace(body.DeviceKey), strings.TrimSpace(body.Metric)
	if body.Metric != "" && body.DeviceKey == "" {
		http.Error(writer, "按点位导出时必须指定设备", http.StatusBadRequest)
		return
	}
	start, err := parseHistoryQueryTime(body.Start)
	if err != nil {
		http.Error(writer, "开始时间格式无效", http.StatusBadRequest)
		return
	}
	end, err := parseHistoryQueryTime(body.End)
	if err != nil || !start.IsZero() && !end.IsZero() && end.Before(start) {
		http.Error(writer, "结束时间格式无效或早于开始时间", http.StatusBadRequest)
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
	if body.TaskKey != "" {
		found := false
		for _, task := range cfg.HistoryStorage.EffectiveTasks() {
			if task.TaskKey == body.TaskKey {
				found = true
				break
			}
		}
		if !found {
			http.Error(writer, "所选历史任务不存在", http.StatusBadRequest)
			return
		}
	}
	path, _, available, ok := resolveHistoryStorage(cfg.HistoryStorage)
	if !ok {
		http.Error(writer, "历史存储不可用", http.StatusServiceUnavailable)
		return
	}
	manager := s.historyExportManager()
	job := manager.create(body.TaskKey, body.DeviceKey, body.Metric, body.Start, body.End, filepath.Join(path, "exports"))
	go func() {
		if err := os.MkdirAll(filepath.Dir(job.filePath), 0700); err != nil {
			manager.finish(job.ID, err)
			return
		}
		file, err := os.OpenFile(job.filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err == nil {
			store := newHistoryStore(path, int64(cfg.HistoryStorage.MaxSizeMB)*1024*1024, available, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
			err = store.ExportFilteredCSV(file, history.QueryOptions{TaskKey: body.TaskKey, DeviceKey: body.DeviceKey, Metric: body.Metric, Start: start, End: end})
			_ = store.Close()
		}
		if closeErr := fileClose(file); err == nil {
			err = closeErr
		}
		manager.finish(job.ID, err)
	}()
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(job)
}

func fileClose(file *os.File) error {
	if file == nil {
		return nil
	}
	return file.Close()
}

func (s *Server) downloadHistoryExport(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	job, ok := s.historyExportManager().get(strings.TrimSpace(request.URL.Query().Get("id")))
	if !ok || job.Status != "completed" || job.filePath == "" {
		http.Error(writer, "导出任务尚未完成或不存在", http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Disposition", `attachment; filename="`+job.FileName+`"`)
	http.ServeFile(writer, request, job.filePath)
}

func (m *historyExportManager) create(taskKey, deviceKey, metric, start, end, dir string) *historyExportJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked()
	now := time.Now()
	id := fmt.Sprintf("history-export-%d-%d", now.UnixMilli(), m.counter.Add(1))
	fileName := "gateway-history-" + now.Format("20060102-150405") + ".csv"
	job := &historyExportJob{ID: id, Status: "running", TaskKey: taskKey, DeviceKey: deviceKey, Metric: metric, Start: start, End: end, FileName: fileName, CreatedAt: now.Format(time.RFC3339Nano), filePath: filepath.Join(dir, id+".csv")}
	m.jobs[id] = job
	copy := *job
	return &copy
}

func (m *historyExportManager) finish(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil {
		return
	}
	job.CompletedAt = time.Now().Format(time.RFC3339Nano)
	if err != nil {
		job.Status, job.Error = "failed", err.Error()
		_ = os.Remove(job.filePath)
	} else {
		job.Status = "completed"
	}
}

func (m *historyExportManager) list() []historyExportJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked()
	result := make([]historyExportJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		result = append(result, *job)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	return result
}

func (m *historyExportManager) get(id string) (historyExportJob, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil {
		return historyExportJob{}, false
	}
	return *job, true
}

func (m *historyExportManager) cleanupLocked() {
	cutoff := time.Now().Add(-24 * time.Hour)
	for id, job := range m.jobs {
		created, _ := time.Parse(time.RFC3339Nano, job.CreatedAt)
		if created.Before(cutoff) {
			_ = os.Remove(job.filePath)
			delete(m.jobs, id)
		}
	}
}

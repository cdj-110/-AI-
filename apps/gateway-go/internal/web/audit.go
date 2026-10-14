package web

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	auditMaxBytes = 2 * 1024 * 1024
	auditMaxLimit = 500
)

type auditEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Username   string    `json:"username,omitempty"`
	RemoteIP   string    `json:"remoteIp,omitempty"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMS int64     `json:"durationMs,omitempty"`
	Result     string    `json:"result"`
	Detail     string    `json:"detail,omitempty"`
}

type auditStore struct {
	mu   sync.Mutex
	path string
}

func newAuditStore(configPath string) *auditStore {
	if strings.TrimSpace(configPath) == "" {
		return &auditStore{}
	}
	dir := filepath.Dir(configPath)
	return &auditStore{path: filepath.Join(dir, ".runtime", "security-audit.jsonl")}
}

func (store *auditStore) append(event auditEvent) error {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0700); err != nil {
		return err
	}
	if info, statErr := os.Stat(store.path); statErr == nil && info.Size()+int64(len(line)+1) > auditMaxBytes {
		_ = os.Remove(store.path + ".1")
		if err := os.Rename(store.path, store.path+".1"); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(store.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (store *auditStore) recent(limit int) ([]auditEvent, error) {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > auditMaxLimit {
		limit = auditMaxLimit
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	file, err := os.Open(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	items := make([]auditEvent, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var event auditEvent
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			items = append(items, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (writer *auditResponseWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *auditResponseWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	return writer.ResponseWriter.Write(body)
}

func (s *Server) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet || request.Method == http.MethodHead || request.Method == http.MethodOptions || request.URL.Path == "/login" {
			next.ServeHTTP(writer, request)
			return
		}
		started := time.Now()
		username := ""
		if user, ok := s.currentUser(request); ok {
			username = user.Username
		}
		captured := &auditResponseWriter{ResponseWriter: writer}
		next.ServeHTTP(captured, request)
		status := captured.status
		if status == 0 {
			status = http.StatusOK
		}
		result := "success"
		if status >= 400 {
			result = "failed"
		}
		s.appendAudit(auditEvent{
			Username: username, RemoteIP: requestRemoteIP(request), Method: request.Method,
			Path: request.URL.Path, Status: status, DurationMS: time.Since(started).Milliseconds(), Result: result,
		})
	})
}

func (s *Server) appendAudit(event auditEvent) {
	if s.audit == nil {
		s.audit = newAuditStore(s.configPath)
	}
	_ = s.audit.append(event)
}

func (s *Server) auditLog(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	if s.audit == nil {
		s.audit = newAuditStore(s.configPath)
	}
	items, err := s.audit.recent(limit)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"events": items})
}

func requestRemoteIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(request.RemoteAddr)
}

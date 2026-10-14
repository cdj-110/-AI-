package remotestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
	"weikong-iot-platform/apps/gateway-go/internal/storage"
)

type Status struct {
	ID          string     `json:"id"`
	Enabled     bool       `json:"enabled"`
	CacheReady  bool       `json:"cacheReady"`
	Pending     int64      `json:"pending"`
	Bytes       int64      `json:"bytes"`
	Sent        uint64     `json:"sent"`
	Dropped     uint64     `json:"dropped"`
	Connected   bool       `json:"connected"`
	LastSuccess *time.Time `json:"lastSuccess,omitempty"`
	LastError   string     `json:"lastError,omitempty"`
	CacheError  string     `json:"cacheError,omitempty"`
	NextRetry   *time.Time `json:"nextRetry,omitempty"`
}
type worker struct {
	mu      sync.Mutex
	status  Status
	cfg     config.RemoteStorageConfig
	target  config.RemoteStorageTarget
	gateway string
	store   *state.Store
	queue   *Queue
	cancel  context.CancelFunc
	done    chan struct{}
}
type Manager struct {
	mu      sync.Mutex
	cfg     config.RemoteStorageConfig
	gateway string
	store   *state.Store
	ctx     context.Context
	workers map[string]*worker
}

func New(store *state.Store) *Manager { return &Manager{store: store, workers: map[string]*worker{}} }
func (m *Manager) Start(ctx context.Context, cfg config.Config) {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
	m.Update(cfg)
}
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, w := range m.workers {
		w.cancel()
	}
	for _, w := range m.workers {
		<-w.done
	}
	m.workers = map[string]*worker{}
}
func (m *Manager) Update(cfg config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil {
		return
	}
	next := cfg.RemoteStorage
	wanted := map[string]config.RemoteStorageTarget{}
	for _, t := range next.Targets {
		wanted[t.ID] = t
	}
	for id, w := range m.workers {
		t, ok := wanted[id]
		if !ok || !reflect.DeepEqual(w.target, t) || m.cfg.StoragePath != next.StoragePath || m.cfg.StorageUUID != next.StorageUUID || m.cfg.MaxSizeMB != next.MaxSizeMB || m.gateway != cfg.GatewayKey {
			w.cancel()
			<-w.done
			delete(m.workers, id)
		}
	}
	m.cfg = next
	m.gateway = cfg.GatewayKey
	for _, t := range next.Targets {
		if m.workers[t.ID] != nil {
			continue
		}
		ctx, cancel := context.WithCancel(m.ctx)
		w := &worker{cfg: next, target: t, gateway: cfg.GatewayKey, store: m.store, cancel: cancel, done: make(chan struct{}), status: Status{ID: t.ID, Enabled: t.Enabled}}
		m.workers[t.ID] = w
		go w.run(ctx)
	}
}
func (m *Manager) Statuses() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := []Status{}
	for _, t := range m.cfg.Targets {
		if w := m.workers[t.ID]; w != nil {
			w.mu.Lock()
			result = append(result, w.status)
			w.mu.Unlock()
		}
	}
	return result
}
func (m *Manager) ValidateChange(next config.RemoteStorageConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	wanted := map[string]config.RemoteStorageTarget{}
	for _, t := range next.Targets {
		wanted[t.ID] = t
	}
	for id, w := range m.workers {
		t, ok := wanted[id]
		changed := !ok || t.DestinationKey() != w.target.DestinationKey() || next.StoragePath != m.cfg.StoragePath || next.StorageUUID != m.cfg.StorageUUID || next.MaxSizeMB < m.cfg.MaxSizeMB
		if changed {
			w.mu.Lock()
			pending, cacheError, ready := w.status.Pending, w.status.CacheError, w.status.CacheReady
			w.mu.Unlock()
			if w.target.Enabled || !ready || pending > 0 || cacheError != "" {
				return fmt.Errorf("任务 %s 请先补传完成，再单独保存为停用状态；确认待补传为 0 后，才可删除、更换目标/缓存位置或缩小容量", id)
			}
		}
	}
	return nil
}
func resolveQueue(c config.RemoteStorageConfig, id string) (string, func() bool, error) {
	root := c.StoragePath
	available := func() bool { return true }
	if filepath.Clean(root) != filepath.Clean(config.DefaultHistoryStoragePath) {
		device, ok := storage.Resolve(root, c.StorageUUID)
		if !ok || device.ReadOnly {
			return "", nil, fmt.Errorf("绑定的缓存卷未挂载或只读，未回退到系统分区")
		}
		root = device.MountPath
		available = storage.Guard(device)
	}
	return filepath.Join(root, ".remote-storage", id+".db"), available, nil
}
func (w *worker) queueStatus() {
	count, bytes, err := w.queue.Count()
	w.mu.Lock()
	defer w.mu.Unlock()
	if err != nil {
		w.status.CacheError = SafeError(err, w.target)
	} else {
		w.status.CacheReady = true
		w.status.Pending = count
		w.status.Bytes = bytes
	}
}
func (w *worker) run(ctx context.Context) {
	defer close(w.done)
	for ctx.Err() == nil {
		path, available, err := resolveQueue(w.cfg, w.target.ID)
		if err == nil {
			if !w.target.Enabled {
				if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
					w.mu.Lock()
					w.status.CacheReady = true
					w.mu.Unlock()
					return
				}
			}
			_, free, spaceErr := storage.Space(filepath.Dir(path))
			_, fileErr := os.Stat(path)
			if spaceErr == nil && free < 4*1024*1024 && os.IsNotExist(fileErr) {
				err = fmt.Errorf("缓存分区可用空间不足 4MB，已暂停")
			} else if spaceErr != nil && spaceErr != storage.ErrSpaceUnsupported {
				err = spaceErr
			}
			if err == nil {
				w.queue, err = OpenQueue(path, w.target.DestinationKey(), int64(w.cfg.MaxSizeMB)*1024*1024, available)
			}
		}
		if err != nil {
			w.mu.Lock()
			w.status.CacheError = SafeError(err, w.target)
			w.status.CacheReady = false
			w.mu.Unlock()
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}
		w.mu.Lock()
		w.status.CacheError = ""
		w.mu.Unlock()
		w.queueStatus()
		if !w.target.Enabled {
			w.queue.Close()
			return
		}
		session, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); w.send(session) }()
		w.sample(session, path, available)
		cancel()
		wg.Wait()
		w.queue.Close()
		w.queue = nil
		if !wait(ctx, time.Second) {
			return
		}
	}
}
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func (w *worker) sample(ctx context.Context, path string, available func() bool) {
	selected := map[string]bool{}
	for _, p := range w.target.Points {
		selected[p.DeviceKey+"\x00"+p.Metric] = true
	}
	for ctx.Err() == nil {
		if !available() {
			return
		}
		var records []Record
		var sampleErr error
		var failed uint64
		payloadBytes := 0
		flush := func() {
			if len(records) == 0 {
				return
			}
			_, free, err := storage.Space(path)
			if err == nil && free < 4*1024*1024 {
				err = fmt.Errorf("缓存分区可用空间不足 4MB，新采样已暂停")
			}
			if err == storage.ErrSpaceUnsupported {
				err = nil
			}
			if err == nil {
				err = w.queue.Append(records)
			}
			if err != nil {
				sampleErr = err
				failed += uint64(len(records))
			}
			records = nil
			payloadBytes = 0
		}
		seen := 0
		for _, p := range w.store.PointStatuses() {
			if ctx.Err() != nil {
				return
			}
			if !selected[p.DeviceKey+"\x00"+p.Metric] {
				continue
			}
			seen++
			r, err := NewRecord(w.gateway, p, time.Now().UTC())
			if err != nil {
				sampleErr = err
				failed++
				continue
			}
			records = append(records, r)
			payloadBytes += len(r.ValueJSON) + len(r.Diagnostic) + 1024
			if len(records) >= 128 || payloadBytes >= 512*1024 {
				flush()
			}
		}
		if seen < len(selected) {
			sampleErr = fmt.Errorf("%d 个所选点位尚无运行状态或已被删除，请检查点位配置", len(selected)-seen)
			failed += uint64(len(selected) - seen)
		}
		flush()
		w.mu.Lock()
		if sampleErr != nil {
			w.status.CacheError = SafeError(sampleErr, w.target)
		} else {
			w.status.CacheError = ""
		}
		w.status.Dropped += failed
		w.mu.Unlock()
		w.queueStatus()
		for i := 0; i < w.target.IntervalSeconds; i++ {
			if !wait(ctx, time.Second) || !available() {
				return
			}
		}
	}
}
func (w *worker) send(ctx context.Context) {
	var adapter Adapter
	defer func() {
		if adapter != nil {
			adapter.Close()
		}
	}()
	backoff := time.Second
	for ctx.Err() == nil {
		batch, err := w.queue.Peek(w.target.BatchSize)
		if err == nil && len(batch) == 0 {
			w.queueStatus()
			if !wait(ctx, time.Second) {
				return
			}
			continue
		}
		if err == nil {
			call, cancel := context.WithTimeout(ctx, time.Duration(w.target.TimeoutSeconds)*time.Second)
			if adapter == nil {
				adapter, err = Open(w.target)
				if err == nil {
					err = adapter.Prepare(call)
				}
			}
			if err == nil {
				records := make([]Record, len(batch))
				for i, b := range batch {
					records[i] = b.Record
				}
				err = adapter.Write(call, records)
			}
			cancel()
			if err == nil {
				err = w.queue.Ack(batch)
			} // Remote commit first; replay is idempotent after a crash.
		}
		w.mu.Lock()
		if err != nil {
			w.status.Connected = false
			w.status.LastError = SafeError(err, w.target)
			next := time.Now().Add(backoff)
			w.status.NextRetry = &next
		} else {
			now := time.Now()
			w.status.Connected = true
			w.status.LastSuccess = &now
			w.status.LastError = ""
			w.status.NextRetry = nil
			w.status.Sent += uint64(len(batch))
		}
		w.mu.Unlock()
		w.queueStatus()
		if err != nil {
			if adapter != nil {
				adapter.Close()
				adapter = nil
			}
			if !wait(ctx, backoff) {
				return
			}
			if backoff < 60*time.Second {
				backoff *= 2
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
			}
		} else {
			backoff = time.Second
			if !wait(ctx, time.Second) {
				return
			}
		}
	}
}

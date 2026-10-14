package collectdiag

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"weikong-iot-platform/apps/gateway-go/internal/state"
)

const retentionLimit = 10000

type Incident struct {
	ID          int64      `json:"id"`
	DeviceKey   string     `json:"deviceKey"`
	PointName   string     `json:"pointName"`
	Metric      string     `json:"metric"`
	Protocol    string     `json:"protocol"`
	Address     string     `json:"address"`
	Reason      string     `json:"reason"`
	Explanation string     `json:"explanation"`
	State       string     `json:"state"`
	Occurrences int64      `json:"occurrences"`
	OccurredAt  time.Time  `json:"occurredAt"`
	LastSeenAt  time.Time  `json:"lastSeenAt"`
	RecoveredAt *time.Time `json:"recoveredAt,omitempty"`
}

type Query struct {
	Page     int
	PageSize int
	State    string
	Protocol string
	Keyword  string
}

type Page struct {
	Items      []Incident `json:"items"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
	Total      int        `json:"total"`
	TotalPages int        `json:"totalPages"`
	Dropped    uint64     `json:"dropped,omitempty"`
	Path       string     `json:"path,omitempty"`
	LastError  string     `json:"lastError,omitempty"`
}

type command struct {
	event *state.PointIncidentEvent
	flush chan struct{}
}

type Store struct {
	db        *sql.DB
	path      string
	queue     chan command
	done      chan struct{}
	closeOnce sync.Once
	queueMu   sync.RWMutex
	closed    bool
	dropped   atomic.Uint64
	errorMu   sync.RWMutex
	lastError string
}

const schema = `
CREATE TABLE IF NOT EXISTS collection_incidents (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_key TEXT NOT NULL,
  point_name TEXT NOT NULL DEFAULT '',
  metric TEXT NOT NULL,
  protocol TEXT NOT NULL DEFAULT '',
  address TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL,
  explanation TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL,
  occurrences INTEGER NOT NULL DEFAULT 1,
  occurred_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  recovered_at TEXT
);
CREATE INDEX IF NOT EXISTS collection_incidents_time ON collection_incidents(occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS collection_incidents_point ON collection_incidents(device_key, metric, state);
CREATE INDEX IF NOT EXISTS collection_incidents_state ON collection_incidents(state, occurred_at DESC);
`

func Open(configPath string) (*Store, error) {
	dir := filepath.Join(filepath.Dir(configPath), ".runtime")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "collection-incidents.db")
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err = db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Older builds could persist the collector context cancellation produced
	// by a deliberate service stop. It is lifecycle noise, not a collection
	// failure, so remove those rows while upgrading the database.
	if _, err = db.Exec(`DELETE FROM collection_incidents WHERE LOWER(reason) LIKE '%context canceled%' OR LOWER(reason) LIKE '%operation canceled%' OR LOWER(reason) LIKE '%operation cancelled%'`); err != nil {
		_ = db.Close()
		return nil, err
	}
	// A process restart cannot prove recovery. Preserve the old failure as an
	// interrupted observation instead of silently presenting it as active.
	now := sqliteTime(time.Now())
	if _, err = db.Exec(`UPDATE collection_incidents SET state='INTERRUPTED', last_seen_at=? WHERE state='ACTIVE'`, now); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &Store{db: db, path: path, queue: make(chan command, 256), done: make(chan struct{})}
	go store.run()
	return store, nil
}

func (s *Store) RecordPointIncident(event state.PointIncidentEvent) {
	if s == nil || s.db == nil || strings.TrimSpace(event.DeviceKey) == "" || strings.TrimSpace(event.Metric) == "" {
		return
	}
	if event.Active && isLifecycleCancellation(event.Reason) {
		return
	}
	copy := event
	if copy.At.IsZero() {
		copy.At = time.Now()
	}
	s.queueMu.RLock()
	if !s.closed {
		select {
		case s.queue <- command{event: &copy}:
		default:
			s.dropped.Add(1)
		}
	}
	s.queueMu.RUnlock()
}

func isLifecycleCancellation(reason string) bool {
	value := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(value, "context canceled") || strings.Contains(value, "operation canceled") || strings.Contains(value, "operation cancelled")
}

func (s *Store) run() {
	defer close(s.done)
	for command := range s.queue {
		if command.event != nil {
			if err := s.apply(*command.event); err != nil {
				s.setError(err)
			} else {
				s.setError(nil)
			}
		}
		if command.flush != nil {
			close(command.flush)
		}
	}
}

func (s *Store) apply(event state.PointIncidentEvent) error {
	if !event.Active {
		_, err := s.db.Exec(`UPDATE collection_incidents SET state='RECOVERED', recovered_at=?, last_seen_at=? WHERE device_key=? AND metric=? AND state='ACTIVE'`, sqliteTime(event.At), sqliteTime(event.At), event.DeviceKey, event.Metric)
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var activeID int64
	var activeReason string
	err = tx.QueryRow(`SELECT id, reason FROM collection_incidents WHERE device_key=? AND metric=? AND state='ACTIVE' ORDER BY id DESC LIMIT 1`, event.DeviceKey, event.Metric).Scan(&activeID, &activeReason)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && activeReason == event.Reason {
		if _, err = tx.Exec(`UPDATE collection_incidents SET last_seen_at=?, occurrences=occurrences+1 WHERE id=?`, sqliteTime(event.At), activeID); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err == nil {
		if _, err = tx.Exec(`UPDATE collection_incidents SET state='CHANGED', recovered_at=?, last_seen_at=? WHERE id=?`, sqliteTime(event.At), sqliteTime(event.At), activeID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO collection_incidents(device_key,point_name,metric,protocol,address,reason,explanation,state,occurrences,occurred_at,last_seen_at) VALUES(?,?,?,?,?,?,?,?,1,?,?)`,
		event.DeviceKey, event.PointName, event.Metric, event.Protocol, event.Address, event.Reason, Explain(event.Reason), "ACTIVE", sqliteTime(event.At), sqliteTime(event.At)); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM collection_incidents WHERE id <= COALESCE((SELECT id FROM collection_incidents ORDER BY id DESC LIMIT 1 OFFSET ?), 0)`, retentionLimit); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Flush(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	done := make(chan struct{})
	s.queueMu.RLock()
	if s.closed {
		s.queueMu.RUnlock()
		return nil
	}
	select {
	case s.queue <- command{flush: done}:
		s.queueMu.RUnlock()
	case <-ctx.Done():
		s.queueMu.RUnlock()
		return ctx.Err()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Store) Query(query Query) (Page, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 50
	}
	if query.PageSize > 200 {
		query.PageSize = 200
	}
	clauses := make([]string, 0, 3)
	args := make([]interface{}, 0, 8)
	if value := strings.ToUpper(strings.TrimSpace(query.State)); value != "" && value != "ALL" {
		clauses = append(clauses, "state=?")
		args = append(args, value)
	}
	if value := strings.ToLower(strings.TrimSpace(query.Protocol)); value != "" {
		clauses = append(clauses, "LOWER(protocol)=?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Keyword); value != "" {
		pattern := "%" + value + "%"
		clauses = append(clauses, `(device_key LIKE ? OR point_name LIKE ? OR metric LIKE ? OR address LIKE ? OR reason LIKE ? OR explanation LIKE ?)`)
		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM collection_incidents`+where, args...).Scan(&total); err != nil {
		return Page{}, err
	}
	rows, err := s.db.Query(`SELECT id,device_key,point_name,metric,protocol,address,reason,explanation,state,occurrences,occurred_at,last_seen_at,recovered_at FROM collection_incidents`+where+` ORDER BY occurred_at DESC,id DESC LIMIT ? OFFSET ?`, append(args, query.PageSize, (query.Page-1)*query.PageSize)...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := make([]Incident, 0, query.PageSize)
	for rows.Next() {
		var item Incident
		var occurredAt, lastSeenAt string
		var recoveredAt sql.NullString
		if err = rows.Scan(&item.ID, &item.DeviceKey, &item.PointName, &item.Metric, &item.Protocol, &item.Address, &item.Reason, &item.Explanation, &item.State, &item.Occurrences, &occurredAt, &lastSeenAt, &recoveredAt); err != nil {
			return Page{}, err
		}
		if item.OccurredAt, err = time.Parse(time.RFC3339Nano, occurredAt); err != nil {
			return Page{}, err
		}
		if item.LastSeenAt, err = time.Parse(time.RFC3339Nano, lastSeenAt); err != nil {
			return Page{}, err
		}
		if recoveredAt.Valid {
			value, parseErr := time.Parse(time.RFC3339Nano, recoveredAt.String)
			if parseErr != nil {
				return Page{}, parseErr
			}
			item.RecoveredAt = &value
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return Page{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	s.errorMu.RLock()
	lastError := s.lastError
	s.errorMu.RUnlock()
	return Page{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total, TotalPages: totalPages, Dropped: s.dropped.Load(), Path: s.path, LastError: lastError}, nil
}

func (s *Store) Clear(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.Flush(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM collection_incidents`)
	return err
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		s.queueMu.Lock()
		s.closed = true
		close(s.queue)
		s.queueMu.Unlock()
		<-s.done
		err = s.db.Close()
	})
	return err
}

func (s *Store) setError(err error) {
	s.errorMu.Lock()
	if err == nil {
		s.lastError = ""
	} else {
		s.lastError = err.Error()
	}
	s.errorMu.Unlock()
}

func sqliteTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func Explain(reason string) string {
	value := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(value, "0x800706ba"), strings.Contains(value, "rpc server is unavailable"), strings.Contains(value, "rpc server unavailable"):
		return "OPC DA/DCOM 的 RPC 服务不可用，请检查 OPC Server、TCP 135、动态 RPC 端口和防火墙。"
	case strings.Contains(value, "0x80070005"), strings.Contains(value, "e_accessdenied"):
		return "远程访问被拒绝，请检查账号密码、DCOM 权限及本机/域安全策略。"
	case strings.Contains(value, "opc_e_unknownitemid"), strings.Contains(value, "0xc0040007"):
		return "OPC DA Item ID 不存在，请重新浏览或核对点位 Item ID。"
	case strings.Contains(value, "deadline exceeded"), strings.Contains(value, "timeout"), strings.Contains(value, "timed out"):
		return "连接或读取超时，请检查设备是否在线、网络链路、端口和超时参数。"
	case strings.Contains(value, "connection refused"):
		return "目标设备拒绝连接，请检查服务是否启动以及端口配置是否正确。"
	case strings.Contains(value, "no route to host"), strings.Contains(value, "network is unreachable"), strings.Contains(value, "host is unreachable"):
		return "目标设备网络不可达，请检查 IP、网关、网线和路由配置。"
	case strings.Contains(value, "authentication"), strings.Contains(value, "unauthorized"), strings.Contains(value, "access denied"), strings.Contains(value, "logon failure"):
		return "身份认证或访问权限失败，请检查账号、密码、安全模式和设备端授权。"
	case strings.Contains(value, "modbus exception"):
		return "设备返回 Modbus 异常响应，请检查功能码、寄存器范围、站号和读写权限。"
	case strings.Contains(value, "x509"), strings.Contains(value, "certificate"):
		return "安全证书校验失败，请检查证书有效期、信任链、主机名和设备时间。"
	case strings.Contains(value, "no such file or directory"), strings.Contains(value, "serial port"):
		return "串口设备不存在或无法打开，请检查串口路径、接线、占用状态和访问权限。"
	case strings.Contains(value, "not found"), strings.Contains(value, "invalid address"), strings.Contains(value, "unknown item"), strings.Contains(value, "bad node"):
		return "点位地址不存在或配置无效，请核对点位地址、数据类型和设备模型。"
	case strings.Contains(value, "connection reset"), strings.Contains(value, "broken pipe"), strings.Contains(value, "unexpected eof"), strings.Contains(value, "closed network connection"), value == "eof":
		return "采集连接被中断，请检查设备重启、网络抖动或服务主动断开。"
	default:
		return "采集请求失败，请检查设备连接、协议参数、点位地址和设备端运行状态。"
	}
}

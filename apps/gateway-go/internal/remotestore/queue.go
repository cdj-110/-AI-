package remotestore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

type Queue struct {
	mu        sync.Mutex
	db        *sql.DB
	available func() bool
	capacity  int64
}
type queued struct {
	Sequence int64
	Record   Record
}

func OpenQueue(path, identity string, maxBytes int64, available func() bool) (*Queue, error) {
	if !available() {
		return nil, fmt.Errorf("缓存存储不可用，未回退到系统分区")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	q := u.Query()
	q.Set("_busy_timeout", "2000")
	q.Set("_journal_mode", "DELETE")
	q.Set("_synchronous", "FULL")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Queue, error) { db.Close(); return nil, err }
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS remote_queue(seq INTEGER PRIMARY KEY AUTOINCREMENT,payload TEXT NOT NULL,bytes INTEGER NOT NULL);CREATE TABLE IF NOT EXISTS remote_meta(key TEXT PRIMARY KEY,value TEXT NOT NULL);`); err != nil {
		return fail(err)
	}
	os.Chmod(path, 0600)
	var prior string
	err = db.QueryRow("SELECT value FROM remote_meta WHERE key='destination'").Scan(&prior)
	if err != nil && err != sql.ErrNoRows {
		return fail(err)
	}
	var count int64
	if err = db.QueryRow("SELECT count(*) FROM remote_queue").Scan(&count); err != nil {
		return fail(err)
	}
	if prior != identity && count > 0 {
		return fail(fmt.Errorf("缓存仍属于原数据库，禁止将未补传数据发送到新目标"))
	}
	if _, err = db.Exec("INSERT OR REPLACE INTO remote_meta(key,value) VALUES('destination',?)", identity); err != nil {
		return fail(err)
	}
	// Hard SQLite page bound as well as payload quota. Deleted pages are reused.
	var size int64
	if err = db.QueryRow("PRAGMA page_size").Scan(&size); err != nil {
		return fail(err)
	}
	var pages int64
	if err = db.QueryRow(fmt.Sprintf("PRAGMA max_page_count=%d", maxBytes/size)).Scan(&pages); err != nil {
		return fail(err)
	}
	if pages*size > maxBytes {
		return fail(fmt.Errorf("缓存文件已大于新容量，请保留原容量；不能自动缩小或删除现有缓存"))
	}
	return &Queue{db: db, available: available, capacity: maxBytes / 2}, nil
}
func (q *Queue) check() error {
	if !q.available() {
		return fmt.Errorf("缓存卷已断开或重新挂载，等待重新连接")
	}
	return nil
}
func (q *Queue) Append(records []Record) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.check(); err != nil {
		return err
	}
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var used int64
	if err = tx.QueryRow("SELECT COALESCE(sum(bytes),0) FROM remote_queue").Scan(&used); err != nil {
		return err
	}
	for _, r := range records {
		payload, err := json.Marshal(r)
		if err != nil {
			return err
		}
		used += int64(len(payload))
		if used > q.capacity {
			return fmt.Errorf("远程存储缓存已满，停止接收新采样；已有待补传数据保留")
		}
		if _, err = tx.Exec("INSERT INTO remote_queue(payload,bytes) VALUES(?,?)", string(payload), len(payload)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (q *Queue) Peek(limit int) ([]queued, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.check(); err != nil {
		return nil, err
	}
	rows, err := q.db.Query("SELECT seq,payload FROM remote_queue ORDER BY seq LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []queued
	loadedBytes := 0
	for rows.Next() {
		var item queued
		var raw string
		if err = rows.Scan(&item.Sequence, &raw); err != nil {
			return nil, err
		}
		// Bound in-flight memory even when a configured batch contains many
		// large text values. Remaining rows stay durable for the next batch.
		if len(result) > 0 && loadedBytes+len(raw) > 2*1024*1024 {
			break
		}
		loadedBytes += len(raw)
		if err = json.Unmarshal([]byte(raw), &item.Record); err != nil {
			return nil, fmt.Errorf("缓存内容损坏，停止补传: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (q *Queue) Ack(batch []queued) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.check(); err != nil {
		return err
	}
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range batch {
		if _, err = tx.Exec("DELETE FROM remote_queue WHERE seq=?", item.Sequence); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (q *Queue) Count() (int64, int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.check(); err != nil {
		return 0, 0, err
	}
	var count, bytes int64
	err := q.db.QueryRow("SELECT count(*),COALESCE(sum(bytes),0) FROM remote_queue").Scan(&count, &bytes)
	return count, bytes, err
}
func (q *Queue) Close() error { q.mu.Lock(); defer q.mu.Unlock(); return q.db.Close() }

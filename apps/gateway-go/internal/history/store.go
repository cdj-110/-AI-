package history

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

var ErrDisabled = errors.New("history storage is disabled")

type Store struct {
	mu        sync.Mutex
	dir       string
	maxBytes  int64
	enabled   bool
	available func() bool
}

type Status struct {
	Enabled  bool   `json:"enabled"`
	Path     string `json:"path,omitempty"`
	Bytes    int64  `json:"bytes"`
	MaxBytes int64  `json:"maxBytes"`
	Files    int    `json:"files"`
}

func NewGuarded(dir string, maxBytes int64, available func() bool) *Store {
	return &Store{dir: dir, maxBytes: maxBytes, enabled: dir != "", available: available}
}

func Disabled() *Store {
	return &Store{}
}

func Path(mountPath string) string {
	return filepath.Join(mountPath, ".weikong", "history")
}

func (s *Store) Append(reading model.Reading) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return ErrDisabled
	}
	raw, err := json.Marshal(reading)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	if s.maxBytes > 0 {
		if err := s.makeRoom(int64(len(raw))); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(filepath.Join(s.dir, historyFileName(reading.Time)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (s *Store) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Store) ExportCSV(writer io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() {
		return ErrDisabled
	}
	files, err := s.historyFilesLocked()
	if err != nil {
		return err
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"time", "deviceKey", "metric", "value"}); err != nil {
		return err
	}
	for _, file := range files {
		if err := exportFileCSV(csvWriter, file); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return nil
	}
	files, err := s.historyFilesLocked()
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func exportFileCSV(writer *csv.Writer, path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 2*1024*1024)
	for scanner.Scan() {
		var reading model.Reading
		if err := json.Unmarshal(scanner.Bytes(), &reading); err != nil {
			continue
		}
		keys := make([]string, 0, len(reading.Metrics))
		for key := range reading.Metrics {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := writer.Write([]string{
				reading.Time.Format(time.RFC3339Nano),
				reading.DeviceKey,
				key,
				fmt.Sprint(reading.Metrics[key]),
			}); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (s *Store) makeRoom(incoming int64) error {
	if incoming > s.maxBytes {
		return errors.New("history record exceeds configured capacity")
	}
	status := s.statusLocked()
	if status.Bytes+incoming <= s.maxBytes {
		return nil
	}
	files, err := s.historyFilesLocked()
	if err != nil {
		return err
	}
	target := s.maxBytes * 9 / 10
	if maximumTarget := s.maxBytes - incoming; target > maximumTarget {
		target = maximumTarget
	}
	current := status.Bytes
	for _, file := range files {
		if current <= target {
			break
		}
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
		current -= info.Size()
	}
	return nil
}

func (s *Store) statusLocked() Status {
	status := Status{Enabled: s.isAvailable(), Path: s.dir, MaxBytes: s.maxBytes}
	files, err := s.historyFilesLocked()
	if err != nil {
		return status
	}
	status.Files = len(files)
	for _, file := range files {
		if info, err := os.Stat(file); err == nil {
			status.Bytes += info.Size()
		}
	}
	return status
}

func (s *Store) historyFilesLocked() ([]string, error) {
	if s.dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		files = append(files, filepath.Join(s.dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func (s *Store) isAvailable() bool {
	return s.enabled && (s.available == nil || s.available())
}

func historyFileName(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return "history-" + t.Format("20060102") + ".jsonl"
}

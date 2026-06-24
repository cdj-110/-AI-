package cache

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

var ErrDisabled = errors.New("offline cache is disabled")

type Spool struct {
	mu        sync.Mutex
	path      string
	maxBytes  int64
	enabled   bool
	available func() bool
}

type Status struct {
	Enabled  bool   `json:"enabled"`
	Path     string `json:"path,omitempty"`
	Bytes    int64  `json:"bytes"`
	MaxBytes int64  `json:"maxBytes"`
}

func New(path string) *Spool {
	return NewLimited(path, 0)
}

func NewLimited(path string, maxBytes int64) *Spool {
	return &Spool{path: path, maxBytes: maxBytes, enabled: path != ""}
}

func NewGuarded(path string, maxBytes int64, available func() bool) *Spool {
	return &Spool{path: path, maxBytes: maxBytes, enabled: path != "", available: available}
}

func Disabled() *Spool {
	return &Spool{}
}

func (s *Spool) Append(reading model.Reading) error {
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
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	if s.maxBytes > 0 {
		if err := s.makeRoom(int64(len(raw))); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
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

func (s *Spool) Drain(publish func(model.Reading) error) error {
	return s.DrainLimit(publish, int(^uint(0)>>1))
}

func (s *Spool) DrainLimit(publish func(model.Reading) error, limit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isAvailable() || limit <= 0 {
		return nil
	}
	input, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	tempPath := s.path + ".drain"
	output, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		_ = input.Close()
		return err
	}
	scanner := bufio.NewScanner(input)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 2*1024*1024)
	attempted := 0
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var reading model.Reading
		valid := json.Unmarshal(line, &reading) == nil
		if valid && attempted < limit {
			attempted++
			if publish(reading) == nil {
				continue
			}
		}
		if valid {
			if _, err := output.Write(append(line, '\n')); err != nil {
				_ = input.Close()
				_ = output.Close()
				return err
			}
		}
	}
	scanErr := scanner.Err()
	closeInputErr := input.Close()
	closeOutputErr := output.Close()
	if scanErr != nil {
		return scanErr
	}
	if closeInputErr != nil {
		return closeInputErr
	}
	if closeOutputErr != nil {
		return closeOutputErr
	}
	info, err := os.Stat(tempPath)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		_ = os.Remove(tempPath)
		return os.Remove(s.path)
	}
	return replaceFile(tempPath, s.path)
}

func (s *Spool) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := Status{Enabled: s.isAvailable(), Path: s.path, MaxBytes: s.maxBytes}
	if info, err := os.Stat(s.path); err == nil {
		status.Bytes = info.Size()
	}
	return status
}

func (s *Spool) makeRoom(incoming int64) error {
	if incoming > s.maxBytes {
		return errors.New("offline cache record exceeds configured capacity")
	}
	info, err := os.Stat(s.path)
	if os.IsNotExist(err) || info == nil || info.Size()+incoming <= s.maxBytes {
		return nil
	}
	if err != nil {
		return err
	}
	target := s.maxBytes * 3 / 4
	if maximumTarget := s.maxBytes - incoming; target > maximumTarget {
		target = maximumTarget
	}
	input, err := os.Open(s.path)
	if err != nil {
		return err
	}
	start := info.Size() - target
	if start < 0 {
		start = 0
	}
	if _, err := input.Seek(start, io.SeekStart); err != nil {
		_ = input.Close()
		return err
	}
	reader := bufio.NewReader(input)
	if start > 0 {
		_, _ = reader.ReadBytes('\n')
	}
	tempPath := s.path + ".trim"
	output, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		_ = input.Close()
		return err
	}
	_, copyErr := io.Copy(output, reader)
	_ = input.Close()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(tempPath, s.path)
}

func (s *Spool) isAvailable() bool {
	return s.enabled && (s.available == nil || s.available())
}

func replaceFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	_ = os.Remove(target)
	return os.Rename(source, target)
}

package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
)

// alarmEventSpool is an at-least-once queue. A cloud duplicate is possible
// only when the process stops after MQTT PUBACK but before the queue rewrite;
// the cloud dedupeKey makes that retry harmless.
type alarmEventSpool struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
}

func newAlarmEventSpool(path string, maxBytes int64) *alarmEventSpool {
	return &alarmEventSpool{path: path, maxBytes: maxBytes}
}

func (s *alarmEventSpool) append(event alarm.Event) error {
	if s == nil || s.path == "" {
		return errors.New("alarm event spool is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	if err := s.makeRoom(int64(len(raw))); err != nil {
		return err
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

func (s *alarmEventSpool) drain(limit int, publish func(alarm.Event) error) error {
	if s == nil || s.path == "" || limit <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	attempted := 0
	blocked := false
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var event alarm.Event
		valid := json.Unmarshal(line, &event) == nil && event.EventID != ""
		if valid && !blocked && attempted < limit {
			attempted++
			if publish(event) == nil {
				continue
			}
			blocked = true
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
	return replaceAlarmEventFile(tempPath, s.path)
}

func (s *alarmEventSpool) makeRoom(incoming int64) error {
	if s.maxBytes <= 0 {
		return nil
	}
	if incoming > s.maxBytes {
		return errors.New("alarm event exceeds spool capacity")
	}
	info, err := os.Stat(s.path)
	if os.IsNotExist(err) || (err == nil && info.Size()+incoming <= s.maxBytes) {
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
	return replaceAlarmEventFile(tempPath, s.path)
}

func replaceAlarmEventFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	_ = os.Remove(target)
	return os.Rename(source, target)
}

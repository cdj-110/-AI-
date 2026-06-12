package cache

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type Spool struct {
	path string
}

func New(path string) Spool {
	return Spool{path: path}
}

func (s Spool) Append(reading model.Reading) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	raw, err := json.Marshal(reading)
	if err != nil {
		return err
	}
	_, err = file.Write(append(raw, '\n'))
	return err
}

func (s Spool) Drain(publish func(model.Reading) error) error {
	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	var failed []model.Reading
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var reading model.Reading
		if err := json.Unmarshal(scanner.Bytes(), &reading); err != nil {
			continue
		}
		if err := publish(reading); err != nil {
			failed = append(failed, reading)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, reading := range failed {
		if err := s.Append(reading); err != nil {
			return err
		}
	}
	return nil
}

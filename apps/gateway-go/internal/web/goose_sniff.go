package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/goose"
)

func (s *Server) gooseSniffSession(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	switch request.Method {
	case http.MethodGet:
		_ = json.NewEncoder(writer).Encode(s.gooseSniff.Snapshot())
	case http.MethodDelete:
		s.gooseSniff.Stop()
		_ = json.NewEncoder(writer).Encode(s.gooseSniff.Snapshot())
	case http.MethodPost:
		var body struct {
			DeviceKey       string `json:"deviceKey"`
			DurationSeconds int    `json:"durationSeconds"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16*1024))
		if err := decoder.Decode(&body); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		expected, err := s.gooseSniffExpected(strings.TrimSpace(body.DeviceKey))
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		duration := time.Duration(body.DurationSeconds) * time.Second
		if err := s.gooseSniff.Start(expected.InterfaceName, duration, expected); err != nil {
			http.Error(writer, err.Error(), http.StatusConflict)
			return
		}
		writer.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(writer).Encode(s.gooseSniff.Snapshot())
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) gooseSniffExpected(deviceKey string) (goose.SniffExpected, error) {
	if deviceKey == "" {
		return goose.SniffExpected{}, fmt.Errorf("请选择 GOOSE 采集或发布设备")
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return goose.SniffExpected{}, fmt.Errorf("读取当前配置失败: %w", err)
	}
	for _, device := range cfg.Devices {
		if device.DeviceKey != deviceKey {
			continue
		}
		if !strings.EqualFold(device.Protocol, "iec61850-goose") {
			return goose.SniffExpected{}, fmt.Errorf("当前设备不是 IEC61850 GOOSE 采集设备")
		}
		interfaceName := strings.TrimSpace(device.Address)
		if interfaceName == "" {
			interfaceName = strings.TrimSpace(device.InterfaceName)
		}
		return goose.SniffExpected{
			DeviceKey:      device.DeviceKey,
			InterfaceName:  interfaceName,
			DestinationMAC: strings.TrimSpace(device.DestinationMAC),
			AppID:          device.AppID,
			GoCBRef:        strings.TrimSpace(device.GoCBRef),
			DataSetRef:     strings.TrimSpace(device.DataSetRef),
			VLANID:         device.VLANID,
		}, nil
	}
	for _, device := range cfg.ForwardDevices {
		if device.DeviceKey != deviceKey {
			continue
		}
		if !strings.EqualFold(device.Protocol, "iec61850-goose-publisher") {
			return goose.SniffExpected{}, fmt.Errorf("当前设备不是 IEC61850 GOOSE 发布设备")
		}
		return goose.SniffExpected{
			DeviceKey:      device.DeviceKey,
			InterfaceName:  strings.TrimSpace(device.InterfaceName),
			DestinationMAC: strings.TrimSpace(device.DestinationMAC),
			AppID:          device.AppID,
			GoCBRef:        strings.TrimSpace(device.GoCBRef),
			DataSetRef:     strings.TrimSpace(device.DataSetRef),
			VLANID:         device.VLANID,
		}, nil
	}
	return goose.SniffExpected{}, fmt.Errorf("未找到设备 %s", deviceKey)
}

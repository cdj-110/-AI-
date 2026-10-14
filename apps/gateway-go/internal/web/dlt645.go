package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func (s *Server) dlt645Dictionary(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"items": collector.DLT645Dictionary()})
}

func (s *Server) dlt645ReadAddress(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body connectionTestRequest
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024)).Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	point := config.PointConfig{DeviceKey: body.DeviceKey, Protocol: "dlt645", Address: body.Address, BaudRate: body.BaudRate, DataBits: body.DataBits, StopBits: body.StopBits, Parity: body.Parity, DLT645: body.DLT645}
	point.ApplyDefaults()
	ctx, cancel := context.WithTimeout(request.Context(), 12*time.Second)
	defer cancel()
	address, err := collector.ReadDLT645Address(ctx, point)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "meterAddress": address, "detectedVersion": "2007", "message": "已读取通信地址，请确认总线上仅连接这一只电表。"})
}

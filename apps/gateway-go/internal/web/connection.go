package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type connectionTestRequest struct {
	Protocol   string `json:"protocol"`
	Address    string `json:"address"`
	SlaveID    byte   `json:"slaveId"`
	Rack       uint8  `json:"rack"`
	Slot       uint8  `json:"slot"`
	LocalTSAP  string `json:"localTsap"`
	RemoteTSAP string `json:"remoteTsap"`
}

func (s *Server) testConnection(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 64*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var body connectionTestRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if body.Protocol == "siemens-s7" {
		result, err := collector.TestS7Connection(ctx, config.PointConfig{
			Protocol:   "siemens-s7",
			Address:    body.Address,
			Rack:       body.Rack,
			Slot:       body.Slot,
			LocalTSAP:  body.LocalTSAP,
			RemoteTSAP: body.RemoteTSAP,
		})
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": result.Address, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok":      true,
			"address": result.Address,
			"message": fmt.Sprintf("连接成功，已完成 S7 握手。参数：%s LocalTSAP=%s RemoteTSAP=%s", result.Endpoint, result.LocalTSAP, result.RemoteTSAP),
		})
		return
	}

	if body.Protocol == "iec104" {
		result, err := collector.TestIEC104Connection(ctx, body.Address, uint16(body.SlaveID))
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": result.Address, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok":      true,
			"address": result.Address,
			"message": fmt.Sprintf("连接成功，已完成 IEC104 STARTDT 握手。公共地址：%d", result.CommonAddress),
		})
		return
	}

	address := normalizeConnectionAddress(body.Protocol, body.Address)
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": address, "message": fmt.Sprintf("连接失败：%v", err)})
		return
	}
	_ = conn.Close()
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "address": address, "message": "连接成功，TCP 端口可达。"})
}

func normalizeConnectionAddress(protocol string, address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		switch protocol {
		case "iec104":
			return "127.0.0.1:2404"
		case "modbus-tcp":
			return "127.0.0.1:502"
		default:
			return "127.0.0.1"
		}
	}
	if strings.Contains(address, ":") {
		return address
	}
	switch protocol {
	case "iec104":
		return address + ":2404"
	case "modbus-tcp":
		return address + ":502"
	default:
		return address
	}
}

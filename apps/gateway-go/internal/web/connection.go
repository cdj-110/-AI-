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
	DeviceKey                  string               `json:"deviceKey"`
	Protocol                   string               `json:"protocol"`
	Address                    string               `json:"address"`
	SlaveID                    byte                 `json:"slaveId"`
	Function                   uint8                `json:"function"`
	Register                   uint16               `json:"register"`
	CommonAddress              uint16               `json:"commonAddress"`
	Rack                       uint8                `json:"rack"`
	Slot                       uint8                `json:"slot"`
	LocalTSAP                  string               `json:"localTsap"`
	RemoteTSAP                 string               `json:"remoteTsap"`
	Username                   string               `json:"username"`
	Password                   string               `json:"password"`
	ProgID                     string               `json:"progId"`
	CLSID                      string               `json:"clsid"`
	Domain                     string               `json:"domain"`
	Workstation                string               `json:"workstation"`
	SecurityMode               string               `json:"securityMode"`
	Authentication             string               `json:"authentication"`
	AuthenticationLevel        string               `json:"authenticationLevel"`
	ConnectTimeoutMilliseconds int                  `json:"connectTimeoutMilliseconds"`
	RequestTimeoutMilliseconds int                  `json:"requestTimeoutMilliseconds"`
	GroupName                  string               `json:"groupName"`
	RequestedUpdateRateMS      int                  `json:"requestedUpdateRateMilliseconds"`
	Deadband                   float64              `json:"deadband"`
	LocaleID                   uint32               `json:"localeId"`
	LocalBindIP                string               `json:"localBindIp"`
	BaudRate                   int                  `json:"baudRate"`
	DataBits                   int                  `json:"dataBits"`
	StopBits                   int                  `json:"stopBits"`
	Parity                     string               `json:"parity"`
	DLT645                     *config.DLT645Config `json:"dlt645,omitempty"`
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
	if config.NormalizeProtocol(body.Protocol) == "opcda" && body.Password == "***" {
		if s.runtime == nil || !resolveMaskedOPCDAPassword(&body, s.runtime.Config()) {
			http.Error(writer, "无法从当前设备配置解析已隐藏的 OPC DA 密码", http.StatusBadRequest)
			return
		}
	}

	testTimeout := 8 * time.Second
	if config.NormalizeProtocol(body.Protocol) == "opcda" {
		requested := time.Duration(body.ConnectTimeoutMilliseconds+body.RequestTimeoutMilliseconds)*time.Millisecond + 2*time.Second
		if requested > testTimeout {
			testTimeout = requested
		}
		if testTimeout > 60*time.Second {
			testTimeout = 60 * time.Second
		}
	}
	ctx, cancel := context.WithTimeout(request.Context(), testTimeout)
	defer cancel()

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if config.NormalizeProtocol(body.Protocol) == "modbus-rtu" {
		result, err := testRTUConnection(ctx, body)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": body.Address, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(writer).Encode(result)
		return
	}
	if config.NormalizeProtocol(body.Protocol) == "dlt645" {
		point := config.PointConfig{DeviceKey: body.DeviceKey, Protocol: "dlt645", Address: body.Address, BaudRate: body.BaudRate, DataBits: body.DataBits, StopBits: body.StopBits, Parity: body.Parity, DLT645: body.DLT645}
		point.ApplyDefaults()
		result, err := collector.TestDLT645Connection(ctx, point)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": body.Address, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "address": result.Address, "meterAddress": result.MeterAddress, "configuredVersion": result.ConfiguredVersion, "detectedVersion": result.DetectedVersion, "elapsedMilliseconds": result.ElapsedMS, "message": fmt.Sprintf("DL/T 645-%s 通信成功，表地址 %s，响应 %d ms。", result.DetectedVersion, result.MeterAddress, result.ElapsedMS)})
		return
	}
	if config.NormalizeProtocol(body.Protocol) == "iec61850" {
		if !iec61850Enabled {
			writer.WriteHeader(http.StatusNotImplemented)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "message": "当前网关程序未包含 IEC 61850 MMS 功能，请升级支持 MMS 的固件。"})
			return
		}
		result, err := collector.TestIEC61850Connection(ctx, body.Address)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": result.Address, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "address": result.Address, "message": "IEC 61850 MMS 连接成功，已建立 MMS 会话并读取设备模型目录。"})
		return
	}
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
		commonAddress := body.CommonAddress
		if commonAddress == 0 {
			commonAddress = uint16(body.SlaveID)
		}
		result, err := collector.TestIEC104Connection(ctx, body.Address, commonAddress)
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

	if body.Protocol == "opcua" {
		result, err := collector.TestOPCUAConnection(ctx, config.PointConfig{
			Protocol: "opcua",
			Address:  body.Address,
			Username: body.Username,
			Password: body.Password,
		})
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": result.Endpoint, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok":      true,
			"address": result.Endpoint,
			"message": "OPC UA 连接成功，已完成端点握手。",
		})
		return
	}

	if config.NormalizeProtocol(body.Protocol) == "opcda" {
		point := config.PointConfig{
			DeviceKey: body.DeviceKey, Protocol: "opcda", Address: body.Address, ProgID: body.ProgID, CLSID: body.CLSID,
			Domain: body.Domain, Workstation: body.Workstation, SecurityMode: body.SecurityMode,
			Username: body.Username, Password: body.Password,
			Authentication: body.Authentication, AuthenticationLevel: body.AuthenticationLevel,
			ConnectTimeoutMilliseconds: body.ConnectTimeoutMilliseconds,
			RequestTimeoutMilliseconds: body.RequestTimeoutMilliseconds,
			GroupName:                  body.GroupName, RequestedUpdateRateMS: body.RequestedUpdateRateMS,
			Deadband: body.Deadband, LocaleID: body.LocaleID, LocalBindIP: body.LocalBindIP,
		}
		point.ApplyDefaults()
		result, err := collector.TestOPCDAConnection(ctx, point)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "address": result.Address, "stage": result.Stage, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok": true, "address": result.Address, "stage": result.Stage,
			"message": "OPC DA 连接成功，已完成 DCOM 激活并取得 IOPCServer。",
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

func testRTUConnection(ctx context.Context, body connectionTestRequest) (map[string]interface{}, error) {
	result, err := collector.TestModbusRTUConnection(ctx, config.PointConfig{DeviceKey: body.DeviceKey, Protocol: "modbus-rtu", Address: body.Address, SlaveID: body.SlaveID, Function: body.Function, Register: body.Register, BaudRate: body.BaudRate, DataBits: body.DataBits, StopBits: body.StopBits, Parity: body.Parity})
	data := map[string]interface{}{"ok": err == nil, "address": result.Address, "slaveId": result.SlaveID, "function": result.Function, "register": result.Register, "elapsedMilliseconds": result.ElapsedMS}
	if err == nil {
		data["message"] = fmt.Sprintf("Modbus RTU 通信成功，从站 %d 已响应读取请求，耗时 %d ms。", result.SlaveID, result.ElapsedMS)
	}
	return data, err
}

func resolveMaskedOPCDAPassword(body *connectionTestRequest, cfg config.Config) bool {
	if body == nil || body.Password != "***" || strings.TrimSpace(body.DeviceKey) == "" {
		return false
	}
	for i := range cfg.Devices {
		if cfg.Devices[i].DeviceKey == body.DeviceKey {
			body.Password = cfg.Devices[i].Password
			return body.Password != "***"
		}
	}
	return false
}

func normalizeConnectionAddress(protocol string, address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		switch protocol {
		case "iec104":
			return "127.0.0.1:2404"
		case "modbus-tcp":
			return "127.0.0.1:502"
		case "siemens-s7", "iec61850":
			return "127.0.0.1:102"
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
	case "siemens-s7", "iec61850":
		return address + ":102"
	default:
		return address
	}
}

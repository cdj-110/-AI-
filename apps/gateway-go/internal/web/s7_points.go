package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
)

type s7ScanRequest struct {
	DeviceKey  string `json:"deviceKey"`
	DeviceName string `json:"deviceName"`
	Address    string `json:"address"`
	Area       string `json:"area"`
	DBNumber   uint16 `json:"dbNumber"`
	Rack       uint8  `json:"rack"`
	Slot       uint8  `json:"slot"`
	LocalTSAP  string `json:"localTsap"`
	RemoteTSAP string `json:"remoteTsap"`
	Start      uint16 `json:"start"`
	End        uint16 `json:"end"`
	DataType   string `json:"dataType"`
}

type s7PointPreviewItem struct {
	config.PointConfig
	Selected bool        `json:"selected"`
	Value    interface{} `json:"value,omitempty"`
}

func (s *Server) scanS7Points(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 64*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var body s7ScanRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	points, warnings := scanS7PointRange(request.Context(), body)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"points":   points,
		"warnings": warnings,
	})
}

func (s *Server) testS7Connection(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 64*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var body s7ScanRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	point := config.PointConfig{
		Protocol:   "siemens-s7",
		Address:    body.Address,
		Rack:       body.Rack,
		Slot:       body.Slot,
		LocalTSAP:  body.LocalTSAP,
		RemoteTSAP: body.RemoteTSAP,
	}
	testCtx, cancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer cancel()

	result, err := collector.TestS7Connection(testCtx, point)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok":      false,
			"address": result.Address,
			"message": err.Error(),
		})
		return
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok":         true,
		"address":    result.Address,
		"endpoint":   result.Endpoint,
		"localTsap":  result.LocalTSAP,
		"remoteTsap": result.RemoteTSAP,
		"message":    "PLC 连接成功，已完成 TCP/COTP/S7 参数协商。",
	})
}

func scanS7PointRange(ctx context.Context, body s7ScanRequest) ([]s7PointPreviewItem, []string) {
	body.Area = strings.ToUpper(strings.TrimSpace(body.Area))
	if body.Area == "" || body.Area == "AUTO" {
		return scanS7SmartAutoPoints(ctx, body)
	}
	if body.Area == "" {
		body.Area = "DB"
	}
	if body.DBNumber == 0 && body.Area == "DB" {
		body.DBNumber = 1
	}
	if body.DataType == "" {
		body.DataType = "uint16"
	}
	if body.End < body.Start {
		body.End = body.Start
	}
	span := int(body.End - body.Start + 1)
	if span > 128 {
		body.End = body.Start + 127
	}
	step := s7ScanStep(body.DataType)
	deviceName := body.DeviceName
	if strings.TrimSpace(deviceName) == "" {
		deviceName = body.DeviceKey
	}
	if strings.TrimSpace(deviceName) == "" {
		deviceName = "PLC"
	}

	scanCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var points []s7PointPreviewItem
	var warnings []string
	for register := body.Start; register <= body.End; {
		point := config.PointConfig{
			DeviceKey:  body.DeviceKey,
			Name:       s7GeneratedPointName(deviceName, body.Area, body.DBNumber, register, body.DataType),
			Metric:     s7GeneratedMetric(body.Area, body.DBNumber, register, body.DataType),
			Protocol:   "siemens-s7",
			Address:    body.Address,
			Area:       body.Area,
			DBNumber:   body.DBNumber,
			Rack:       body.Rack,
			Slot:       body.Slot,
			LocalTSAP:  body.LocalTSAP,
			RemoteTSAP: body.RemoteTSAP,
			Register:   register,
			Quantity:   1,
			DataType:   body.DataType,
			ByteOrder:  "big",
			WordOrder:  "normal",
			Scale:      1,
		}
		point.ApplyDefaults()
		value, err := gatewayruntime.ReadPoint(scanCtx, point)
		if err == nil {
			points = append(points, s7PointPreviewItem{PointConfig: point, Selected: true, Value: value.Value})
		} else if len(warnings) < 3 {
			warnings = append(warnings, fmt.Sprintf("%s 读取失败：%v", point.Name, err))
		}
		if len(points) >= 80 {
			warnings = append(warnings, "本次最多返回 80 个候选点位，范围过大时请缩小扫描范围。")
			break
		}
		if body.End-register < step {
			break
		}
		register += step
	}
	if len(points) == 0 && len(warnings) == 0 {
		warnings = append(warnings, "未扫描到可读取点位，请确认 PLC IP、102 端口、PUT/GET 访问和 DB 地址范围。")
	}
	return points, warnings
}

func scanS7SmartAutoPoints(ctx context.Context, body s7ScanRequest) ([]s7PointPreviewItem, []string) {
	deviceName := body.DeviceName
	if strings.TrimSpace(deviceName) == "" {
		deviceName = body.DeviceKey
	}
	if strings.TrimSpace(deviceName) == "" {
		deviceName = "SMART200"
	}

	scanCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	type target struct {
		area     string
		dataType string
		start    uint16
		end      uint16
		step     uint16
	}
	targets := []target{
		{area: "V", dataType: "uint16", start: 0, end: 200, step: 2},
		{area: "M", dataType: "bool", start: 0, end: 31, step: 1},
		{area: "I", dataType: "bool", start: 0, end: 15, step: 1},
		{area: "Q", dataType: "bool", start: 0, end: 15, step: 1},
	}

	var points []s7PointPreviewItem
	var warnings []string
	for _, item := range targets {
		for register := item.start; register <= item.end; register += item.step {
			point := config.PointConfig{
				DeviceKey:  body.DeviceKey,
				Name:       s7SmartPointName(deviceName, item.area, register, item.dataType),
				Metric:     s7GeneratedMetric(item.area, 0, register, item.dataType),
				Protocol:   "siemens-s7",
				Address:    body.Address,
				Area:       item.area,
				Rack:       body.Rack,
				Slot:       body.Slot,
				LocalTSAP:  body.LocalTSAP,
				RemoteTSAP: body.RemoteTSAP,
				Register:   register,
				Quantity:   1,
				DataType:   item.dataType,
				ByteOrder:  "big",
				WordOrder:  "normal",
				Scale:      1,
			}
			if item.area == "V" {
				point.DBNumber = 1
			}
			point.ApplyDefaults()
			value, err := gatewayruntime.ReadPoint(scanCtx, point)
			if err != nil {
				if len(warnings) < 3 {
					warnings = append(warnings, fmt.Sprintf("%s 读取失败：%v", point.Name, err))
				}
				continue
			}
			points = append(points, s7PointPreviewItem{PointConfig: point, Selected: true, Value: value.Value})
			if len(points) >= 120 {
				warnings = append(warnings, "本次最多返回 120 个候选点位，更多地址可后续按范围扫描。")
				return points, warnings
			}
		}
	}
	if len(points) == 0 {
		warnings = append(warnings, "没有扫描到可读取点位。请确认 S7-200 SMART 已允许 S7 通信，并确认 V/M/I/Q 区域存在可访问地址。")
	}
	warnings = append(warnings, "普通 S7 通信不能读取 PLC 程序里的变量名，候选点位名称已按地址自动生成。")
	return points, warnings
}

func s7ScanStep(dataType string) uint16 {
	switch dataType {
	case "uint32", "int32", "float32":
		return 4
	default:
		return 2
	}
}

func s7GeneratedPointName(deviceName string, area string, dbNumber uint16, register uint16, dataType string) string {
	if area == "DB" {
		return fmt.Sprintf("%s_%s%d_%d_%s", deviceName, area, dbNumber, register, dataType)
	}
	return fmt.Sprintf("%s_%s_%d_%s", deviceName, area, register, dataType)
}

func s7SmartPointName(deviceName string, area string, register uint16, dataType string) string {
	return fmt.Sprintf("%s_%s%d_%s", deviceName, area, register, dataType)
}

func s7GeneratedMetric(area string, dbNumber uint16, register uint16, dataType string) string {
	if area == "DB" {
		return fmt.Sprintf("s7_db%d_%d_%s", dbNumber, register, dataType)
	}
	return fmt.Sprintf("s7_%s_%d_%s", strings.ToLower(area), register, dataType)
}

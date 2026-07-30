//go:build linux

package forward

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/goose"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func (m *Manager) serveIEC61850GOOSE(ctx context.Context) error {
	cfg := m.config()
	devices := make([]config.ForwardDeviceConfig, 0)
	for _, device := range cfg.ForwardDevices {
		if device.IsEnabled() && strings.EqualFold(device.Protocol, ProtocolIEC61850GOOSE) {
			devices = append(devices, device)
		}
	}
	if len(devices) == 0 {
		return nil
	}
	var workers sync.WaitGroup
	for _, configured := range devices {
		device := configured
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := m.publishIEC61850GOOSE(ctx, device); err != nil && ctx.Err() == nil {
				log.Printf("IEC61850 GOOSE device %s stopped: %v", device.DeviceKey, err)
				m.store.AddError(fmt.Sprintf("IEC61850 GOOSE 转发设备 %s 停止: %v", device.Name, err))
			}
		}()
	}
	<-ctx.Done()
	workers.Wait()
	return nil
}

func (m *Manager) publishIEC61850GOOSE(ctx context.Context, device config.ForwardDeviceConfig) error {
	destinationMAC, err := net.ParseMAC(strings.TrimSpace(device.DestinationMAC))
	if err != nil || len(destinationMAC) != 6 {
		return fmt.Errorf("invalid GOOSE destination MAC %q", device.DestinationMAC)
	}
	socket, err := goose.Open(strings.TrimSpace(device.InterfaceName))
	if err != nil {
		return err
	}
	defer socket.Close()

	ttl := device.TimeAllowedToLiveMS
	if ttl == 0 {
		ttl = 2000
	}
	stableInterval := time.Duration(ttl/2) * time.Millisecond
	if stableInterval < 500*time.Millisecond {
		stableInterval = 500 * time.Millisecond
	}
	retransmissions := []time.Duration{20 * time.Millisecond, 40 * time.Millisecond, 100 * time.Millisecond, 500 * time.Millisecond}
	stateNumber := uint32(1)
	sequenceNumber := uint32(0)
	values := m.gooseForwardValues(device)
	nextPublish := time.Now()
	retransmissionIndex := 0
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()

	log.Printf(
		"IEC61850 GOOSE publisher %s active on %s APPID=0x%04X goCbRef=%s dataSet=%s",
		device.DeviceKey, device.InterfaceName, device.AppID, device.GoCBRef, device.DataSetRef,
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-poll.C:
			current := m.gooseForwardValues(device)
			if !reflect.DeepEqual(values, current) {
				values = current
				stateNumber++
				sequenceNumber = 0
				retransmissionIndex = 0
				nextPublish = now
			}
			if now.Before(nextPublish) {
				continue
			}
			message := goose.Message{
				AppID:             device.AppID,
				GoCBRef:           device.GoCBRef,
				TimeAllowedToLive: ttl,
				DataSetRef:        device.DataSetRef,
				GoID:              device.DeviceKey,
				Timestamp:         now,
				StateNumber:       stateNumber,
				SequenceNumber:    sequenceNumber,
				ConfRev:           device.ConfRev,
				Values:            values,
			}
			frame, encodeErr := goose.EncodeFrame(
				message,
				socket.SourceMAC(),
				destinationMAC,
				device.VLANID,
				device.VLANPriority,
			)
			if encodeErr != nil {
				return encodeErr
			}
			sendErr := socket.Send(frame)
			summary := fmt.Sprintf(
				"GOOSE 发布 APPID=0x%04X stNum=%d sqNum=%d 数据集成员=%d",
				device.AppID, stateNumber, sequenceNumber, len(values),
			)
			monitorFrame := packetmonitor.Frame{
				Protocol:  ProtocolIEC61850GOOSE,
				Direction: "tx",
				DeviceKey: device.DeviceKey,
				Address:   device.InterfaceName,
				Summary:   summary,
			}
			if sendErr != nil {
				monitorFrame.Error = sendErr.Error()
			}
			packetmonitor.Record(monitorFrame, frame)
			if sendErr != nil {
				return fmt.Errorf("send GOOSE frame: %w", sendErr)
			}
			sequenceNumber++
			if retransmissionIndex < len(retransmissions) {
				nextPublish = now.Add(retransmissions[retransmissionIndex])
				retransmissionIndex++
			} else {
				nextPublish = now.Add(stableInterval)
			}
		}
	}
}

func (m *Manager) gooseForwardValues(device config.ForwardDeviceConfig) []goose.Value {
	statusByKey := make(map[string]state.PointStatus)
	for _, status := range m.store.PointStatuses() {
		statusByKey[status.DeviceKey+"::"+status.Metric] = status
	}
	values := make([]goose.Value, 0, len(device.Points))
	for _, point := range device.Points {
		kind := normalizeGOOSEDataType(point.DataType)
		status := statusByKey[point.SourceDeviceKey+"::"+point.SourceMetric]
		value := zeroGOOSEValue(kind)
		if status.UpdatedAt != nil && status.Error == "" {
			value = convertGOOSEForwardValue(status.Value, kind)
		}
		values = append(values, goose.Value{Kind: kind, Value: value})
	}
	return values
}

func normalizeGOOSEDataType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bool", "boolean", "single", "double":
		return "bool"
	case "int8", "int16", "int32", "int64":
		return strings.ToLower(strings.TrimSpace(value))
	case "uint8", "uint16", "uint32", "uint64":
		return strings.ToLower(strings.TrimSpace(value))
	case "float64", "double-float":
		return "float64"
	case "string", "visible-string":
		return "string"
	default:
		return "float32"
	}
}

func zeroGOOSEValue(kind string) interface{} {
	switch kind {
	case "bool":
		return false
	case "string":
		return ""
	case "float32", "float64":
		return float64(0)
	case "int8", "int16", "int32", "int64":
		return int64(0)
	default:
		return uint64(0)
	}
}

func convertGOOSEForwardValue(value interface{}, kind string) interface{} {
	switch kind {
	case "bool":
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
			if err == nil {
				return parsed
			}
		}
		return gooseForwardFloat(value) != 0
	case "string":
		return fmt.Sprint(value)
	case "float32", "float64":
		return gooseForwardFloat(value)
	case "int8", "int16", "int32", "int64":
		return int64(gooseForwardFloat(value))
	default:
		number := gooseForwardFloat(value)
		if number < 0 {
			number = 0
		}
		return uint64(number)
	}
}

func gooseForwardFloat(value interface{}) float64 {
	switch typed := value.(type) {
	case float32:
		return float64(typed)
	case float64:
		return typed
	case int:
		return float64(typed)
	case int8:
		return float64(typed)
	case int16:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint8:
		return float64(typed)
	case uint16:
		return float64(typed)
	case uint32:
		return float64(typed)
	case uint64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
		return parsed
	}
}

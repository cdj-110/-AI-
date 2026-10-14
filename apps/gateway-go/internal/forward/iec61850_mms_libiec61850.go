//go:build iec61850_mms && cgo

package forward

/*
#cgo LDFLAGS: -liec61850
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "iec61850_dynamic_model.h"
#include "iec61850_server.h"

typedef struct {
	IedModel* model;
	IedServer server;
} WkMmsForwardServer;

static WkMmsForwardServer* wkMmsForwardServer_create(const char* iedName) {
	WkMmsForwardServer* runtime = (WkMmsForwardServer*) calloc(1, sizeof(WkMmsForwardServer));
	if (runtime == NULL)
		return NULL;
	runtime->model = IedModel_create(iedName);
	if (runtime->model == NULL) {
		free(runtime);
		return NULL;
	}
	IedModel_setIedNameForDynamicModel(runtime->model, iedName);
	return runtime;
}

static LogicalDevice* wkMmsForwardServer_addLogicalDevice(WkMmsForwardServer* runtime, const char* name) {
	return LogicalDevice_create(name, runtime->model);
}

static LogicalNode* wkMmsForwardServer_addLogicalNode(LogicalDevice* device, const char* name) {
	return LogicalNode_create(name, device);
}

static void wkMmsForwardServer_addQualityAndTimestamp(DataObject* object, FunctionalConstraint fc,
		DataAttribute** quality, DataAttribute** timestamp) {
	*quality = DataAttribute_create("q", (ModelNode*) object, IEC61850_QUALITY,
		fc, TRG_OPT_QUALITY_CHANGED, 0, 0);
	*timestamp = DataAttribute_create("t", (ModelNode*) object, IEC61850_TIMESTAMP,
		fc, 0, 0, 0);
}

static DataAttribute* wkMmsForwardServer_addBoolean(LogicalNode* node, const char* name,
		DataAttribute** quality, DataAttribute** timestamp) {
	DataObject* object = DataObject_create(name, (ModelNode*) node, 0);
	if (object == NULL)
		return NULL;
	wkMmsForwardServer_addQualityAndTimestamp(object, IEC61850_FC_ST, quality, timestamp);
	return DataAttribute_create("stVal", (ModelNode*) object, IEC61850_BOOLEAN,
		IEC61850_FC_ST, TRG_OPT_DATA_CHANGED, 0, 0);
}

static DataAttribute* wkMmsForwardServer_addString(LogicalNode* node, const char* name,
		DataAttribute** quality, DataAttribute** timestamp) {
	DataObject* object = DataObject_create(name, (ModelNode*) node, 0);
	if (object == NULL)
		return NULL;
	wkMmsForwardServer_addQualityAndTimestamp(object, IEC61850_FC_ST, quality, timestamp);
	return DataAttribute_create("stVal", (ModelNode*) object, IEC61850_VISIBLE_STRING_255,
		IEC61850_FC_ST, TRG_OPT_DATA_CHANGED, 0, 0);
}

static DataAttribute* wkMmsForwardServer_addFloat(LogicalNode* node, const char* name,
		DataAttribute** quality, DataAttribute** timestamp) {
	DataObject* object = DataObject_create(name, (ModelNode*) node, 0);
	if (object == NULL)
		return NULL;
	wkMmsForwardServer_addQualityAndTimestamp(object, IEC61850_FC_MX, quality, timestamp);
	DataAttribute* magnitude = DataAttribute_create("mag", (ModelNode*) object,
		IEC61850_CONSTRUCTED, IEC61850_FC_MX, 0, 0, 0);
	if (magnitude == NULL)
		return NULL;
	return DataAttribute_create("f", (ModelNode*) magnitude, IEC61850_FLOAT32,
		IEC61850_FC_MX, TRG_OPT_DATA_CHANGED, 0, 0);
}

static bool wkMmsForwardServer_start(WkMmsForwardServer* runtime, const char* host, int port) {
	if (runtime == NULL || runtime->model == NULL)
		return false;
	runtime->server = IedServer_create(runtime->model);
	if (runtime->server == NULL)
		return false;
	IedServer_setServerIdentity(runtime->server, "Weikong", "Edge Gateway", "1.0");
	if (host != NULL && host[0] != '\0' && strcmp(host, "0.0.0.0") != 0)
		IedServer_setLocalIpAddress(runtime->server, host);
	IedServer_start(runtime->server, port);
	return IedServer_isRunning(runtime->server);
}

static void wkMmsForwardServer_updateBoolean(WkMmsForwardServer* runtime, DataAttribute* attribute, bool value) {
	IedServer_updateBooleanAttributeValue(runtime->server, attribute, value);
}

static void wkMmsForwardServer_updateFloat(WkMmsForwardServer* runtime, DataAttribute* attribute, float value) {
	IedServer_updateFloatAttributeValue(runtime->server, attribute, value);
}

static void wkMmsForwardServer_updateString(WkMmsForwardServer* runtime, DataAttribute* attribute, char* value) {
	IedServer_updateVisibleStringAttributeValue(runtime->server, attribute, value);
}

static void wkMmsForwardServer_updateMetadata(WkMmsForwardServer* runtime,
		DataAttribute* quality, DataAttribute* timestamp, Quality qualityValue, uint64_t milliseconds) {
	if (quality != NULL)
		IedServer_updateQuality(runtime->server, quality, qualityValue);
	if (timestamp != NULL)
		IedServer_updateUTCTimeAttributeValue(runtime->server, timestamp, milliseconds);
}

static int wkMmsForwardServer_connectionCount(WkMmsForwardServer* runtime) {
	if (runtime == NULL || runtime->server == NULL)
		return 0;
	return IedServer_getNumberOfOpenConnections(runtime->server);
}

static void wkMmsForwardServer_destroy(WkMmsForwardServer* runtime) {
	if (runtime == NULL)
		return;
	if (runtime->server != NULL) {
		IedServer_stop(runtime->server);
		IedServer_destroy(runtime->server);
	}
	if (runtime->model != NULL)
		IedModel_destroy(runtime->model);
	free(runtime);
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

type iec61850ForwardValueKind uint8

const (
	iec61850ForwardFloat iec61850ForwardValueKind = iota
	iec61850ForwardBoolean
	iec61850ForwardString
)

type iec61850ForwardAttribute struct {
	attribute       *C.DataAttribute
	quality         *C.DataAttribute
	timestamp       *C.DataAttribute
	sourceDeviceKey string
	sourceMetric    string
	objectRef       string
	kind            iec61850ForwardValueKind
	lastValue       interface{}
	hasValue        bool
	lastQuality     uint16
	hasQuality      bool
}

type iec61850ForwardDefinition struct {
	iedName         string
	logicalDevice   string
	logicalNode     string
	dataObject      string
	objectRef       string
	sourceDeviceKey string
	sourceMetric    string
	kind            iec61850ForwardValueKind
}

func (m *Manager) serveIEC61850MMS(ctx context.Context, listen string) error {
	if listen == "" {
		listen = "0.0.0.0:102"
	}
	host, portText, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid IEC61850 MMS listen address %q: %w", listen, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid IEC61850 MMS listen port %q", portText)
	}

	definitions, err := m.iec61850ForwardDefinitions()
	if err != nil {
		return err
	}
	if len(definitions) == 0 {
		return fmt.Errorf("IEC61850 MMS 转发没有可发布的点位")
	}

	cIEDName := C.CString(definitions[0].iedName)
	runtime := C.wkMmsForwardServer_create(cIEDName)
	C.free(unsafe.Pointer(cIEDName))
	if runtime == nil {
		return fmt.Errorf("create IEC61850 MMS data model failed")
	}
	defer C.wkMmsForwardServer_destroy(runtime)

	logicalDevices := map[string]*C.LogicalDevice{}
	logicalNodes := map[string]*C.LogicalNode{}
	attributes := make([]iec61850ForwardAttribute, 0, len(definitions))
	for _, definition := range definitions {
		device := logicalDevices[definition.logicalDevice]
		if device == nil {
			cName := C.CString(definition.logicalDevice)
			device = C.wkMmsForwardServer_addLogicalDevice(runtime, cName)
			C.free(unsafe.Pointer(cName))
			if device == nil {
				return fmt.Errorf("create IEC61850 logical device %s failed", definition.logicalDevice)
			}
			logicalDevices[definition.logicalDevice] = device
			for _, baseNodeName := range []string{"LLN0", "LPHD1"} {
				cBaseName := C.CString(baseNodeName)
				baseNode := C.wkMmsForwardServer_addLogicalNode(device, cBaseName)
				C.free(unsafe.Pointer(cBaseName))
				if baseNode != nil {
					logicalNodes[definition.logicalDevice+"/"+baseNodeName] = baseNode
				}
			}
		}
		nodeKey := definition.logicalDevice + "/" + definition.logicalNode
		node := logicalNodes[nodeKey]
		if node == nil {
			cName := C.CString(definition.logicalNode)
			node = C.wkMmsForwardServer_addLogicalNode(device, cName)
			C.free(unsafe.Pointer(cName))
			if node == nil {
				return fmt.Errorf("create IEC61850 logical node %s failed", nodeKey)
			}
			logicalNodes[nodeKey] = node
		}
		cName := C.CString(definition.dataObject)
		var attribute *C.DataAttribute
		var quality *C.DataAttribute
		var timestamp *C.DataAttribute
		switch definition.kind {
		case iec61850ForwardBoolean:
			attribute = C.wkMmsForwardServer_addBoolean(node, cName, &quality, &timestamp)
		case iec61850ForwardString:
			attribute = C.wkMmsForwardServer_addString(node, cName, &quality, &timestamp)
		default:
			attribute = C.wkMmsForwardServer_addFloat(node, cName, &quality, &timestamp)
		}
		C.free(unsafe.Pointer(cName))
		if attribute == nil {
			return fmt.Errorf("create IEC61850 data object %s failed", definition.objectRef)
		}
		attributes = append(attributes, iec61850ForwardAttribute{
			attribute: attribute, quality: quality, timestamp: timestamp, sourceDeviceKey: definition.sourceDeviceKey,
			sourceMetric: definition.sourceMetric, objectRef: definition.objectRef, kind: definition.kind,
		})
	}

	cHost := C.CString(host)
	started := bool(C.wkMmsForwardServer_start(runtime, cHost, C.int(port)))
	C.free(unsafe.Pointer(cHost))
	if !started {
		return fmt.Errorf("IEC61850 MMS listen on %s failed", listen)
	}
	log.Printf("protocol forward IEC61850 MMS server listening on %s with %d points", listen, len(attributes))

	m.updateIEC61850ForwardValues(runtime, attributes)
	interval := 250 * time.Millisecond
	if len(attributes) > 5000 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastConnections := -1
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.updateIEC61850ForwardValues(runtime, attributes)
			connections := int(C.wkMmsForwardServer_connectionCount(runtime))
			if connections != lastConnections {
				log.Printf("IEC61850 MMS forward clients: %d", connections)
				if lastConnections >= 0 {
					summary := fmt.Sprintf("IEC61850 MMS 客户端连接数变更：%d → %d", lastConnections, connections)
					m.recordForwardPacket(ProtocolIEC61850MMS, "rx", listen, summary, nil, 0, 0, nil)
				}
				lastConnections = connections
			}
		}
	}
}

func (m *Manager) iec61850ForwardDefinitions() ([]iec61850ForwardDefinition, error) {
	cfg := m.config()
	definitions := make([]iec61850ForwardDefinition, 0)
	for _, device := range cfg.ForwardDevices {
		if !device.IsEnabled() || !strings.EqualFold(device.Protocol, ProtocolIEC61850MMS) {
			continue
		}
		iedName := sanitizeIEC61850ServerName(device.IEDName, "WEIKONG")
		logicalDevice := sanitizeIEC61850ServerName(device.LogicalDevice, "LD1")
		for index, point := range device.Points {
			definition, err := parseIEC61850ForwardDefinition(iedName, logicalDevice, point, index)
			if err != nil {
				return nil, fmt.Errorf("IEC61850 MMS 转发设备 %s: %w", device.DeviceKey, err)
			}
			definitions = append(definitions, definition)
		}
	}
	return definitions, nil
}

func parseIEC61850ForwardDefinition(iedName, defaultLD string, point config.ForwardPointConfig, index int) (iec61850ForwardDefinition, error) {
	objectRef := strings.TrimSpace(point.ObjectRef)
	if objectRef == "" {
		return iec61850ForwardDefinition{}, fmt.Errorf("point %s object reference is empty", point.Metric)
	}
	slash := strings.Index(objectRef, "/")
	if slash < 1 || slash == len(objectRef)-1 {
		return iec61850ForwardDefinition{}, fmt.Errorf("point %s object reference %q is invalid", point.Metric, objectRef)
	}
	fullLD := objectRef[:slash]
	logicalDevice := fullLD
	if strings.HasPrefix(strings.ToUpper(fullLD), strings.ToUpper(iedName)) {
		logicalDevice = fullLD[len(iedName):]
	}
	if logicalDevice == "" {
		logicalDevice = defaultLD
	}
	parts := strings.Split(objectRef[slash+1:], ".")
	if len(parts) < 3 {
		return iec61850ForwardDefinition{}, fmt.Errorf("point %s object reference %q must include LN, DO and DA", point.Metric, objectRef)
	}
	logicalNode := sanitizeIEC61850ServerName(parts[0], "GGIO1")
	dataObject := sanitizeIEC61850ServerName(parts[1], fmt.Sprintf("Point%d", index+1))
	dataType := strings.ToLower(strings.TrimSpace(point.DataType))
	kind := iec61850ForwardFloat
	expectedFC := "MX"
	expectedLeaf := "mag.f"
	if dataType == "bool" {
		kind, expectedFC, expectedLeaf = iec61850ForwardBoolean, "ST", "stVal"
	} else if dataType == "string" {
		kind, expectedFC, expectedLeaf = iec61850ForwardString, "ST", "stVal"
	}
	fc := strings.ToUpper(strings.TrimSpace(point.FC))
	if fc != expectedFC {
		return iec61850ForwardDefinition{}, fmt.Errorf("point %s FC %s does not match %s value (want %s)", point.Metric, fc, dataType, expectedFC)
	}
	leaf := strings.Join(parts[2:], ".")
	if !strings.EqualFold(leaf, expectedLeaf) {
		return iec61850ForwardDefinition{}, fmt.Errorf("point %s object reference leaf %s does not match %s value (want %s)", point.Metric, leaf, dataType, expectedLeaf)
	}
	return iec61850ForwardDefinition{
		iedName: iedName, logicalDevice: logicalDevice, logicalNode: logicalNode,
		dataObject: dataObject, objectRef: objectRef, sourceDeviceKey: point.SourceDeviceKey,
		sourceMetric: point.SourceMetric, kind: kind,
	}, nil
}

func sanitizeIEC61850ServerName(value, fallback string) string {
	var result strings.Builder
	for _, character := range strings.TrimSpace(value) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' {
			result.WriteRune(character)
		}
	}
	name := result.String()
	if name == "" {
		name = fallback
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "P" + name
	}
	return name
}

func (m *Manager) updateIEC61850ForwardValues(runtime *C.WkMmsForwardServer, attributes []iec61850ForwardAttribute) {
	statuses := m.store.PointStatuses()
	statusByPoint := make(map[string]state.PointStatus, len(statuses))
	for _, status := range statuses {
		statusByPoint[status.DeviceKey+"::"+status.Metric] = status
	}
	for index := range attributes {
		attribute := &attributes[index]
		status, exists := statusByPoint[attribute.sourceDeviceKey+"::"+attribute.sourceMetric]
		if !exists {
			continue
		}
		quality := uint16(C.QUALITY_VALIDITY_GOOD)
		eventAt := status.UpdatedAt
		if status.Error != "" || status.Stale || status.UpdatedAt == nil {
			quality = uint16(C.QUALITY_VALIDITY_INVALID | C.QUALITY_DETAIL_FAILURE)
			eventAt = status.ErrorAt
			if !attribute.hasQuality || attribute.lastQuality != quality {
				milliseconds := time.Now().UnixMilli()
				if eventAt != nil {
					milliseconds = eventAt.UnixMilli()
				}
				C.wkMmsForwardServer_updateMetadata(runtime, attribute.quality, attribute.timestamp, C.Quality(quality), C.uint64_t(milliseconds))
				attribute.lastQuality = quality
				attribute.hasQuality = true
			}
			continue
		}
		value := status.Value
		valueChanged := !attribute.hasValue || !reflect.DeepEqual(attribute.lastValue, value)
		qualityChanged := !attribute.hasQuality || attribute.lastQuality != quality
		if !valueChanged && !qualityChanged {
			continue
		}
		switch attribute.kind {
		case iec61850ForwardBoolean:
			converted, ok := iec61850ForwardBool(value)
			if !ok {
				continue
			}
			C.wkMmsForwardServer_updateBoolean(runtime, attribute.attribute, C.bool(converted))
		case iec61850ForwardString:
			converted := fmt.Sprint(value)
			cValue := C.CString(converted)
			C.wkMmsForwardServer_updateString(runtime, attribute.attribute, cValue)
			C.free(unsafe.Pointer(cValue))
		default:
			converted, ok := iec61850ForwardFloat64(value)
			if !ok {
				continue
			}
			C.wkMmsForwardServer_updateFloat(runtime, attribute.attribute, C.float(converted))
		}
		milliseconds := time.Now().UnixMilli()
		if eventAt != nil {
			milliseconds = eventAt.UnixMilli()
		}
		C.wkMmsForwardServer_updateMetadata(runtime, attribute.quality, attribute.timestamp, C.Quality(quality), C.uint64_t(milliseconds))
		attribute.lastValue = value
		attribute.hasValue = true
		attribute.lastQuality = quality
		attribute.hasQuality = true
	}
}

func iec61850ForwardFloat64(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func iec61850ForwardBool(value interface{}) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return parsed, true
		}
		number, numberErr := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number != 0, numberErr == nil
	default:
		number, ok := iec61850ForwardFloat64(value)
		return number != 0, ok
	}
}

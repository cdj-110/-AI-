//go:build iec61850_mms && cgo

package collector

/*
#cgo LDFLAGS: -liec61850
#include <stdlib.h>
#include "iec61850_client.h"

static void* wkLinkedListData(LinkedList item) {
	return item->data;
}
*/
import "C"

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

var iec61850FCNamePattern = regexp.MustCompile(`^(.+)\[([A-Z]{2})\]$`)

func BrowseIEC61850Nodes(ctx context.Context, address string, parentRef string, recursive bool) ([]IEC61850BrowseNode, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	address = iec61850Address(address)
	host, port := splitIEC61850HostPort(address)
	cHost := C.CString(host)
	defer C.free(unsafe.Pointer(cHost))

	conn := C.IedConnection_create()
	defer C.IedConnection_destroy(conn)

	var err C.IedClientError
	C.IedConnection_connect(conn, &err, cHost, C.int(port))
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 MMS connect failed %s: error %d", address, int(err))
	}
	defer C.IedConnection_close(conn)

	if strings.TrimSpace(parentRef) != "" {
		return browseIEC61850DataDirectory(conn, parentRef, recursive)
	}
	return browseIEC61850Server(conn, recursive)
}

func browseIEC61850Server(conn C.IedConnection, recursive bool) ([]IEC61850BrowseNode, error) {
	var err C.IedClientError
	devices := C.IedConnection_getServerDirectory(conn, &err, false)
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 get server directory failed: error %d", int(err))
	}
	defer C.LinkedList_destroy(devices)

	nodes := []IEC61850BrowseNode{}
	for item := C.LinkedList_getNext(devices); item != nil; item = C.LinkedList_getNext(item) {
		ld := cStringListItem(item)
		if ld == "" {
			continue
		}
		node := IEC61850BrowseNode{Name: ld, ObjectRef: ld, Leaf: false}
		if !recursive {
			nodes = append(nodes, node)
			continue
		}
		childNodes, childErr := browseIEC61850LogicalDevice(conn, ld)
		if childErr != nil {
			return nil, childErr
		}
		node.Children = childNodes
		node.Leaf = len(childNodes) == 0
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func browseIEC61850LogicalDevice(conn C.IedConnection, ld string) ([]IEC61850BrowseNode, error) {
	cLD := C.CString(ld)
	defer C.free(unsafe.Pointer(cLD))
	var err C.IedClientError
	lns := C.IedConnection_getLogicalDeviceDirectory(conn, &err, cLD)
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 get logical device %s failed: error %d", ld, int(err))
	}
	defer C.LinkedList_destroy(lns)

	nodes := []IEC61850BrowseNode{}
	for item := C.LinkedList_getNext(lns); item != nil; item = C.LinkedList_getNext(item) {
		ln := cStringListItem(item)
		if ln == "" {
			continue
		}
		lnRef := ld + "/" + ln
		node := IEC61850BrowseNode{Name: ln, ObjectRef: lnRef, Leaf: false}
		doNodes, err := browseIEC61850LogicalNode(conn, lnRef)
		if err != nil {
			return nil, err
		}
		node.Children = doNodes
		node.Leaf = len(doNodes) == 0
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func browseIEC61850LogicalNode(conn C.IedConnection, lnRef string) ([]IEC61850BrowseNode, error) {
	cLN := C.CString(lnRef)
	defer C.free(unsafe.Pointer(cLN))
	var err C.IedClientError
	dos := C.IedConnection_getLogicalNodeDirectory(conn, &err, cLN, C.ACSI_CLASS_DATA_OBJECT)
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 get logical node %s failed: error %d", lnRef, int(err))
	}
	defer C.LinkedList_destroy(dos)

	nodes := []IEC61850BrowseNode{}
	for item := C.LinkedList_getNext(dos); item != nil; item = C.LinkedList_getNext(item) {
		doName := cStringListItem(item)
		if doName == "" {
			continue
		}
		dataRef := lnRef + "." + doName
		node := IEC61850BrowseNode{Name: doName, ObjectRef: dataRef, FC: inferIEC61850FC(dataRef, ""), DataType: inferIEC61850DataType(dataRef), Leaf: false}
		dataNodes, err := browseIEC61850DataDirectory(conn, dataRef, true)
		if err != nil {
			node.Leaf = true
			nodes = append(nodes, node)
			continue
		}
		node.Children = dataNodes
		node.Leaf = len(dataNodes) == 0
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func browseIEC61850DataDirectory(conn C.IedConnection, objectRef string, recursive bool) ([]IEC61850BrowseNode, error) {
	cRef := C.CString(objectRef)
	defer C.free(unsafe.Pointer(cRef))
	var err C.IedClientError
	attrs := C.IedConnection_getDataDirectoryFC(conn, &err, cRef)
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 get data directory %s failed: error %d", objectRef, int(err))
	}
	defer C.LinkedList_destroy(attrs)

	nodes := []IEC61850BrowseNode{}
	for item := C.LinkedList_getNext(attrs); item != nil; item = C.LinkedList_getNext(item) {
		attr, fc := parseIEC61850DirectoryName(cStringListItem(item))
		if attr == "" {
			continue
		}
		ref := objectRef + "." + attr
		node := IEC61850BrowseNode{Name: attr, ObjectRef: ref, FC: inferIEC61850FC(ref, fc), DataType: inferIEC61850DataType(ref), Leaf: false}
		if !recursive {
			node.Leaf = true
			nodes = append(nodes, node)
			continue
		}
		children, err := browseIEC61850DataDirectory(conn, ref, true)
		if err != nil || len(children) == 0 {
			node.Leaf = true
			nodes = append(nodes, node)
			continue
		}
		node.Children = children
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func readIEC61850Point(ctx context.Context, point config.PointConfig, objectRef string, fc string) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	address := iec61850Address(point.Address)
	host, port := splitIEC61850HostPort(address)
	cHost := C.CString(host)
	defer C.free(unsafe.Pointer(cHost))
	cRef := C.CString(objectRef)
	defer C.free(unsafe.Pointer(cRef))

	conn := C.IedConnection_create()
	defer C.IedConnection_destroy(conn)
	C.IedConnection_setConnectTimeout(conn, 3000)
	C.IedConnection_setRequestTimeout(conn, 3000)

	var err C.IedClientError
	C.IedConnection_connect(conn, &err, cHost, C.int(port))
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 MMS connect failed %s: error %d", address, int(err))
	}
	defer C.IedConnection_close(conn)

	value := C.IedConnection_readObject(conn, &err, cRef, iec61850FunctionalConstraint(fc))
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 read %s[%s] failed: error %d", objectRef, fc, int(err))
	}
	if value == nil {
		return nil, fmt.Errorf("IEC61850 read %s[%s] returned empty value", objectRef, fc)
	}
	defer C.MmsValue_delete(value)

	converted := iec61850MmsValue(value)
	return applyIEC61850Scale(point, converted), nil
}

func iec61850FunctionalConstraint(fc string) C.FunctionalConstraint {
	switch strings.ToUpper(strings.TrimSpace(fc)) {
	case "MX":
		return C.IEC61850_FC_MX
	case "SP":
		return C.IEC61850_FC_SP
	case "SV":
		return C.IEC61850_FC_SV
	case "CF":
		return C.IEC61850_FC_CF
	case "DC":
		return C.IEC61850_FC_DC
	case "SG":
		return C.IEC61850_FC_SG
	case "SE":
		return C.IEC61850_FC_SE
	case "SR":
		return C.IEC61850_FC_SR
	case "OR":
		return C.IEC61850_FC_OR
	case "BL":
		return C.IEC61850_FC_BL
	case "EX":
		return C.IEC61850_FC_EX
	case "CO":
		return C.IEC61850_FC_CO
	case "US":
		return C.IEC61850_FC_US
	case "MS":
		return C.IEC61850_FC_MS
	case "RP":
		return C.IEC61850_FC_RP
	case "BR":
		return C.IEC61850_FC_BR
	case "LG":
		return C.IEC61850_FC_LG
	case "GO":
		return C.IEC61850_FC_GO
	default:
		return C.IEC61850_FC_ST
	}
}

func iec61850MmsValue(value *C.MmsValue) interface{} {
	switch C.MmsValue_getType(value) {
	case C.MMS_BOOLEAN:
		return bool(C.MmsValue_getBoolean(value))
	case C.MMS_INTEGER:
		return int64(C.MmsValue_toInt64(value))
	case C.MMS_UNSIGNED:
		return uint64(C.MmsValue_toUint32(value))
	case C.MMS_FLOAT:
		return float64(C.MmsValue_toDouble(value))
	case C.MMS_VISIBLE_STRING, C.MMS_STRING:
		return C.GoString(C.MmsValue_toString(value))
	case C.MMS_UTC_TIME:
		ms := int64(C.MmsValue_getUtcTimeInMs(value))
		return time.Unix(0, ms*int64(time.Millisecond)).UTC().Format(time.RFC3339Nano)
	case C.MMS_BINARY_TIME:
		ms := int64(C.MmsValue_getBinaryTimeAsUtcMs(value))
		return time.Unix(0, ms*int64(time.Millisecond)).UTC().Format(time.RFC3339Nano)
	case C.MMS_ARRAY, C.MMS_STRUCTURE:
		size := int(C.MmsValue_getArraySize(value))
		items := make([]interface{}, 0, size)
		for index := 0; index < size; index++ {
			element := C.MmsValue_getElement(value, C.int(index))
			if element == nil {
				items = append(items, nil)
				continue
			}
			items = append(items, iec61850MmsValue(element))
		}
		return items
	default:
		var buffer [512]C.char
		C.MmsValue_printToBuffer(value, &buffer[0], C.int(len(buffer)))
		return C.GoString(&buffer[0])
	}
}

func applyIEC61850Scale(point config.PointConfig, value interface{}) interface{} {
	if point.Scale == 1 && point.Offset == 0 {
		return value
	}
	var number float64
	switch typed := value.(type) {
	case int64:
		number = float64(typed)
	case uint64:
		number = float64(typed)
	case float64:
		number = typed
	default:
		return value
	}
	scaled := number*point.Scale + point.Offset
	if point.Decimals > 0 {
		factor := math.Pow10(point.Decimals)
		scaled = math.Round(scaled*factor) / factor
	}
	return scaled
}

func parseIEC61850DirectoryName(name string) (string, string) {
	name = strings.TrimSpace(name)
	matches := iec61850FCNamePattern.FindStringSubmatch(name)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return name, ""
}

func inferIEC61850FC(objectRef string, fc string) string {
	if fc != "" {
		return fc
	}
	lower := strings.ToLower(objectRef)
	switch {
	case strings.Contains(lower, ".mag.") || strings.Contains(lower, ".cval.") || strings.HasSuffix(lower, ".mag") || strings.HasSuffix(lower, ".instmag"):
		return "MX"
	case strings.Contains(lower, ".setmag.") || strings.HasSuffix(lower, ".setval"):
		return "SP"
	case strings.HasSuffix(lower, ".d") || strings.HasSuffix(lower, ".dutr") || strings.Contains(lower, ".desc"):
		return "DC"
	case strings.Contains(lower, ".ctl") || strings.Contains(lower, ".oper") || strings.Contains(lower, ".sbo"):
		return "CO"
	default:
		return "ST"
	}
}

func inferIEC61850DataType(objectRef string) string {
	lower := strings.ToLower(objectRef)
	switch {
	case strings.HasSuffix(lower, ".q"):
		return "quality"
	case strings.HasSuffix(lower, ".t") || strings.HasSuffix(lower, ".t0") || strings.HasSuffix(lower, ".time"):
		return "timestamp"
	case strings.HasSuffix(lower, ".stval") || strings.HasSuffix(lower, ".ctlval") || strings.HasSuffix(lower, ".general"):
		return "auto"
	case strings.HasSuffix(lower, ".f") || strings.HasSuffix(lower, ".mag.f") || strings.Contains(lower, ".mag."):
		return "float32"
	default:
		return "auto"
	}
}

func cStringListItem(item C.LinkedList) string {
	data := C.wkLinkedListData(item)
	if data == nil {
		return ""
	}
	return C.GoString((*C.char)(data))
}

func splitIEC61850HostPort(address string) (string, int) {
	address = iec61850Address(address)
	host := address
	port := 102
	if index := strings.LastIndex(address, ":"); index > -1 {
		host = address[:index]
		fmt.Sscanf(address[index+1:], "%d", &port)
	}
	return host, port
}

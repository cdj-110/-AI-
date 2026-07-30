//go:build iec61850_mms && cgo

package collector

/*
#cgo LDFLAGS: -liec61850
#include <stdlib.h>
#include "iec61850_client.h"
#include "mms_client_connection.h"

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
	"sync"
	"time"
	"unsafe"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

var iec61850FCNamePattern = regexp.MustCompile(`^(.+)\[([A-Z]{2})\]$`)

type iec61850ClientSession struct {
	mu      sync.Mutex
	address string
	conn    C.IedConnection
}

var iec61850ClientSessions = struct {
	sync.Mutex
	items map[string]*iec61850ClientSession
}{items: map[string]*iec61850ClientSession{}}

func getIEC61850ClientSession(address string) *iec61850ClientSession {
	iec61850ClientSessions.Lock()
	defer iec61850ClientSessions.Unlock()
	session := iec61850ClientSessions.items[address]
	if session == nil {
		session = &iec61850ClientSession{address: address}
		iec61850ClientSessions.items[address] = session
	}
	return session
}

func (s *iec61850ClientSession) closeLocked() {
	if s.conn == nil {
		return
	}
	C.IedConnection_close(s.conn)
	C.IedConnection_destroy(s.conn)
	s.conn = nil
}

func (s *iec61850ClientSession) connectLocked() error {
	if s.conn != nil {
		return nil
	}
	host, port := splitIEC61850HostPort(s.address)
	cHost := C.CString(host)
	defer C.free(unsafe.Pointer(cHost))
	conn := C.IedConnection_create()
	C.IedConnection_setConnectTimeout(conn, 3000)
	C.IedConnection_setRequestTimeout(conn, 3000)
	var err C.IedClientError
	C.IedConnection_connect(conn, &err, cHost, C.int(port))
	if err != C.IED_ERROR_OK {
		C.IedConnection_destroy(conn)
		return iec61850ClientError("MMS association", s.address, err)
	}
	s.conn = conn
	return nil
}

// CloseIEC61850Connections closes persistent MMS client associations. It is
// called before a configuration reload so changed addresses never reuse an old
// connection.
func CloseIEC61850Connections() {
	iec61850ClientSessions.Lock()
	sessions := make([]*iec61850ClientSession, 0, len(iec61850ClientSessions.items))
	for _, session := range iec61850ClientSessions.items {
		sessions = append(sessions, session)
	}
	iec61850ClientSessions.items = map[string]*iec61850ClientSession{}
	iec61850ClientSessions.Unlock()
	for _, session := range sessions {
		session.mu.Lock()
		session.closeLocked()
		session.mu.Unlock()
	}
}

func iec61850ClientError(operation, target string, err C.IedClientError) error {
	message := C.GoString(C.IedClientError_toString(err))
	if message == "" {
		message = "unknown error"
	}
	return fmt.Errorf("IEC61850 %s %s failed: %s (error %d)", operation, target, message, int(err))
}

func testIEC61850Association(ctx context.Context, address string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	host, port := splitIEC61850HostPort(address)
	cHost := C.CString(host)
	defer C.free(unsafe.Pointer(cHost))
	conn := C.IedConnection_create()
	defer C.IedConnection_destroy(conn)
	C.IedConnection_setConnectTimeout(conn, 3000)
	C.IedConnection_setRequestTimeout(conn, 3000)
	var err C.IedClientError
	C.IedConnection_connect(conn, &err, cHost, C.int(port))
	if err != C.IED_ERROR_OK {
		return iec61850ClientError("MMS association", address, err)
	}
	defer C.IedConnection_close(conn)
	devices := C.IedConnection_getServerDirectory(conn, &err, false)
	if devices != nil {
		C.LinkedList_destroy(devices)
	}
	if err != C.IED_ERROR_OK {
		return iec61850ClientError("model directory read", address, err)
	}
	return nil
}

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
		return nil, iec61850ClientError("MMS association", address, err)
	}
	defer C.IedConnection_close(conn)

	parentRef = strings.TrimSpace(parentRef)
	if parentRef != "" {
		if !strings.Contains(parentRef, "/") {
			return browseIEC61850LogicalDevice(conn, parentRef)
		}
		if slash := strings.Index(parentRef, "/"); slash >= 0 && !strings.Contains(parentRef[slash+1:], ".") {
			return browseIEC61850LogicalNode(conn, parentRef)
		}
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
	nodes = mergeIEC61850VMDVariables(conn, nodes)
	return nodes, nil
}

func mergeIEC61850VMDVariables(conn C.IedConnection, nodes []IEC61850BrowseNode) []IEC61850BrowseNode {
	mmsConn := C.IedConnection_getMmsConnection(conn)
	if mmsConn == nil {
		return nodes
	}
	var mmsErr C.MmsError
	variables := C.MmsConnection_getVMDVariableNames(mmsConn, &mmsErr)
	if mmsErr != C.MMS_ERROR_NONE || variables == nil {
		return nodes
	}
	defer C.LinkedList_destroy(variables)
	for item := C.LinkedList_getNext(variables); item != nil; item = C.LinkedList_getNext(item) {
		variable := cStringListItem(item)
		node, ok := iec61850VMDVariableNameNode(variable)
		if !ok {
			continue
		}
		nodes = insertIEC61850BrowseNode(nodes, node.ObjectRef, node.FC, node.DataType)
	}
	return nodes
}

func iec61850VMDVariableNameNode(variable string) (IEC61850BrowseNode, bool) {
	variable = strings.TrimSpace(variable)
	if variable == "" {
		return IEC61850BrowseNode{}, false
	}
	slash := strings.Index(variable, "/")
	if slash < 1 {
		return IEC61850BrowseNode{}, false
	}
	ld := variable[:slash]
	node, ok := iec61850VariableNameNode(ld, variable[slash+1:])
	if !ok {
		return IEC61850BrowseNode{}, false
	}
	return node, true
}

func browseIEC61850LogicalDevice(conn C.IedConnection, ld string) ([]IEC61850BrowseNode, error) {
	cLD := C.CString(ld)
	defer C.free(unsafe.Pointer(cLD))
	var err C.IedClientError
	lns := C.IedConnection_getLogicalDeviceDirectory(conn, &err, cLD)
	if err != C.IED_ERROR_OK {
		nodes, variableErr := browseIEC61850LogicalDeviceVariables(conn, ld)
		if variableErr == nil && len(nodes) > 0 {
			return nodes, nil
		}
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

func browseIEC61850LogicalDeviceVariables(conn C.IedConnection, ld string) ([]IEC61850BrowseNode, error) {
	cLD := C.CString(ld)
	defer C.free(unsafe.Pointer(cLD))
	var err C.IedClientError
	variables := C.IedConnection_getLogicalDeviceVariables(conn, &err, cLD)
	if err != C.IED_ERROR_OK {
		return nil, fmt.Errorf("IEC61850 get logical device variables %s failed: error %d", ld, int(err))
	}
	defer C.LinkedList_destroy(variables)

	nodes := []IEC61850BrowseNode{}
	for item := C.LinkedList_getNext(variables); item != nil; item = C.LinkedList_getNext(item) {
		variable := cStringListItem(item)
		node, ok := iec61850VariableNameNode(ld, variable)
		if !ok {
			continue
		}
		nodes = insertIEC61850BrowseNode(nodes, node.ObjectRef, node.FC, node.DataType)
	}
	return nodes, nil
}

func iec61850VariableNameNode(ld string, variable string) (IEC61850BrowseNode, bool) {
	variable = strings.TrimSpace(variable)
	if variable == "" {
		return IEC61850BrowseNode{}, false
	}
	parts := strings.Split(variable, "$")
	if len(parts) < 3 {
		return IEC61850BrowseNode{}, false
	}
	ln := strings.TrimSpace(parts[0])
	fc := strings.TrimSpace(parts[1])
	if ln == "" || fc == "" {
		return IEC61850BrowseNode{}, false
	}
	ref := ld + "/" + ln + "." + strings.Join(parts[2:], ".")
	return IEC61850BrowseNode{
		Name:      parts[len(parts)-1],
		ObjectRef: ref,
		FC:        fc,
		DataType:  inferIEC61850DataType(ref),
		Leaf:      true,
	}, true
}

func insertIEC61850BrowseNode(nodes []IEC61850BrowseNode, objectRef string, fc string, dataType string) []IEC61850BrowseNode {
	parts := splitIEC61850ObjectRef(objectRef)
	if len(parts) == 0 {
		return nodes
	}
	insertIEC61850BrowseNodeAt(&nodes, parts, objectRef, fc, dataType, 0)
	return nodes
}

func MergeIEC61850BrowseNodes(nodes []IEC61850BrowseNode, extra []IEC61850BrowseNode) []IEC61850BrowseNode {
	for _, node := range extra {
		nodes = mergeIEC61850BrowseNode(nodes, node)
	}
	return nodes
}

func mergeIEC61850BrowseNode(nodes []IEC61850BrowseNode, node IEC61850BrowseNode) []IEC61850BrowseNode {
	index := -1
	for i := range nodes {
		if nodes[i].ObjectRef == node.ObjectRef || nodes[i].Name == node.Name {
			index = i
			break
		}
	}
	if index == -1 {
		return append(nodes, node)
	}
	if node.FC != "" {
		nodes[index].FC = node.FC
	}
	if node.DataType != "" {
		nodes[index].DataType = node.DataType
	}
	nodes[index].Leaf = node.Leaf && len(node.Children) == 0
	nodes[index].Children = MergeIEC61850BrowseNodes(nodes[index].Children, node.Children)
	if len(nodes[index].Children) > 0 {
		nodes[index].Leaf = false
	}
	return nodes
}

func DiscoverIEC61850TemplateNodes(ctx context.Context, address string, iedName string) ([]IEC61850BrowseNode, error) {
	iedName = strings.Trim(strings.TrimSpace(iedName), "/.")
	if iedName == "" {
		return nil, nil
	}
	candidates := iec61850TemplateProbeCandidates(iedName)
	probed, err := ProbeIEC61850ObjectRefs(ctx, address, candidates)
	if err != nil {
		return nil, err
	}
	nodes := []IEC61850BrowseNode{}
	for _, result := range probed {
		if !result.Exists {
			continue
		}
		nodes = insertIEC61850BrowseNode(nodes, result.ObjectRef, result.FC, inferIEC61850DataType(result.ObjectRef))
	}
	return nodes, nil
}

func iec61850TemplateProbeCandidates(iedName string) []IEC61850ProbeResult {
	const maxYCIndex = 256
	const maxMMXUIndex = 128
	results := make([]IEC61850ProbeResult, 0, maxYCIndex+maxMMXUIndex)
	for index := 1; index <= maxYCIndex; index++ {
		results = append(results, IEC61850ProbeResult{
			ObjectRef: fmt.Sprintf("%s/MMXU1.YC%d.mag.f", iedName, index),
			FC:        "MX",
		})
	}
	for index := 1; index <= maxMMXUIndex; index++ {
		results = append(results, IEC61850ProbeResult{
			ObjectRef: fmt.Sprintf("%s/MMXU%d.Charges.mag.f", iedName, index),
			FC:        "MX",
		})
	}
	return results
}

func insertIEC61850BrowseNodeAt(nodes *[]IEC61850BrowseNode, parts []string, objectRef string, fc string, dataType string, depth int) {
	if depth >= len(parts) {
		return
	}
	ref := joinIEC61850ObjectRef(parts[:depth+1])
	index := -1
	for i := range *nodes {
		if (*nodes)[i].ObjectRef == ref || (*nodes)[i].Name == parts[depth] {
			index = i
			break
		}
	}
	if index == -1 {
		*nodes = append(*nodes, IEC61850BrowseNode{Name: parts[depth], ObjectRef: ref, Leaf: depth == len(parts)-1})
		index = len(*nodes) - 1
	}
	if depth == len(parts)-1 {
		(*nodes)[index].ObjectRef = objectRef
		(*nodes)[index].FC = fc
		(*nodes)[index].DataType = dataType
		(*nodes)[index].Leaf = true
		return
	}
	(*nodes)[index].Leaf = false
	insertIEC61850BrowseNodeAt(&(*nodes)[index].Children, parts, objectRef, fc, dataType, depth+1)
}

func splitIEC61850ObjectRef(objectRef string) []string {
	objectRef = strings.TrimSpace(objectRef)
	if objectRef == "" {
		return nil
	}
	slash := strings.Index(objectRef, "/")
	if slash < 0 {
		return []string{objectRef}
	}
	parts := []string{objectRef[:slash]}
	for _, part := range strings.Split(objectRef[slash+1:], ".") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func joinIEC61850ObjectRef(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + "/" + strings.Join(parts[1:], ".")
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
	cRef := C.CString(objectRef)
	defer C.free(unsafe.Pointer(cRef))
	session := getIEC61850ClientSession(address)
	session.mu.Lock()
	defer session.mu.Unlock()
	for attempt := 0; attempt < 2; attempt++ {
		if err := session.connectLocked(); err != nil {
			return nil, err
		}
		var err C.IedClientError
		value := C.IedConnection_readObject(session.conn, &err, cRef, iec61850FunctionalConstraint(fc))
		if err == C.IED_ERROR_OK && value != nil {
			if C.MmsValue_getType(value) == C.MMS_DATA_ACCESS_ERROR {
				accessError := int(C.MmsValue_getDataAccessError(value))
				C.MmsValue_delete(value)
				return nil, fmt.Errorf("IEC61850 read %s[%s] data access error %d", objectRef, fc, accessError)
			}
			converted := iec61850MmsValue(value)
			C.MmsValue_delete(value)
			return applyIEC61850Scale(point, converted), nil
		}
		if value != nil {
			C.MmsValue_delete(value)
		}
		session.closeLocked()
		if attempt == 1 {
			if err != C.IED_ERROR_OK {
				return nil, iec61850ClientError("read "+objectRef+"["+fc+"]", address, err)
			}
			return nil, fmt.Errorf("IEC61850 read %s[%s] returned empty value", objectRef, fc)
		}
	}
	return nil, fmt.Errorf("IEC61850 read %s[%s] failed", objectRef, fc)
}

func ProbeIEC61850ObjectRefs(ctx context.Context, address string, refs []IEC61850ProbeResult) ([]IEC61850ProbeResult, error) {
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
	C.IedConnection_setConnectTimeout(conn, 3000)
	C.IedConnection_setRequestTimeout(conn, 3000)

	var err C.IedClientError
	C.IedConnection_connect(conn, &err, cHost, C.int(port))
	if err != C.IED_ERROR_OK {
		return nil, iec61850ClientError("MMS association", address, err)
	}
	defer C.IedConnection_close(conn)

	results := make([]IEC61850ProbeResult, 0, len(refs))
	for _, ref := range refs {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		ref.ObjectRef = strings.TrimSpace(ref.ObjectRef)
		if ref.ObjectRef == "" {
			continue
		}
		fc := strings.TrimSpace(ref.FC)
		if fc == "" {
			fc = inferIEC61850FC(ref.ObjectRef, "")
		}
		cRef := C.CString(ref.ObjectRef)
		spec := C.IedConnection_getVariableSpecification(conn, &err, cRef, iec61850FunctionalConstraint(fc))
		ref.FC = fc
		ref.Exists = err == C.IED_ERROR_OK && spec != nil
		if spec != nil {
			C.MmsVariableSpecification_destroy(spec)
		}
		C.free(unsafe.Pointer(cRef))
		results = append(results, ref)
	}
	return results, nil
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

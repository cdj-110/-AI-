//go:build opcda && cgo

package collector

/*
#cgo CFLAGS: -I${SRCDIR}/../../native/wk-opcda/api
#cgo LDFLAGS: -lwk_opcda
#include <stdlib.h>
#include "wk_opcda.h"
extern void wkOpcdaGoDataCallback(wk_opcda_value_t*, size_t, void*);
static inline void wk_opcda_go_data_callback_bridge(const wk_opcda_value_t* values, size_t count, void* user_data) {
	wkOpcdaGoDataCallback((wk_opcda_value_t*)values, count, user_data);
}
static inline wk_opcda_error_t wk_opcda_subscribe_go(wk_opcda_handle_t* handle, void* user_data) {
	return wk_opcda_subscribe(handle, wk_opcda_go_data_callback_bridge, user_data);
}
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"runtime/cgo"
	"strings"
	"sync"
	"time"
	"unsafe"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type opcdaNativeSession struct {
	mu                    sync.Mutex
	deviceKey             string
	handle                *C.wk_opcda_handle_t
	pointKey              map[string]C.uint32_t
	pointType             map[string]string
	pointAccess           map[string]uint32
	nextHandle            uint32
	cacheMu               sync.RWMutex
	cache                 map[uint32]opcdaCachedValue
	callbackItems         map[uint32]opcdaCallbackItem
	subscribed            bool
	callbackID            cgo.Handle
	subscriptionRetryAt   time.Time
	subscriptionFailures  int
	subscriptionLastError string
}

type opcdaCallbackItem struct {
	deviceKey string
	address   string
	itemID    string
}

type opcdaCachedValue struct {
	value          interface{}
	actualDataType string
	quality        model.PointQuality
	rawQuality     uint32
	sourceTime     time.Time
	receiveTime    time.Time
	hresult        int32
	err            error
}

var opcdaNativeSessions = struct {
	sync.Mutex
	items map[string]*opcdaNativeSession
}{items: map[string]*opcdaNativeSession{}}

func readOPCDAPoints(ctx context.Context, points []config.PointConfig) []PointReadResult {
	if err := ctx.Err(); err != nil {
		return opcdaFailedResults(points, err)
	}
	key := (OPCDA{}).BatchKey(points[0])
	session := getOPCDANativeSession(key, points[0].DeviceKey)
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := session.ensureConnected(points[0]); err != nil {
		return opcdaFailedResults(points, err)
	}

	handles, itemErrors, sessionErr := session.ensureItems(points)
	if sessionErr != nil {
		return opcdaFailedResults(points, sessionErr)
	}
	if points[0].ReadMode == "subscription" {
		now := time.Now()
		if !session.subscribed && !now.Before(session.subscriptionRetryAt) {
			if err := session.ensureSubscription(); err != nil {
				session.subscriptionFailures++
				session.subscriptionRetryAt = now.Add(opcdaSubscriptionRetryDelay(session.subscriptionFailures))
				session.subscriptionLastError = err.Error()
				// A failed Advise leaves the native association in FAULT. Rebuild it
				// for the next subscription attempt, but never call IOPCSyncIO::Read
				// when the configured mode is asynchronous subscription.
				session.reset()
				return opcdaFailedResults(points, fmt.Errorf("OPC DA asynchronous subscription failed; IOPCSyncIO::Read is disabled in subscription mode: %w", err))
			} else {
				session.subscriptionFailures = 0
				session.subscriptionRetryAt = time.Time{}
				session.subscriptionLastError = ""
			}
		}
		if session.subscribed {
			return session.subscriptionResults(points, handles, itemErrors)
		}
		detail := session.subscriptionLastError
		if detail == "" {
			detail = "subscription retry is pending"
		}
		return opcdaFailedResults(points, fmt.Errorf("OPC DA asynchronous subscription unavailable; IOPCSyncIO::Read is disabled in subscription mode: %s", detail))
	}
	return session.readSync(ctx, points, handles, itemErrors)
}

func (s *opcdaNativeSession) readSync(ctx context.Context, points []config.PointConfig, handles []C.uint32_t, itemErrors []error) []PointReadResult {
	results := make([]PointReadResult, len(points))
	completed := make([]bool, len(points))
	for index, err := range itemErrors {
		if err != nil {
			results[index] = PointReadResult{Index: index, Err: err}
			completed[index] = true
		}
	}
	batchSize := points[0].BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	source := C.WK_OPCDA_READ_DEVICE
	if points[0].ReadSource == "cache" {
		source = C.WK_OPCDA_READ_CACHE
	}
	sourceName := "DEVICE"
	if source == C.WK_OPCDA_READ_CACHE {
		sourceName = "CACHE"
	}

	var readBatch func([]int) error
	readBatch = func(indexes []int) error {
		if len(indexes) == 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		readHandles := make([]C.uint32_t, len(indexes))
		for offset, index := range indexes {
			readHandles[offset] = handles[index]
		}
		values := make([]C.wk_opcda_value_t, len(readHandles))
		recordOPCDASyncRead(points[indexes[0]].DeviceKey, points[indexes[0]].Address, sourceName, len(readHandles), "tx", nil)
		code := C.wk_opcda_read(s.handle, C.wk_opcda_read_source_t(source), &readHandles[0], C.size_t(len(readHandles)), &values[0])
		if code == C.WK_OPCDA_OK {
			recordOPCDASyncRead(points[indexes[0]].DeviceKey, points[indexes[0]].Address, sourceName, len(readHandles), "rx", nil)
			for offset, index := range indexes {
				key := opcdaItemKey(points[index])
				results[index] = opcdaPointResult(index, points[index], values[offset], s.pointType[key])
				completed[index] = true
			}
			return nil
		}

		err := s.nativeError("read", code)
		recordOPCDASyncRead(points[indexes[0]].DeviceKey, points[indexes[0]].Address, sourceName, len(readHandles), "rx", err)
		if isOPCDAVariantDecodeError(err) {
			if len(indexes) == 1 {
				index := indexes[0]
				results[index] = PointReadResult{
					Index: index,
					Err:   fmt.Errorf("OPC DA SyncRead itemId=%s VARIANT decode failed: %w", points[index].ItemID, err),
				}
				completed[index] = true
				return nil
			}
			middle := len(indexes) / 2
			if err := readBatch(indexes[:middle]); err != nil {
				return err
			}
			return readBatch(indexes[middle:])
		}

		// RPC and OPC session errors can invalidate handles and security state.
		// Keep the existing full-session rebuild behavior for those failures.
		s.reset()
		return err
	}

	for start := 0; start < len(points); start += batchSize {
		if err := ctx.Err(); err != nil {
			for index := start; index < len(points); index++ {
				if !completed[index] {
					results[index] = PointReadResult{Index: index, Err: err}
					completed[index] = true
				}
			}
			break
		}
		end := start + batchSize
		if end > len(points) {
			end = len(points)
		}
		var readIndexes []int
		for index := start; index < end; index++ {
			if itemErrors[index] == nil {
				readIndexes = append(readIndexes, index)
			}
		}
		if len(readIndexes) == 0 {
			continue
		}
		if err := readBatch(readIndexes); err != nil {
			for index := start; index < len(points); index++ {
				if !completed[index] {
					results[index] = PointReadResult{Index: index, Err: err}
					completed[index] = true
				}
			}
			break
		}
	}
	return results
}

func testOPCDAConnection(ctx context.Context, point config.PointConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key := (OPCDA{}).BatchKey(point)
	session, shared := existingOPCDANativeSession(key)
	if !shared {
		session = newOPCDANativeSession(point.DeviceKey)
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !shared {
		defer session.reset()
	}
	return session.ensureConnected(point)
}

func browseOPCDANodes(ctx context.Context, point config.PointConfig, branchItemID string, limit int) ([]OPCDABrowseNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	operationKey := "browse\x00" + opcdaBrowseSessionID(ctx)
	session := getOPCDABrowseSession(operationKey, (OPCDA{}).BatchKey(point), point.DeviceKey)
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := session.ensureConnected(point); err != nil {
		return nil, err
	}
	branch := C.CString(branchItemID)
	defer C.free(unsafe.Pointer(branch))
	entries := make([]C.wk_opcda_browse_entry_t, limit)
	var count C.size_t
	code := C.wk_opcda_browse(session.handle, branch, &entries[0], C.size_t(limit), &count)
	if code != C.WK_OPCDA_OK {
		err := session.nativeError("browse", code)
		session.reset()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int(count) > len(entries) {
		return nil, fmt.Errorf("OPC DA browse returned invalid entry count %d", int(count))
	}
	result := make([]OPCDABrowseNode, 0, int(count))
	leaves := make([]config.PointConfig, 0, int(count))
	for index := 0; index < int(count); index++ {
		entry := entries[index]
		isBranch := entry.is_branch != 0
		result = append(result, OPCDABrowseNode{
			Name: C.GoString(entry.name), ItemID: C.GoString(entry.item_id),
			DataType: C.GoString(entry.data_type), Access: "unknown",
			AccessRights: uint32(entry.access_rights), IsBranch: isBranch,
			HasChildren: isBranch, Selectable: !isBranch,
		})
		if !isBranch {
			active := false
			leaves = append(leaves, config.PointConfig{
				DeviceKey: point.DeviceKey, Protocol: "opcda", Address: point.Address,
				ItemID: result[len(result)-1].ItemID, RequestedDataType: "auto",
				Active: &active,
			})
		}
	}
	if len(leaves) > 0 {
		_, itemErrors, addErr := session.ensureItems(leaves)
		if addErr == nil {
			leafIndex := 0
			for index := range result {
				if result[index].IsBranch {
					continue
				}
				if leafIndex < len(itemErrors) {
					key := opcdaItemKey(leaves[leafIndex])
					if dataType := session.pointType[key]; dataType != "" {
						result[index].DataType = dataType
						result[index].AccessRights = session.pointAccess[key]
						result[index].Access = opcdaAccessName(result[index].AccessRights)
						result[index].Selectable = opcdaCollectionTypeSupported(result[index].DataType)
					}
				}
				leafIndex++
			}
		}
	}
	return result, nil
}

func writeOPCDAPoint(ctx context.Context, point config.PointConfig, value interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key := (OPCDA{}).BatchKey(point)
	session := getOPCDANativeSession(key, point.DeviceKey)
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := session.ensureConnected(point); err != nil {
		return err
	}
	handles, itemErrors, sessionErr := session.ensureItems([]config.PointConfig{point})
	if sessionErr != nil {
		return sessionErr
	}
	if len(itemErrors) != 1 {
		return fmt.Errorf("OPC DA AddItems returned no item result")
	}
	if itemErrors[0] != nil {
		return itemErrors[0]
	}
	targetType := session.pointType[opcdaItemKey(point)]
	if targetType == "" || targetType == "empty" || targetType == "variant" {
		targetType = strings.ToLower(strings.TrimSpace(point.RequestedDataType))
		if targetType == "" || targetType == "auto" {
			targetType = strings.ToLower(strings.TrimSpace(point.DataType))
		}
	}
	nativeValue, cleanup, err := opcdaWriteValue(value, targetType)
	if err != nil {
		return err
	}
	defer cleanup()
	var itemResult C.int32_t
	if code := C.wk_opcda_write(session.handle, &handles[0], &nativeValue, 1, &itemResult); code != C.WK_OPCDA_OK {
		err := session.nativeError("write", code)
		session.reset()
		return err
	}
	if itemResult < 0 {
		return fmt.Errorf("OPC DA SyncWrite itemId=%s HRESULT=0x%08X", point.ItemID, uint32(itemResult))
	}
	return nil
}

func getOPCDANativeSession(key, deviceKey string) *opcdaNativeSession {
	opcdaNativeSessions.Lock()
	defer opcdaNativeSessions.Unlock()
	session := opcdaNativeSessions.items[key]
	if session == nil {
		session = newOPCDANativeSession(deviceKey)
		opcdaNativeSessions.items[key] = session
	}
	return session
}

func existingOPCDANativeSession(key string) (*opcdaNativeSession, bool) {
	opcdaNativeSessions.Lock()
	defer opcdaNativeSessions.Unlock()
	session := opcdaNativeSessions.items[key]
	return session, session != nil
}

func getOPCDABrowseSession(operationKey, collectionKey, deviceKey string) *opcdaNativeSession {
	opcdaNativeSessions.Lock()
	defer opcdaNativeSessions.Unlock()
	// A browse initiated from an already-running configured device can safely
	// reuse that association. This avoids opening and later tearing down a
	// second DCOM activation beside the subscription callback association.
	if session := opcdaNativeSessions.items[collectionKey]; session != nil {
		return session
	}
	if session := opcdaNativeSessions.items[operationKey]; session != nil {
		return session
	}
	session := newOPCDANativeSession(deviceKey)
	opcdaNativeSessions.items[operationKey] = session
	return session
}

func newOPCDANativeSession(deviceKey string) *opcdaNativeSession {
	return &opcdaNativeSession{
		deviceKey: strings.ToLower(strings.TrimSpace(deviceKey)),
		pointKey:  map[string]C.uint32_t{}, pointType: map[string]string{}, pointAccess: map[string]uint32{},
		cache: map[uint32]opcdaCachedValue{}, callbackItems: map[uint32]opcdaCallbackItem{}, nextHandle: 1,
	}
}

func (s *opcdaNativeSession) ensureConnected(point config.PointConfig) error {
	if s.handle != nil {
		var status C.wk_opcda_status_t
		if code := C.wk_opcda_get_status(s.handle, &status); code == C.WK_OPCDA_OK && status.state == C.WK_OPCDA_STATE_CONNECTED {
			return nil
		}
		s.reset()
	}
	if uint32(C.wk_opcda_abi_version()) != uint32(C.WK_OPCDA_ABI_VERSION) {
		return fmt.Errorf("OPC DA native ABI mismatch: library=%d gateway=%d", uint32(C.wk_opcda_abi_version()), uint32(C.WK_OPCDA_ABI_VERSION))
	}
	if point.SecurityMode == config.OPCDASecurityLegacyConnect {
		log.Printf("[AUTH] OPC DA device=%s explicitly uses Legacy Connect; packet integrity is disabled", point.DeviceKey)
	}
	strings := newOPCDAConfigStrings(point)
	defer strings.free()
	native := C.wk_opcda_config_t{
		abi_version: C.WK_OPCDA_ABI_VERSION,
		host:        strings.host, prog_id: strings.progID, clsid: strings.clsid,
		domain: strings.domain, username: strings.username, password: strings.password,
		workstation: strings.workstation, security_mode: strings.securityMode,
		authentication: strings.authentication, authentication_level: strings.authenticationLevel,
		group_name: strings.groupName, local_bind_ip: strings.localBindIP,
		locale_id: C.uint32_t(point.LocaleID), requested_update_rate_ms: C.uint32_t(point.RequestedUpdateRateMS),
		connect_timeout_ms: C.uint32_t(point.ConnectTimeoutMilliseconds), request_timeout_ms: C.uint32_t(point.RequestTimeoutMilliseconds),
		deadband: C.float(point.Deadband), auto_reconnect: boolByte(point.AutoReconnect == nil || *point.AutoReconnect),
	}
	if code := C.wk_opcda_create(&native, &s.handle); code != C.WK_OPCDA_OK {
		s.handle = nil
		return fmt.Errorf("OPC DA create failed: code=%d", int(code))
	}
	if code := C.wk_opcda_connect(s.handle); code != C.WK_OPCDA_OK {
		err := s.nativeError("connect", code)
		C.wk_opcda_destroy(s.handle)
		s.handle = nil
		return err
	}
	return nil
}

func opcdaWriteValue(value interface{}, targetType string) (C.wk_opcda_value_t, func(), error) {
	native := C.wk_opcda_value_t{}
	cleanup := func() {}
	normalized, err := normalizeOPCDAWriteValue(value, targetType)
	if err != nil {
		return native, cleanup, err
	}
	switch typed := normalized.(type) {
	case bool:
		native._type = C.WK_OPCDA_VALUE_BOOL
		if typed {
			native.int64_value = 1
		}
	case int:
		native._type, native.int64_value = C.WK_OPCDA_VALUE_INT64, C.int64_t(typed)
	case int8:
		native._type, native.int64_value = C.WK_OPCDA_VALUE_INT64, C.int64_t(typed)
	case int16:
		native._type, native.int64_value = C.WK_OPCDA_VALUE_INT64, C.int64_t(typed)
	case int32:
		native._type, native.int64_value = C.WK_OPCDA_VALUE_INT64, C.int64_t(typed)
	case int64:
		native._type, native.int64_value = C.WK_OPCDA_VALUE_INT64, C.int64_t(typed)
	case uint:
		native._type, native.uint64_value = C.WK_OPCDA_VALUE_UINT64, C.uint64_t(typed)
	case uint8:
		native._type, native.uint64_value = C.WK_OPCDA_VALUE_UINT64, C.uint64_t(typed)
	case uint16:
		native._type, native.uint64_value = C.WK_OPCDA_VALUE_UINT64, C.uint64_t(typed)
	case uint32:
		native._type, native.uint64_value = C.WK_OPCDA_VALUE_UINT64, C.uint64_t(typed)
	case uint64:
		native._type, native.uint64_value = C.WK_OPCDA_VALUE_UINT64, C.uint64_t(typed)
	case float32:
		native._type, native.float64_value = C.WK_OPCDA_VALUE_FLOAT64, C.double(typed)
	case float64:
		native._type, native.float64_value = C.WK_OPCDA_VALUE_FLOAT64, C.double(typed)
	case string:
		text := C.CString(typed)
		native._type, native.string_value, native.string_length = C.WK_OPCDA_VALUE_STRING, text, C.size_t(len(typed))
		cleanup = func() { C.free(unsafe.Pointer(text)) }
	case time.Time:
		native._type, native.int64_value = C.WK_OPCDA_VALUE_DATETIME, C.int64_t(typed.UnixMilli())
	default:
		return native, cleanup, fmt.Errorf("OPC DA Write does not support Go value type %T", normalized)
	}
	return native, cleanup, nil
}

func (s *opcdaNativeSession) ensureItems(points []config.PointConfig) ([]C.uint32_t, []error, error) {
	handles := make([]C.uint32_t, len(points))
	errors := make([]error, len(points))
	var definitions []C.wk_opcda_item_definition_t
	var definitionIndexes []int
	var allocated []*C.char
	defer func() {
		for _, value := range allocated {
			C.free(unsafe.Pointer(value))
		}
	}()
	for index, point := range points {
		key := opcdaItemKey(point)
		if handle := s.pointKey[key]; handle != 0 {
			handles[index] = handle
			if !opcdaCollectionTypeSupported(s.pointType[key]) {
				errors[index] = fmt.Errorf("OPC DA ARRAY itemId=%s dataType=%s is visible in Browse but collection is not supported by the current VARIANT codec", point.ItemID, s.pointType[key])
			}
			s.rememberCallbackItem(uint32(handle), point)
			continue
		}
		handle := C.uint32_t(s.nextHandle)
		s.nextHandle++
		itemID := C.CString(point.ItemID)
		accessPath := C.CString(point.AccessPath)
		dataType := C.CString(point.RequestedDataType)
		allocated = append(allocated, itemID, accessPath, dataType)
		active := point.Active == nil || *point.Active
		definitions = append(definitions, C.wk_opcda_item_definition_t{client_handle: handle, item_id: itemID, access_path: accessPath, requested_data_type: dataType, active: boolByte(active)})
		definitionIndexes = append(definitionIndexes, index)
		handles[index] = handle
		s.rememberCallbackItem(uint32(handle), point)
	}
	if len(definitions) == 0 {
		return handles, errors, nil
	}
	nativeResults := make([]C.wk_opcda_item_result_t, len(definitions))
	code := C.wk_opcda_add_items(s.handle, &definitions[0], C.size_t(len(definitions)), &nativeResults[0])
	if code != C.WK_OPCDA_OK {
		err := s.nativeError("add items", code)
		for _, index := range definitionIndexes {
			errors[index] = err
		}
		s.reset()
		return handles, errors, err
	}
	for offset, result := range nativeResults {
		index := definitionIndexes[offset]
		if result.hresult < 0 {
			errors[index] = fmt.Errorf("OPC DA AddItems itemId=%s HRESULT=0x%08X", points[index].ItemID, uint32(result.hresult))
			continue
		}
		key := opcdaItemKey(points[index])
		s.pointKey[key] = handles[index]
		s.pointType[key] = opcdaCanonicalDataType(uint16(result.canonical_data_type))
		s.pointAccess[key] = uint32(result.access_rights)
		if !opcdaCollectionTypeSupported(s.pointType[key]) {
			errors[index] = fmt.Errorf("OPC DA ARRAY itemId=%s dataType=%s is visible in Browse but collection is not supported by the current VARIANT codec", points[index].ItemID, s.pointType[key])
		}
	}
	return handles, errors, nil
}

func (s *opcdaNativeSession) rememberCallbackItem(handle uint32, point config.PointConfig) {
	s.cacheMu.Lock()
	s.callbackItems[handle] = opcdaCallbackItem{
		deviceKey: point.DeviceKey,
		address:   point.Address,
		itemID:    point.ItemID,
	}
	s.cacheMu.Unlock()
}

func (s *opcdaNativeSession) ensureSubscription() error {
	if s.subscribed {
		return nil
	}
	callbackID := cgo.NewHandle(s)
	code := C.wk_opcda_subscribe_go(s.handle, unsafe.Pointer(uintptr(callbackID)))
	if code != C.WK_OPCDA_OK {
		callbackID.Delete()
		return s.nativeError("subscribe", code)
	}
	s.callbackID = callbackID
	s.subscribed = true
	return nil
}

func (s *opcdaNativeSession) subscriptionResults(points []config.PointConfig, handles []C.uint32_t, itemErrors []error) []PointReadResult {
	results := make([]PointReadResult, len(points))
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	for index, point := range points {
		if itemErrors[index] != nil {
			results[index] = PointReadResult{Index: index, Err: itemErrors[index]}
			continue
		}
		cached, ok := s.cache[uint32(handles[index])]
		if !ok {
			results[index] = PointReadResult{Index: index, Err: fmt.Errorf("OPC DA subscription is waiting for the first OnDataChange itemId=%s", point.ItemID)}
			continue
		}
		if cached.err != nil {
			results[index] = PointReadResult{Index: index, Err: cached.err}
			continue
		}
		value := model.PointValue{
			DeviceKey: point.DeviceKey, Metric: point.Metric,
			Value:          applyOPCDAScale(point, cached.value),
			ActualDataType: cached.actualDataType,
			Quality:        cached.quality, RawQuality: cached.rawQuality,
			SourceTimestamp: cached.sourceTime, ReceiveTimestamp: cached.receiveTime,
			CollectionMode: "subscription", DiagnosticCode: int64(cached.hresult),
		}
		results[index] = PointReadResult{Index: index, Value: value}
	}
	return results
}

//export wkOpcdaGoDataCallback
func wkOpcdaGoDataCallback(values *C.wk_opcda_value_t, valueCount C.size_t, userData unsafe.Pointer) {
	if values == nil || valueCount == 0 || userData == nil {
		return
	}
	session, ok := cgo.Handle(uintptr(userData)).Value().(*opcdaNativeSession)
	if !ok || session == nil {
		return
	}
	nativeValues := unsafe.Slice(values, int(valueCount))
	now := time.Now()
	updates := make(map[uint32]opcdaCachedValue, len(nativeValues))
	for index := range nativeValues {
		native := nativeValues[index]
		handle := uint32(native.client_handle)
		cached := opcdaCachedValue{
			receiveTime: now, rawQuality: uint32(native.quality),
			quality:        normalizeOPCDAQuality(uint32(native.quality)),
			hresult:        int32(native.hresult),
			actualDataType: opcdaNativeValueTypeName(int(native._type)),
		}
		if native.hresult < 0 {
			cached.err = fmt.Errorf("OPC DA OnDataChange clientHandle=%d HRESULT=0x%08X", uint32(native.client_handle), uint32(native.hresult))
		} else {
			cached.value, cached.err = opcdaValue(native)
		}
		if native.source_timestamp_unix_ms > 0 {
			cached.sourceTime = time.UnixMilli(int64(native.source_timestamp_unix_ms))
		}
		session.cacheMu.RLock()
		item := session.callbackItems[handle]
		_, previouslyCached := session.cache[handle]
		session.cacheMu.RUnlock()
		itemID := item.itemID
		if itemID == "" {
			itemID = fmt.Sprintf("clientHandle=%d", handle)
		}
		recordOPCDAOnDataChange(item.deviceKey, item.address, itemID, cached.value, cached.quality, cached.sourceTime, cached.receiveTime, !previouslyCached, cached.err)
		updates[handle] = cached
	}
	session.cacheMu.Lock()
	for handle, value := range updates {
		session.cache[handle] = value
	}
	session.cacheMu.Unlock()
}

// reset discards a native association after an RPC or OPC session failure.
// The runtime retry scheduler will call ensureConnected again after its
// configured backoff, which performs a complete DCOM activation and AddGroup.
func (s *opcdaNativeSession) reset() {
	if s.handle != nil {
		if s.subscribed {
			if code := C.wk_opcda_unsubscribe(s.handle); code != C.WK_OPCDA_OK {
				log.Printf("OPC DA unsubscribe during reset failed: %s", s.nativeError("unsubscribe", code))
			}
			s.subscribed = false
			if s.callbackID != 0 {
				s.callbackID.Delete()
				s.callbackID = 0
			}
		}
		C.wk_opcda_disconnect(s.handle)
		C.wk_opcda_destroy(s.handle)
		s.handle = nil
	}
	s.pointKey = map[string]C.uint32_t{}
	s.pointType = map[string]string{}
	s.pointAccess = map[string]uint32{}
	s.nextHandle = 1
	s.cacheMu.Lock()
	s.cache = map[uint32]opcdaCachedValue{}
	s.callbackItems = map[uint32]opcdaCallbackItem{}
	s.cacheMu.Unlock()
}

func opcdaItemKey(point config.PointConfig) string {
	return point.ItemID + "\x00" + point.AccessPath
}

func opcdaPointResult(index int, point config.PointConfig, value C.wk_opcda_value_t, actualDataType string) PointReadResult {
	if value.hresult < 0 {
		return PointReadResult{Index: index, Err: fmt.Errorf("OPC DA SyncRead itemId=%s HRESULT=0x%08X", point.ItemID, uint32(value.hresult))}
	}
	converted, err := opcdaValue(value)
	if err != nil {
		return PointReadResult{Index: index, Err: err}
	}
	quality := normalizeOPCDAQuality(uint32(value.quality))
	converted = applyOPCDAScale(point, converted)
	if actualDataType == "" {
		actualDataType = opcdaNativeValueTypeName(int(value._type))
	}
	result := model.PointValue{
		DeviceKey: point.DeviceKey, Metric: point.Metric, Value: converted,
		ActualDataType: actualDataType,
		Quality:        quality, RawQuality: uint32(value.quality),
		ReceiveTimestamp: time.Now(), CollectionMode: "sync", DiagnosticCode: int64(value.hresult),
	}
	if value.source_timestamp_unix_ms > 0 {
		result.SourceTimestamp = time.UnixMilli(int64(value.source_timestamp_unix_ms))
	}
	return PointReadResult{Index: index, Value: result}
}

func opcdaValue(value C.wk_opcda_value_t) (interface{}, error) {
	switch value._type {
	case C.WK_OPCDA_VALUE_EMPTY:
		return nil, nil
	case C.WK_OPCDA_VALUE_BOOL:
		return value.int64_value != 0, nil
	case C.WK_OPCDA_VALUE_INT64:
		return int64(value.int64_value), nil
	case C.WK_OPCDA_VALUE_UINT64:
		return uint64(value.uint64_value), nil
	case C.WK_OPCDA_VALUE_FLOAT64:
		return float64(value.float64_value), nil
	case C.WK_OPCDA_VALUE_STRING:
		return C.GoStringN(value.string_value, C.int(value.string_length)), nil
	case C.WK_OPCDA_VALUE_DATETIME:
		return time.UnixMilli(int64(value.int64_value)), nil
	case C.WK_OPCDA_VALUE_FLOAT64_ARRAY:
		if value.array_length == 0 {
			return []float64{}, nil
		}
		if value.float64_array_values == nil {
			return nil, fmt.Errorf("OPC DA returned a null float64 SAFEARRAY with length %d", uint64(value.array_length))
		}
		nativeValues := unsafe.Slice(value.float64_array_values, int(value.array_length))
		converted := make([]float64, len(nativeValues))
		for index := range nativeValues {
			converted[index] = float64(nativeValues[index])
		}
		return converted, nil
	case C.WK_OPCDA_VALUE_STRING_ARRAY:
		if value.array_length == 0 {
			return []string{}, nil
		}
		if value.string_array_values == nil || value.string_array_lengths == nil {
			return nil, fmt.Errorf("OPC DA returned a null string SAFEARRAY with length %d", uint64(value.array_length))
		}
		nativeValues := unsafe.Slice(value.string_array_values, int(value.array_length))
		nativeLengths := unsafe.Slice(value.string_array_lengths, int(value.array_length))
		converted := make([]string, len(nativeValues))
		for index := range nativeValues {
			if nativeValues[index] == nil && nativeLengths[index] != 0 {
				return nil, fmt.Errorf("OPC DA returned a null BSTR element %d with length %d", index, uint64(nativeLengths[index]))
			}
			converted[index] = C.GoStringN(nativeValues[index], C.int(nativeLengths[index]))
		}
		return converted, nil
	default:
		return nil, fmt.Errorf("OPC DA returned unsupported native value type %d", int(value._type))
	}
}

func opcdaCollectionTypeSupported(dataType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(dataType))
	return !strings.HasSuffix(normalized, "[]") || normalized == "float64[]" || normalized == "string[]"
}

func normalizeOPCDAQuality(raw uint32) model.PointQuality {
	switch raw & 0xC0 {
	case 0xC0:
		return model.QualityGood
	case 0x40:
		return model.QualityUncertain
	default:
		return model.QualityBad
	}
}

func (s *opcdaNativeSession) nativeError(operation string, code C.wk_opcda_error_t) error {
	detail := ""
	if s.handle != nil {
		detail = C.GoString(C.wk_opcda_last_error(s.handle))
	}
	return fmt.Errorf("OPC DA %s failed: code=%d detail=%s", operation, int(code), detail)
}

func opcdaFailedResults(points []config.PointConfig, err error) []PointReadResult {
	results := make([]PointReadResult, len(points))
	for index := range points {
		results[index] = PointReadResult{Index: index, Err: err}
	}
	return results
}

func closeOPCDASessions() {
	opcdaNativeSessions.Lock()
	sessions := opcdaNativeSessions.items
	opcdaNativeSessions.items = map[string]*opcdaNativeSession{}
	opcdaNativeSessions.Unlock()
	for _, session := range sessions {
		session.mu.Lock()
		session.reset()
		session.mu.Unlock()
	}
}

func closeOPCDABrowseSession(sessionID string) {
	if sessionID == "" {
		return
	}
	key := "browse\x00" + sessionID
	opcdaNativeSessions.Lock()
	session := opcdaNativeSessions.items[key]
	delete(opcdaNativeSessions.items, key)
	opcdaNativeSessions.Unlock()
	if session == nil {
		return
	}
	session.mu.Lock()
	session.reset()
	session.mu.Unlock()
}

func closeOPCDASessionsForDevices(deviceKeys []string) {
	if len(deviceKeys) == 0 {
		return
	}
	targets := make(map[string]struct{}, len(deviceKeys))
	for _, deviceKey := range deviceKeys {
		if deviceKey = strings.ToLower(strings.TrimSpace(deviceKey)); deviceKey != "" {
			targets[deviceKey] = struct{}{}
		}
	}
	if len(targets) == 0 {
		return
	}
	opcdaNativeSessions.Lock()
	sessions := make([]*opcdaNativeSession, 0, len(targets))
	for key, session := range opcdaNativeSessions.items {
		if _, ok := targets[session.deviceKey]; !ok {
			continue
		}
		delete(opcdaNativeSessions.items, key)
		sessions = append(sessions, session)
	}
	opcdaNativeSessions.Unlock()
	for _, session := range sessions {
		session.mu.Lock()
		log.Printf("[OPCDA][HOT-RELOAD] rebuilding device session device=%s", session.deviceKey)
		session.reset()
		session.subscriptionRetryAt = time.Time{}
		session.subscriptionFailures = 0
		session.subscriptionLastError = ""
		session.mu.Unlock()
	}
}

type opcdaConfigStrings struct {
	host, progID, clsid, domain, username, password                *C.char
	workstation, securityMode, authentication, authenticationLevel *C.char
	groupName, localBindIP                                         *C.char
}

func newOPCDAConfigStrings(point config.PointConfig) opcdaConfigStrings {
	return opcdaConfigStrings{
		host: C.CString(point.Address), progID: C.CString(point.ProgID), clsid: C.CString(point.CLSID),
		domain: C.CString(point.Domain), username: C.CString(point.Username), password: C.CString(point.Password),
		workstation: C.CString(point.Workstation), securityMode: C.CString(point.SecurityMode),
		authentication: C.CString(point.Authentication), authenticationLevel: C.CString(point.AuthenticationLevel),
		groupName: C.CString(point.GroupName), localBindIP: C.CString(point.LocalBindIP),
	}
}

func (s opcdaConfigStrings) free() {
	for _, value := range []*C.char{s.host, s.progID, s.clsid, s.domain, s.username, s.password, s.workstation, s.securityMode, s.authentication, s.authenticationLevel, s.groupName, s.localBindIP} {
		C.free(unsafe.Pointer(value))
	}
}

func boolByte(value bool) C.uint8_t {
	if value {
		return 1
	}
	return 0
}

package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/mapper"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

var ErrWritablePointNotFound = errors.New("writable point not found")

type Manager struct {
	mu            sync.RWMutex
	cfg           config.Config
	state         *state.Store
	lanes         map[string]CollectionLane
	laneOrder     []string
	retryAfter    map[string]time.Time
	retryAttempts map[string]int
	lastCollected map[string]time.Time
	lastWritten   map[string]time.Time
}

type CollectionLane struct {
	Key       string
	DeviceKey string
	Channel   string
	Points    []config.PointConfig
	Interval  time.Duration
}

func NewManager(cfg config.Config, store *state.Store) *Manager {
	lanes, order := buildCollectionLanes(cfg)
	return &Manager{cfg: cfg, state: store, lanes: lanes, laneOrder: order, retryAfter: map[string]time.Time{}, retryAttempts: map[string]int{}, lastCollected: map[string]time.Time{}, lastWritten: map[string]time.Time{}}
}

func (m *Manager) UpdateConfig(cfg config.Config) {
	previous := m.Config()
	if !protocolPointsEqual(previous.Points, cfg.Points, "modbus-rtu") {
		collector.CloseRTUConnections()
	}
	if !protocolPointsEqual(previous.Points, cfg.Points, "dlt645") {
		collector.CloseDLT645Connections()
	}
	if !protocolPointsEqual(previous.Points, cfg.Points, "iec104") {
		collector.CloseIEC104Connections()
	}
	if !protocolPointsEqual(previous.Points, cfg.Points, "iec61850") {
		collector.CloseIEC61850Connections()
	}
	if !protocolPointsEqual(previous.Points, cfg.Points, "iec61850-goose") {
		collector.CloseIEC61850GOOSEConnections()
	}
	if devices := collector.OPCDASessionResetDevices(previous.Points, cfg.Points); len(devices) > 0 {
		collector.CloseOPCDAConnectionsForDevices(devices)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.lanes, m.laneOrder = buildCollectionLanes(cfg)
	m.retryAfter = map[string]time.Time{}
	m.retryAttempts = map[string]int{}
	m.lastCollected = map[string]time.Time{}
	m.lastWritten = map[string]time.Time{}
	m.state.ReplaceConfig(cfg)
}

func protocolPointsEqual(previous, next []config.PointConfig, protocol string) bool {
	selectProtocol := func(points []config.PointConfig) []config.PointConfig {
		selected := make([]config.PointConfig, 0)
		for _, point := range points {
			if config.NormalizeProtocol(point.Protocol) == protocol {
				selected = append(selected, point)
			}
		}
		return selected
	}
	return reflect.DeepEqual(selectProtocol(previous), selectProtocol(next))
}

func (m *Manager) Config() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) CollectInterval() time.Duration {
	cfg := m.Config()
	return cfg.CollectInterval()
}

func (m *Manager) CollectOnce(ctx context.Context) map[string]map[string]interface{} {
	return m.collect(ctx, false)
}

// CollectNow is a user-triggered collection and intentionally bypasses retry backoff.
func (m *Manager) CollectNow(ctx context.Context) map[string]map[string]interface{} {
	return m.collect(ctx, true)
}

func (m *Manager) WritePoint(ctx context.Context, deviceKey, metric string, value interface{}) (config.PointConfig, error) {
	point, ok := findWritablePoint(m.Config(), deviceKey, metric)
	if !ok {
		return config.PointConfig{}, fmt.Errorf("%w for metric %s", ErrWritablePointNotFound, metric)
	}
	writer, err := collector.New(point.Protocol)
	if err != nil {
		return config.PointConfig{}, err
	}
	pointWriter, ok := writer.(collector.PointWriter)
	if !ok {
		return config.PointConfig{}, fmt.Errorf("protocol %s does not support write", point.Protocol)
	}
	if err := pointWriter.WritePoint(ctx, point, value); err != nil {
		return config.PointConfig{}, err
	}
	m.markWritten(point)
	if config.NormalizeProtocol(point.Protocol) == "dlt645" && point.DLT645 != nil && point.DLT645.Access != "write" {
		if reader, ok := writer.(collector.Collector); ok {
			if refreshed, refreshErr := reader.ReadPoint(ctx, point); refreshErr == nil {
				refreshed.ApplyDefaults(time.Now())
				m.state.SetPointResult(refreshed)
				return point, nil
			}
		}
	}
	m.state.SetPointValue(point.DeviceKey, point.Metric, value)
	return point, nil
}

func findWritablePoint(cfg config.Config, deviceKey, metric string) (config.PointConfig, bool) {
	for _, point := range cfg.Points {
		if point.Metric != metric || (deviceKey != "" && point.DeviceKey != deviceKey) {
			continue
		}
		if _, canWrite := PointAccess(point); canWrite {
			return point, true
		}
	}
	return config.PointConfig{}, false
}

func pointSupportsWrite(point config.PointConfig) bool {
	protocol := config.NormalizeProtocol(point.Protocol)
	return ((protocol == "modbus-tcp" || protocol == "modbus-rtu") && (point.Function == 1 || point.Function == 3)) ||
		(protocol == "siemens-s7" && collector.SiemensS7SupportsWrite(point)) ||
		(protocol == "opcda" && (point.Access == "write" || point.Access == "read_write")) ||
		(protocol == "dlt645" && point.DLT645 != nil && (point.DLT645.Access == "write" || point.DLT645.Access == "read_write"))
}

// PointAccess reports executable capabilities, not permissions edited in the cloud.
func PointAccess(point config.PointConfig) (canRead, canWrite bool) {
	if point.Enabled != nil && !*point.Enabled {
		return false, false
	}
	canRead = isPointCollectable(point)
	if config.NormalizeProtocol(point.Protocol) == "dlt645" && point.DLT645 != nil && point.DLT645.Access == "write" {
		canRead = false
	}
	return canRead, pointSupportsWrite(point)
}

func (m *Manager) PointAccesses() []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, point := range m.Config().Points {
		canRead, canWrite := PointAccess(point)
		result = append(result, map[string]interface{}{"deviceKey": point.DeviceKey, "metric": point.Metric, "canRead": canRead, "canWrite": canWrite})
	}
	return result
}

// WritablePoints reports the same capability rules used by WritePoint.
func (m *Manager) WritablePoints() []map[string]string {
	result := []map[string]string{}
	for _, point := range m.Config().Points {
		if pointSupportsWrite(point) && (point.Enabled == nil || *point.Enabled) {
			result = append(result, map[string]string{"deviceKey": point.DeviceKey, "metric": point.Metric})
		}
	}
	return result
}

func (m *Manager) CollectionLanes() []CollectionLane {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]CollectionLane, 0, len(m.laneOrder))
	for _, key := range m.laneOrder {
		result = append(result, m.lanes[key])
	}
	return result
}

func buildCollectionLanes(cfg config.Config) (map[string]CollectionLane, []string) {
	fallback := cfg.CollectInterval()
	type laneBuild struct {
		lane       CollectionLane
		start      int
		last       int
		contiguous bool
	}
	lanes := map[string]*laneBuild{}
	var order []string
	for pointIndex, point := range cfg.Points {
		if point.DeviceKey == "" || point.Metric == "" || !isPointCollectable(point) {
			continue
		}
		key := collectionLaneKey(point)
		build := lanes[key]
		if build == nil {
			build = &laneBuild{
				lane: CollectionLane{
					Key:       key,
					DeviceKey: point.DeviceKey,
					Channel:   collectionLaneChannel(point),
				},
				start: pointIndex, last: pointIndex, contiguous: true,
			}
			lanes[key] = build
			order = append(order, key)
		} else {
			if pointIndex != build.last+1 {
				build.contiguous = false
			}
			build.last = pointIndex
		}
		interval := pointCollectInterval(point, fallback)
		if build.lane.Interval <= 0 || interval < build.lane.Interval {
			build.lane.Interval = interval
		}
	}
	result := make(map[string]CollectionLane, len(order))
	for _, key := range order {
		build := lanes[key]
		lane := build.lane
		if build.contiguous {
			lane.Points = cfg.Points[build.start : build.last+1]
		} else {
			lane.Points = make([]config.PointConfig, 0, build.last-build.start+1)
			for _, point := range cfg.Points {
				if isPointCollectable(point) && collectionLaneKey(point) == key {
					lane.Points = append(lane.Points, point)
				}
			}
		}
		configuredInterval := lane.Interval
		lane.Interval = protectedLargeLaneInterval(len(lane.Points), lane.Interval)
		if lane.Interval != configuredInterval {
			log.Printf("large collection lane protected key=%s points=%d configured=%s effective=%s", lane.Key, len(lane.Points), configuredInterval, lane.Interval)
		}
		result[key] = lane
	}
	return result, order
}

const largeLanePointsPerSecond = 50000

func protectedLargeLaneInterval(pointCount int, configured time.Duration) time.Duration {
	if pointCount <= largeLanePointsPerSecond {
		return configured
	}
	minimum := time.Duration(float64(pointCount) / float64(largeLanePointsPerSecond) * float64(time.Second))
	if minimum < time.Second {
		minimum = time.Second
	}
	if configured <= 0 || configured < minimum {
		return minimum
	}
	return configured
}

func (m *Manager) CollectLane(ctx context.Context, laneKey string, force bool) map[string]map[string]interface{} {
	return m.CollectLaneDetailed(ctx, laneKey, force).Grouped
}

// CollectedBatch keeps the protocol-neutral PointValue metadata alongside the
// legacy grouped payload. Every collector, regardless of protocol, reaches
// this boundary before telemetry is published or persisted.
type CollectedBatch struct {
	Grouped map[string]map[string]interface{}
	Points  map[string]map[string]model.PointValue
}

func (m *Manager) CollectLaneDetailed(ctx context.Context, laneKey string, force bool) CollectedBatch {
	m.mu.RLock()
	lane, ok := m.lanes[laneKey]
	m.mu.RUnlock()
	if !ok {
		return CollectedBatch{}
	}
	return m.collectPointSet(ctx, lane.Points, lane.Interval, force)
}

func (m *Manager) collect(ctx context.Context, force bool) map[string]map[string]interface{} {
	m.mu.RLock()
	allPoints := append([]config.PointConfig(nil), m.cfg.Points...)
	interval := m.cfg.CollectInterval()
	m.mu.RUnlock()
	return m.collectPointSet(ctx, allPoints, interval, force).Grouped
}

func (m *Manager) collectPointSet(ctx context.Context, allPoints []config.PointConfig, interval time.Duration, force bool) CollectedBatch {
	m.state.MarkCollect()
	now := time.Now()
	points := allPoints
	if !force {
		points = m.duePoints(allPoints, now, interval)
	}
	points = orderPointsForCollection(points)
	collectCtx, cancel := context.WithTimeout(ctx, collectTimeoutForPoints(points, interval))
	defer cancel()

	grouped := make(map[string]map[string]interface{})
	details := make(map[string]map[string]model.PointValue)
	for _, result := range collectPointsWithDLTContext(collectCtx, ctx, points) {
		if result.Index < 0 || result.Index >= len(points) {
			continue
		}
		point := points[result.Index]
		if m.wasWrittenAfter(point, now) {
			continue
		}
		if result.Err != nil {
			if errors.Is(result.Err, collector.ErrCollectionDeferred) {
				continue
			}
			m.state.SetPointError(point, result.Err)
			m.markRetryLater(point, now)
			continue
		}
		m.markCollected(point, now)
		if result.Value.ActualDataType == "" && !strings.EqualFold(strings.TrimSpace(point.DataType), "auto") {
			result.Value.ActualDataType = point.DataType
		}
		result.Value.ApplyDefaults(time.Now())
		if result.Value.CollectionMode == "" {
			result.Value.CollectionMode = "poll"
		}
		m.state.SetPointResult(result.Value)
		if grouped[result.Value.DeviceKey] == nil {
			grouped[result.Value.DeviceKey] = map[string]interface{}{}
		}
		grouped[result.Value.DeviceKey][result.Value.Metric] = result.Value.Value
		if details[result.Value.DeviceKey] == nil {
			details[result.Value.DeviceKey] = map[string]model.PointValue{}
		}
		details[result.Value.DeviceKey][result.Value.Metric] = result.Value
	}
	return CollectedBatch{Grouped: grouped, Points: details}
}

func (m *Manager) markWritten(point config.PointConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.lastWritten[pointKey(point)] = now
	for _, configured := range m.cfg.Points {
		if configured.DeviceKey == point.DeviceKey {
			delete(m.retryAfter, pointKey(configured))
			delete(m.retryAttempts, pointKey(configured))
			delete(m.lastCollected, pointKey(configured))
		}
	}
}

func (m *Manager) wasWrittenAfter(point config.PointConfig, collectionStartedAt time.Time) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	writtenAt, ok := m.lastWritten[pointKey(point)]
	return ok && writtenAt.After(collectionStartedAt)
}

func collectionLaneKey(point config.PointConfig) string {
	if config.NormalizeProtocol(point.Protocol) == "dlt645" {
		return "dlt645::" + point.Address
	}
	return point.DeviceKey + "::" + collectionLaneChannel(point)
}

func collectionLaneChannel(point config.PointConfig) string {
	if point.ChannelKey != "" {
		return point.ChannelKey
	}
	return fmt.Sprintf("%s#%s#%d", point.Protocol, point.Address, point.SlaveID)
}

func collectionLaneInterval(points []config.PointConfig, fallback time.Duration) time.Duration {
	interval := fallback
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for _, point := range points {
		pointInterval := pointCollectInterval(point, fallback)
		if interval <= 0 || pointInterval < interval {
			interval = pointInterval
		}
	}
	return interval
}

func (m *Manager) duePoints(points []config.PointConfig, now time.Time, fallbackInterval time.Duration) []config.PointConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	dueCount := 0
	for _, point := range points {
		key := pointKey(point)
		next, ok := m.retryAfter[key]
		if ok && now.Before(next) {
			continue
		}
		interval := pointCollectInterval(point, fallbackInterval)
		if last, ok := m.lastCollected[key]; ok && now.Add(dueTolerance(interval)).Sub(last) < interval {
			continue
		}
		dueCount++
	}
	if dueCount == len(points) {
		return points
	}
	if dueCount == 0 {
		return nil
	}
	due := make([]config.PointConfig, 0, dueCount)
	for _, point := range points {
		key := pointKey(point)
		if next, ok := m.retryAfter[key]; ok && now.Before(next) {
			continue
		}
		interval := pointCollectInterval(point, fallbackInterval)
		if last, ok := m.lastCollected[key]; ok && now.Add(dueTolerance(interval)).Sub(last) < interval {
			continue
		}
		due = append(due, point)
	}
	return due
}

func (m *Manager) markRetryLater(point config.PointConfig, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := pointKey(point)
	delay := deviceReconnectDelay(point, m.cfg.Devices)
	if point.Protocol == "opcda" {
		m.retryAttempts[key]++
		delay = opcdaReconnectDelay(m.retryAttempts[key])
	}
	m.retryAfter[key] = now.Add(delay)
}

func (m *Manager) markCollected(point config.PointConfig, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := pointKey(point)
	delete(m.retryAfter, key)
	delete(m.retryAttempts, key)
	m.lastCollected[key] = now
}

func opcdaReconnectDelay(attempt int) time.Duration {
	steps := [...]time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	if attempt <= 1 {
		return steps[0]
	}
	if attempt >= len(steps) {
		return steps[len(steps)-1]
	}
	return steps[attempt-1]
}

func pointKey(point config.PointConfig) string {
	return point.DeviceKey + "::" + point.Metric
}

func orderPointsForCollection(points []config.PointConfig) []config.PointConfig {
	lastPriority := -1
	orderedAlready := true
	for _, point := range points {
		priority := collectProtocolPriority(point.Protocol)
		if priority < lastPriority {
			orderedAlready = false
			break
		}
		lastPriority = priority
	}
	if orderedAlready {
		return points
	}
	ordered := append([]config.PointConfig(nil), points...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return collectProtocolPriority(ordered[i].Protocol) < collectProtocolPriority(ordered[j].Protocol)
	})
	return ordered
}

func collectProtocolPriority(protocol string) int {
	switch protocol {
	case "modbus-tcp", "modbus-rtu", "dlt645":
		return 0
	case "siemens-s7":
		return 1
	case "iec104":
		return 2
	case "opcua", "opcda":
		return 3
	case "iec61850":
		return 4
	case "iec61850-goose":
		return 4
	default:
		return 5
	}
}

func deviceReconnectDelay(point config.PointConfig, devices []config.DeviceConfig) time.Duration {
	if point.Protocol == "iec61850-goose" {
		// A GOOSE subscription is persistent and receives asynchronously.
		// Retrying it like a disconnected TCP device delays the first visible
		// value by the device's usual 30-second reconnect interval.
		return 500 * time.Millisecond
	}
	seconds := 30
	for _, device := range devices {
		if device.DeviceKey == point.DeviceKey {
			if device.ReconnectIntervalSeconds > 0 {
				seconds = device.ReconnectIntervalSeconds
			}
			break
		}
	}
	return time.Duration(seconds) * time.Second
}

func dueTolerance(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 100 * time.Millisecond
	}
	tolerance := interval / 10
	if tolerance > 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	if tolerance < 10*time.Millisecond {
		return 10 * time.Millisecond
	}
	return tolerance
}

func pointCollectInterval(point config.PointConfig, fallback time.Duration) time.Duration {
	return point.CollectInterval(fallback)
}

func maxPointCollectInterval(points []config.PointConfig, fallback time.Duration) time.Duration {
	maxInterval := fallback
	if maxInterval <= 0 {
		maxInterval = 5 * time.Second
	}
	for _, point := range points {
		if interval := pointCollectInterval(point, fallback); interval > maxInterval {
			maxInterval = interval
		}
	}
	return maxInterval
}

func collectTimeout(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 5 * time.Second
	}
	if interval <= time.Second {
		timeout := interval - interval/10
		// Collection frequency and request timeout are separate concerns. A
		// 100ms schedule must not turn normal 100-200ms device responses into
		// false communication failures.
		if timeout < 250*time.Millisecond {
			return 250 * time.Millisecond
		}
		return timeout
	}
	timeout := interval - 250*time.Millisecond
	if timeout < time.Second {
		return time.Second
	}
	return timeout
}

// collectTimeoutForPoints separates an IEC 61850 MMS transaction budget from
// its scheduling interval. A one-second schedule must not cancel a multi-batch
// read merely because the device has hundreds of selected data attributes.
func collectTimeoutForPoints(points []config.PointConfig, interval time.Duration) time.Duration {
	timeout := collectTimeout(interval)
	iec61850Points := 0
	for _, point := range points {
		if config.NormalizeProtocol(point.Protocol) == "iec61850" {
			iec61850Points++
		}
	}
	if iec61850Points == 0 {
		return timeout
	}
	batches := (iec61850Points + collector.IEC61850BatchSize - 1) / collector.IEC61850BatchSize
	minimum := 2*time.Second + time.Duration(batches)*200*time.Millisecond
	if minimum > 30*time.Second {
		minimum = 30 * time.Second
	}
	if timeout < minimum {
		return minimum
	}
	return timeout
}

type collectResult struct {
	Index int
	Value model.PointValue
	Err   error
}

const maxBatchRegisters uint16 = 125
const maxConcurrentReads = 8

func collectPoints(ctx context.Context, points []config.PointConfig) []collectResult {
	return collectPointsWithDLTContext(ctx, ctx, points)
}

// Serial DL/T645 batches use the caller's cancellation context, not the
// schedule-derived deadline. Other protocols keep their existing budgets.
func collectPointsWithDLTContext(ctx, dltCtx context.Context, points []config.PointConfig) []collectResult {
	type collectionTask struct {
		indexes []int
		run     func() []collectResult
		ctx     context.Context
	}
	var tasks []collectionTask
	modbusGroups := map[string][]int{}
	var modbusOrder []string
	type batchBuild struct {
		reader  collector.BatchCollector
		indexes []int
	}
	batchGroups := map[string]*batchBuild{}
	var batchOrder []string
	for index, point := range points {
		if point.Metric == "" || !isPointCollectable(point) {
			continue
		}
		if point.Protocol == "modbus-tcp" || point.Protocol == "modbus-rtu" {
			key := serializedModbusKey(point)
			if _, ok := modbusGroups[key]; !ok {
				modbusOrder = append(modbusOrder, key)
			}
			modbusGroups[key] = append(modbusGroups[key], index)
			continue
		}
		if reader, err := collector.New(point.Protocol); err == nil {
			if batchReader, ok := reader.(collector.BatchCollector); ok {
				key := point.Protocol + "#" + batchReader.BatchKey(point)
				build := batchGroups[key]
				if build == nil {
					build = &batchBuild{reader: batchReader}
					batchGroups[key] = build
					batchOrder = append(batchOrder, key)
				}
				build.indexes = append(build.indexes, index)
				continue
			}
		}
		pointIndex := index
		pointCopy := point
		tasks = append(tasks, collectionTask{indexes: []int{pointIndex}, run: func() []collectResult {
			value, err := ReadPoint(ctx, pointCopy)
			return []collectResult{{Index: pointIndex, Value: value, Err: err}}
		}})
	}
	for _, key := range batchOrder {
		build := batchGroups[key]
		indexes := append([]int(nil), build.indexes...)
		reader := build.reader
		batchCtx := ctx
		if _, ok := reader.(collector.DLT645); ok {
			batchCtx = dltCtx
		}
		tasks = append(tasks, collectionTask{indexes: indexes, ctx: batchCtx, run: func() []collectResult {
			batchPoints := make([]config.PointConfig, len(indexes))
			for offset, index := range indexes {
				batchPoints[offset] = points[index]
			}
			batchResults := reader.ReadPoints(batchCtx, batchPoints)
			results := make([]collectResult, 0, len(indexes))
			seen := make([]bool, len(indexes))
			for _, item := range batchResults {
				if item.Index < 0 || item.Index >= len(indexes) {
					continue
				}
				seen[item.Index] = true
				results = append(results, collectResult{Index: indexes[item.Index], Value: item.Value, Err: item.Err})
			}
			for offset, index := range indexes {
				if !seen[offset] {
					results = append(results, collectResult{Index: index, Err: fmt.Errorf("protocol %s batch read omitted result %d", batchPoints[offset].Protocol, offset)})
				}
			}
			return results
		}})
	}
	for _, key := range modbusOrder {
		indexes := append([]int(nil), modbusGroups[key]...)
		tasks = append(tasks, collectionTask{indexes: indexes, run: func() []collectResult {
			return collectSerializedModbusPoints(ctx, indexes, points)
		}})
	}
	if len(tasks) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentReads)
	resultsCh := make(chan []collectResult, len(tasks))
	for _, task := range tasks {
		task := task
		if task.ctx == nil {
			task.ctx = ctx
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				resultsCh <- task.run()
			case <-task.ctx.Done():
				items := make([]collectResult, 0, len(task.indexes))
				for _, index := range task.indexes {
					failure := task.ctx.Err()
					if config.NormalizeProtocol(points[index].Protocol) == "dlt645" {
						failure = collector.ErrCollectionDeferred
					}
					items = append(items, collectResult{Index: index, Err: failure})
				}
				resultsCh <- items
			}
		}()
	}
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var results []collectResult
	for items := range resultsCh {
		results = append(results, items...)
	}
	return results
}

func isPointCollectable(point config.PointConfig) bool {
	return config.NormalizeProtocol(point.Protocol) != "opcda" ||
		strings.ToLower(strings.TrimSpace(point.Access)) != "write"
}

func serializedModbusKey(point config.PointConfig) string {
	if point.Protocol == "modbus-rtu" {
		return point.Protocol + "#" + point.Address
	}
	return fmt.Sprintf("%s#%s#%d", point.Protocol, point.Address, point.SlaveID)
}

func collectSerializedModbusPoints(ctx context.Context, indexes []int, points []config.PointConfig) []collectResult {
	byFunction := map[byte][]int{}
	for _, index := range indexes {
		function := points[index].Function
		byFunction[function] = append(byFunction[function], index)
	}
	var results []collectResult
	for _, function := range []byte{3, 4, 1, 2} {
		functionIndexes := byFunction[function]
		if len(functionIndexes) == 0 {
			continue
		}
		if function == 3 || function == 4 {
			results = append(results, collectRegisterBatches(ctx, functionIndexes, points)...)
			continue
		}
		results = append(results, collectBitBatches(ctx, functionIndexes, points)...)
	}
	return results
}

func collectBitBatches(ctx context.Context, indexes []int, points []config.PointConfig) []collectResult {
	sort.Slice(indexes, func(i, j int) bool { return points[indexes[i]].Register < points[indexes[j]].Register })
	const maxBatchBits uint16 = 2000
	var results []collectResult
	for start := 0; start < len(indexes); {
		end := start + 1
		rangeStart := points[indexes[start]].Register
		rangeEnd := rangeStart + 1
		for end < len(indexes) {
			nextEnd := points[indexes[end]].Register + 1
			if nextEnd-rangeStart > maxBatchBits {
				break
			}
			rangeEnd = nextEnd
			end++
		}
		batchIndexes := indexes[start:end]
		raw, err := readRegisterRange(ctx, points[batchIndexes[0]], rangeStart, rangeEnd-rangeStart)
		if err != nil {
			for _, index := range batchIndexes {
				results = append(results, collectResult{Index: index, Err: err})
			}
			start = end
			continue
		}
		for _, index := range batchIndexes {
			point := points[index]
			bitOffset := int(point.Register - rangeStart)
			byteOffset := bitOffset / 8
			if byteOffset >= len(raw) {
				results = append(results, collectResult{Index: index, Err: fmt.Errorf("batched bit response too short for address %d", point.Register)})
				continue
			}
			value := raw[byteOffset]&(1<<uint(bitOffset%8)) != 0
			results = append(results, collectResult{Index: index, Value: model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}})
		}
		start = end
	}
	return results
}

func groupBatchablePoints(points []config.PointConfig) (map[string][]int, []string) {
	groups := make(map[string][]int)
	var order []string
	for index, point := range points {
		if point.Metric == "" {
			continue
		}
		if !isBatchable(point) {
			continue
		}
		key := groupKey(point)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], index)
	}
	return groups, order
}

func groupKey(point config.PointConfig) string {
	return fmt.Sprintf("%s#%s#%d#%d", point.Protocol, point.Address, point.SlaveID, point.Function)
}

func isBatchable(point config.PointConfig) bool {
	return (point.Protocol == "modbus-tcp" || point.Protocol == "modbus-rtu") &&
		(point.Function == 3 || point.Function == 4)
}

func collectRegisterBatches(ctx context.Context, indexes []int, points []config.PointConfig) []collectResult {
	sort.Slice(indexes, func(i, j int) bool {
		return points[indexes[i]].Register < points[indexes[j]].Register
	})

	var results []collectResult
	for start := 0; start < len(indexes); {
		end := start + 1
		rangeStart := points[indexes[start]].Register
		rangeEnd := pointEnd(points[indexes[start]])
		for end < len(indexes) && points[indexes[end]].Register <= rangeEnd {
			nextEnd := pointEnd(points[indexes[end]])
			if nextEnd-rangeStart > maxBatchRegisters {
				break
			}
			if nextEnd > rangeEnd {
				rangeEnd = nextEnd
			}
			end++
		}

		batchIndexes := indexes[start:end]
		quantity := rangeEnd - rangeStart
		raw, err := readRegisterRange(ctx, points[batchIndexes[0]], rangeStart, quantity)
		if err != nil {
			for _, index := range batchIndexes {
				results = append(results, collectResult{Index: index, Err: err})
			}
			start = end
			continue
		}

		for _, index := range batchIndexes {
			point := points[index]
			offset := int(point.Register-rangeStart) * 2
			length := int(point.Quantity) * 2
			if offset < 0 || offset+length > len(raw) {
				results = append(results, collectResult{Index: index, Err: fmt.Errorf("batched response too short for register %d", point.Register)})
				continue
			}
			value, err := mapper.Decode(point, raw[offset:offset+length])
			if err != nil {
				results = append(results, collectResult{Index: index, Err: err})
				continue
			}
			results = append(results, collectResult{
				Index: index,
				Value: model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value},
			})
		}
		start = end
	}
	return results
}

func pointEnd(point config.PointConfig) uint16 {
	if point.Quantity == 0 {
		point.ApplyDefaults()
	}
	return point.Register + point.Quantity
}

func readRegisterRange(ctx context.Context, point config.PointConfig, start uint16, quantity uint16) ([]byte, error) {
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	reader, err := collector.New(point.Protocol)
	if err != nil {
		return nil, err
	}
	rangeReader, ok := reader.(collector.RegisterRangeReader)
	if !ok {
		return nil, fmt.Errorf("protocol %s does not support batched register read", point.Protocol)
	}
	return rangeReader.ReadRegisterRange(readCtx, point, start, quantity)
}

func ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	if config.NormalizeProtocol(point.Protocol) == "dlt645" {
		return (collector.DLT645{}).ReadPoint(ctx, point)
	}
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	reader, err := collector.New(point.Protocol)
	if err != nil {
		return model.PointValue{}, err
	}
	return reader.ReadPoint(readCtx, point)
}

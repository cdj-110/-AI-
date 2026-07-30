package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
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
	return &Manager{cfg: cfg, state: store, lanes: lanes, laneOrder: order, retryAfter: map[string]time.Time{}, lastCollected: map[string]time.Time{}, lastWritten: map[string]time.Time{}}
}

func (m *Manager) UpdateConfig(cfg config.Config) {
	collector.CloseRTUConnections()
	collector.CloseIEC104Connections()
	collector.CloseIEC61850Connections()
	collector.CloseIEC61850GOOSEConnections()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.lanes, m.laneOrder = buildCollectionLanes(cfg)
	m.retryAfter = map[string]time.Time{}
	m.lastCollected = map[string]time.Time{}
	m.lastWritten = map[string]time.Time{}
	m.state.ReplaceConfig(cfg)
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
	m.state.SetPointValue(point.DeviceKey, point.Metric, value)
	return point, nil
}

func findWritablePoint(cfg config.Config, deviceKey, metric string) (config.PointConfig, bool) {
	for _, point := range cfg.Points {
		if point.Metric != metric || (deviceKey != "" && point.DeviceKey != deviceKey) {
			continue
		}
		if (point.Protocol == "modbus-tcp" || point.Protocol == "modbus-rtu") && (point.Function == 1 || point.Function == 3) {
			return point, true
		}
	}
	return config.PointConfig{}, false
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
		if point.DeviceKey == "" || point.Metric == "" {
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
				if collectionLaneKey(point) == key {
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
	m.mu.RLock()
	lane, ok := m.lanes[laneKey]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	return m.collectPointSet(ctx, lane.Points, lane.Interval, force)
}

func (m *Manager) collect(ctx context.Context, force bool) map[string]map[string]interface{} {
	m.mu.RLock()
	allPoints := append([]config.PointConfig(nil), m.cfg.Points...)
	interval := m.cfg.CollectInterval()
	m.mu.RUnlock()
	return m.collectPointSet(ctx, allPoints, interval, force)
}

func (m *Manager) collectPointSet(ctx context.Context, allPoints []config.PointConfig, interval time.Duration, force bool) map[string]map[string]interface{} {
	m.state.MarkCollect()
	now := time.Now()
	points := allPoints
	if !force {
		points = m.duePoints(allPoints, now, interval)
	}
	points = orderPointsForCollection(points)
	collectCtx, cancel := context.WithTimeout(ctx, collectTimeout(interval))
	defer cancel()

	grouped := make(map[string]map[string]interface{})
	for _, result := range collectPoints(collectCtx, points) {
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
		m.state.SetPointValue(result.Value.DeviceKey, result.Value.Metric, result.Value.Value)
		if grouped[result.Value.DeviceKey] == nil {
			grouped[result.Value.DeviceKey] = map[string]interface{}{}
		}
		grouped[result.Value.DeviceKey][result.Value.Metric] = result.Value.Value
	}
	return grouped
}

func (m *Manager) markWritten(point config.PointConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.lastWritten[pointKey(point)] = now
	for _, configured := range m.cfg.Points {
		if configured.DeviceKey == point.DeviceKey {
			delete(m.retryAfter, pointKey(configured))
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
	m.retryAfter[pointKey(point)] = now.Add(deviceReconnectDelay(point, m.cfg.Devices))
}

func (m *Manager) markCollected(point config.PointConfig, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := pointKey(point)
	delete(m.retryAfter, key)
	m.lastCollected[key] = now
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
	case "modbus-tcp", "modbus-rtu":
		return 0
	case "siemens-s7":
		return 1
	case "iec104":
		return 2
	case "opcua":
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

type collectResult struct {
	Index int
	Value model.PointValue
	Err   error
}

const maxBatchRegisters uint16 = 125
const maxConcurrentReads = 8

func collectPoints(ctx context.Context, points []config.PointConfig) []collectResult {
	type collectionTask struct {
		indexes []int
		run     func() []collectResult
	}
	var tasks []collectionTask
	modbusGroups := map[string][]int{}
	var modbusOrder []string
	for index, point := range points {
		if point.Metric == "" {
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
		pointIndex := index
		pointCopy := point
		tasks = append(tasks, collectionTask{indexes: []int{pointIndex}, run: func() []collectResult {
			value, err := ReadPoint(ctx, pointCopy)
			return []collectResult{{Index: pointIndex, Value: value, Err: err}}
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
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				resultsCh <- task.run()
			case <-ctx.Done():
				items := make([]collectResult, 0, len(task.indexes))
				for _, index := range task.indexes {
					items = append(items, collectResult{Index: index, Err: ctx.Err()})
				}
				resultsCh <- items
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var results []collectResult
	for pending := len(tasks); pending > 0; {
		select {
		case items := <-resultsCh:
			results = append(results, items...)
			pending--
		case <-done:
			return results
		}
	}
	return results
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
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	reader, err := collector.New(point.Protocol)
	if err != nil {
		return model.PointValue{}, err
	}
	return reader.ReadPoint(readCtx, point)
}

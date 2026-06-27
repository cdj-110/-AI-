package runtime

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/mapper"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

type Manager struct {
	mu            sync.RWMutex
	cfg           config.Config
	state         *state.Store
	retryAfter    map[string]time.Time
	lastCollected map[string]time.Time
}

func NewManager(cfg config.Config, store *state.Store) *Manager {
	return &Manager{cfg: cfg, state: store, retryAfter: map[string]time.Time{}, lastCollected: map[string]time.Time{}}
}

func (m *Manager) UpdateConfig(cfg config.Config) {
	collector.CloseRTUConnections()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.retryAfter = map[string]time.Time{}
	m.lastCollected = map[string]time.Time{}
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

func (m *Manager) collect(ctx context.Context, force bool) map[string]map[string]interface{} {
	m.mu.RLock()
	allPoints := append([]config.PointConfig(nil), m.cfg.Points...)
	interval := m.cfg.CollectInterval()
	m.mu.RUnlock()

	m.state.MarkCollect()
	now := time.Now()
	points := allPoints
	if !force {
		points = m.duePoints(allPoints, now)
	}
	timeoutInterval := maxPointCollectInterval(points, interval)
	collectCtx, cancel := context.WithTimeout(ctx, collectTimeout(timeoutInterval))
	defer cancel()

	grouped := make(map[string]map[string]interface{})
	for _, point := range collectPoints(collectCtx, points) {
		if point.Err != nil {
			m.state.SetPointError(point.Point, point.Err)
			m.markRetryLater(point.Point, now)
			continue
		}
		m.markCollected(point.Point, now)
		m.state.SetPointValue(point.Value.DeviceKey, point.Value.Metric, point.Value.Value)
		if grouped[point.Value.DeviceKey] == nil {
			grouped[point.Value.DeviceKey] = map[string]interface{}{}
		}
		grouped[point.Value.DeviceKey][point.Value.Metric] = point.Value.Value
	}
	return grouped
}

func (m *Manager) duePoints(points []config.PointConfig, now time.Time) []config.PointConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	var due []config.PointConfig
	for _, point := range points {
		key := pointKey(point)
		next, ok := m.retryAfter[key]
		if ok && now.Before(next) {
			continue
		}
		interval := pointCollectInterval(point)
		if last, ok := m.lastCollected[key]; ok && now.Sub(last) < interval {
			continue
		}
		due = append(due, point)
	}
	return due
}

func (m *Manager) markRetryLater(point config.PointConfig, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryAfter[pointKey(point)] = now.Add(retryDelay(pointCollectInterval(point)))
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

func retryDelay(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = time.Second
	}
	delay := interval * 3
	if delay < interval {
		delay = interval
	}
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

func pointCollectInterval(point config.PointConfig) time.Duration {
	if point.CollectIntervalSeconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(point.CollectIntervalSeconds) * time.Second
}

func maxPointCollectInterval(points []config.PointConfig, fallback time.Duration) time.Duration {
	maxInterval := fallback
	if maxInterval <= 0 {
		maxInterval = 5 * time.Second
	}
	for _, point := range points {
		if interval := pointCollectInterval(point); interval > maxInterval {
			maxInterval = interval
		}
	}
	return maxInterval
}

func collectTimeout(interval time.Duration) time.Duration {
	if interval <= time.Second {
		return 950 * time.Millisecond
	}
	timeout := interval - 250*time.Millisecond
	if timeout < time.Second {
		return time.Second
	}
	return timeout
}

type collectResult struct {
	Point config.PointConfig
	Value model.PointValue
	Err   error
}

const maxBatchRegisters uint16 = 125
const maxConcurrentReads = 8

func collectPoints(ctx context.Context, points []config.PointConfig) []collectResult {
	groups := groupBatchablePoints(points)
	used := make(map[int]bool)
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentReads)

	taskCount := len(groups)
	for index, point := range points {
		if point.Metric == "" {
			continue
		}
		if _, ok := groups[groupKey(point)]; ok && isBatchable(point) {
			_ = index
			continue
		}
		taskCount++
	}
	resultsCh := make(chan []collectResult, taskCount)
	appendResults := func(items []collectResult) {
		select {
		case resultsCh <- items:
		case <-ctx.Done():
		}
	}
	for _, indexes := range groups {
		for _, index := range indexes {
			used[index] = true
		}
		batchIndexes := append([]int(nil), indexes...)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			appendResults(collectRegisterBatches(ctx, batchIndexes, points))
		}()
	}

	for index, point := range points {
		if used[index] {
			continue
		}
		if point.Metric == "" {
			continue
		}
		point := point
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			value, err := ReadPoint(ctx, point)
			if err != nil {
				appendResults([]collectResult{{Point: point, Err: err}})
				return
			}
			appendResults([]collectResult{{Point: point, Value: value}})
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var results []collectResult
	for pending := taskCount; pending > 0; {
		select {
		case items := <-resultsCh:
			results = append(results, items...)
			pending--
		case <-done:
			return results
		case <-ctx.Done():
			return results
		}
	}
	return results
}

func groupBatchablePoints(points []config.PointConfig) map[string][]int {
	groups := make(map[string][]int)
	for index, point := range points {
		if point.Metric == "" {
			continue
		}
		if !isBatchable(point) {
			continue
		}
		groups[groupKey(point)] = append(groups[groupKey(point)], index)
	}
	return groups
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
				results = append(results, collectResult{Point: points[index], Err: err})
			}
			start = end
			continue
		}

		for _, index := range batchIndexes {
			point := points[index]
			offset := int(point.Register-rangeStart) * 2
			length := int(point.Quantity) * 2
			if offset < 0 || offset+length > len(raw) {
				results = append(results, collectResult{Point: point, Err: fmt.Errorf("batched response too short for register %d", point.Register)})
				continue
			}
			value, err := mapper.Decode(point, raw[offset:offset+length])
			if err != nil {
				results = append(results, collectResult{Point: point, Err: err})
				continue
			}
			results = append(results, collectResult{
				Point: point,
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

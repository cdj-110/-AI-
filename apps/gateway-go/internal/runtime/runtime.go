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
	mu    sync.RWMutex
	cfg   config.Config
	state *state.Store
}

func NewManager(cfg config.Config, store *state.Store) *Manager {
	return &Manager{cfg: cfg, state: store}
}

func (m *Manager) UpdateConfig(cfg config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
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
	m.mu.RLock()
	points := append([]config.PointConfig(nil), m.cfg.Points...)
	m.mu.RUnlock()

	m.state.MarkCollect()
	grouped := make(map[string]map[string]interface{})
	for _, point := range collectPoints(ctx, points) {
		if point.Err != nil {
			m.state.SetPointError(point.Point, point.Err)
			continue
		}
		m.state.SetPointValue(point.Value.DeviceKey, point.Value.Metric, point.Value.Value)
		if grouped[point.Value.DeviceKey] == nil {
			grouped[point.Value.DeviceKey] = map[string]interface{}{}
		}
		grouped[point.Value.DeviceKey][point.Value.Metric] = point.Value.Value
	}
	return grouped
}

type collectResult struct {
	Point config.PointConfig
	Value model.PointValue
	Err   error
}

const maxBatchRegisters uint16 = 125

func collectPoints(ctx context.Context, points []config.PointConfig) []collectResult {
	groups := groupBatchablePoints(points)
	used := make(map[int]bool)
	var results []collectResult
	for _, indexes := range groups {
		for _, index := range indexes {
			used[index] = true
		}
		results = append(results, collectRegisterBatches(ctx, indexes, points)...)
	}

	for index, point := range points {
		if used[index] {
			continue
		}
		if point.Metric == "" {
			continue
		}
		value, err := ReadPoint(ctx, point)
		if err != nil {
			results = append(results, collectResult{Point: point, Err: err})
			continue
		}
		results = append(results, collectResult{Point: point, Value: value})
	}
	return results
}

func groupBatchablePoints(points []config.PointConfig) map[string][]int {
	groups := make(map[string][]int)
	for index, point := range points {
		if point.Metric == "" {
			continue
		}
		if point.Protocol != "modbus-tcp" || (point.Function != 3 && point.Function != 4) {
			continue
		}
		key := fmt.Sprintf("%s#%d#%d", point.Address, point.SlaveID, point.Function)
		groups[key] = append(groups[key], index)
	}
	return groups
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

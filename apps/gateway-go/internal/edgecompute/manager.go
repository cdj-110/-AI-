package edgecompute

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

const debounceInterval = 50 * time.Millisecond

type Batch struct {
	Time      time.Time
	Grouped   map[string]map[string]interface{}
	Heartbeat bool
}

type Manager struct {
	store                 *state.Store
	mu                    sync.RWMutex
	program               *Program
	revision              uint64
	lastEvaluatedRevision uint64
	observedInputs        map[string]inputSnapshot
	wake                  chan struct{}
	output                chan Batch
}

type inputSnapshot struct {
	Value     interface{}
	Error     string
	Available bool
}

func NewManager(cfg config.Config, store *state.Store) (*Manager, error) {
	program, err := Compile(cfg)
	if err != nil {
		return nil, err
	}
	return &Manager{store: store, program: program, revision: 1, observedInputs: map[string]inputSnapshot{}, wake: make(chan struct{}, 1), output: make(chan Batch, 256)}, nil
}

func (m *Manager) Output() <-chan Batch { return m.output }

func (m *Manager) UpdateConfig(cfg config.Config) error {
	program, err := Compile(cfg)
	if err != nil {
		return err
	}
	m.mu.Lock()
	previous := m.program
	m.program = program
	m.revision++
	m.mu.Unlock()
	for key, point := range program.ByKey {
		var previousPoint *CompiledPoint
		if previous != nil {
			previousPoint = previous.ByKey[key]
		}
		if previousPoint == nil || previousPoint.Signature != point.Signature {
			m.store.ResetPoint(point.Group.GroupKey, point.Config.Metric)
		}
	}
	m.signal()
	return nil
}

func (m *Manager) Start(ctx context.Context) {
	updates, unsubscribe := m.store.Subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(time.Second)
	defer heartbeat.Stop()
	lastHeartbeat := map[string]time.Time{}
	m.signal()
	var debounce *time.Timer
	var debounceChannel <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-updates:
			if debounce == nil {
				debounce = time.NewTimer(debounceInterval)
			} else {
				if !debounce.Stop() {
					select {
					case <-debounce.C:
					default:
					}
				}
				debounce.Reset(debounceInterval)
			}
			debounceChannel = debounce.C
		case <-m.wake:
			m.evaluate()
		case <-debounceChannel:
			debounceChannel = nil
			m.evaluate()
		case now := <-heartbeat.C:
			m.emitHeartbeats(now, lastHeartbeat)
		}
	}
}

func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) currentProgram() (*Program, uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.program, m.revision
}

func (m *Manager) evaluate() {
	program, revision := m.currentProgram()
	if program == nil || len(program.Points) == 0 {
		return
	}
	statuses := m.store.PointStatuses()
	byKey := make(map[string]state.PointStatus, len(statuses))
	for _, status := range statuses {
		byKey[sourceKey(status.DeviceKey, status.Metric)] = status
	}
	forceAll := revision != m.lastEvaluatedRevision
	changedInputs := map[string]bool{}
	for _, compiled := range program.Points {
		for _, input := range compiled.Config.Inputs {
			key := sourceKey(input.SourceDeviceKey, input.SourceMetric)
			current := snapshotOf(byKey[key])
			if forceAll || !reflect.DeepEqual(m.observedInputs[key], current) {
				changedInputs[key] = true
			}
		}
	}
	grouped := map[string]map[string]interface{}{}
	dirty := map[string]bool{}
	for _, compiled := range program.Points {
		if forceAll {
			dirty[compiled.Key] = true
		} else {
			for _, input := range compiled.Config.Inputs {
				key := sourceKey(input.SourceDeviceKey, input.SourceMetric)
				if changedInputs[key] || dirty[key] {
					dirty[compiled.Key] = true
					break
				}
			}
		}
		if !dirty[compiled.Key] {
			continue
		}
		pointConfig := computedPointConfig(compiled)
		values := map[string]interface{}{}
		var inputErr error
		for _, input := range compiled.Config.Inputs {
			key := sourceKey(input.SourceDeviceKey, input.SourceMetric)
			status, ok := byKey[key]
			if !ok || status.UpdatedAt == nil {
				inputErr = fmt.Errorf("输入 %s 尚无实时值", key)
				break
			}
			if status.Error != "" {
				inputErr = fmt.Errorf("输入 %s 异常: %s", key, status.Error)
				break
			}
			normalized, err := NormalizeInput(compiled.InputTypes[input.Alias], status.Value)
			if err != nil {
				inputErr = fmt.Errorf("输入 %s: %w", key, err)
				break
			}
			values[input.Alias] = normalized
		}
		if inputErr != nil {
			m.store.SetPointError(pointConfig, inputErr)
			markLocalError(byKey, compiled.Key, inputErr)
			continue
		}
		out, _, err := compiled.Program.Eval(values)
		if err != nil {
			evalErr := fmt.Errorf("表达式执行失败: %w", err)
			m.store.SetPointError(pointConfig, evalErr)
			markLocalError(byKey, compiled.Key, evalErr)
			continue
		}
		value, err := ConvertOutput(compiled.Config.DataType, out.Value())
		if err != nil {
			m.store.SetPointError(pointConfig, err)
			markLocalError(byKey, compiled.Key, err)
			continue
		}
		current := byKey[compiled.Key]
		changed := current.UpdatedAt == nil || current.Error != "" || !reflect.DeepEqual(current.Value, value)
		if changed {
			m.store.SetPointValue(compiled.Group.GroupKey, compiled.Config.Metric, value)
			if grouped[compiled.Group.GroupKey] == nil {
				grouped[compiled.Group.GroupKey] = map[string]interface{}{}
			}
			grouped[compiled.Group.GroupKey][compiled.Config.Metric] = value
			now := time.Now()
			current.Value = value
			current.Error = ""
			current.UpdatedAt = &now
			byKey[compiled.Key] = current
		}
	}
	m.lastEvaluatedRevision = revision
	for _, compiled := range program.Points {
		for _, input := range compiled.Config.Inputs {
			key := sourceKey(input.SourceDeviceKey, input.SourceMetric)
			m.observedInputs[key] = snapshotOf(byKey[key])
		}
	}
	if len(grouped) > 0 {
		m.emit(Batch{Time: time.Now(), Grouped: grouped})
	}
}

func snapshotOf(status state.PointStatus) inputSnapshot {
	return inputSnapshot{Value: status.Value, Error: status.Error, Available: status.UpdatedAt != nil}
}

func markLocalError(statuses map[string]state.PointStatus, key string, err error) {
	status := statuses[key]
	status.Error = err.Error()
	statuses[key] = status
}

func (m *Manager) emitHeartbeats(now time.Time, last map[string]time.Time) {
	program, _ := m.currentProgram()
	if program == nil {
		return
	}
	statuses := map[string]state.PointStatus{}
	for _, status := range m.store.PointStatuses() {
		statuses[sourceKey(status.DeviceKey, status.Metric)] = status
	}
	grouped := map[string]map[string]interface{}{}
	for _, point := range program.Points {
		interval := time.Duration(point.Group.HeartbeatSeconds) * time.Second
		if interval <= 0 {
			interval = program.Config.CollectInterval()
		}
		if lastTime := last[point.Group.GroupKey]; !lastTime.IsZero() && now.Sub(lastTime) < interval {
			continue
		}
		status := statuses[point.Key]
		if status.Error != "" || status.UpdatedAt == nil {
			continue
		}
		if grouped[point.Group.GroupKey] == nil {
			grouped[point.Group.GroupKey] = map[string]interface{}{}
		}
		grouped[point.Group.GroupKey][point.Config.Metric] = status.Value
	}
	for groupKey := range grouped {
		last[groupKey] = now
	}
	if len(grouped) > 0 {
		m.emit(Batch{Time: now, Grouped: grouped, Heartbeat: true})
	}
}

func (m *Manager) emit(batch Batch) {
	select {
	case m.output <- batch:
	default:
	}
}

func computedPointConfig(point *CompiledPoint) config.PointConfig {
	return config.PointConfig{DeviceKey: point.Group.GroupKey, Name: point.Config.Name, Metric: point.Config.Metric, Protocol: "edge-compute", Address: point.Group.GroupKey, DataType: point.Config.DataType, Unit: point.Config.Unit, Decimals: point.Config.Decimals, Quantity: 1, Scale: 1}
}

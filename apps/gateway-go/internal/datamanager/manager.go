package datamanager

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/state"
)

const diffFlushInterval = 50 * time.Millisecond

type Message struct {
	Type      string          `json:"type"`
	Sequence  uint64          `json:"sequence"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type PointDiff struct {
	Replace bool                `json:"replace"`
	Points  []state.PointStatus `json:"points"`
}

type DeviceStatus struct {
	DeviceKey string `json:"deviceKey"`
	Status    string `json:"status"`
}

type DeviceDiff struct {
	Replace bool           `json:"replace"`
	Devices []DeviceStatus `json:"devices"`
}

type CommunicationStatus struct {
	MQTTEnabled   bool                               `json:"mqttEnabled"`
	MQTTConnected bool                               `json:"mqttConnected"`
	MQTTChannels  map[string]state.MQTTChannelStatus `json:"mqttChannels"`
	Links         []LinkStatus                       `json:"links"`
}

type LinkStatus struct {
	Key      string `json:"key"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Status   string `json:"status"`
}

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type LogDiff struct {
	Replace bool       `json:"replace"`
	Entries []LogEntry `json:"entries"`
}

type Manager struct {
	store *state.Store

	mu          sync.Mutex
	subscribers map[chan Message]struct{}
	sequence    uint64
	logs        []LogEntry
	startOnce   sync.Once
}

func New(store *state.Store) *Manager {
	return &Manager{store: store, subscribers: map[chan Message]struct{}{}}
}

func (m *Manager) Start(ctx context.Context) {
	m.startOnce.Do(func() {
		ready := make(chan struct{})
		go m.run(ctx, ready)
		<-ready
	})
}

func (m *Manager) Subscribe(buffer int) (<-chan Message, func()) {
	if buffer < 1 {
		buffer = 64
	}
	updates := make(chan Message, buffer)
	m.mu.Lock()
	m.subscribers[updates] = struct{}{}
	m.mu.Unlock()
	return updates, func() {
		m.mu.Lock()
		if _, ok := m.subscribers[updates]; ok {
			delete(m.subscribers, updates)
			close(updates)
		}
		m.mu.Unlock()
	}
}

func (m *Manager) InitialMessages() []Message {
	snapshot := m.store.Snapshot()
	return []Message{
		m.Message("snapshot", snapshot),
		m.Message("devices.diff", DeviceDiff{Replace: true, Devices: deviceStatuses(snapshot.Points)}),
		m.Message("communications.diff", communicationStatus(snapshot)),
		m.Message("logs.diff", LogDiff{Replace: true, Entries: m.initialLogs(snapshot.Errors)}),
	}
}

func (m *Manager) Message(kind string, payload interface{}) Message {
	raw, _ := json.Marshal(payload)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sequence++
	return Message{Type: kind, Sequence: m.sequence, Timestamp: time.Now(), Payload: raw}
}

func (m *Manager) PublishLog(level string, message string) {
	entry := LogEntry{Time: time.Now(), Level: level, Message: message}
	m.mu.Lock()
	m.logs = append([]LogEntry{entry}, m.logs...)
	if len(m.logs) > 200 {
		m.logs = m.logs[:200]
	}
	m.sequence++
	msg := messageLocked(m.sequence, "logs.diff", LogDiff{Entries: []LogEntry{entry}})
	m.broadcastLocked(msg)
	m.mu.Unlock()
}

func (m *Manager) run(ctx context.Context, ready chan<- struct{}) {
	updates, unsubscribe := m.store.Subscribe()
	defer unsubscribe()

	pointRevision, statusRevision := m.store.Revisions()
	snapshot := m.store.Snapshot()
	previousPoints := indexPoints(snapshot.Points)
	previousDevices := indexDevices(deviceStatuses(snapshot.Points))
	previousCommunication := communicationStatus(snapshot)
	knownLogs := indexEvents(snapshot.Errors)
	close(ready)

	ticker := time.NewTicker(diffFlushInterval)
	defer ticker.Stop()
	dirty := false
	for {
		select {
		case _, open := <-updates:
			if !open {
				return
			}
			dirty = true
		case <-ticker.C:
			if !dirty {
				continue
			}
			dirty = false
			nextPointRevision, nextStatusRevision := m.store.Revisions()
			if nextPointRevision != pointRevision {
				currentPoints := m.store.PointStatuses()
				pointDiff, nextPoints := changedPoints(previousPoints, currentPoints)
				if pointDiff.Replace || len(pointDiff.Points) > 0 {
					m.broadcast("points.diff", pointDiff)
				}
				currentDevices := deviceStatuses(currentPoints)
				deviceDiff, nextDevices := changedDevices(previousDevices, currentDevices)
				if deviceDiff.Replace || len(deviceDiff.Devices) > 0 {
					m.broadcast("devices.diff", deviceDiff)
				}
				previousPoints = nextPoints
				previousDevices = nextDevices
				communication := previousCommunication
				communication.Links = linkStatuses(currentPoints)
				if !reflect.DeepEqual(previousCommunication, communication) {
					m.broadcast("communications.diff", communication)
					previousCommunication = communication
				}
				pointRevision = nextPointRevision
			}
			if nextStatusRevision != statusRevision {
				current := m.store.Snapshot()
				communication := communicationStatus(current)
				if !reflect.DeepEqual(previousCommunication, communication) {
					m.broadcast("communications.diff", communication)
					previousCommunication = communication
				}
				for _, entry := range newEvents(knownLogs, current.Errors) {
					m.broadcast("logs.diff", LogDiff{Entries: []LogEntry{{Time: entry.Time, Level: entry.Level, Message: entry.Message}}})
				}
				knownLogs = indexEvents(current.Errors)
				current.Points = nil
				current.Errors = nil
				m.broadcast("gateway.status", current)
				statusRevision = nextStatusRevision
			}
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) broadcast(kind string, payload interface{}) {
	raw, _ := json.Marshal(payload)
	m.mu.Lock()
	m.sequence++
	m.broadcastLocked(Message{Type: kind, Sequence: m.sequence, Timestamp: time.Now(), Payload: raw})
	m.mu.Unlock()
}

func (m *Manager) broadcastLocked(message Message) {
	for subscriber := range m.subscribers {
		select {
		case subscriber <- message:
		default:
			delete(m.subscribers, subscriber)
			close(subscriber)
		}
	}
}

func (m *Manager) initialLogs(events []state.Event) []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]LogEntry, 0, len(events)+len(m.logs))
	for _, event := range events {
		result = append(result, LogEntry{Time: event.Time, Level: event.Level, Message: event.Message})
	}
	result = append(result, m.logs...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].Time.After(result[j].Time) })
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

func messageLocked(sequence uint64, kind string, payload interface{}) Message {
	raw, _ := json.Marshal(payload)
	return Message{Type: kind, Sequence: sequence, Timestamp: time.Now(), Payload: raw}
}

func indexPoints(points []state.PointStatus) map[string]state.PointStatus {
	result := make(map[string]state.PointStatus, len(points))
	for _, point := range points {
		result[point.DeviceKey+"::"+point.Metric] = point
	}
	return result
}

func changedPoints(previous map[string]state.PointStatus, current []state.PointStatus) (PointDiff, map[string]state.PointStatus) {
	next := indexPoints(current)
	replace := keySetChanged(previous, next)
	if replace {
		return PointDiff{Replace: true, Points: current}, next
	}
	changed := make([]state.PointStatus, 0)
	for _, point := range current {
		if !reflect.DeepEqual(previous[point.DeviceKey+"::"+point.Metric], point) {
			changed = append(changed, point)
		}
	}
	return PointDiff{Points: changed}, next
}

func deviceStatuses(points []state.PointStatus) []DeviceStatus {
	grouped := map[string][]state.PointStatus{}
	for _, point := range points {
		grouped[point.DeviceKey] = append(grouped[point.DeviceKey], point)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]DeviceStatus, 0, len(keys))
	for _, key := range keys {
		status := "connecting"
		for _, point := range grouped[key] {
			if point.Error == "" && point.UpdatedAt != nil {
				status = "online"
				break
			}
			if point.Error != "" {
				status = "offline"
			}
		}
		result = append(result, DeviceStatus{DeviceKey: key, Status: status})
	}
	return result
}

func indexDevices(devices []DeviceStatus) map[string]DeviceStatus {
	result := make(map[string]DeviceStatus, len(devices))
	for _, device := range devices {
		result[device.DeviceKey] = device
	}
	return result
}

func changedDevices(previous map[string]DeviceStatus, current []DeviceStatus) (DeviceDiff, map[string]DeviceStatus) {
	next := indexDevices(current)
	if keySetChanged(previous, next) {
		return DeviceDiff{Replace: true, Devices: current}, next
	}
	changed := make([]DeviceStatus, 0)
	for _, device := range current {
		if previous[device.DeviceKey] != device {
			changed = append(changed, device)
		}
	}
	return DeviceDiff{Devices: changed}, next
}

func communicationStatus(snapshot state.Snapshot) CommunicationStatus {
	return CommunicationStatus{
		MQTTEnabled:   snapshot.MQTTEnabled,
		MQTTConnected: snapshot.MQTTConnected,
		MQTTChannels:  snapshot.MQTTChannels,
		Links:         linkStatuses(snapshot.Points),
	}
}

func linkStatuses(points []state.PointStatus) []LinkStatus {
	grouped := map[string][]state.PointStatus{}
	for _, point := range points {
		key := point.Protocol + "::" + point.Address
		grouped[key] = append(grouped[key], point)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]LinkStatus, 0, len(keys))
	for _, key := range keys {
		items := grouped[key]
		status := "connecting"
		for _, point := range items {
			if point.Error == "" && point.UpdatedAt != nil {
				status = "online"
				break
			}
			if point.Error != "" {
				status = "offline"
			}
		}
		result = append(result, LinkStatus{Key: key, Protocol: items[0].Protocol, Address: items[0].Address, Status: status})
	}
	return result
}

func indexEvents(events []state.Event) map[string]struct{} {
	result := make(map[string]struct{}, len(events))
	for _, event := range events {
		result[event.Time.Format(time.RFC3339Nano)+"::"+event.Level+"::"+event.Message] = struct{}{}
	}
	return result
}

func newEvents(previous map[string]struct{}, current []state.Event) []state.Event {
	result := make([]state.Event, 0)
	for index := len(current) - 1; index >= 0; index-- {
		event := current[index]
		key := event.Time.Format(time.RFC3339Nano) + "::" + event.Level + "::" + event.Message
		if _, ok := previous[key]; !ok {
			result = append(result, event)
		}
	}
	return result
}

func keySetChanged[A any, B any](previous map[string]A, next map[string]B) bool {
	if len(previous) != len(next) {
		return true
	}
	for key := range previous {
		if _, ok := next[key]; !ok {
			return true
		}
	}
	return false
}

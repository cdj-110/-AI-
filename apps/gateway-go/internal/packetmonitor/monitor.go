package packetmonitor

import (
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const defaultCapacity = 1000

type Frame struct {
	ID        uint64    `json:"id"`
	Time      time.Time `json:"time"`
	Protocol  string    `json:"protocol"`
	Direction string    `json:"direction"`
	DeviceKey string    `json:"deviceKey,omitempty"`
	Address   string    `json:"address,omitempty"`
	Summary   string    `json:"summary"`
	Hex       string    `json:"hex,omitempty"`
	Length    int       `json:"length"`
	Error     string    `json:"error,omitempty"`
}

type Monitor struct {
	mu          sync.Mutex
	capacity    int
	nextID      uint64
	frames      []Frame
	subscribers map[chan Frame]struct{}
}

func New(capacity int) *Monitor {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return &Monitor{capacity: capacity, subscribers: map[chan Frame]struct{}{}}
}

var Default = New(defaultCapacity)

func Record(frame Frame, payload []byte) {
	Default.Record(frame, payload)
}

func (m *Monitor) Record(frame Frame, payload []byte) {
	frame.Time = time.Now()
	frame.Direction = strings.ToLower(strings.TrimSpace(frame.Direction))
	frame.Protocol = strings.ToLower(strings.TrimSpace(frame.Protocol))
	frame.Length = len(payload)
	if len(payload) > 0 {
		frame.Hex = strings.ToUpper(hex.EncodeToString(payload))
	}
	m.mu.Lock()
	m.nextID++
	frame.ID = m.nextID
	m.frames = append(m.frames, frame)
	if overflow := len(m.frames) - m.capacity; overflow > 0 {
		copy(m.frames, m.frames[overflow:])
		m.frames = m.frames[:m.capacity]
	}
	for subscriber := range m.subscribers {
		select {
		case subscriber <- frame:
		default:
			delete(m.subscribers, subscriber)
			close(subscriber)
		}
	}
	m.mu.Unlock()
}

func (m *Monitor) Snapshot(deviceKey, protocol string, limit int) []Frame {
	if limit <= 0 || limit > m.capacity {
		limit = m.capacity
	}
	deviceKey = strings.TrimSpace(deviceKey)
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Frame, 0, limit)
	for index := len(m.frames) - 1; index >= 0 && len(result) < limit; index-- {
		frame := m.frames[index]
		if deviceKey != "" && frame.DeviceKey != deviceKey {
			continue
		}
		if protocol != "" && frame.Protocol != protocol {
			continue
		}
		result = append(result, frame)
	}
	return result
}

func (m *Monitor) Subscribe(buffer int) (<-chan Frame, func()) {
	if buffer <= 0 {
		buffer = 128
	}
	updates := make(chan Frame, buffer)
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

package state

import (
	"fmt"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
)

type Store struct {
	mu             sync.RWMutex
	startedAt      time.Time
	gatewayKey     string
	hardwareID     hardware.Identity
	collectSeconds int
	mqttEnabled    bool
	mqttConnected  bool
	mqttChannels   map[string]MQTTChannelStatus
	lastCollectAt  time.Time
	lastPublishAt  time.Time
	points         map[string]PointStatus
	pointOrder     []string
	errors         []Event
	pointRevision  uint64
	statusRevision uint64
	subscribers    map[chan struct{}]struct{}
}

type PointStatus struct {
	DeviceKey string      `json:"deviceKey"`
	Name      string      `json:"name"`
	Metric    string      `json:"metric"`
	Protocol  string      `json:"protocol"`
	Address   string      `json:"address"`
	Unit      string      `json:"unit,omitempty"`
	Decimals  int         `json:"decimals,omitempty"`
	Value     interface{} `json:"value"`
	UpdatedAt *time.Time  `json:"updatedAt,omitempty"`
	Error     string      `json:"error,omitempty"`
	ErrorAt   *time.Time  `json:"errorAt,omitempty"`
}

type Event struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type MQTTChannelStatus struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Name      string `json:"name,omitempty"`
}

type Snapshot struct {
	GatewayKey     string                       `json:"gatewayKey"`
	HardwareID     hardware.Identity            `json:"hardwareIdentity"`
	StartedAt      time.Time                    `json:"startedAt"`
	UptimeSeconds  int64                        `json:"uptimeSeconds"`
	CollectSeconds int                          `json:"collectSeconds"`
	MQTTEnabled    bool                         `json:"mqttEnabled"`
	MQTTConnected  bool                         `json:"mqttConnected"`
	MQTTChannels   map[string]MQTTChannelStatus `json:"mqttChannels"`
	LastCollectAt  *time.Time                   `json:"lastCollectAt,omitempty"`
	LastPublishAt  *time.Time                   `json:"lastPublishAt,omitempty"`
	PointCount     int                          `json:"pointCount"`
	HealthyCount   int                          `json:"healthyCount"`
	ErrorCount     int                          `json:"errorCount"`
	Points         []PointStatus                `json:"points"`
	Errors         []Event                      `json:"errors"`
	SystemMetrics  hardware.SystemMetrics       `json:"systemMetrics"`
}

func New(cfg config.Config) *Store {
	statusPoints := cfg.StatusPoints()
	points := make(map[string]PointStatus, len(statusPoints))
	pointOrder := make([]string, 0, len(statusPoints))
	for _, point := range statusPoints {
		key := pointKey(point.DeviceKey, point.Metric)
		points[key] = PointStatus{
			DeviceKey: point.DeviceKey,
			Name:      point.Name,
			Metric:    point.Metric,
			Protocol:  point.Protocol,
			Address:   point.Address,
			Unit:      point.Unit,
			Decimals:  point.Decimals,
		}
		pointOrder = append(pointOrder, key)
	}
	return &Store{
		startedAt:      time.Now(),
		gatewayKey:     cfg.GatewayKey,
		hardwareID:     hardware.ReadIdentity(),
		collectSeconds: collectSecondsForConfig(cfg),
		mqttEnabled:    cfg.ManualMQTTEnabled() || cfg.Activation.IsEnabled(),
		mqttChannels:   mqttChannelsForConfig(cfg),
		points:         points,
		pointOrder:     pointOrder,
		subscribers:    map[chan struct{}]struct{}{},
	}
}

func (s *Store) SetMQTTConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mqttConnected == connected {
		return
	}
	s.mqttConnected = connected
	s.notifyLocked(false, true)
}

func (s *Store) ResetMQTTChannels(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.mqttChannels
	next := mqttChannelsForConfig(cfg)
	for key, channel := range next {
		if channel.Enabled {
			channel.Connected = previous[key].Connected
			next[key] = channel
		}
	}
	s.mqttChannels = next
	s.mqttEnabled = cfg.ManualMQTTEnabled() || cfg.Activation.IsEnabled()
	s.mqttConnected = false
	for _, current := range s.mqttChannels {
		if current.Enabled && current.Connected {
			s.mqttConnected = true
			break
		}
	}
	s.notifyLocked(false, true)
}

func (s *Store) SetMQTTChannelConnected(name string, connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.mqttChannels[name]
	if !ok {
		return
	}
	channel.Connected = connected
	s.mqttChannels[name] = channel
	s.mqttConnected = false
	for _, current := range s.mqttChannels {
		if current.Enabled && current.Connected {
			s.mqttConnected = true
			break
		}
	}
	s.notifyLocked(false, true)
}

func (s *Store) ReplaceConfig(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gatewayKey = cfg.GatewayKey
	s.collectSeconds = collectSecondsForConfig(cfg)
	s.mqttEnabled = cfg.ManualMQTTEnabled() || cfg.Activation.IsEnabled()
	statusPoints := cfg.StatusPoints()
	next := make(map[string]PointStatus, len(statusPoints))
	pointOrder := make([]string, 0, len(statusPoints))
	for _, point := range statusPoints {
		key := pointKey(point.DeviceKey, point.Metric)
		status := s.points[key]
		status.DeviceKey = point.DeviceKey
		status.Name = point.Name
		status.Metric = point.Metric
		status.Protocol = point.Protocol
		status.Address = point.Address
		status.Unit = point.Unit
		status.Decimals = point.Decimals
		next[key] = status
		pointOrder = append(pointOrder, key)
	}
	s.points = next
	s.pointOrder = pointOrder
	s.notifyLocked(true, true)
}

func collectSecondsForConfig(cfg config.Config) int {
	seconds := int(cfg.CollectInterval().Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func (s *Store) MarkCollect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.lastCollectAt = now
	s.notifyLocked(false, true)
}

func (s *Store) MarkPublish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.lastPublishAt = now
	s.notifyLocked(false, true)
}

func (s *Store) SetPointValue(deviceKey string, metric string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	point := s.points[pointKey(deviceKey, metric)]
	hadError := point.Error != ""
	point.DeviceKey = deviceKey
	point.Metric = metric
	point.Value = value
	point.UpdatedAt = &now
	point.Error = ""
	point.ErrorAt = nil
	s.points[pointKey(deviceKey, metric)] = point
	s.notifyLocked(true, hadError)
}

func (s *Store) ResetPoint(deviceKey string, metric string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := pointKey(deviceKey, metric)
	point, ok := s.points[key]
	if !ok {
		return
	}
	point.Value = nil
	point.UpdatedAt = nil
	point.Error = ""
	point.ErrorAt = nil
	s.points[key] = point
	s.notifyLocked(true, true)
}

func (s *Store) SetPointError(point config.PointConfig, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	key := pointKey(point.DeviceKey, point.Metric)
	status := s.points[key]
	message := err.Error()
	if status.Error == message {
		return
	}
	status.DeviceKey = point.DeviceKey
	status.Name = point.Name
	status.Metric = point.Metric
	status.Protocol = point.Protocol
	status.Address = point.Address
	status.Unit = point.Unit
	status.Decimals = point.Decimals
	status.Error = message
	status.ErrorAt = &now
	s.points[key] = status
	s.appendErrorLocked(Event{Time: now, Level: "ERROR", Message: point.DeviceKey + "/" + point.Metric + ": " + message})
	s.notifyLocked(true, true)
}

func (s *Store) AddError(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendErrorLocked(Event{Time: time.Now(), Level: "ERROR", Message: message})
	s.notifyLocked(false, true)
}

// Subscribe returns a coalesced change signal. Consumers compare revisions and
// read the latest snapshot, so a slow browser cannot block data collection.
func (s *Store) Subscribe() (<-chan struct{}, func()) {
	updates := make(chan struct{}, 1)
	s.mu.Lock()
	if s.subscribers == nil {
		s.subscribers = map[chan struct{}]struct{}{}
	}
	s.subscribers[updates] = struct{}{}
	s.mu.Unlock()
	return updates, func() {
		s.mu.Lock()
		if _, ok := s.subscribers[updates]; ok {
			delete(s.subscribers, updates)
			close(updates)
		}
		s.mu.Unlock()
	}
}

func (s *Store) Revisions() (point uint64, status uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pointRevision, s.statusRevision
}

func (s *Store) notifyLocked(points bool, status bool) {
	if points {
		s.pointRevision++
	}
	if status {
		s.statusRevision++
	}
	if !points && !status {
		return
	}
	for subscriber := range s.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	points := s.pointStatusesLocked()
	healthy := 0
	errorCount := 0
	for _, point := range points {
		if point.Error == "" {
			healthy++
		} else {
			errorCount++
		}
	}
	errors := append([]Event{}, s.errors...)
	snapshot := Snapshot{
		GatewayKey:     s.gatewayKey,
		HardwareID:     s.hardwareID,
		StartedAt:      s.startedAt,
		UptimeSeconds:  int64(time.Since(s.startedAt).Seconds()),
		CollectSeconds: s.collectSeconds,
		MQTTEnabled:    s.mqttEnabled,
		MQTTConnected:  s.mqttConnected,
		MQTTChannels:   copyMQTTChannels(s.mqttChannels),
		PointCount:     len(points),
		HealthyCount:   healthy,
		ErrorCount:     errorCount,
		Points:         points,
		Errors:         errors,
	}
	if !s.lastCollectAt.IsZero() {
		lastCollectAt := s.lastCollectAt
		snapshot.LastCollectAt = &lastCollectAt
	}
	if !s.lastPublishAt.IsZero() {
		lastPublishAt := s.lastPublishAt
		snapshot.LastPublishAt = &lastPublishAt
	}
	s.mu.RUnlock()
	snapshot.SystemMetrics = hardware.ReadSystemMetrics()
	return snapshot
}

// PointStatuses returns a lightweight point-only snapshot for protocol
// forwarding and other high-frequency consumers. Unlike Snapshot it does not
// read CPU, memory, storage, or network hardware metrics.
func (s *Store) PointStatuses() []PointStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pointStatusesLocked()
}

func (s *Store) pointStatusesLocked() []PointStatus {
	points := make([]PointStatus, 0, len(s.points))
	for _, key := range s.pointOrder {
		point, ok := s.points[key]
		if ok {
			points = append(points, point)
		}
	}
	return points
}

func mqttChannelsForConfig(cfg config.Config) map[string]MQTTChannelStatus {
	channels := map[string]MQTTChannelStatus{
		"activation": {Enabled: cfg.Activation.IsEnabled()},
	}
	for index, channel := range cfg.ManualMQTTChannels() {
		channels[fmt.Sprintf("manual-%d", index+1)] = MQTTChannelStatus{Enabled: channel.IsEnabled(), Name: channel.Name}
	}
	return channels
}

func copyMQTTChannels(source map[string]MQTTChannelStatus) map[string]MQTTChannelStatus {
	result := make(map[string]MQTTChannelStatus, len(source))
	for name, channel := range source {
		result[name] = channel
	}
	return result
}

func (s *Store) appendErrorLocked(event Event) {
	s.errors = append([]Event{event}, s.errors...)
	if len(s.errors) > 50 {
		s.errors = s.errors[:50]
	}
}

func pointKey(deviceKey string, metric string) string {
	return deviceKey + "::" + metric
}

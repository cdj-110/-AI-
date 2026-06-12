package state

import (
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
	lastCollectAt  time.Time
	lastPublishAt  time.Time
	points         map[string]PointStatus
	pointOrder     []string
	errors         []Event
}

type PointStatus struct {
	DeviceKey string      `json:"deviceKey"`
	Name      string      `json:"name"`
	Metric    string      `json:"metric"`
	Protocol  string      `json:"protocol"`
	Address   string      `json:"address"`
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

type Snapshot struct {
	GatewayKey     string            `json:"gatewayKey"`
	HardwareID     hardware.Identity `json:"hardwareIdentity"`
	StartedAt      time.Time         `json:"startedAt"`
	UptimeSeconds  int64             `json:"uptimeSeconds"`
	CollectSeconds int               `json:"collectSeconds"`
	MQTTEnabled    bool              `json:"mqttEnabled"`
	MQTTConnected  bool              `json:"mqttConnected"`
	LastCollectAt  *time.Time        `json:"lastCollectAt,omitempty"`
	LastPublishAt  *time.Time        `json:"lastPublishAt,omitempty"`
	PointCount     int               `json:"pointCount"`
	HealthyCount   int               `json:"healthyCount"`
	ErrorCount     int               `json:"errorCount"`
	Points         []PointStatus     `json:"points"`
	Errors         []Event           `json:"errors"`
}

func New(cfg config.Config) *Store {
	points := make(map[string]PointStatus, len(cfg.Points))
	pointOrder := make([]string, 0, len(cfg.Points))
	for _, point := range cfg.Points {
		key := pointKey(point.DeviceKey, point.Metric)
		points[key] = PointStatus{
			DeviceKey: point.DeviceKey,
			Name:      point.Name,
			Metric:    point.Metric,
			Protocol:  point.Protocol,
			Address:   point.Address,
		}
		pointOrder = append(pointOrder, key)
	}
	return &Store{
		startedAt:      time.Now(),
		gatewayKey:     cfg.GatewayKey,
		hardwareID:     hardware.ReadIdentity(),
		collectSeconds: cfg.CollectIntervalSeconds,
		mqttEnabled:    cfg.MQTT.IsEnabled(),
		points:         points,
		pointOrder:     pointOrder,
	}
}

func (s *Store) SetMQTTConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mqttConnected = connected
}

func (s *Store) ReplaceConfig(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gatewayKey = cfg.GatewayKey
	s.collectSeconds = cfg.CollectIntervalSeconds
	s.mqttEnabled = cfg.MQTT.IsEnabled()
	next := make(map[string]PointStatus, len(cfg.Points))
	pointOrder := make([]string, 0, len(cfg.Points))
	for _, point := range cfg.Points {
		key := pointKey(point.DeviceKey, point.Metric)
		status := s.points[key]
		status.DeviceKey = point.DeviceKey
		status.Name = point.Name
		status.Metric = point.Metric
		status.Protocol = point.Protocol
		status.Address = point.Address
		next[key] = status
		pointOrder = append(pointOrder, key)
	}
	s.points = next
	s.pointOrder = pointOrder
}

func (s *Store) MarkCollect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.lastCollectAt = now
}

func (s *Store) MarkPublish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.lastPublishAt = now
}

func (s *Store) SetPointValue(deviceKey string, metric string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	point := s.points[pointKey(deviceKey, metric)]
	point.DeviceKey = deviceKey
	point.Metric = metric
	point.Value = value
	point.UpdatedAt = &now
	point.Error = ""
	point.ErrorAt = nil
	s.points[pointKey(deviceKey, metric)] = point
}

func (s *Store) SetPointError(point config.PointConfig, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	key := pointKey(point.DeviceKey, point.Metric)
	status := s.points[key]
	status.DeviceKey = point.DeviceKey
	status.Name = point.Name
	status.Metric = point.Metric
	status.Protocol = point.Protocol
	status.Address = point.Address
	status.Error = err.Error()
	status.ErrorAt = &now
	s.points[key] = status
	s.appendErrorLocked(Event{Time: now, Level: "ERROR", Message: point.DeviceKey + "/" + point.Metric + ": " + err.Error()})
}

func (s *Store) AddError(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendErrorLocked(Event{Time: time.Now(), Level: "ERROR", Message: message})
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	points := make([]PointStatus, 0, len(s.points))
	healthy := 0
	errorCount := 0
	for _, key := range s.pointOrder {
		point, ok := s.points[key]
		if !ok {
			continue
		}
		points = append(points, point)
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
	return snapshot
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

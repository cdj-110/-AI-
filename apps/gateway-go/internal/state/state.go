package state

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/securechip"
)

type Store struct {
	mu                  sync.RWMutex
	startedAt           time.Time
	gatewayKey          string
	hardwareID          hardware.Identity
	secureChip          SecureChipStatus
	collectSeconds      int
	collectMilliseconds int
	mqttEnabled         bool
	mqttConnected       bool
	mqttChannels        map[string]MQTTChannelStatus
	lastCollectAt       time.Time
	lastPublishAt       time.Time
	lastProgressAt      time.Time
	points              map[string]PointStatus
	pointOrder          []string
	errors              []Event
	pointRevision       uint64
	statusRevision      uint64
	subscribers         map[chan struct{}]struct{}
	incidentRecorder    PointIncidentRecorder
	license             config.LicenseStatus
}

type PointStatus struct {
	DeviceKey         string             `json:"deviceKey"`
	Name              string             `json:"name"`
	Metric            string             `json:"metric"`
	Protocol          string             `json:"protocol"`
	Address           string             `json:"address"`
	Unit              string             `json:"unit,omitempty"`
	Decimals          int                `json:"decimals,omitempty"`
	Value             interface{}        `json:"value"`
	ValueText         string             `json:"valueText,omitempty"`
	ActualDataType    string             `json:"actualDataType,omitempty"`
	UpdatedAt         *time.Time         `json:"updatedAt,omitempty"`
	Quality           model.PointQuality `json:"quality,omitempty"`
	RawQuality        uint32             `json:"rawQuality"`
	SourceTimestamp   *time.Time         `json:"sourceTimestamp,omitempty"`
	ReceiveTimestamp  *time.Time         `json:"receiveTimestamp,omitempty"`
	CollectionMode    string             `json:"collectionMode,omitempty"`
	DiagnosticCode    int64              `json:"diagnosticCode,omitempty"`
	Diagnostic        string             `json:"diagnostic,omitempty"`
	Error             string             `json:"error,omitempty"`
	ErrorAt           *time.Time         `json:"errorAt,omitempty"`
	Stale             bool               `json:"stale,omitempty"`
	StaleAfterSeconds int                `json:"staleAfterSeconds,omitempty"`
}

type Event struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// PointIncidentEvent is emitted only when a point enters an error state, its
// reason changes, or it recovers. The recorder must return quickly because
// collection must never wait for diagnostic persistence.
type PointIncidentEvent struct {
	Active    bool
	DeviceKey string
	PointName string
	Metric    string
	Protocol  string
	Address   string
	Reason    string
	At        time.Time
}

type PointIncidentRecorder interface {
	RecordPointIncident(PointIncidentEvent)
}

type MQTTChannelStatus struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Name      string `json:"name,omitempty"`
}

type SecureChipStatus struct {
	USID      string `json:"usid,omitempty"`
	Available bool   `json:"available"`
	Message   string `json:"message,omitempty"`
}

type Snapshot struct {
	GatewayKey          string                       `json:"gatewayKey"`
	HardwareID          hardware.Identity            `json:"hardwareIdentity"`
	SecureChip          SecureChipStatus             `json:"secureChipIdentity"`
	StartedAt           time.Time                    `json:"startedAt"`
	UptimeSeconds       int64                        `json:"uptimeSeconds"`
	CollectSeconds      int                          `json:"collectSeconds"`
	CollectMilliseconds int                          `json:"collectMilliseconds"`
	MQTTEnabled         bool                         `json:"mqttEnabled"`
	MQTTConnected       bool                         `json:"mqttConnected"`
	MQTTChannels        map[string]MQTTChannelStatus `json:"mqttChannels"`
	LastCollectAt       *time.Time                   `json:"lastCollectAt,omitempty"`
	LastPublishAt       *time.Time                   `json:"lastPublishAt,omitempty"`
	PointCount          int                          `json:"pointCount"`
	HealthyCount        int                          `json:"healthyCount"`
	ErrorCount          int                          `json:"errorCount"`
	PendingCount        int                          `json:"pendingCount"`
	StaleCount          int                          `json:"staleCount"`
	Points              []PointStatus                `json:"points"`
	Errors              []Event                      `json:"errors"`
	SystemMetrics       hardware.SystemMetrics       `json:"systemMetrics"`
	ProcessMetrics      hardware.ProcessMetrics      `json:"processMetrics"`
	License             config.LicenseStatus         `json:"license"`
}

type HealthSnapshot struct {
	Healthy        bool       `json:"healthy"`
	StartedAt      time.Time  `json:"startedAt"`
	UptimeSeconds  int64      `json:"uptimeSeconds"`
	LastProgressAt *time.Time `json:"lastProgressAt,omitempty"`
	LastCollectAt  *time.Time `json:"lastCollectAt,omitempty"`
	PointCount     int        `json:"pointCount"`
	Reason         string     `json:"reason,omitempty"`
}

func New(cfg config.Config) *Store {
	statusPoints := cfg.StatusPoints()
	points := make(map[string]PointStatus, len(statusPoints))
	pointOrder := make([]string, 0, len(statusPoints))
	for _, point := range statusPoints {
		key := pointKey(point.DeviceKey, point.Metric)
		points[key] = PointStatus{
			DeviceKey:         point.DeviceKey,
			Name:              point.Name,
			Metric:            point.Metric,
			Protocol:          point.Protocol,
			Address:           point.Address,
			Unit:              point.Unit,
			Decimals:          point.Decimals,
			StaleAfterSeconds: staleAfterSeconds(point),
		}
		pointOrder = append(pointOrder, key)
	}
	identity := hardware.ReadIdentity()
	chipIdentity, chipErr := securechip.Authenticate()
	chipStatus := SecureChipStatus{USID: chipIdentity.USID, Available: chipErr == nil && chipIdentity.USID != ""}
	if chipErr != nil {
		chipStatus.Message = chipErr.Error()
	}
	startedAt := time.Now()
	return &Store{
		startedAt:           startedAt,
		lastProgressAt:      startedAt,
		gatewayKey:          cfg.GatewayKey,
		hardwareID:          identity,
		secureChip:          chipStatus,
		collectSeconds:      collectSecondsForConfig(cfg),
		collectMilliseconds: int(cfg.CollectInterval() / time.Millisecond),
		license:             cfg.License,
		mqttEnabled:         cfg.ManualMQTTEnabled() || cfg.CloudConnectionIsEnabled(),
		mqttChannels:        mqttChannelsForConfig(cfg),
		points:              points,
		pointOrder:          pointOrder,
		subscribers:         map[chan struct{}]struct{}{},
	}
}

func (s *Store) MarkProgress() {
	s.mu.Lock()
	s.lastProgressAt = time.Now()
	s.mu.Unlock()
}

func (s *Store) SetPointIncidentRecorder(recorder PointIncidentRecorder) {
	s.mu.Lock()
	s.incidentRecorder = recorder
	s.mu.Unlock()
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
	s.mqttEnabled = cfg.ManualMQTTEnabled() || cfg.CloudConnectionIsEnabled()
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
	s.collectMilliseconds = int(cfg.CollectInterval() / time.Millisecond)
	s.mqttEnabled = cfg.ManualMQTTEnabled() || cfg.CloudConnectionIsEnabled()
	s.license = cfg.License
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
		status.StaleAfterSeconds = staleAfterSeconds(point)
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
	s.SetPointResult(model.PointValue{DeviceKey: deviceKey, Metric: metric, Value: value})
}

// SetPointResult stores a value together with protocol quality and timestamps.
// Existing SetPointValue callers continue to receive GOOD quality and a local
// receive timestamp through PointValue.ApplyDefaults.
func (s *Store) SetPointResult(result model.PointValue) {
	s.mu.Lock()
	now := time.Now()
	result.ApplyDefaults(now)
	point := s.points[pointKey(result.DeviceKey, result.Metric)]
	hadError := point.Error != ""
	point.DeviceKey = result.DeviceKey
	point.Metric = result.Metric
	point.Value = result.Value
	point.ValueText = exactIntegerText(result.Value)
	point.ActualDataType = result.ActualDataType
	point.Quality = result.Quality
	point.RawQuality = result.RawQuality
	point.DiagnosticCode = result.DiagnosticCode
	point.Diagnostic = result.Diagnostic
	point.ReceiveTimestamp = timePointer(result.ReceiveTimestamp)
	point.SourceTimestamp = timePointer(result.SourceTimestamp)
	point.CollectionMode = result.CollectionMode
	point.UpdatedAt = point.ReceiveTimestamp
	point.Error = ""
	point.ErrorAt = nil
	point.Stale = false
	s.points[pointKey(result.DeviceKey, result.Metric)] = point
	s.notifyLocked(true, hadError)
	recorder := s.incidentRecorder
	incident := PointIncidentEvent{Active: false, DeviceKey: point.DeviceKey, PointName: point.Name, Metric: point.Metric, Protocol: point.Protocol, Address: point.Address, At: now}
	s.mu.Unlock()
	if hadError && recorder != nil {
		recorder.RecordPointIncident(incident)
	}
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func exactIntegerText(value interface{}) string {
	switch typed := value.(type) {
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		return ""
	}
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
	point.ValueText = ""
	point.ActualDataType = ""
	point.UpdatedAt = nil
	point.Quality = ""
	point.RawQuality = 0
	point.SourceTimestamp = nil
	point.ReceiveTimestamp = nil
	point.CollectionMode = ""
	point.DiagnosticCode = 0
	point.Diagnostic = ""
	point.Error = ""
	point.ErrorAt = nil
	s.points[key] = point
	s.notifyLocked(true, true)
}

func (s *Store) SetPointError(point config.PointConfig, err error) {
	s.mu.Lock()
	now := time.Now()
	key := pointKey(point.DeviceKey, point.Metric)
	status := s.points[key]
	message := err.Error()
	if status.Error == message {
		s.mu.Unlock()
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
	status.Quality = model.QualityBad
	status.Diagnostic = message
	status.ErrorAt = &now
	s.points[key] = status
	s.appendErrorLocked(Event{Time: now, Level: "ERROR", Message: point.DeviceKey + "/" + point.Metric + ": " + message})
	s.notifyLocked(true, true)
	recorder := s.incidentRecorder
	s.mu.Unlock()
	if recorder != nil {
		recorder.RecordPointIncident(PointIncidentEvent{Active: true, DeviceKey: point.DeviceKey, PointName: point.Name, Metric: point.Metric, Protocol: point.Protocol, Address: point.Address, Reason: message, At: now})
	}
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
	points := s.pointStatusesLocked(time.Now())
	healthy := 0
	errorCount := 0
	pendingCount := 0
	staleCount := 0
	for _, point := range points {
		if point.Error != "" {
			errorCount++
		} else if point.Stale {
			staleCount++
		} else if point.UpdatedAt == nil {
			pendingCount++
		} else {
			healthy++
		}
	}
	errors := append([]Event{}, s.errors...)
	snapshot := Snapshot{
		GatewayKey:          s.gatewayKey,
		HardwareID:          s.hardwareID,
		SecureChip:          s.secureChip,
		StartedAt:           s.startedAt,
		UptimeSeconds:       int64(time.Since(s.startedAt).Seconds()),
		CollectSeconds:      s.collectSeconds,
		CollectMilliseconds: s.collectMilliseconds,
		MQTTEnabled:         s.mqttEnabled,
		MQTTConnected:       s.mqttConnected,
		MQTTChannels:        copyMQTTChannels(s.mqttChannels),
		PointCount:          len(points),
		HealthyCount:        healthy,
		ErrorCount:          errorCount,
		PendingCount:        pendingCount,
		StaleCount:          staleCount,
		Points:              points,
		Errors:              errors,
		License:             s.license,
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
	snapshot.ProcessMetrics = hardware.ReadProcessMetrics()
	return snapshot
}

// CompactSnapshot reports aggregate health without allocating a PointStatus
// slice. It is used by very large projects whose point rows are fetched in
// independent visible windows.
func (s *Store) CompactSnapshot() Snapshot {
	s.mu.RLock()
	now := time.Now()
	healthy, errorCount, pendingCount, staleCount := 0, 0, 0, 0
	for _, point := range s.points {
		stale := pointIsStale(point, now)
		if point.Error != "" {
			errorCount++
		} else if stale {
			staleCount++
		} else if point.UpdatedAt == nil {
			pendingCount++
		} else {
			healthy++
		}
	}
	snapshot := Snapshot{
		GatewayKey:          s.gatewayKey,
		HardwareID:          s.hardwareID,
		SecureChip:          s.secureChip,
		StartedAt:           s.startedAt,
		UptimeSeconds:       int64(time.Since(s.startedAt).Seconds()),
		License:             s.license,
		CollectSeconds:      s.collectSeconds,
		CollectMilliseconds: s.collectMilliseconds,
		MQTTEnabled:         s.mqttEnabled,
		MQTTConnected:       s.mqttConnected,
		MQTTChannels:        copyMQTTChannels(s.mqttChannels),
		PointCount:          len(s.points),
		HealthyCount:        healthy,
		ErrorCount:          errorCount,
		PendingCount:        pendingCount,
		StaleCount:          staleCount,
		Points:              []PointStatus{},
		Errors:              append([]Event{}, s.errors...),
	}
	if !s.lastCollectAt.IsZero() {
		value := s.lastCollectAt
		snapshot.LastCollectAt = &value
	}
	if !s.lastPublishAt.IsZero() {
		value := s.lastPublishAt
		snapshot.LastPublishAt = &value
	}
	s.mu.RUnlock()
	snapshot.SystemMetrics = hardware.ReadSystemMetrics()
	snapshot.ProcessMetrics = hardware.ReadProcessMetrics()
	return snapshot
}

func (s *Store) Health(maxProgressAge time.Duration) HealthSnapshot {
	if maxProgressAge <= 0 {
		maxProgressAge = 20 * time.Second
	}
	s.mu.RLock()
	now := time.Now()
	result := HealthSnapshot{
		Healthy:       true,
		StartedAt:     s.startedAt,
		UptimeSeconds: int64(now.Sub(s.startedAt).Seconds()),
		PointCount:    len(s.points),
	}
	if !s.lastProgressAt.IsZero() {
		lastProgress := s.lastProgressAt
		result.LastProgressAt = &lastProgress
		if now.Sub(lastProgress) > maxProgressAge {
			result.Healthy = false
			result.Reason = "application progress is stale"
		}
	} else {
		result.Healthy = false
		result.Reason = "application progress has not started"
	}
	if !s.lastCollectAt.IsZero() {
		lastCollect := s.lastCollectAt
		result.LastCollectAt = &lastCollect
	}
	s.mu.RUnlock()
	return result
}

// PointStatuses returns a lightweight point-only snapshot for protocol
// forwarding and other high-frequency consumers. Unlike Snapshot it does not
// read CPU, memory, storage, or network hardware metrics.
func (s *Store) PointStatuses() []PointStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pointStatusesLocked(time.Now())
}

// PointStatusesByMetric returns only explicitly requested points. Large
// configuration pages use this path so opening one visible table window does
// not allocate a snapshot containing every point in the gateway.
func (s *Store) PointStatusesByMetric(deviceKey string, metrics []string) []PointStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	points := make([]PointStatus, 0, len(metrics))
	for _, metric := range metrics {
		point, ok := s.points[pointKey(deviceKey, metric)]
		if !ok {
			continue
		}
		point.Stale = pointIsStale(point, now)
		points = append(points, point)
	}
	return points
}

func (s *Store) pointStatusesLocked(now time.Time) []PointStatus {
	points := make([]PointStatus, 0, len(s.points))
	for _, key := range s.pointOrder {
		point, ok := s.points[key]
		if ok {
			point.Stale = pointIsStale(point, now)
			points = append(points, point)
		}
	}
	return points
}

func pointIsStale(point PointStatus, now time.Time) bool {
	// An OPC DA subscription reports changes, not periodic snapshots. An
	// unchanged value can therefore keep its original source and receive time
	// indefinitely while the callback session remains healthy. Subscription
	// liveness and reconnect failures are reported by the collector session.
	if point.CollectionMode == "subscription" {
		return false
	}
	return point.Error == "" && point.UpdatedAt != nil &&
		point.StaleAfterSeconds > 0 &&
		now.Sub(*point.UpdatedAt) > time.Duration(point.StaleAfterSeconds)*time.Second
}

func staleAfterSeconds(point config.PointConfig) int {
	// Event-driven protocols can legitimately keep a value unchanged for a
	// long time. Their communication liveness is tracked by the session.
	if point.Protocol == "iec104" || point.Protocol == "iec61850" || point.Protocol == "edge-compute" {
		return 0
	}
	seconds := point.CollectIntervalSeconds * 3
	if seconds < 15 {
		seconds = 15
	}
	return seconds
}

func mqttChannelsForConfig(cfg config.Config) map[string]MQTTChannelStatus {
	channels := map[string]MQTTChannelStatus{
		"activation": {Enabled: cfg.CloudConnectionIsEnabled()},
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

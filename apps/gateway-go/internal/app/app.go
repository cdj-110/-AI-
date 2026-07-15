package app

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cache"
	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
	"weikong-iot-platform/apps/gateway-go/internal/forward"
	"weikong-iot-platform/apps/gateway-go/internal/history"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
	"weikong-iot-platform/apps/gateway-go/internal/storage"
	"weikong-iot-platform/apps/gateway-go/internal/web"
)

type App struct {
	cfg           config.Config
	configPath    string
	cloudMu       sync.Mutex
	clouds        []*cloud.Client
	cloudState    map[string]bool
	cloudConfig   map[string]string
	spoolMu       sync.Mutex
	spool         *cache.Spool
	historyMu     sync.Mutex
	history       *history.Store
	state         *state.Store
	runtime       *gatewayruntime.Manager
	forward       *forward.Manager
	edge          *edgecompute.Manager
	collectMu     sync.Mutex
	runCtx        context.Context
	collectCancel context.CancelFunc
	publishMu     sync.Mutex
	publishQueue  chan telemetryBatch
}

type telemetryBatch struct {
	Time    time.Time
	Grouped map[string]map[string]interface{}
}

func New(cfg config.Config, configPath string) (*App, error) {
	store := state.New(cfg)
	manager := gatewayruntime.NewManager(cfg, store)
	edge, err := edgecompute.NewManager(cfg, store)
	if err != nil {
		return nil, err
	}
	return &App{
		cfg:          cfg,
		configPath:   configPath,
		cloudState:   map[string]bool{},
		cloudConfig:  map[string]string{},
		spool:        offlineSpool(cfg),
		history:      historyStore(cfg),
		state:        store,
		runtime:      manager,
		forward:      forward.NewManager(store, manager),
		edge:         edge,
		publishQueue: make(chan telemetryBatch, 10000),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	go web.New(a.cfg.Web, a.state, a.runtime, a.configPath, a.ApplyConfig).Run(ctx)
	a.syncCloud(a.cfg)
	defer a.disconnectCloud()
	a.forward.Update(ctx, a.cfg)
	defer a.forward.Stop()

	log.Printf("gateway %s started, points=%d", a.cfg.GatewayKey, len(a.cfg.Points))
	go a.heartbeatLoop(ctx)
	go a.topologyLoop(ctx)
	go a.publishLoop(ctx)
	go a.edge.Start(ctx)
	go a.edgeBatchLoop(ctx)
	a.startCollectWorkers(ctx)
	defer a.stopCollectWorkers()
	<-ctx.Done()
	return nil
}

func (a *App) heartbeatLoop(ctx context.Context) {
	a.publishGatewayHeartbeat()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.publishGatewayHeartbeat()
		}
	}
}

func (a *App) publishGatewayHeartbeat() {
	if len(a.publishQueue) > 0 {
		return
	}
	a.cloudMu.Lock()
	clients := a.connectedCloudsLocked()
	a.cloudMu.Unlock()
	a.publishMu.Lock()
	defer a.publishMu.Unlock()
	var heartbeatClients []*cloud.Client
	for _, client := range clients {
		if client.IsManual() {
			continue
		}
		if err := client.PublishGatewayHeartbeat(); err != nil {
			log.Printf("publish gateway heartbeat failed channel=%s: %v", client.Name(), err)
			a.state.AddError("publish gateway heartbeat failed " + client.Name() + ": " + err.Error())
			continue
		}
		heartbeatClients = append(heartbeatClients, client)
		a.state.MarkPublish()
	}
	a.publishChildHeartbeatsLocked(heartbeatClients)
}

func (a *App) topologyLoop(ctx context.Context) {
	a.publishTopology()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.publishTopology()
		}
	}
}

func (a *App) publishTopology() {
	if len(a.publishQueue) > 0 {
		return
	}
	a.cloudMu.Lock()
	clients := a.connectedCloudsLocked()
	a.cloudMu.Unlock()
	topology := topologySnapshot(a.runtime.Config())
	a.publishMu.Lock()
	defer a.publishMu.Unlock()
	for _, client := range clients {
		if client.IsManual() {
			continue
		}
		if err := client.PublishTopology(topology); err != nil {
			log.Printf("publish topology failed channel=%s: %v", client.Name(), err)
			a.state.AddError("publish topology failed " + client.Name() + ": " + err.Error())
			continue
		}
		a.state.MarkPublish()
	}
}

func (a *App) ApplyConfig(cfg config.Config) {
	if err := edgecompute.Validate(cfg); err != nil {
		a.state.AddError("edge compute config rejected: " + err.Error())
		return
	}
	oldCfg := a.runtime.Config()
	a.runtime.UpdateConfig(cfg)
	if err := a.edge.UpdateConfig(cfg); err != nil {
		a.state.AddError("edge compute reload failed: " + err.Error())
		return
	}
	a.forward.Update(context.Background(), cfg)
	a.cfg = cfg
	a.spoolMu.Lock()
	a.spool = offlineSpool(cfg)
	a.spoolMu.Unlock()
	a.historyMu.Lock()
	a.history = historyStore(cfg)
	a.historyMu.Unlock()
	if !oldCfg.MQTT.Equal(cfg.MQTT) || !config.EqualMQTTChannels(oldCfg.MQTTChannels, cfg.MQTTChannels) || !oldCfg.Activation.Equal(cfg.Activation) {
		a.syncCloud(cfg)
	}
	a.restartCollectWorkers()
	go a.publishTopology()
}

func (a *App) edgeBatchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-a.edge.Output():
			telemetry := telemetryBatch{Time: batch.Time, Grouped: batch.Grouped}
			a.recordHistory(telemetry)
			a.enqueueTelemetry(telemetry)
		}
	}
}

func (a *App) startCollectWorkers(parent context.Context) {
	if parent == nil {
		return
	}
	a.collectMu.Lock()
	defer a.collectMu.Unlock()
	if a.collectCancel != nil {
		a.collectCancel()
	}
	workerCtx, cancel := context.WithCancel(parent)
	a.runCtx = parent
	a.collectCancel = cancel
	lanes := a.runtime.CollectionLanes()
	if len(lanes) == 0 {
		log.Printf("collect workers skipped: no points configured")
		return
	}
	log.Printf("collect workers started lanes=%d", len(lanes))
	for _, lane := range lanes {
		lane := lane
		log.Printf("collect lane started key=%s device=%s channel=%s points=%d interval=%s", lane.Key, lane.DeviceKey, lane.Channel, len(lane.Points), lane.Interval)
		go a.collectLaneLoop(workerCtx, lane)
	}
}

func (a *App) restartCollectWorkers() {
	a.collectMu.Lock()
	parent := a.runCtx
	a.collectMu.Unlock()
	if parent == nil {
		return
	}
	a.startCollectWorkers(parent)
}

func (a *App) stopCollectWorkers() {
	a.collectMu.Lock()
	defer a.collectMu.Unlock()
	if a.collectCancel != nil {
		a.collectCancel()
		a.collectCancel = nil
	}
}

func (a *App) collectLaneLoop(ctx context.Context, lane gatewayruntime.CollectionLane) {
	a.collectLaneAndPublish(ctx, lane.Key)
	interval := lane.Interval
	if interval <= 0 {
		interval = a.runtime.CollectInterval()
	}
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.collectLaneAndPublish(ctx, lane.Key)
		}
	}
}

func (a *App) collectLaneAndPublish(ctx context.Context, laneKey string) {
	grouped := a.runtime.CollectLane(ctx, laneKey, false)
	if len(grouped) == 0 {
		return
	}
	batch := telemetryBatch{Time: time.Now(), Grouped: grouped}
	a.recordHistory(batch)
	a.enqueueTelemetry(batch)
}

func (a *App) publishLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-a.publishQueue:
			a.publishToCloud(batch)
		}
	}
}

func (a *App) enqueueTelemetry(batch telemetryBatch) {
	select {
	case a.publishQueue <- batch:
	default:
		log.Printf("publish queue full, spooling telemetry batch devices=%d", len(batch.Grouped))
		a.state.AddError("publish queue full, spooling telemetry batch")
		a.spoolTelemetry(batch)
	}
}

func (a *App) publishToCloud(batch telemetryBatch) {
	a.publishMu.Lock()
	defer a.publishMu.Unlock()
	grouped := batch.Grouped
	grouped = mqttGroupedValues(a.runtime.Config(), grouped)
	a.cloudMu.Lock()
	clients := a.connectedCloudsLocked()
	a.cloudMu.Unlock()
	if platformClient := firstPlatformClient(clients); platformClient != nil {
		a.spoolMu.Lock()
		_ = a.spool.DrainLimit(platformClient.PublishTelemetry, 5)
		a.spoolMu.Unlock()
	}
	publishedAny := false
	for _, client := range clients {
		if client.IsManual() {
			if err := client.PublishManual(grouped); err != nil {
				log.Printf("publish attributes failed channel=%s: %v", client.Name(), err)
				a.state.AddError("publish attributes failed " + client.Name() + ": " + err.Error())
			} else {
				publishedAny = true
			}
			continue
		}

	}

	for deviceKey, metrics := range grouped {
		reading := model.Reading{DeviceKey: deviceKey, Time: batch.Time, Metrics: metrics}
		platformPublished := false
		for _, client := range clients {
			if client.IsManual() {
				continue
			}
			if err := client.PublishTelemetry(reading); err != nil {
				log.Printf("publish telemetry failed channel=%s device=%s: %v", client.Name(), deviceKey, err)
				a.state.AddError("publish telemetry failed " + client.Name() + "/" + deviceKey + ": " + err.Error())
			} else {
				platformPublished = true
				publishedAny = true
			}
		}
		if !platformPublished {
			a.spoolMu.Lock()
			cacheErr := a.spool.Append(reading)
			a.spoolMu.Unlock()
			if cacheErr != nil && cacheErr != cache.ErrDisabled {
				log.Printf("append spool failed: %v", cacheErr)
				a.state.AddError("append spool failed: " + cacheErr.Error())
			}
		}
	}
	if publishedAny {
		a.state.MarkPublish()
	}
}

func (a *App) publishChildHeartbeatsLocked(clients []*cloud.Client) {
	for _, deviceKey := range childDeviceKeys(a.runtime.Config()) {
		for _, client := range clients {
			if client.IsManual() {
				continue
			}
			if err := client.PublishChildHeartbeat(deviceKey); err != nil {
				log.Printf("publish child heartbeat failed channel=%s device=%s: %v", client.Name(), deviceKey, err)
				a.state.AddError("publish child heartbeat failed " + client.Name() + "/" + deviceKey + ": " + err.Error())
			}
		}
	}
}

func (a *App) spoolTelemetry(batch telemetryBatch) {
	grouped := mqttGroupedValues(a.runtime.Config(), batch.Grouped)
	for deviceKey, metrics := range grouped {
		reading := model.Reading{DeviceKey: deviceKey, Time: batch.Time, Metrics: metrics}
		a.spoolMu.Lock()
		cacheErr := a.spool.Append(reading)
		a.spoolMu.Unlock()
		if cacheErr != nil && cacheErr != cache.ErrDisabled {
			log.Printf("append spool failed: %v", cacheErr)
			a.state.AddError("append spool failed: " + cacheErr.Error())
		}
	}
}

func (a *App) recordHistory(batch telemetryBatch) {
	grouped := mqttGroupedValues(a.runtime.Config(), batch.Grouped)
	a.historyMu.Lock()
	store := a.history
	a.historyMu.Unlock()
	for deviceKey, metrics := range grouped {
		if len(metrics) == 0 {
			continue
		}
		if err := store.Append(model.Reading{DeviceKey: deviceKey, Time: batch.Time, Metrics: metrics}); err != nil && err != history.ErrDisabled {
			log.Printf("append history failed: %v", err)
			a.state.AddError("append history failed: " + err.Error())
		}
	}
}

func mqttGroupedValues(cfg config.Config, grouped map[string]map[string]interface{}) map[string]map[string]interface{} {
	converters := modbusCoilMetricSet(cfg.Points)
	if len(converters) == 0 || len(grouped) == 0 {
		return grouped
	}
	converted := make(map[string]map[string]interface{}, len(grouped))
	for deviceKey, metrics := range grouped {
		convertedMetrics := make(map[string]interface{}, len(metrics))
		for metric, value := range metrics {
			if converters[deviceKey+"::"+metric] {
				convertedMetrics[metric] = boolAsNumber(value)
				continue
			}
			convertedMetrics[metric] = value
		}
		converted[deviceKey] = convertedMetrics
	}
	return converted
}

type topologyMetric struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name,omitempty"`
	DataType   string `json:"dataType,omitempty"`
	Unit       string `json:"unit,omitempty"`
}

type topologyDevice struct {
	DeviceKey     string           `json:"deviceKey"`
	Name          string           `json:"name,omitempty"`
	Protocol      string           `json:"protocol,omitempty"`
	Address       string           `json:"address,omitempty"`
	SlaveID       byte             `json:"slaveId,omitempty"`
	CommonAddress uint16           `json:"commonAddress,omitempty"`
	PointCount    int              `json:"pointCount"`
	Metrics       []topologyMetric `json:"metrics"`
}

func topologySnapshot(cfg config.Config) map[string]interface{} {
	devices := topologyDevices(cfg)
	raw, _ := json.Marshal(devices)
	sum := sha1.Sum(raw)
	return map[string]interface{}{
		"gatewayKey":    cfg.GatewayKey,
		"configVersion": fmt.Sprintf("%x", sum[:8]),
		"updatedAt":     time.Now().Format(time.RFC3339Nano),
		"devices":       devices,
	}
}

func topologyDevices(cfg config.Config) []topologyDevice {
	if len(cfg.Devices) > 0 {
		devices := make([]topologyDevice, 0, len(cfg.Devices))
		for _, device := range cfg.Devices {
			if device.DeviceKey == "" {
				continue
			}
			metrics := topologyMetrics(device.Points)
			devices = append(devices, topologyDevice{
				DeviceKey:     device.DeviceKey,
				Name:          device.Name,
				Protocol:      device.Protocol,
				Address:       device.Address,
				SlaveID:       device.SlaveID,
				CommonAddress: device.CommonAddress,
				PointCount:    len(metrics),
				Metrics:       metrics,
			})
		}
		devices = append(devices, edgeTopologyDevices(cfg)...)
		return devices
	}
	byKey := map[string]*topologyDevice{}
	order := []string{}
	for _, point := range cfg.Points {
		if point.DeviceKey == "" {
			continue
		}
		current := byKey[point.DeviceKey]
		if current == nil {
			current = &topologyDevice{
				DeviceKey:     point.DeviceKey,
				Protocol:      point.Protocol,
				Address:       point.Address,
				SlaveID:       point.SlaveID,
				CommonAddress: point.CommonAddress,
			}
			byKey[point.DeviceKey] = current
			order = append(order, point.DeviceKey)
		}
		current.Metrics = append(current.Metrics, topologyMetric{
			Identifier: point.Metric,
			Name:       point.Name,
			DataType:   point.DataType,
			Unit:       point.Unit,
		})
		current.PointCount = len(current.Metrics)
	}
	devices := make([]topologyDevice, 0, len(order))
	for _, key := range order {
		devices = append(devices, *byKey[key])
	}
	devices = append(devices, edgeTopologyDevices(cfg)...)
	return devices
}

func edgeTopologyDevices(cfg config.Config) []topologyDevice {
	if !cfg.EdgeComputing.Enabled {
		return nil
	}
	var devices []topologyDevice
	for _, group := range cfg.EdgeComputing.Groups {
		if !group.IsEnabled() {
			continue
		}
		var metrics []topologyMetric
		for _, point := range group.Points {
			if point.IsEnabled() {
				metrics = append(metrics, topologyMetric{Identifier: point.Metric, Name: point.Name, DataType: point.DataType, Unit: point.Unit})
			}
		}
		devices = append(devices, topologyDevice{DeviceKey: group.GroupKey, Name: group.Name, Protocol: "edge-compute", Address: group.GroupKey, PointCount: len(metrics), Metrics: metrics})
	}
	return devices
}

func topologyMetrics(points []config.PointConfig) []topologyMetric {
	metrics := make([]topologyMetric, 0, len(points))
	for _, point := range points {
		if point.Metric == "" {
			continue
		}
		metrics = append(metrics, topologyMetric{
			Identifier: point.Metric,
			Name:       point.Name,
			DataType:   point.DataType,
			Unit:       point.Unit,
		})
	}
	return metrics
}

func childDeviceKeys(cfg config.Config) []string {
	seen := map[string]bool{}
	var keys []string
	for _, device := range cfg.Devices {
		if device.DeviceKey == "" || seen[device.DeviceKey] {
			continue
		}
		seen[device.DeviceKey] = true
		keys = append(keys, device.DeviceKey)
	}
	if len(cfg.Devices) == 0 {
		for _, point := range cfg.Points {
			if point.DeviceKey == "" || seen[point.DeviceKey] {
				continue
			}
			seen[point.DeviceKey] = true
			keys = append(keys, point.DeviceKey)
		}
	}
	if cfg.EdgeComputing.Enabled {
		for _, group := range cfg.EdgeComputing.Groups {
			if group.IsEnabled() && group.GroupKey != "" && !seen[group.GroupKey] {
				seen[group.GroupKey] = true
				keys = append(keys, group.GroupKey)
			}
		}
	}
	return keys
}

func modbusCoilMetricSet(points []config.PointConfig) map[string]bool {
	result := map[string]bool{}
	for _, point := range points {
		if point.Function != 1 {
			continue
		}
		if point.Protocol != "modbus-tcp" && point.Protocol != "modbus-rtu" {
			continue
		}
		if point.DeviceKey == "" || point.Metric == "" {
			continue
		}
		result[point.DeviceKey+"::"+point.Metric] = true
	}
	return result
}

func boolAsNumber(value interface{}) interface{} {
	typed, ok := value.(bool)
	if !ok {
		return value
	}
	if typed {
		return 1
	}
	return 0
}

func offlineSpool(cfg config.Config) *cache.Spool {
	if !cfg.OfflineCache.Enabled {
		return cache.Disabled()
	}
	device, ok := storage.Find(cfg.OfflineCache.StoragePath)
	if !ok {
		log.Printf("offline cache disabled: removable storage %q is not mounted", cfg.OfflineCache.StoragePath)
		return cache.Disabled()
	}
	maxBytes := int64(cfg.OfflineCache.MaxSizeMB) * 1024 * 1024
	path := filepath.Join(device.MountPath, ".weikong", "gateway-spool.jsonl")
	log.Printf("offline cache enabled path=%s max=%dMB free=%dMB", path, cfg.OfflineCache.MaxSizeMB, device.FreeBytes/1024/1024)
	return cache.NewGuarded(path, maxBytes, func() bool {
		current, mounted := storage.Find(device.MountPath)
		return mounted && current.Device == device.Device
	})
}

func historyStore(cfg config.Config) *history.Store {
	if !cfg.HistoryStorage.Enabled {
		return history.Disabled()
	}
	device, ok := storage.Find(cfg.HistoryStorage.StoragePath)
	if !ok {
		log.Printf("history storage disabled: removable storage %q is not mounted", cfg.HistoryStorage.StoragePath)
		return history.Disabled()
	}
	maxBytes := int64(cfg.HistoryStorage.MaxSizeMB) * 1024 * 1024
	path := history.Path(device.MountPath)
	log.Printf("history storage enabled path=%s max=%dMB free=%dMB", path, cfg.HistoryStorage.MaxSizeMB, device.FreeBytes/1024/1024)
	return history.NewGuarded(path, maxBytes, func() bool {
		current, mounted := storage.Find(device.MountPath)
		return mounted && current.Device == device.Device
	})
}

func firstPlatformClient(clients []*cloud.Client) *cloud.Client {
	for _, client := range clients {
		if !client.IsManual() {
			return client
		}
	}
	return nil
}

func cloudClientByName(clients []*cloud.Client) map[string]*cloud.Client {
	result := make(map[string]*cloud.Client, len(clients))
	for _, client := range clients {
		result[client.Name()] = client
	}
	return result
}

func mqttChannelSignature(gatewayKey string, channel config.MQTTConfig) string {
	raw, _ := json.Marshal(struct {
		GatewayKey string            `json:"gatewayKey"`
		Channel    config.MQTTConfig `json:"channel"`
	}{GatewayKey: gatewayKey, Channel: channel})
	return string(raw)
}

func activationSignature(activation config.ActivationConfig) string {
	raw, _ := json.Marshal(activation)
	return string(raw)
}

func (a *App) syncCloud(cfg config.Config) {
	a.cloudMu.Lock()
	defer a.cloudMu.Unlock()
	existing := cloudClientByName(a.clouds)
	used := map[string]bool{}
	nextClients := make([]*cloud.Client, 0, len(a.clouds))
	nextConfig := map[string]string{}
	a.state.ResetMQTTChannels(cfg)

	if cfg.Activation.IsReady() && cfg.Activation.Broker != "" {
		channelKey := "activation"
		signature := activationSignature(cfg.Activation)
		client := existing[channelKey]
		if client != nil && a.cloudConfig[channelKey] == signature {
			nextClients = append(nextClients, client)
			nextConfig[channelKey] = signature
			used[channelKey] = true
			a.setCloudChannelConnectedLocked(channelKey, client.IsConnected())
		} else {
			if client != nil {
				client.Disconnect()
				delete(existing, channelKey)
			}
			client = cloud.NewActivationMQTT(cfg, a.setCloudChannelConnected(channelKey))
			nextClients = append(nextClients, client)
			nextConfig[channelKey] = signature
			used[channelKey] = true
			if err := client.Connect(); err != nil {
				log.Printf("activation mqtt connect failed: %v", err)
				a.state.AddError("activation mqtt connect failed: " + err.Error())
			} else {
				a.setCloudChannelConnectedLocked(channelKey, client.IsConnected())
				if err := client.SubscribeRemoteConfig(a.applyRemoteConfig, a.remoteConfigSnapshot); err != nil {
					log.Printf("subscribe remote config failed channel=activation: %v", err)
				}
			}
		}
	} else if cfg.Activation.IsReady() && cfg.Activation.Broker == "" {
		log.Printf("activation channel broker is empty, skipped")
	} else if cfg.Activation.IsEnabled() && cfg.Activation.File != "" {
		log.Printf("activation channel not ready, cloud publish skipped")
	}

	for index, channel := range cfg.ManualMQTTChannels() {
		if !channel.IsEnabled() {
			continue
		}
		channelKey := fmt.Sprintf("manual-%d", index+1)
		signature := mqttChannelSignature(cfg.GatewayKey, channel)
		client := existing[channelKey]
		if client != nil && a.cloudConfig[channelKey] == signature {
			nextClients = append(nextClients, client)
			nextConfig[channelKey] = signature
			used[channelKey] = true
			a.setCloudChannelConnectedLocked(channelKey, client.IsConnected())
			continue
		}
		if client != nil {
			client.Disconnect()
			delete(existing, channelKey)
		}
		client = cloud.NewManualMQTTChannel(channelKey, cfg.GatewayKey, channel, a.setCloudChannelConnected(channelKey))
		nextClients = append(nextClients, client)
		nextConfig[channelKey] = signature
		used[channelKey] = true
		if err := client.Connect(); err != nil {
			log.Printf("manual mqtt connect failed channel=%s: %v", channel.Name, err)
			a.state.AddError("manual mqtt connect failed " + channel.Name + ": " + err.Error())
		} else {
			a.setCloudChannelConnectedLocked(channelKey, client.IsConnected())
			if !cfg.Activation.IsReady() {
				if err := client.SubscribeRemoteConfig(a.applyRemoteConfig, a.remoteConfigSnapshot); err != nil {
					log.Printf("subscribe remote config failed channel=%s: %v", channel.Name, err)
				}
			}
			if err := client.SubscribeAttributes(a.handleMQTTAttributeSet); err != nil {
				log.Printf("subscribe mqtt attributes failed channel=%s: %v", channel.Name, err)
				a.state.AddError("subscribe mqtt attributes failed " + channel.Name + ": " + err.Error())
			}
		}
	}

	for key, client := range existing {
		if used[key] {
			continue
		}
		client.Disconnect()
		a.setCloudChannelConnectedLocked(key, false)
	}
	a.clouds = nextClients
	a.cloudConfig = nextConfig
	a.cloudState = map[string]bool{}
	for _, client := range a.clouds {
		a.cloudState[client.Name()] = client.IsConnected()
	}

	if len(a.clouds) == 0 {
		log.Printf("all mqtt channels disabled, cloud publish skipped")
	}
}

func (a *App) applyRemoteConfig(command cloud.RemoteConfigCommand) cloud.RemoteConfigResult {
	current := a.runtime.Config()
	desired, err := mergeRemoteConfig(current, command.Config)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "閰嶇疆 JSON 鏃犳晥: " + err.Error()}
	}
	raw, err := json.Marshal(desired)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "閰嶇疆搴忓垪鍖栧け璐? " + err.Error()}
	}
	validated, err := config.Parse(raw)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "閰嶇疆鏍￠獙澶辫触: " + err.Error()}
	}
	if err := edgecompute.Validate(validated); err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "边缘计算配置无效: " + err.Error()}
	}
	oldRaw, err := os.ReadFile(a.configPath)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "璇诲彇褰撳墠閰嶇疆澶辫触: " + err.Error()}
	}
	if err := os.WriteFile(a.configPath+".remote.bak", oldRaw, 0600); err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "澶囦唤褰撳墠閰嶇疆澶辫触: " + err.Error()}
	}
	if err := config.Save(a.configPath, validated); err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "淇濆瓨杩滅▼閰嶇疆澶辫触: " + err.Error()}
	}
	a.ApplyConfig(validated)
	a.runtime.CollectOnce(context.Background())
	log.Printf("remote config applied task=%s version=%d", command.TaskID, command.Version)
	return cloud.RemoteConfigResult{Status: "APPLIED", Message: "配置已校验、备份并热加载"}
}

type attributeSetCommand struct {
	DeviceKey string      `json:"deviceKey"`
	Metric    string      `json:"metric"`
	Value     interface{} `json:"value"`
}

func (a *App) handleMQTTAttributeSet(topic string, payload []byte) {
	commands, err := parseAttributeSetCommands(payload)
	if err != nil {
		log.Printf("mqtt attribute set parse failed topic=%s: %v", topic, err)
		a.state.AddError("mqtt attribute set parse failed: " + err.Error())
		return
	}
	for _, command := range commands {
		if err := a.writeAttribute(command); err != nil {
			log.Printf("mqtt attribute write failed topic=%s metric=%s: %v", topic, command.Metric, err)
			a.state.AddError("mqtt attribute write failed " + command.Metric + ": " + err.Error())
		}
	}
}

func parseAttributeSetCommands(payload []byte) ([]attributeSetCommand, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var raw interface{}
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	switch typed := raw.(type) {
	case map[string]interface{}:
		if metric, _ := typed["metric"].(string); metric != "" {
			return []attributeSetCommand{{DeviceKey: stringValue(typed["deviceKey"]), Metric: metric, Value: typed["value"]}}, nil
		}
		commands := make([]attributeSetCommand, 0, len(typed))
		for metric, value := range typed {
			commands = append(commands, attributeSetCommand{Metric: metric, Value: value})
		}
		return commands, nil
	default:
		return nil, fmt.Errorf("payload must be JSON object")
	}
}

func (a *App) writeAttribute(command attributeSetCommand) error {
	if command.Metric == "" {
		return fmt.Errorf("metric is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := a.runtime.WritePoint(ctx, command.DeviceKey, command.Metric, command.Value)
	return err
}

func stringValue(value interface{}) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
func mergeRemoteConfig(current config.Config, patchRaw json.RawMessage) (config.Config, error) {
	currentRaw, err := json.Marshal(current)
	if err != nil {
		return config.Config{}, err
	}
	var base map[string]interface{}
	var patch map[string]interface{}
	if err := json.Unmarshal(currentRaw, &base); err != nil {
		return config.Config{}, err
	}
	if err := json.Unmarshal(patchRaw, &patch); err != nil {
		return config.Config{}, err
	}
	if patch == nil {
		return config.Config{}, fmt.Errorf("閰嶇疆蹇呴』鏄?JSON 瀵硅薄")
	}
	// Connectivity and local storage settings stay under local control.
	for _, key := range []string{"gatewayKey", "activation", "mqtt", "web", "networkPorts", "wifi", "offlineCache", "historyStorage"} {
		delete(patch, key)
	}
	preserveRemoteSecrets(base, patch)
	mergeConfigObject(base, patch)
	mergedRaw, err := json.Marshal(base)
	if err != nil {
		return config.Config{}, err
	}
	return config.Parse(mergedRaw)
}

func (a *App) remoteConfigSnapshot() interface{} {
	cfg := a.runtime.Config()
	devices := append([]config.DeviceConfig(nil), cfg.Devices...)
	for index := range devices {
		devices[index].Points = append([]config.PointConfig(nil), devices[index].Points...)
		if devices[index].Password != "" {
			devices[index].Password = "***"
		}
		for pointIndex := range devices[index].Points {
			if devices[index].Points[pointIndex].Password != "" {
				devices[index].Points[pointIndex].Password = "***"
			}
		}
	}
	return map[string]interface{}{
		"collectIntervalSeconds": cfg.CollectIntervalSeconds,
		"serialPorts":            cfg.SerialPorts,
		"devices":                devices,
		"forwardSlave":           cfg.ForwardSlave,
	}
}

func preserveRemoteSecrets(base, patch map[string]interface{}) {
	baseDevices, baseOK := base["devices"].([]interface{})
	patchDevices, patchOK := patch["devices"].([]interface{})
	if !baseOK || !patchOK {
		return
	}
	baseByKey := map[string]map[string]interface{}{}
	for _, item := range baseDevices {
		if device, ok := item.(map[string]interface{}); ok {
			baseByKey[fmt.Sprint(device["deviceKey"])] = device
		}
	}
	for _, item := range patchDevices {
		device, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		current := baseByKey[fmt.Sprint(device["deviceKey"])]
		if current == nil {
			continue
		}
		preserveMaskedValue(current, device, "password")
		currentPoints, _ := current["points"].([]interface{})
		patchPoints, _ := device["points"].([]interface{})
		pointsByMetric := map[string]map[string]interface{}{}
		for _, pointItem := range currentPoints {
			if point, ok := pointItem.(map[string]interface{}); ok {
				pointsByMetric[fmt.Sprint(point["metric"])] = point
			}
		}
		for _, pointItem := range patchPoints {
			point, ok := pointItem.(map[string]interface{})
			if !ok {
				continue
			}
			if currentPoint := pointsByMetric[fmt.Sprint(point["metric"])]; currentPoint != nil {
				preserveMaskedValue(currentPoint, point, "password")
			}
		}
	}
}

func preserveMaskedValue(current, patch map[string]interface{}, key string) {
	if patch[key] == "***" {
		patch[key] = current[key]
	}
}

func mergeConfigObject(target, patch map[string]interface{}) {
	for key, value := range patch {
		patchObject, patchIsObject := value.(map[string]interface{})
		targetObject, targetIsObject := target[key].(map[string]interface{})
		if patchIsObject && targetIsObject {
			mergeConfigObject(targetObject, patchObject)
			continue
		}
		target[key] = value
	}
}

func (a *App) disconnectCloud() {
	a.cloudMu.Lock()
	defer a.cloudMu.Unlock()
	for _, client := range a.clouds {
		client.Disconnect()
		a.state.SetMQTTChannelConnected(client.Name(), false)
	}
	a.clouds = nil
	a.cloudState = map[string]bool{}
	a.cloudConfig = map[string]string{}
}

func (a *App) connectedCloudsLocked() []*cloud.Client {
	var clients []*cloud.Client
	for _, client := range a.clouds {
		if client.IsConnected() {
			clients = append(clients, client)
		}
	}
	return clients
}

func (a *App) setCloudChannelConnected(name string) func(bool) {
	return func(connected bool) {
		go func() {
			a.cloudMu.Lock()
			defer a.cloudMu.Unlock()
			a.setCloudChannelConnectedLocked(name, connected)
		}()
	}
}

func (a *App) setCloudChannelConnectedLocked(name string, connected bool) {
	a.cloudState[name] = connected
	a.state.SetMQTTChannelConnected(name, connected)
}

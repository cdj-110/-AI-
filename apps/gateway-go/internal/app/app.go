package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cache"
	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/forward"
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
	state         *state.Store
	runtime       *gatewayruntime.Manager
	forward       *forward.Manager
	collectSem    chan struct{}
	configChanged chan struct{}
}

func New(cfg config.Config, configPath string) *App {
	store := state.New(cfg)
	manager := gatewayruntime.NewManager(cfg, store)
	return &App{
		cfg:           cfg,
		configPath:    configPath,
		cloudState:    map[string]bool{},
		cloudConfig:   map[string]string{},
		spool:         offlineSpool(cfg),
		state:         store,
		runtime:       manager,
		forward:       forward.NewManager(store),
		collectSem:    make(chan struct{}, 1),
		configChanged: make(chan struct{}, 1),
	}
}

func (a *App) Run(ctx context.Context) error {
	go web.New(a.cfg.Web, a.state, a.runtime, a.configPath, a.ApplyConfig).Run(ctx)
	a.syncCloud(a.cfg)
	defer a.disconnectCloud()
	a.forward.Update(ctx, a.cfg)
	defer a.forward.Stop()

	log.Printf("gateway %s started, points=%d", a.cfg.GatewayKey, len(a.cfg.Points))
	go a.heartbeatLoop(ctx)
	go a.collectAndPublish(ctx)
	for {
		interval := a.runtime.CollectInterval()
		if interval <= 0 {
			interval = time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-a.configChanged:
			timer.Stop()
			continue
		case <-timer.C:
			go a.collectAndPublish(ctx)
		}
	}
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
	a.cloudMu.Lock()
	clients := a.connectedCloudsLocked()
	a.cloudMu.Unlock()
	for _, client := range clients {
		if client.IsManual() {
			continue
		}
		if err := client.PublishGatewayHeartbeat(); err != nil {
			log.Printf("publish gateway heartbeat failed channel=%s: %v", client.Name(), err)
			a.state.AddError("publish gateway heartbeat failed " + client.Name() + ": " + err.Error())
			continue
		}
		a.state.MarkPublish()
	}
}

func (a *App) ApplyConfig(cfg config.Config) {
	oldCfg := a.runtime.Config()
	a.runtime.UpdateConfig(cfg)
	a.forward.Update(context.Background(), cfg)
	a.cfg = cfg
	a.spoolMu.Lock()
	a.spool = offlineSpool(cfg)
	a.spoolMu.Unlock()
	if !oldCfg.MQTT.Equal(cfg.MQTT) || !config.EqualMQTTChannels(oldCfg.MQTTChannels, cfg.MQTTChannels) || !oldCfg.Activation.Equal(cfg.Activation) {
		a.syncCloud(cfg)
	}
	select {
	case a.configChanged <- struct{}{}:
	default:
	}
}

func (a *App) collectAndPublish(ctx context.Context) {
	select {
	case a.collectSem <- struct{}{}:
	default:
		return
	}
	grouped := a.runtime.CollectOnce(ctx)
	<-a.collectSem
	if len(grouped) == 0 {
		return
	}
	a.publishToCloud(grouped)
}

func (a *App) publishToCloud(grouped map[string]map[string]interface{}) {
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
		reading := model.Reading{DeviceKey: deviceKey, Time: time.Now(), Metrics: metrics}
		platformPublished := false
		for _, client := range clients {
			if client.IsManual() {
				continue
			}
			if err := client.PublishChildHeartbeat(deviceKey); err != nil {
				log.Printf("publish child heartbeat failed channel=%s device=%s: %v", client.Name(), deviceKey, err)
				a.state.AddError("publish child heartbeat failed " + client.Name() + "/" + deviceKey + ": " + err.Error())
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
	point, ok := findWritablePoint(a.runtime.Config(), command.DeviceKey, command.Metric)
	if !ok {
		return fmt.Errorf("writable point not found for metric %s", command.Metric)
	}
	writer, err := collector.New(point.Protocol)
	if err != nil {
		return err
	}
	pointWriter, ok := writer.(collector.PointWriter)
	if !ok {
		return fmt.Errorf("protocol %s does not support write", point.Protocol)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pointWriter.WritePoint(ctx, point, command.Value); err != nil {
		return err
	}
	a.state.SetPointValue(point.DeviceKey, point.Metric, command.Value)
	return nil
}

func findWritablePoint(cfg config.Config, deviceKey string, metric string) (config.PointConfig, bool) {
	for _, point := range cfg.Points {
		if point.Metric != metric {
			continue
		}
		if deviceKey != "" && point.DeviceKey != deviceKey {
			continue
		}
		if !isWritablePoint(point) {
			continue
		}
		return point, true
	}
	return config.PointConfig{}, false
}

func isWritablePoint(point config.PointConfig) bool {
	switch point.Protocol {
	case "modbus-tcp", "modbus-rtu":
		return point.Function == 1 || point.Function == 3
	default:
		return false
	}
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
	for _, key := range []string{"gatewayKey", "activation", "mqtt", "web", "networkPorts", "wifi", "offlineCache"} {
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

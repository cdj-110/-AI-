package app

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
	"weikong-iot-platform/apps/gateway-go/internal/cache"
	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/collectdiag"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
	"weikong-iot-platform/apps/gateway-go/internal/forward"
	"weikong-iot-platform/apps/gateway-go/internal/history"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/remoteaccess"
	"weikong-iot-platform/apps/gateway-go/internal/remotestore"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
	"weikong-iot-platform/apps/gateway-go/internal/storage"
	"weikong-iot-platform/apps/gateway-go/internal/web"
)

type App struct {
	remoteStorage       *remotestore.Manager
	cfg                 config.Config
	configPath          string
	cloudMu             sync.Mutex
	clouds              []*cloud.Client
	cloudState          map[string]bool
	cloudConfig         map[string]string
	remoteMu            sync.Mutex
	remoteCancel        context.CancelFunc
	spoolMu             sync.Mutex
	spool               *cache.Spool
	alarmEventSpool     *alarmEventSpool
	alarmEventWake      chan struct{}
	historyMu           sync.Mutex
	history             *history.Store
	historyWriter       *history.AsyncWriter
	historyLastSaved    map[string]historySavedSample
	state               *state.Store
	runtime             *gatewayruntime.Manager
	forward             *forward.Manager
	edge                *edgecompute.Manager
	alarms              *alarm.Manager
	collectionIssues    *collectdiag.Store
	collectMu           sync.Mutex
	runCtx              context.Context
	collectCancel       context.CancelFunc
	collectWG           sync.WaitGroup
	publishMu           sync.Mutex
	remoteConfigMu      sync.Mutex
	pointCommands       *cloud.PointCommandExecutor
	maintenanceCommands *cloud.MaintenanceCommandExecutor
	publishQueue        *latestTelemetryQueue
	telemetryMu         sync.RWMutex
	latestTelemetry     map[string]map[string]interface{}
	publishBackoffMu    sync.Mutex
	publishBackoff      map[string]*publishBackoff
}

type telemetryBatch struct {
	Time    time.Time
	Grouped map[string]map[string]interface{}
	Points  map[string]map[string]model.PointValue
}

type historySavedSample struct {
	At        time.Time
	ValueJSON string
	Numeric   *big.Rat
	IsNumeric bool
}

func New(cfg config.Config, configPath string) (*App, error) {
	store := state.New(cfg)
	manager := gatewayruntime.NewManager(cfg, store)
	edge, err := edgecompute.NewManager(cfg, store)
	if err != nil {
		return nil, err
	}
	collectionIssues, incidentErr := collectdiag.Open(configPath)
	if incidentErr != nil {
		log.Printf("collection incident database unavailable: %v", incidentErr)
		store.AddError("采集异常记录数据库不可用: " + incidentErr.Error())
	} else {
		store.SetPointIncidentRecorder(collectionIssues)
	}
	alarms := alarm.NewManager(cfg, store, configPath)
	app := &App{
		cfg:                 cfg,
		configPath:          configPath,
		pointCommands:       cloud.NewPointCommandExecutor(configPath + ".point-commands.json"),
		maintenanceCommands: cloud.NewMaintenanceCommandExecutor(configPath + ".maintenance-commands.json"),
		cloudState:          map[string]bool{},
		cloudConfig:         map[string]string{},
		spool:               offlineSpool(cfg),
		history:             historyStore(cfg),
		historyLastSaved:    map[string]historySavedSample{},
		state:               store,
		runtime:             manager,
		forward:             forward.NewManager(store, manager),
		edge:                edge,
		alarms:              alarms,
		alarmEventSpool:     newAlarmEventSpool(alarmEventSpoolPath(configPath), 4*1024*1024),
		alarmEventWake:      make(chan struct{}, 1),
		collectionIssues:    collectionIssues,
		// Realtime telemetry is coalesced by device. Keeping intermediate
		// samples is harmful on low-memory gateways when a broker is slow.
		publishQueue:    newLatestTelemetryQueue(),
		latestTelemetry: map[string]map[string]interface{}{},
		publishBackoff:  map[string]*publishBackoff{},
	}
	alarms.SetEventSink(func(event alarm.Event) error {
		if err := app.alarmEventSpool.append(event); err != nil {
			return err
		}
		select {
		case app.alarmEventWake <- struct{}{}:
		default:
		}
		return nil
	})
	app.historyWriter = history.NewAsyncWriter(app.history, app.handleHistoryError)
	app.remoteStorage = remotestore.New(store)
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	defer func() {
		a.state.SetPointIncidentRecorder(nil)
		if a.collectionIssues != nil {
			_ = a.collectionIssues.Close()
		}
	}()
	defer func() {
		a.historyMu.Lock()
		writer := a.historyWriter
		a.historyWriter = nil
		a.historyMu.Unlock()
		if writer != nil {
			_ = writer.Close()
		}
	}()
	webServer := web.New(a.cfg.Web, a.state, a.runtime, a.configPath, a.ApplyConfig, a.alarms)
	if !a.cfg.License.Restricted && a.cfg.Activation.AllowsFeature("remote-storage") {
		a.remoteStorage.Start(ctx, a.cfg)
		defer a.remoteStorage.Close()
	}
	webServer.SetRemoteStorage(a.remoteStorage)
	webServer.SetHistoryWriterStatus(func() history.WriterStatus {
		a.historyMu.Lock()
		writer := a.historyWriter
		a.historyMu.Unlock()
		return writer.Status()
	})
	webServer.SetCollectionIncidents(a.collectionIssues)
	go webServer.Run(ctx)
	go a.healthLoop(ctx)
	if a.cfg.License.Restricted {
		a.state.AddError("网关授权无效，当前仅开放维护与授权导入: " + a.cfg.License.Reason)
		log.Printf("gateway started in restricted maintenance mode: %s", a.cfg.License.Reason)
		<-ctx.Done()
		return nil
	}
	if a.cfg.Activation.AllowsFeature("remote-access") {
		a.syncRemoteAccess(ctx, a.cfg)
	}
	defer a.stopRemoteAccess()
	if a.cfg.Activation.AllowsFeature("cloud") {
		a.syncCloud(a.cfg)
		go a.otaReceiptLoop(ctx)
	}
	defer a.disconnectCloud()
	if a.cfg.Activation.AllowsFeature("forward") {
		a.forward.Update(ctx, a.cfg)
	}
	defer a.forward.Stop()

	log.Printf("gateway %s started, points=%d", a.cfg.GatewayKey, len(a.cfg.Points))
	go a.heartbeatLoop(ctx)
	go a.topologyLoop(ctx)
	if a.cfg.Activation.AllowsFeature("cloud") {
		go a.publishLoop(ctx)
		go a.alarmEventPublishLoop(ctx)
		go a.manualPublishLoop(ctx)
	}
	if a.cfg.Activation.AllowsFeature("edge") {
		go a.edge.Start(ctx)
		go a.edgeBatchLoop(ctx)
	}
	if a.cfg.Activation.AllowsFeature("alarms") {
		go a.alarms.Start(ctx)
	}
	if a.cfg.Activation.AllowsFeature("collection") {
		a.startCollectWorkers(ctx)
	}
	defer func() {
		// Collector cancellation during a deliberate service stop is not a
		// device failure and must not create one incident per point.
		a.state.SetPointIncidentRecorder(nil)
		a.stopCollectWorkers()
	}()
	<-ctx.Done()
	return nil
}

func (a *App) alarmEventPublishLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.alarmEventWake:
			a.publishAlarmEvents()
		case <-ticker.C:
			a.publishAlarmEvents()
		}
	}
}

func (a *App) publishAlarmEvents() {
	a.cloudMu.Lock()
	platform := firstPlatformClient(a.connectedCloudsLocked())
	a.cloudMu.Unlock()
	if platform == nil {
		return
	}
	a.publishMu.Lock()
	defer a.publishMu.Unlock()
	err := a.alarmEventSpool.drain(50, func(event alarm.Event) error {
		return platform.PublishAlarmEvent(event)
	})
	if err != nil {
		log.Printf("publish alarm events failed: %v", err)
		return
	}
	a.state.MarkPublish()
}

func alarmEventSpoolPath(configPath string) string {
	if strings.TrimSpace(configPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), ".runtime", "cloud-alarm-events.jsonl")
}

func remoteAccessCredential(cfg config.Config) (string, string) {
	if cfg.RemoteAccess.Username != "" || cfg.RemoteAccess.Password != "" {
		return cfg.RemoteAccess.Username, cfg.RemoteAccess.Password
	}
	if cfg.Activation.IsReady() {
		return "factory:" + cfg.Activation.HardwareID, cfg.Activation.DeviceSecret
	}
	for _, channel := range cfg.ManualMQTTChannels() {
		if channel.IsEnabled() && channel.Username != "" && channel.Password != "" {
			return channel.Username, channel.Password
		}
	}
	return cfg.MQTT.Username, cfg.MQTT.Password
}

func (a *App) syncRemoteAccess(parent context.Context, cfg config.Config) {
	a.stopRemoteAccess()
	if parent == nil || !cfg.RemoteAccess.Enabled {
		return
	}
	username, password := remoteAccessCredential(cfg)
	remoteCtx, cancel := context.WithCancel(parent)
	a.remoteMu.Lock()
	a.remoteCancel = cancel
	a.remoteMu.Unlock()
	go remoteaccess.New(cfg.RemoteAccess, cfg.GatewayKey, username, password).Run(remoteCtx)
}

func (a *App) stopRemoteAccess() {
	a.remoteMu.Lock()
	cancel := a.remoteCancel
	a.remoteCancel = nil
	a.remoteMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) healthLoop(ctx context.Context) {
	a.state.MarkProgress()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.state.MarkProgress()
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
	// Authorization/liveness must not wait for the telemetry backlog to drain.
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
	if a.publishQueue.HasPending() {
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

func (a *App) ApplyConfig(cfg config.Config) error {
	if cfg.License.Restricted {
		return fmt.Errorf("网关授权无效，仅允许维护和导入有效授权")
	}
	if cfg.Activation.Signature != "" {
		if err := config.CheckLicenseReplay(a.configPath, cfg.Activation); err != nil {
			return err
		}
	}
	if a.remoteStorage != nil {
		if err := a.remoteStorage.ValidateChange(cfg.RemoteStorage); err != nil {
			return err
		}
	}
	if err := edgecompute.Validate(cfg); err != nil {
		a.state.AddError("edge compute config rejected: " + err.Error())
		return err
	}
	oldCfg := a.runtime.Config()
	if err := a.edge.UpdateConfig(cfg); err != nil {
		a.state.AddError("edge compute reload failed: " + err.Error())
		return err
	}
	// Stop and drain the old collection generation before replacing protocol
	// sessions. This prevents an old OPC DA lane from reconnecting with stale
	// SyncIO settings while the selected device is being hot-rebuilt for
	// IOPCDataCallback.
	a.stopCollectWorkers()
	// Only tear down active protocol sessions after every synchronous compiler
	// has accepted the candidate configuration.
	a.runtime.UpdateConfig(cfg)
	a.alarms.UpdateConfig(cfg, a.configPath)
	// Values from removed devices or points must not remain in full-snapshot
	// manual MQTT payloads after a hot reload. New collectors repopulate this
	// cache before the first cadence-controlled publication.
	a.telemetryMu.Lock()
	a.latestTelemetry = map[string]map[string]interface{}{}
	a.telemetryMu.Unlock()
	if cfg.Activation.AllowsFeature("forward") {
		a.forward.Update(context.Background(), cfg)
	} else {
		a.forward.Stop()
	}
	a.cfg = cfg
	a.spoolMu.Lock()
	a.spool = offlineSpool(cfg)
	a.spoolMu.Unlock()
	if !reflect.DeepEqual(oldCfg.HistoryStorage, cfg.HistoryStorage) {
		a.replaceHistory(cfg)
	}
	if oldCfg.CloudConnectionIsEnabled() != cfg.CloudConnectionIsEnabled() || !oldCfg.MQTT.Equal(cfg.MQTT) || !config.EqualMQTTChannels(oldCfg.MQTTChannels, cfg.MQTTChannels) || !oldCfg.Activation.Equal(cfg.Activation) {
		if cfg.Activation.AllowsFeature("cloud") {
			a.syncCloud(cfg)
		} else {
			a.disconnectCloud()
		}
	}
	if !oldCfg.RemoteAccess.Equal(cfg.RemoteAccess) || oldCfg.GatewayKey != cfg.GatewayKey || !oldCfg.MQTT.Equal(cfg.MQTT) || !oldCfg.Activation.Equal(cfg.Activation) {
		if cfg.Activation.AllowsFeature("remote-access") {
			a.syncRemoteAccess(a.runCtx, cfg)
		} else {
			a.stopRemoteAccess()
		}
	}
	if cfg.Activation.AllowsFeature("collection") {
		a.startCollectWorkers(a.runCtx)
	}
	// Remote database cancellation/reconnection must not prolong the stopped
	// collection window during a configuration reload.
	if a.remoteStorage != nil && cfg.Activation.AllowsFeature("remote-storage") {
		a.remoteStorage.Update(cfg)
	}
	go a.publishTopology()
	return nil
}

// Keep unchanged history connections across ordinary device edits. When the
// storage configuration changes, drain/close before opening its replacement.
func (a *App) replaceHistory(cfg config.Config) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	var err error
	if a.historyWriter != nil {
		err = a.historyWriter.Close()
	} else if a.history != nil {
		err = a.history.Close()
	}
	if err != nil {
		a.handleHistoryError(err)
	}
	a.history = historyStore(cfg)
	a.historyWriter = history.NewAsyncWriter(a.history, a.handleHistoryError)
	a.historyLastSaved = map[string]historySavedSample{}
}

func (a *App) edgeBatchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-a.edge.Output():
			telemetry := telemetryBatch{Time: batch.Time, Grouped: batch.Grouped, Points: batch.Points}
			a.recordHistory(telemetry)
			a.enqueueTelemetry(telemetry)
		}
	}
}

func (a *App) startCollectWorkers(parent context.Context) {
	if parent == nil {
		return
	}
	a.stopCollectWorkers()
	a.collectMu.Lock()
	defer a.collectMu.Unlock()
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
		a.collectWG.Add(1)
		go func() {
			defer a.collectWG.Done()
			a.collectLaneLoop(workerCtx, lane)
		}()
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
	cancel := a.collectCancel
	a.collectCancel = nil
	a.collectMu.Unlock()
	if cancel != nil {
		cancel()
		a.collectWG.Wait()
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
			if len(lane.Points) > 0 && lane.Points[0].Protocol == "dlt645" {
				// Drop missed ticks: a slow serial round must not build a backlog
				// or immediately compete with itself after it finally completes.
				ticker.Reset(interval)
			}
		}
	}
}

func (a *App) collectLaneAndPublish(ctx context.Context, laneKey string) {
	collected := a.runtime.CollectLaneDetailed(ctx, laneKey, false)
	if len(collected.Grouped) == 0 {
		return
	}
	batch := telemetryBatch{Time: time.Now(), Grouped: collected.Grouped, Points: collected.Points}
	a.recordHistory(batch)
	a.enqueueTelemetry(batch)
}

func (a *App) publishLoop(ctx context.Context) {
	for {
		batch, ok := a.publishQueue.Take(ctx)
		if !ok {
			return
		}
		a.publishToCloud(batch)
	}
}

// manualPublishLoop is deliberately independent from the activation/platform
// publisher. A connected platform broker can still stop acknowledging QoS 1
// messages; its timeout must never stretch a 1-second manual MQTT channel to
// two seconds or prevent other manual channels from publishing.
func (a *App) manualPublishLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.publishManualChannels(now)
		}
	}
}

func (a *App) publishManualChannels(now time.Time) {
	grouped := mqttGroupedValues(a.runtime.Config(), a.latestTelemetrySnapshot())
	if len(grouped) == 0 {
		return
	}
	a.cloudMu.Lock()
	clients := a.connectedCloudsLocked()
	a.cloudMu.Unlock()
	for _, client := range clients {
		if !client.IsManual() || !a.manualPublishReady(client.Name(), now, client.PublishInterval()) {
			continue
		}
		go a.publishManualChannel(client, grouped, now)
	}
}

func (a *App) publishManualChannel(client *cloud.Client, grouped map[string]map[string]interface{}, now time.Time) {
	err := client.PublishManual(grouped)
	if err == cloud.ErrNoManualPayload {
		return
	}
	if err != nil {
		delay := a.manualPublishFailed(client.Name(), time.Now())
		log.Printf("publish attributes failed channel=%s retry_in=%s: %v", client.Name(), delay, err)
		a.state.AddError("publish attributes failed " + client.Name() + ": " + err.Error())
		a.setCloudChannelConnected(client.Name())(false)
		return
	}
	a.manualPublishSucceeded(client.Name())
	a.setCloudChannelConnected(client.Name())(true)
	a.state.MarkPublish()
}

func (a *App) enqueueTelemetry(batch telemetryBatch) {
	a.mergeLatestTelemetry(batch.Grouped)
	a.publishQueue.Put(batch)
}

func (a *App) mergeLatestTelemetry(grouped map[string]map[string]interface{}) {
	a.telemetryMu.Lock()
	defer a.telemetryMu.Unlock()
	for deviceKey, metrics := range grouped {
		current := a.latestTelemetry[deviceKey]
		if current == nil {
			current = make(map[string]interface{}, len(metrics))
			a.latestTelemetry[deviceKey] = current
		}
		for metric, value := range metrics {
			current[metric] = value
		}
	}
}

func (a *App) latestTelemetrySnapshot() map[string]map[string]interface{} {
	a.telemetryMu.RLock()
	defer a.telemetryMu.RUnlock()
	result := make(map[string]map[string]interface{}, len(a.latestTelemetry))
	for deviceKey, metrics := range a.latestTelemetry {
		copyMetrics := make(map[string]interface{}, len(metrics))
		for metric, value := range metrics {
			copyMetrics[metric] = value
		}
		result[deviceKey] = copyMetrics
	}
	return result
}

func (a *App) publishToCloud(batch telemetryBatch) {
	a.publishMu.Lock()
	defer a.publishMu.Unlock()
	grouped := mqttGroupedValues(a.runtime.Config(), batch.Grouped)
	a.cloudMu.Lock()
	clients := a.connectedCloudsLocked()
	a.cloudMu.Unlock()
	if platformClient := firstPlatformClient(clients); platformClient != nil {
		a.spoolMu.Lock()
		_ = a.spool.DrainLimit(platformClient.PublishTelemetry, 5)
		a.spoolMu.Unlock()
	}
	publishedAny := false
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
	cfg := a.runtime.Config()
	if !cfg.HistoryStorage.Enabled {
		return
	}
	grouped := mqttGroupedValues(cfg, batch.Grouped)
	a.historyMu.Lock()
	grouped, taskKeys := historyTaskGroupedValuesWithTasks(cfg.HistoryStorage, batch.Time, grouped, a.historyLastSaved)
	writer := a.historyWriter
	writer.Enqueue(historySamplesWithTasks(batch, grouped, taskKeys))
	a.historyMu.Unlock()
}

func (a *App) handleHistoryError(err error) {
	log.Printf("append history failed: %v", err)
	a.state.AddError("append history failed: " + err.Error())
}

func historySamples(batch telemetryBatch, grouped map[string]map[string]interface{}) []history.Sample {
	return historySamplesWithTasks(batch, grouped, nil)
}

func historySamplesWithTasks(batch telemetryBatch, grouped map[string]map[string]interface{}, taskKeys map[string][]string) []history.Sample {
	samples := make([]history.Sample, 0)
	for deviceKey, metrics := range grouped {
		for metric, value := range metrics {
			pointKey := deviceKey + "\x00" + metric
			pointValue := model.PointValue{DeviceKey: deviceKey, Metric: metric, Value: value}
			if devicePoints := batch.Points[deviceKey]; devicePoints != nil {
				if detailed, ok := devicePoints[metric]; ok {
					pointValue = detailed
					pointValue.Value = value
				}
			}
			pointValue.ApplyDefaults(batch.Time)
			samples = append(samples, history.Sample{
				Time:             batch.Time,
				DeviceKey:        deviceKey,
				Metric:           metric,
				Value:            pointValue.Value,
				ActualDataType:   pointValue.ActualDataType,
				Quality:          pointValue.Quality,
				RawQuality:       pointValue.RawQuality,
				SourceTimestamp:  pointValue.SourceTimestamp,
				ReceiveTimestamp: pointValue.ReceiveTimestamp,
				CollectionMode:   pointValue.CollectionMode,
				DiagnosticCode:   pointValue.DiagnosticCode,
				Diagnostic:       pointValue.Diagnostic,
				TaskKeys:         append([]string(nil), taskKeys[pointKey]...),
			})
		}
	}
	return samples
}

func applyHistorySavePolicy(historyConfig config.HistoryStorageConfig, at time.Time, grouped map[string]map[string]interface{}, last map[string]historySavedSample) map[string]map[string]interface{} {
	return applyHistorySavePolicyScoped(historyConfig, at, grouped, last, "")
}

func applyHistorySavePolicyScoped(historyConfig config.HistoryStorageConfig, at time.Time, grouped map[string]map[string]interface{}, last map[string]historySavedSample, scope string) map[string]map[string]interface{} {
	if len(grouped) == 0 {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	mode := historyConfig.SaveMode
	if mode == "" {
		mode = "interval"
	}
	interval := time.Duration(historyConfig.SaveIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Second
	}
	result := make(map[string]map[string]interface{})
	for deviceKey, metrics := range grouped {
		for metric, value := range metrics {
			key := scope + deviceKey + "\x00" + metric
			previous, exists := last[key]
			valueJSON, numeric, isNumeric := historyValueIdentity(value)
			keep := !exists
			if exists && mode == "change" {
				if historyConfig.ChangeDeadband > 0 && isNumeric && previous.IsNumeric {
					difference := new(big.Rat).Sub(numeric, previous.Numeric)
					if difference.Sign() < 0 {
						difference.Neg(difference)
					}
					keep = difference.Cmp(new(big.Rat).SetFloat64(historyConfig.ChangeDeadband)) > 0
				} else {
					keep = valueJSON != previous.ValueJSON
				}
			} else if exists {
				keep = at.Sub(previous.At) >= interval
			}
			if !keep {
				continue
			}
			if result[deviceKey] == nil {
				result[deviceKey] = make(map[string]interface{})
			}
			result[deviceKey][metric] = value
			last[key] = historySavedSample{At: at, ValueJSON: valueJSON, Numeric: numeric, IsNumeric: isNumeric}
		}
	}
	return result
}

func historyTaskGroupedValues(historyConfig config.HistoryStorageConfig, at time.Time, grouped map[string]map[string]interface{}, last map[string]historySavedSample) map[string]map[string]interface{} {
	result, _ := historyTaskGroupedValuesWithTasks(historyConfig, at, grouped, last)
	return result
}

func historyTaskGroupedValuesWithTasks(historyConfig config.HistoryStorageConfig, at time.Time, grouped map[string]map[string]interface{}, last map[string]historySavedSample) (map[string]map[string]interface{}, map[string][]string) {
	if at.IsZero() {
		at = time.Now()
	}
	result := make(map[string]map[string]interface{})
	taskKeys := make(map[string][]string)
	for _, task := range historyConfig.EffectiveTasks() {
		if task.StatusAt(at) != "running" {
			continue
		}
		policy := config.HistoryStorageConfig{
			SelectionMode: task.SelectionMode, Points: task.Points,
			SaveMode: task.SaveMode, SaveIntervalSeconds: task.SaveIntervalSeconds,
			ChangeDeadband: task.ChangeDeadband,
		}
		selected := historyGroupedValues(policy, grouped)
		selected = applyHistorySavePolicyScoped(policy, at, selected, last, task.TaskKey+"\x00")
		for deviceKey, metrics := range selected {
			if result[deviceKey] == nil {
				result[deviceKey] = make(map[string]interface{})
			}
			for metric, value := range metrics {
				result[deviceKey][metric] = value
				key := deviceKey + "\x00" + metric
				taskKeys[key] = append(taskKeys[key], task.TaskKey)
			}
		}
	}
	return result, taskKeys
}

func historyValueIdentity(value interface{}) (string, *big.Rat, bool) {
	return history.ValueIdentity(value)
}

func historyGroupedValues(historyConfig config.HistoryStorageConfig, grouped map[string]map[string]interface{}) map[string]map[string]interface{} {
	if historyConfig.SelectionMode == "" || historyConfig.SelectionMode == "all" {
		return grouped
	}
	if historyConfig.SelectionMode != "selected" || len(historyConfig.Points) == 0 || len(grouped) == 0 {
		return nil
	}
	selected := make(map[string]struct{}, len(historyConfig.Points))
	for _, point := range historyConfig.Points {
		selected[point.DeviceKey+"\x00"+point.Metric] = struct{}{}
	}
	filtered := make(map[string]map[string]interface{})
	for deviceKey, metrics := range grouped {
		for metric, value := range metrics {
			if _, ok := selected[deviceKey+"\x00"+metric]; !ok {
				continue
			}
			if filtered[deviceKey] == nil {
				filtered[deviceKey] = make(map[string]interface{})
			}
			filtered[deviceKey][metric] = value
		}
	}
	return filtered
}

func mqttGroupedValues(_ config.Config, grouped map[string]map[string]interface{}) map[string]map[string]interface{} {
	// Collectors decode point types before values reach this boundary. Preserve
	// native booleans (including Modbus coils) for MQTT, offline spool and new
	// history samples; never infer a boolean from an ordinary numeric 0 or 1.
	return grouped
}

type topologyMetric struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name,omitempty"`
	DataType   string `json:"dataType,omitempty"`
	Unit       string `json:"unit,omitempty"`
	CanRead    bool   `json:"canRead"`
	CanWrite   bool   `json:"canWrite"`
}

type topologyDevice struct {
	PointAccessVersion int              `json:"pointAccessVersion"`
	DeviceKey          string           `json:"deviceKey"`
	Name               string           `json:"name,omitempty"`
	Protocol           string           `json:"protocol,omitempty"`
	Address            string           `json:"address,omitempty"`
	SlaveID            byte             `json:"slaveId,omitempty"`
	CommonAddress      uint16           `json:"commonAddress,omitempty"`
	PointCount         int              `json:"pointCount"`
	Metrics            []topologyMetric `json:"metrics"`
}

func topologySnapshot(cfg config.Config) map[string]interface{} {
	devices := topologyDevices(cfg)
	raw, _ := json.Marshal(devices)
	sum := sha1.Sum(raw)
	return map[string]interface{}{
		"gatewayKey":         cfg.GatewayKey,
		"pointAccessVersion": 1,
		"configVersion":      fmt.Sprintf("%x", sum[:8]),
		"updatedAt":          time.Now().Format(time.RFC3339Nano),
		"devices":            devices,
	}
}

func topologyDevices(cfg config.Config) []topologyDevice {
	if len(cfg.Devices) > 0 {
		devices := make([]topologyDevice, 0, len(cfg.Devices))
		for _, device := range cfg.Devices {
			if device.DeviceKey == "" {
				continue
			}
			// Flattened runtime points contain inherited protocol/resource settings.
			points := []config.PointConfig{}
			for _, point := range cfg.Points {
				if point.DeviceKey == device.DeviceKey {
					points = append(points, point)
				}
			}
			if len(points) == 0 {
				points = append(points, device.Points...)
				for i := range points {
					if points[i].Protocol == "" {
						points[i].Protocol = device.Protocol
					}
				}
			}
			metrics := topologyMetrics(points)
			devices = append(devices, topologyDevice{
				PointAccessVersion: 1,
				DeviceKey:          device.DeviceKey,
				Name:               device.Name,
				Protocol:           device.Protocol,
				Address:            device.Address,
				SlaveID:            device.SlaveID,
				CommonAddress:      device.CommonAddress,
				PointCount:         len(metrics),
				Metrics:            metrics,
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
				PointAccessVersion: 1,
				DeviceKey:          point.DeviceKey,
				Protocol:           point.Protocol,
				Address:            point.Address,
				SlaveID:            point.SlaveID,
				CommonAddress:      point.CommonAddress,
			}
			byKey[point.DeviceKey] = current
			order = append(order, point.DeviceKey)
		}
		current.Metrics = append(current.Metrics, topologyMetrics([]config.PointConfig{point})...)
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
				metrics = append(metrics, topologyMetric{Identifier: point.Metric, Name: point.Name, DataType: point.DataType, Unit: point.Unit, CanRead: true})
			}
		}
		devices = append(devices, topologyDevice{PointAccessVersion: 1, DeviceKey: group.GroupKey, Name: group.Name, Protocol: "edge-compute", Address: group.GroupKey, PointCount: len(metrics), Metrics: metrics})
	}
	return devices
}

func topologyMetrics(points []config.PointConfig) []topologyMetric {
	metrics := make([]topologyMetric, 0, len(points))
	for _, point := range points {
		if point.Metric == "" {
			continue
		}
		canRead, canWrite := gatewayruntime.PointAccess(point)
		metrics = append(metrics, topologyMetric{
			Identifier: point.Metric,
			Name:       point.Name,
			DataType:   point.DataType,
			Unit:       point.Unit,
			CanRead:    canRead,
			CanWrite:   canWrite,
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
	maxBytes := int64(cfg.HistoryStorage.MaxSizeMB) * 1024 * 1024
	if cfg.HistoryStorage.UsesLocalStorage() {
		path := history.Path(config.DefaultHistoryStoragePath)
		log.Printf("history storage enabled on local storage path=%s max=%dMB", path, cfg.HistoryStorage.MaxSizeMB)
		return guardedHistoryStore(path, maxBytes, nil, cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
	}
	device, ok := storage.Resolve(cfg.HistoryStorage.StoragePath, cfg.HistoryStorage.StorageUUID)
	if !ok {
		log.Printf("history storage disabled: removable storage %q is not mounted", cfg.HistoryStorage.StoragePath)
		return history.Disabled()
	}
	path := history.Path(device.MountPath)
	log.Printf("history storage enabled path=%s max=%dMB free=%dMB", path, cfg.HistoryStorage.MaxSizeMB, device.FreeBytes/1024/1024)
	return guardedHistoryStore(path, maxBytes, storage.Guard(device), cfg.HistoryStorage.RetentionDays, cfg.HistoryStorage.AggregationEnabled)
}

func guardedHistoryStore(path string, maxBytes int64, available func() bool, retentionDays int, aggregationEnabled bool) *history.Store {
	const minHistoryFreeBytes = 4 * 1024 * 1024
	store := history.NewGuarded(path, maxBytes, available)
	return store.SetRetentionDays(retentionDays).SetAggregationEnabled(aggregationEnabled).SetDiskGuard(func() (uint64, uint64, error) {
		total, free, err := storage.Space(path)
		if errors.Is(err, storage.ErrSpaceUnsupported) {
			return 0, ^uint64(0), nil
		}
		return total, free, err
	}, minHistoryFreeBytes)
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

	if cfg.CloudConnectionIsEnabled() && cfg.Activation.IsReady() && cfg.Activation.Broker != "" {
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
			// Register before connect so subscriptions also recover if the first
			// connection attempt fails and Paho reconnects in the background.
			_ = client.SubscribePointCommands(a.applyPointCommand)
			_ = client.SubscribeOTA(a.applyOTA, otaResultPath)
			_ = client.SubscribeMaintenance(a.applyMaintenance, a.configPath+".maintenance-result.json")
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
			// Manual MQTT channels are telemetry integrations. Publishing the
			// complete gateway configuration here can be several megabytes and
			// blocks the Paho writer before realtime values are sent. Remote
			// configuration remains exclusive to the activation/platform
			// channel above.
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
	a.remoteConfigMu.Lock()
	defer a.remoteConfigMu.Unlock()
	if command.Version > 0 {
		if applied := a.lastRemoteConfigVersion(); command.Version <= applied {
			return cloud.RemoteConfigResult{Status: "FAILED", Message: fmt.Sprintf("remote config version %d is not newer than applied version %d", command.Version, applied)}
		}
	}
	current := a.runtime.Config()
	desired, err := mergeRemoteConfig(current, command.Config)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "配置 JSON 无效: " + err.Error()}
	}
	raw, err := json.Marshal(desired)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "配置序列化失败: " + err.Error()}
	}
	validated, err := config.Parse(raw)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "配置校验失败: " + err.Error()}
	}
	if err := validated.ValidateNewGlobalIdentifierDuplicates(current); err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "remote config identifier validation failed: " + err.Error()}
	}
	if err := edgecompute.Validate(validated); err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "边缘计算配置无效: " + err.Error()}
	}
	oldRaw, err := os.ReadFile(a.configPath)
	if err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "读取当前配置失败: " + err.Error()}
	}
	if err := os.WriteFile(a.configPath+".remote.bak", oldRaw, 0600); err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "备份当前配置失败: " + err.Error()}
	}
	if err := config.Save(a.configPath, validated); err != nil {
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "保存远程配置失败: " + err.Error()}
	}
	if err := a.ApplyConfig(validated); err != nil {
		_ = os.WriteFile(a.configPath, oldRaw, 0600)
		_ = a.ApplyConfig(current)
		return cloud.RemoteConfigResult{Status: "FAILED", Message: "apply remote config failed and previous config was restored: " + err.Error()}
	}
	// The reloaded collection workers will sample on their own schedules. A
	// synchronous full-device collection here used to delay the MQTT reply for
	// minutes when any downstream device was offline, causing an applied task to
	// be shown as timed out by the cloud platform.
	if command.Version > 0 {
		if err := os.WriteFile(a.configPath+".remote.version", []byte(strconv.Itoa(command.Version)+"\n"), 0600); err != nil {
			a.state.AddError("persist remote config version failed: " + err.Error())
		}
	}
	log.Printf("remote config applied task=%s version=%d", command.TaskID, command.Version)
	return cloud.RemoteConfigResult{Status: "APPLIED", Message: "配置已校验、备份并热加载"}
}

func (a *App) lastRemoteConfigVersion() int {
	raw, err := os.ReadFile(a.configPath + ".remote.version")
	if err != nil {
		return 0
	}
	value, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	return value
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
	for _, key := range []string{"gatewayKey", "activation", "mqtt", "remoteAccess", "web", "networkPorts", "wifi", "offlineCache", "historyStorage"} {
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
	channels := append([]config.ChannelConfig(nil), cfg.Channels...)
	for index := range channels {
		// Devices are returned through the top-level collection/forward arrays.
		// Keeping the channel copy structural avoids duplicate editable devices.
		channels[index].Devices = nil
	}
	devices := append([]config.DeviceConfig(nil), cfg.Devices...)
	for index := range devices {
		devices[index].Points = append([]config.PointConfig(nil), devices[index].Points...)
		// The persisted project may keep the collection period on the channel
		// rather than on each device. The runtime flattens that hierarchy onto
		// points before scheduling, so report the same effective period here.
		// Otherwise the cloud editor falls back to the global period and shows a
		// value that does not match the device's actual collection schedule.
		if interval := effectiveDeviceCollectionIntervalMilliseconds(cfg, devices[index]); interval > 0 {
			devices[index].CollectIntervalMilliseconds = interval
			devices[index].CollectIntervalSeconds = 0
		}
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
		"pointWriteVersion":           1,
		"writablePoints":              a.runtime.WritablePoints(),
		"pointAccessVersion":          1,
		"pointAccesses":               a.runtime.PointAccesses(),
		"collectIntervalSeconds":      cfg.CollectIntervalSeconds,
		"collectIntervalMilliseconds": cfg.CollectIntervalMilliseconds,
		"resources":                   cfg.Resources,
		"channels":                    channels,
		"serialPorts":                 cfg.SerialPorts,
		"devices":                     devices,
		"forwardDevices":              cfg.ForwardDevices,
		"forwardSlave":                cfg.ForwardSlave,
	}
}

func effectiveDeviceCollectionIntervalMilliseconds(cfg config.Config, device config.DeviceConfig) int {
	interval := 0
	for _, point := range cfg.Points {
		if point.DeviceKey != device.DeviceKey {
			continue
		}
		if device.ChannelKey != "" && point.ChannelKey != "" && point.ChannelKey != device.ChannelKey {
			continue
		}
		candidate := collectionIntervalMilliseconds(point.CollectIntervalMilliseconds, point.CollectIntervalSeconds)
		if candidate > 0 && (interval == 0 || candidate < interval) {
			interval = candidate
		}
	}
	if interval > 0 {
		return interval
	}
	if interval = collectionIntervalMilliseconds(device.CollectIntervalMilliseconds, device.CollectIntervalSeconds); interval > 0 {
		return interval
	}
	for _, channel := range cfg.Channels {
		if channel.ChannelKey == device.ChannelKey {
			if interval = collectionIntervalMilliseconds(channel.CollectIntervalMilliseconds, channel.CollectIntervalSeconds); interval > 0 {
				return interval
			}
			break
		}
	}
	return collectionIntervalMilliseconds(cfg.CollectIntervalMilliseconds, cfg.CollectIntervalSeconds)
}

func collectionIntervalMilliseconds(milliseconds, seconds int) int {
	if milliseconds > 0 {
		return milliseconds
	}
	if seconds > 0 {
		return seconds * 1000
	}
	return 0
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
			if connected && a.manualPublishDegraded(name) {
				connected = false
			}
			a.cloudMu.Lock()
			defer a.cloudMu.Unlock()
			a.setCloudChannelConnectedLocked(name, connected)
		}()
	}
}

func (a *App) manualPublishReady(name string, now time.Time, interval time.Duration) bool {
	a.publishBackoffMu.Lock()
	defer a.publishBackoffMu.Unlock()
	backoff := a.publishBackoff[name]
	if backoff == nil {
		if interval <= 0 {
			interval = time.Second
		}
		// Allow all collection lanes to contribute to the initial snapshot.
		// Publishing immediately on the first lane is the common cause of a
		// custom JSON payload containing only part of a multi-lane device.
		backoff = &publishBackoff{nextAt: now.Add(interval)}
		a.publishBackoff[name] = backoff
		return false
	}
	if !backoff.Ready(now) {
		return false
	}
	if interval <= 0 {
		interval = time.Second
	}
	// Reserve the next slot before starting the network operation so a slow
	// broker cannot create overlapping goroutines for the same channel.
	backoff.nextAt = now.Add(interval)
	return true
}

func (a *App) manualPublishFailed(name string, now time.Time) time.Duration {
	a.publishBackoffMu.Lock()
	defer a.publishBackoffMu.Unlock()
	backoff := a.publishBackoff[name]
	if backoff == nil {
		backoff = &publishBackoff{}
		a.publishBackoff[name] = backoff
	}
	return backoff.Fail(now)
}

func (a *App) manualPublishSucceeded(name string) {
	a.publishBackoffMu.Lock()
	defer a.publishBackoffMu.Unlock()
	if backoff := a.publishBackoff[name]; backoff != nil {
		backoff.Success()
	}
}

func (a *App) manualPublishDegraded(name string) bool {
	a.publishBackoffMu.Lock()
	defer a.publishBackoffMu.Unlock()
	backoff := a.publishBackoff[name]
	return backoff != nil && backoff.failures > 0
}

func (a *App) setCloudChannelConnectedLocked(name string, connected bool) {
	a.cloudState[name] = connected
	a.state.SetMQTTChannelConnected(name, connected)
}

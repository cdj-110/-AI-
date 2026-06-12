package app

import (
	"context"
	"log"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cache"
	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
	"weikong-iot-platform/apps/gateway-go/internal/web"
)

type App struct {
	cfg        config.Config
	configPath string
	cloudMu    sync.Mutex
	cloud      *cloud.Client
	spool      cache.Spool
	state      *state.Store
	runtime    *gatewayruntime.Manager
	publishSem chan struct{}
}

func New(cfg config.Config, configPath string) *App {
	store := state.New(cfg)
	manager := gatewayruntime.NewManager(cfg, store)
	return &App{
		cfg:        cfg,
		configPath: configPath,
		spool:      cache.New(cfg.CacheFile),
		state:      store,
		runtime:    manager,
		publishSem: make(chan struct{}, 1),
	}
}

func (a *App) Run(ctx context.Context) error {
	go web.New(a.cfg.Web, a.state, a.runtime, a.configPath, a.ApplyConfig).Run(ctx)
	a.syncCloud(a.cfg)
	defer a.disconnectCloud()

	log.Printf("gateway %s started, points=%d", a.cfg.GatewayKey, len(a.cfg.Points))
	a.collectAndPublish(ctx)
	for {
		timer := time.NewTimer(a.runtime.CollectInterval())
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
			a.collectAndPublish(ctx)
		}
	}
}

func (a *App) ApplyConfig(cfg config.Config) {
	oldCfg := a.runtime.Config()
	a.runtime.UpdateConfig(cfg)
	a.cfg = cfg
	if !oldCfg.MQTT.Equal(cfg.MQTT) {
		a.syncCloud(cfg)
	}
}

func (a *App) collectAndPublish(ctx context.Context) {
	grouped := a.runtime.CollectOnce(ctx)
	if len(grouped) == 0 {
		return
	}
	select {
	case a.publishSem <- struct{}{}:
		go func() {
			defer func() { <-a.publishSem }()
			a.publishToCloud(grouped)
		}()
	default:
	}
}

func (a *App) publishToCloud(grouped map[string]map[string]interface{}) {
	if !a.runtime.Config().MQTT.IsEnabled() {
		return
	}
	a.cloudMu.Lock()
	defer a.cloudMu.Unlock()
	if a.cloud == nil || !a.cloud.IsConnected() {
		return
	}

	if err := a.cloud.PublishGatewayHeartbeat(); err != nil {
		log.Printf("publish gateway heartbeat failed: %v", err)
		a.state.AddError("publish gateway heartbeat failed: " + err.Error())
	}

	_ = a.spool.Drain(a.cloud.PublishTelemetry)
	for deviceKey, metrics := range grouped {
		reading := model.Reading{DeviceKey: deviceKey, Time: time.Now(), Metrics: metrics}
		if err := a.cloud.PublishChildHeartbeat(deviceKey); err != nil {
			log.Printf("publish child heartbeat failed device=%s: %v", deviceKey, err)
			a.state.AddError("publish child heartbeat failed " + deviceKey + ": " + err.Error())
		}
		if err := a.cloud.PublishTelemetry(reading); err != nil {
			log.Printf("publish telemetry failed device=%s: %v", deviceKey, err)
			a.state.AddError("publish telemetry failed " + deviceKey + ": " + err.Error())
			if cacheErr := a.spool.Append(reading); cacheErr != nil {
				log.Printf("append spool failed: %v", cacheErr)
				a.state.AddError("append spool failed: " + cacheErr.Error())
			}
		} else {
			a.state.MarkPublish()
		}
	}
}

func (a *App) syncCloud(cfg config.Config) {
	a.cloudMu.Lock()
	defer a.cloudMu.Unlock()
	if a.cloud != nil {
		a.cloud.Disconnect()
		a.cloud = nil
		a.state.SetMQTTConnected(false)
	}
	if !cfg.MQTT.IsEnabled() {
		log.Printf("mqtt disabled, cloud publish skipped")
		return
	}
	client := cloud.NewMQTT(cfg, a.state.SetMQTTConnected)
	a.cloud = client
	if err := client.Connect(); err != nil {
		log.Printf("mqtt connect failed: %v", err)
		a.state.SetMQTTConnected(false)
		a.state.AddError("mqtt connect failed: " + err.Error())
	}
}

func (a *App) disconnectCloud() {
	a.cloudMu.Lock()
	defer a.cloudMu.Unlock()
	if a.cloud != nil {
		a.cloud.Disconnect()
		a.cloud = nil
		a.state.SetMQTTConnected(false)
	}
}

package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/datamanager"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
	"weikong-iot-platform/apps/gateway-go/internal/goose"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

type Server struct {
	auth       *authManager
	cfg        config.ListenerConfig
	store      *state.Store
	runtime    *gatewayruntime.Manager
	configPath string
	onConfig   func(config.Config) error
	data       *datamanager.Manager
	audit      *auditStore
	ai         *aiManager
	gooseSniff *goose.Sniffer
	configMu   sync.Mutex
}

func New(cfg config.ListenerConfig, store *state.Store, runtime *gatewayruntime.Manager, configPath string, onConfig func(config.Config) error) *Server {
	return &Server{cfg: cfg, store: store, runtime: runtime, configPath: configPath, onConfig: onConfig, auth: newAuthManager(), data: datamanager.New(store), audit: newAuditStore(configPath), ai: newAIManager(configPath), gooseSniff: goose.NewSniffer()}
}

func (s *Server) Run(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}
	listen := s.cfg.Listen
	if listen == "" {
		listen = "0.0.0.0:8088"
	}
	s.data.Start(ctx)
	s.ai.start(ctx)

	server := &http.Server{
		Addr:              listen,
		Handler:           s.securityHeaders(s.auditMiddleware(s.routes())),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	scheme := "http"
	serve := server.ListenAndServe
	if s.cfg.TLSCertFile != "" && s.cfg.TLSKeyFile != "" {
		scheme = "https"
		serve = func() error { return server.ListenAndServeTLS(s.cfg.TLSCertFile, s.cfg.TLSKeyFile) }
	}
	log.Printf("gateway web status listening on %s://%s", scheme, listen)
	if err := serve(); err != nil && err != http.ErrServerClosed {
		log.Printf("gateway web server stopped: %v", err)
	}
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if request.TLS != nil {
			writer.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) routes() http.Handler {
	protected := http.NewServeMux()
	protected.HandleFunc("/ui-v2/", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
	})
	protected.HandleFunc("/packet-monitor", s.requirePermission("config.manage", s.packetMonitorPage))
	protected.HandleFunc("/api/packets", s.requirePermission("config.manage", s.packetFrames))
	protected.HandleFunc("/api/packets/stream", s.requirePermission("config.manage", s.packetFrameStream))
	protected.HandleFunc("/api/goose/sniff", s.requirePermission("config.manage", s.gooseSniffSession))
	protected.HandleFunc("/api/config", s.configFile)
	protected.HandleFunc("/api/config/bootstrap", s.requirePermission("config.view", s.configBootstrap))
	protected.HandleFunc("/api/config/points", s.requirePermission("config.view", s.configPoints))
	protected.HandleFunc("/api/config/point-mutations", s.requirePermission("config.manage", s.mutateConfigPoints))
	protected.HandleFunc("/api/config/compact", s.requirePermission("config.manage", s.saveCompactConfig))
	protected.HandleFunc("/api/project/export", s.projectFile)
	protected.HandleFunc("/api/project/import", s.projectFile)
	protected.HandleFunc("/api/activation", s.requirePermission("cloud.manage", s.activationFile))
	protected.HandleFunc("/api/collect-now", s.requirePermission("config.manage", s.collectNow))
	protected.HandleFunc("/api/points/write", s.requirePermission("config.manage", s.writePoint))
	protected.HandleFunc("/api/edge-compute/validate", s.requirePermission("config.manage", s.validateEdgeCompute))
	protected.HandleFunc("/api/mqtt/test-publish", s.requirePermission("cloud.manage", s.testMQTTPublish))
	protected.HandleFunc("/api/iec61850/cid/preview", s.requirePermission("config.manage", s.previewIEC61850CID))
	protected.HandleFunc("/api/s7-points/scan", s.requirePermission("config.manage", s.scanS7Points))
	protected.HandleFunc("/api/s7-connection/test", s.requirePermission("config.manage", s.testS7Connection))
	protected.HandleFunc("/api/connection/test", s.requirePermission("config.manage", s.testConnection))
	protected.HandleFunc("/api/opcua/browse", s.requirePermission("config.manage", s.browseOPCUA))
	protected.HandleFunc("/api/ai/settings", s.aiSettingsFile)
	protected.HandleFunc("/api/ai/settings/test", s.requirePermission("ai.manage", s.testAISettings))
	protected.HandleFunc("/api/ai/chat", s.requirePermission("ai.use", s.aiChat))
	protected.HandleFunc("/api/ai/chat-actions/", s.requirePermission("ai.use", s.applyAIChatAction))
	protected.HandleFunc("/api/ai/config-jobs", s.requirePermission("ai.use", s.aiConfigJobs))
	protected.HandleFunc("/api/ai/config-jobs/", s.requirePermission("ai.use", s.aiJobItem))
	protected.HandleFunc("/api/ai/config-drafts/", s.requirePermission("ai.use", s.applyAIDraft))
	s.registerOptionalRoutes(protected)
	protected.HandleFunc("/api/session", s.currentUserFile)
	protected.HandleFunc("/api/status", s.requirePermission("status.view", s.status))
	protected.HandleFunc("/api/point-status", s.requirePermission("status.view", s.pointStatus))
	protected.HandleFunc("/api/network/interfaces", s.requirePermission("network.manage", s.networkInterfaces))
	protected.HandleFunc("/api/network/apply", s.requirePermission("network.manage", s.applyNetworkInterface))
	protected.HandleFunc("/api/network/wifi", s.requirePermission("network.manage", s.wifiNetwork))
	protected.HandleFunc("/api/network/wifi/boot", s.requirePermission("network.manage", s.wifiBoot))
	protected.HandleFunc("/api/network/wifi/scan", s.requirePermission("network.manage", s.scanWiFiNetwork))
	protected.HandleFunc("/api/network/cellular", s.requirePermission("network.manage", s.cellularNetwork))
	protected.HandleFunc("/api/storage", s.requirePermission("config.manage", s.storageStatus))
	protected.HandleFunc("/api/history-storage", s.requirePermission("config.manage", s.historyStorageStatus))
	protected.HandleFunc("/api/history-storage/export", s.requirePermission("config.manage", s.exportHistoryStorage))
	protected.HandleFunc("/api/maintenance/ping", s.requirePermission("maintenance.run", s.pingDiagnostic))
	protected.HandleFunc("/api/maintenance/time", s.requirePermission("maintenance.run", s.timeSync))
	protected.HandleFunc("/api/maintenance/restart", s.requirePermission("maintenance.run", s.restartService))
	protected.HandleFunc("/api/maintenance/reboot", s.requirePermission("maintenance.run", s.rebootGateway))
	protected.HandleFunc("/api/maintenance/factory-reset", s.requirePermission("maintenance.run", s.factoryReset))
	protected.HandleFunc("/api/maintenance/audit-log", s.requirePermission("maintenance.run", s.auditLog))
	protected.HandleFunc("/api/security", s.securityFile)
	protected.HandleFunc("/api/ws", s.webSocket)
	protected.Handle("/", vueFrontendHandler())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", s.health)
	mux.HandleFunc("/brand-logo.png", serveBrandLogo)
	mux.HandleFunc("/login", s.login)
	mux.HandleFunc("/logout", s.logout)
	mux.Handle("/", s.requireAuth(protected))
	return mux
}

func (s *Server) validateEdgeCompute(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.store == nil {
		http.Error(writer, "gateway state is unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Content  string `json:"content"`
		GroupKey string `json:"groupKey"`
		Metric   string `json:"metric"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 32*1024*1024))
	if err := decoder.Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := config.Parse([]byte(body.Content))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	value, inputs, err := edgecompute.Preview(cfg, s.store, strings.TrimSpace(body.GroupKey), strings.TrimSpace(body.Metric))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "value": value, "inputs": inputs})
}

func (s *Server) requirePermission(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !s.hasPermission(request, permission) {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		next(writer, request)
	}
}

func (s *Server) status(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if request.URL.Query().Get("compact") == "1" {
		_ = json.NewEncoder(writer).Encode(s.store.CompactSnapshot())
		return
	}
	_ = json.NewEncoder(writer).Encode(s.store.Snapshot())
}

func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isLoopbackRequest(request) && !s.hasPermission(request, "status.view") {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	health := s.store.Health(20 * time.Second)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if !health.Healthy {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(writer).Encode(health)
}

func (s *Server) pointStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requestedMetrics := request.URL.Query()["metric"]
	requestedDevices := request.URL.Query()["deviceKey"]
	if len(requestedMetrics) > 0 && len(requestedDevices) == 1 {
		deviceKey := strings.TrimSpace(requestedDevices[0])
		metrics := make([]string, 0, len(requestedMetrics))
		for _, metric := range requestedMetrics {
			if metric = strings.TrimSpace(metric); metric != "" {
				metrics = append(metrics, metric)
			}
		}
		points := s.store.PointStatusesByMetric(deviceKey, metrics)
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(points)
		return
	}
	points := s.store.PointStatuses()
	if len(requestedDevices) > 0 {
		allowed := make(map[string]struct{}, len(requestedDevices))
		for _, deviceKey := range requestedDevices {
			if deviceKey = strings.TrimSpace(deviceKey); deviceKey != "" {
				allowed[deviceKey] = struct{}{}
			}
		}
		filtered := make([]state.PointStatus, 0, len(points))
		for _, point := range points {
			if _, ok := allowed[point.DeviceKey]; ok {
				filtered = append(filtered, point)
			}
		}
		points = filtered
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(points)
}

func (s *Server) configFile(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		if !s.hasPermission(request, "config.view") && !s.hasPermission(request, "config.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		s.getConfig(writer)
	case http.MethodPut:
		if !s.hasPermission(request, "config.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		s.saveConfig(writer, request)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfig(writer http.ResponseWriter) {
	revision, revisionErr := configFileRevision(s.configPath)
	if revisionErr != nil {
		http.Error(writer, revisionErr.Error(), http.StatusInternalServerError)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	raw, err := json.MarshalIndent(config.RedactSecrets(cfg), "", "  ")
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("ETag", quoteETag(revision))
	_ = json.NewEncoder(writer).Encode(map[string]string{"content": string(raw)})
}

func (s *Server) saveConfig(writer http.ResponseWriter, request *http.Request) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if expected := strings.TrimSpace(request.Header.Get("If-Match")); expected != "" && expected != "*" {
		current, err := configFileRevision(s.configPath)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if strings.Trim(expected, "\"") != current {
			http.Error(writer, "配置已被其他会话修改，请重新加载后再保存", http.StatusConflict)
			return
		}
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 128*1024*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	configRaw := raw
	if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/vnd.weikong.config+json") {
		var body struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		configRaw = []byte(body.Content)
	}
	cfg, err := config.Parse(configRaw)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	previous, previousErr := config.Load(s.configPath)
	if previousErr == nil {
		config.PreserveMaskedSecrets(previous, &cfg)
		cfg.ApplyDefaults()
		if err := cfg.ValidateNewGlobalIdentifierDuplicates(previous); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err := edgecompute.Validate(cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.onConfig != nil {
		if err := s.onConfig(cfg); err != nil {
			if previousErr == nil {
				_ = config.Save(s.configPath, previous)
				_ = s.onConfig(previous)
			}
			http.Error(writer, "apply config failed and previous configuration was restored: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		s.runtime.UpdateConfig(cfg)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if revision, err := configFileRevision(s.configPath); err == nil {
		writer.Header().Set("ETag", quoteETag(revision))
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "restartRequired": false})
}

func configFileRevision(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func quoteETag(value string) string { return "\"" + value + "\"" }

func isLoopbackRequest(request *http.Request) bool {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}

func (s *Server) collectNow(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	collectCtx, cancel := context.WithTimeout(request.Context(), 20*time.Second)
	defer cancel()
	s.runtime.CollectOnce(collectCtx)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(s.store.Snapshot())
}

func (s *Server) writePoint(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.runtime == nil {
		http.Error(writer, "gateway runtime is unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		DeviceKey string      `json:"deviceKey"`
		Metric    string      `json:"metric"`
		Value     interface{} `json:"value"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	body.DeviceKey = strings.TrimSpace(body.DeviceKey)
	body.Metric = strings.TrimSpace(body.Metric)
	if body.DeviceKey == "" || body.Metric == "" || body.Value == nil {
		http.Error(writer, "deviceKey, metric and value are required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	point, err := s.runtime.WritePoint(ctx, body.DeviceKey, body.Metric, body.Value)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, gatewayruntime.ErrWritablePointNotFound) {
			status = http.StatusBadRequest
		}
		http.Error(writer, err.Error(), status)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok": true, "deviceKey": point.DeviceKey, "metric": point.Metric,
		"value": body.Value, "readFunction": point.Function,
	})
}

func (s *Server) activationFile(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.getActivation(writer)
	case http.MethodPut:
		s.saveActivation(writer, request)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getActivation(writer http.ResponseWriter) {
	raw, err := os.ReadFile(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	var cfg config.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	activationRaw, err := os.ReadFile(cfg.Activation.File)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(map[string]string{"content": "{}"})
		return
	}
	var activation config.ActivationConfig
	if json.Unmarshal(activationRaw, &activation) == nil && activation.DeviceSecret != "" {
		activation.DeviceSecret = "***"
		if masked, marshalErr := json.MarshalIndent(activation, "", "  "); marshalErr == nil {
			activationRaw = masked
		}
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]string{"content": string(activationRaw)})
}

func (s *Server) saveActivation(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 1024*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var activation config.ActivationConfig
	if err := json.Unmarshal([]byte(body.Content), &activation); err != nil {
		http.Error(writer, "parse activation file failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := s.readRawConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	activationPath := body.File
	if activationPath == "" {
		activationPath = cfg.Activation.File
	}
	if activationPath == "" {
		activationPath = "activation.local.json"
	}
	if activation.DeviceSecret == "***" {
		if previousRaw, readErr := os.ReadFile(activationPath); readErr == nil {
			var previous config.ActivationConfig
			if json.Unmarshal(previousRaw, &previous) == nil {
				activation.DeviceSecret = previous.DeviceSecret
			}
		}
	}
	identity := hardware.ReadIdentity()
	if activation.IsEnabled() && activation.HardwareID == "" {
		activation.HardwareID = identity.ID
	}
	if activation.IsEnabled() && (activation.HardwareID == "" || activation.SN == "" || activation.DeviceSecret == "" || activation.Broker == "") {
		http.Error(writer, "activation requires hardwareId, sn, deviceSecret and broker", http.StatusBadRequest)
		return
	}
	if activation.IsEnabled() && identity.Available && !strings.EqualFold(activation.HardwareID, identity.ID) {
		http.Error(writer, "activation hardwareId does not match this gateway", http.StatusBadRequest)
		return
	}
	activationRaw, err := json.MarshalIndent(activation, "", "  ")
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(activationPath, append(activationRaw, '\n'), 0600); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg.Activation = config.ActivationConfig{Enabled: activation.Enabled, File: activationPath}
	if err := config.Save(s.configPath, cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	activatedCfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if s.store != nil {
		s.store.ResetMQTTChannels(activatedCfg)
	}
	if s.onConfig != nil {
		go func() {
			if err := s.onConfig(activatedCfg); err != nil && s.store != nil {
				s.store.AddError("apply activation config failed: " + err.Error())
			}
		}()
	} else {
		s.runtime.UpdateConfig(activatedCfg)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "file": activationPath, "identity": identity})
}

func (s *Server) readRawConfig() (config.Config, error) {
	raw, err := os.ReadFile(s.configPath)
	if err != nil {
		return config.Config{}, err
	}
	var cfg config.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func (s *Server) testMQTTPublish(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var channel config.MQTTConfig
	for _, current := range cfg.ManualMQTTChannels() {
		if current.IsEnabled() {
			channel = current
			break
		}
	}
	if !channel.IsEnabled() {
		http.Error(writer, "manual mqtt channel is disabled", http.StatusBadRequest)
		return
	}
	channel.ClientID = channel.ClientID + "_test_" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	client := cloud.NewManualMQTTChannel("manual-test", cfg.GatewayKey, channel, nil)
	if err := client.Connect(); err != nil {
		http.Error(writer, "mqtt connect failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer client.Disconnect()

	grouped := latestPointGroups(s.store.Snapshot())
	if len(grouped) == 0 {
		grouped = map[string]map[string]interface{}{"test": map[string]interface{}{"switch": 0}}
	}
	if err := client.PublishManual(grouped); err != nil {
		http.Error(writer, "mqtt publish failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "topic": client.RenderManualTopic(), "groups": grouped})
}

func latestPointGroups(snapshot state.Snapshot) map[string]map[string]interface{} {
	grouped := map[string]map[string]interface{}{}
	for _, point := range snapshot.Points {
		if point.Metric == "" || point.Value == nil {
			continue
		}
		if grouped[point.DeviceKey] == nil {
			grouped[point.DeviceKey] = map[string]interface{}{}
		}
		grouped[point.DeviceKey][point.Metric] = point.Value
	}
	return grouped
}
func (s *Server) packetMonitorPage(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write([]byte(renderPacketMonitorHTML()))
}

func (s *Server) packetFrames(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"frames": packetmonitor.Default.Snapshot(request.URL.Query().Get("deviceKey"), request.URL.Query().Get("protocol"), limit),
	})
}

func (s *Server) packetFrameStream(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	deviceKey := strings.TrimSpace(request.URL.Query().Get("deviceKey"))
	protocol := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("protocol")))
	updates, unsubscribe := packetmonitor.Default.Subscribe(256)
	defer unsubscribe()
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Accel-Buffering", "no")
	_, _ = io.WriteString(writer, ": connected\n\n")
	flusher.Flush()
	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case frame, open := <-updates:
			if !open {
				return
			}
			if deviceKey != "" && frame.DeviceKey != deviceKey {
				continue
			}
			if protocol != "" && frame.Protocol != protocol {
				continue
			}
			payload, _ := json.Marshal(frame)
			if _, err := fmt.Fprintf(writer, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		case <-keepAlive.C:
			if _, err := io.WriteString(writer, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-request.Context().Done():
			return
		}
	}
}

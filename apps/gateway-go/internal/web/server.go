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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/collectdiag"
	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/datamanager"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
	"weikong-iot-platform/apps/gateway-go/internal/goose"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
	"weikong-iot-platform/apps/gateway-go/internal/history"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
	"weikong-iot-platform/apps/gateway-go/internal/valueformat"
)

type Server struct {
	remoteStorage        remoteStorageRuntime
	historyWriterStatus  func() history.WriterStatus
	auth                 *authManager
	cfg                  config.ListenerConfig
	store                *state.Store
	runtime              *gatewayruntime.Manager
	configPath           string
	onConfig             func(config.Config) error
	data                 *datamanager.Manager
	audit                *auditStore
	ai                   *aiManager
	gooseSniff           *goose.Sniffer
	alarms               *alarm.Manager
	collectionIncidents  *collectdiag.Store
	exportJobs           *historyExportManager
	configMu             sync.Mutex
	historyMaintenanceMu sync.Mutex
	bootID               string
}

func New(cfg config.ListenerConfig, store *state.Store, runtime *gatewayruntime.Manager, configPath string, onConfig func(config.Config) error, alarmManagers ...*alarm.Manager) *Server {
	var alarms *alarm.Manager
	if len(alarmManagers) > 0 {
		alarms = alarmManagers[0]
	}
	return &Server{cfg: cfg, store: store, runtime: runtime, configPath: configPath, onConfig: onConfig, alarms: alarms, exportJobs: newHistoryExportManager(), auth: newAuthManager(), data: datamanager.New(store), audit: newAuditStore(configPath), ai: newAIManager(configPath), gooseSniff: goose.NewSniffer(), bootID: newBootID()}
}

func (s *Server) SetCollectionIncidents(store *collectdiag.Store) {
	s.collectionIncidents = store
}

// Set before Run. The callback reads the active writer, not a temporary query connection.
func (s *Server) SetHistoryWriterStatus(status func() history.WriterStatus) {
	s.historyWriterStatus = status
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
	go s.historyMaintenanceLoop(ctx)

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
	protected.HandleFunc("/packet-monitor", s.requirePermission("packet.view", s.packetMonitorPage))
	protected.HandleFunc("/api/packets", s.requirePermission("packet.view", s.packetFrames))
	protected.HandleFunc("/api/packets/stream", s.requirePermission("packet.view", s.packetFrameStream))
	protected.HandleFunc("/api/goose/sniff", s.requirePermission("packet.view", s.gooseSniffSession))
	protected.HandleFunc("/api/config", s.configFile)
	protected.HandleFunc("/api/status/overview", s.collectionOverview)
	protected.HandleFunc("/api/config/bootstrap", s.requirePermission("config.view", s.configBootstrap))
	protected.HandleFunc("/api/config/points", s.requirePermission("config.view", s.configPoints))
	protected.HandleFunc("/api/config/point-mutations", s.requirePermission("config.manage", s.mutateConfigPoints))
	protected.HandleFunc("/api/config/compact", s.requirePermission("config.manage", s.saveCompactConfig))
	protected.HandleFunc("/api/project/export", s.projectFile)
	protected.HandleFunc("/api/project/import", s.projectFile)
	protected.HandleFunc("/api/activation", s.requirePermission("cloud.manage", s.activationFile))
	protected.HandleFunc("/api/license/details", s.licenseDetails)
	protected.HandleFunc("/api/collect-now", s.requirePermission("collection.run", s.collectNow))
	protected.HandleFunc("/api/points/writable", s.requirePermission("points.write", s.writablePoints))
	protected.HandleFunc("/api/points/write", s.requirePermission("points.write", s.writePoint))
	protected.HandleFunc("/api/edge-compute/validate", s.requirePermission("edge.manage", s.validateEdgeCompute))
	protected.HandleFunc("/api/edge-compute/config", s.saveEdgeConfig)
	protected.HandleFunc("/api/alarms", s.requirePermission("alarms.view", s.alarmEvents))
	protected.HandleFunc("/api/alarms/history", s.requirePermission("alarms.view", s.alarmHistory))
	protected.HandleFunc("/api/alarms/acknowledge", s.requirePermission("alarms.manage", s.acknowledgeAlarm))
	protected.HandleFunc("/api/alarms/manual-recover", s.requirePermission("alarms.manage", s.manualRecoverAlarm))
	protected.HandleFunc("/api/alarms/settings", s.alarmSettings)
	protected.HandleFunc("/api/alarm-rules", s.alarmRules)
	protected.HandleFunc("/api/mqtt/test-publish", s.requirePermission("cloud.manage", s.testMQTTPublish))
	protected.HandleFunc("/api/iec61850/cid/preview", s.requirePermission("collection.run", s.previewIEC61850CID))
	protected.HandleFunc("/api/s7-points/scan", s.requirePermission("collection.run", s.scanS7Points))
	protected.HandleFunc("/api/s7-connection/test", s.requirePermission("collection.run", s.testS7Connection))
	protected.HandleFunc("/api/connection/test", s.requirePermission("collection.run", s.testConnection))
	protected.HandleFunc("/api/dlt645/dictionary", s.requirePermission("config.view", s.dlt645Dictionary))
	protected.HandleFunc("/api/dlt645/read-address", s.requirePermission("config.manage", s.dlt645ReadAddress))
	protected.HandleFunc("/api/opcua/browse", s.requirePermission("collection.run", s.browseOPCUA))
	protected.HandleFunc("/api/opcda/browse", s.requirePermission("collection.run", s.browseOPCDA))
	protected.HandleFunc("/api/ai/settings", s.aiSettingsFile)
	protected.HandleFunc("/api/ai/settings/test", s.requirePermission("ai.manage", s.testAISettings))
	protected.HandleFunc("/api/ai/chat", s.requirePermission("ai.use", s.aiChat))
	protected.HandleFunc("/api/ai/chat-actions/", s.requirePermission("ai.use", s.applyAIChatAction))
	protected.HandleFunc("/api/ai/config-jobs", s.requirePermission("ai.use", s.aiConfigJobs))
	protected.HandleFunc("/api/ai/config-jobs/", s.requirePermission("ai.use", s.aiJobItem))
	protected.HandleFunc("/api/ai/config-drafts/", s.requirePermission("ai.use", s.applyAIDraft))
	s.registerOptionalRoutes(protected)
	protected.HandleFunc("/api/session", s.currentUserFile)
	protected.HandleFunc("/api/session/activity", s.sessionActivity)
	protected.HandleFunc("/api/system/version", s.systemVersion)
	protected.HandleFunc("/api/status", s.requirePermission("status.view", s.status))
	protected.HandleFunc("/api/collection-incidents", s.requirePermission("collection_incidents.view", s.collectionIncidentHistory))
	protected.HandleFunc("/api/collection-incidents/clear", s.requirePermission("collection_incidents.manage", s.clearCollectionIncidents))
	protected.HandleFunc("/api/point-status", s.requirePermission("status.view", s.pointStatus))
	protected.HandleFunc("/api/network/interfaces", s.requirePermission("network.manage", s.networkInterfaces))
	protected.HandleFunc("/api/network/apply", s.requirePermission("network.manage", s.applyNetworkInterface))
	protected.HandleFunc("/api/network/wifi", s.requirePermission("network.manage", s.wifiNetwork))
	protected.HandleFunc("/api/network/wifi/boot", s.requirePermission("network.manage", s.wifiBoot))
	protected.HandleFunc("/api/network/wifi/scan", s.requirePermission("network.manage", s.scanWiFiNetwork))
	protected.HandleFunc("/api/network/cellular", s.requirePermission("network.manage", s.cellularNetwork))
	protected.HandleFunc("/api/storage", s.requirePermission("offline.manage", s.storageStatus))
	protected.HandleFunc("/api/history-storage", s.historyStorageStatus)
	protected.HandleFunc("/api/remote-storage", s.remoteStorageConfig)
	protected.HandleFunc("/api/remote-storage/status", s.requirePermission("history.view", s.remoteStorageStatus))
	protected.HandleFunc("/api/remote-storage/test", s.requirePermission("history.manage", s.testRemoteStorage))
	protected.HandleFunc("/api/history-storage/points", s.requirePermission("history.view", s.historyStoragePoints))
	protected.HandleFunc("/api/history-storage/query", s.requirePermission("history.view", s.queryHistoryStorage))
	protected.HandleFunc("/api/history-storage/migrate", s.requirePermission("history.manage", s.migrateHistoryStorage))
	protected.HandleFunc("/api/history-storage/exports", s.historyStorageExports)
	protected.HandleFunc("/api/history-storage/exports/download", s.requirePermission("history.view", s.downloadHistoryExport))
	protected.HandleFunc("/api/history-storage/export", s.requirePermission("history.manage", s.exportHistoryStorage))
	protected.HandleFunc("/api/history-storage/maintenance", s.requirePermission("history.manage", s.historyMaintenance))
	protected.HandleFunc("/api/history-storage/backups/download", s.requirePermission("history.manage", s.downloadHistoryBackup))
	protected.HandleFunc("/api/maintenance/ping", s.requirePermission("maintenance.diagnose", s.pingDiagnostic))
	protected.HandleFunc("/api/maintenance/diagnostic-bundle", s.requirePermission("maintenance.diagnose", s.diagnosticBundle))
	protected.HandleFunc("/api/maintenance/time", s.requirePermission("maintenance.time", s.timeSync))
	protected.HandleFunc("/api/maintenance/ssh", s.requirePermission("maintenance.remote", s.sshAccess))
	protected.HandleFunc("/api/maintenance/restart", s.requirePermission("maintenance.restart", s.restartService))
	protected.HandleFunc("/api/maintenance/reboot", s.requirePermission("maintenance.restart", s.rebootGateway))
	protected.HandleFunc("/api/maintenance/factory-reset", s.requirePermission("maintenance.factory_reset", s.factoryReset))
	protected.HandleFunc("/api/maintenance/audit-log", s.requirePermission("maintenance.audit", s.auditLog))
	protected.HandleFunc("/api/security", s.securityFile)
	protected.HandleFunc("/api/security/https", s.httpsSettings)
	protected.HandleFunc("/api/security/mfa", s.mfaStatus)
	protected.HandleFunc("/api/security/mfa/setup", s.mfaSetupBegin)
	protected.HandleFunc("/api/security/mfa/enable", s.mfaEnable)
	protected.HandleFunc("/api/security/mfa/disable", s.mfaDisable)
	protected.HandleFunc("/api/ws", s.webSocket)
	protected.Handle("/", vueFrontendHandler())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", s.health)
	mux.HandleFunc("/api/readyz", s.readiness)
	mux.HandleFunc("/api/license/status", s.licenseEntryStatus)
	mux.HandleFunc("/api/activation/initial", s.initialActivation)
	mux.HandleFunc("/brand-logo.png", serveBrandLogo)
	mux.HandleFunc("/favicon.ico", serveBrandLogo)
	mux.HandleFunc("/apple-touch-icon.png", serveBrandLogo)
	mux.HandleFunc("/login", s.login)
	mux.HandleFunc("/login/verify", s.loginMFA)
	mux.HandleFunc("/change-password", s.changePassword)
	mux.HandleFunc("/change-password/skip", s.skipPasswordChange)
	mux.HandleFunc("/logout", s.logout)
	secured := s.requireAuth(protected)
	frontend := vueFrontendHandler()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The restricted public entry never bypasses business or import guards.
		if s.store != nil && s.store.CompactSnapshot().License.Restricted &&
			(r.Method == http.MethodGet || r.Method == http.MethodHead) &&
			(r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/assets/")) {
			w.Header().Set("Cache-Control", "no-store")
			frontend.ServeHTTP(w, r)
			return
		}
		secured.ServeHTTP(w, r)
	}))
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
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "value": valueformat.Value(value), "inputs": valueformat.Value(inputs)})
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
	if previousErr != nil {
		http.Error(writer, "无法读取原配置，已取消保存", http.StatusInternalServerError)
		return
	}
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
	if err := s.authorizeConfigChanges(request, previous, cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusForbidden)
		return
	}
	var opcdaSessionDevices []string
	if previousErr == nil {
		opcdaSessionDevices = collector.OPCDASessionResetDevices(previous.Points, cfg.Points)
	}
	if err := s.saveAndApplyConfigLocked(cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if revision, err := configFileRevision(s.configPath); err == nil {
		writer.Header().Set("ETag", quoteETag(revision))
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok":                   true,
		"restartRequired":      false,
		"opcdaSessionReloaded": len(opcdaSessionDevices) > 0,
		"opcdaSessionDevices":  opcdaSessionDevices,
	})
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

func (s *Server) writablePoints(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.runtime == nil {
		http.Error(writer, "gateway runtime is unavailable", http.StatusServiceUnavailable)
		return
	}
	deviceKey := strings.TrimSpace(request.URL.Query().Get("deviceKey"))
	if deviceKey == "" {
		http.Error(writer, "deviceKey is required", http.StatusBadRequest)
		return
	}
	metrics := []string{}
	for _, point := range s.runtime.WritablePoints() {
		if point["deviceKey"] == deviceKey {
			metrics = append(metrics, point["metric"])
		}
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"metrics": metrics})
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
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(describePointWriteError(err))
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok": true, "deviceKey": point.DeviceKey, "metric": point.Metric,
		"value": body.Value, "readFunction": point.Function,
	})
}

var activationImportMutex sync.Mutex

func (s *Server) activationFile(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.getActivation(writer)
	case http.MethodPut:
		s.saveActivation(writer, request)
	case http.MethodPost:
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
	activationPath := s.activationPath(cfg.Activation.File)
	activationRaw, err := os.ReadFile(activationPath)
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
	s.saveActivationImport(writer, request, false)
}

func (s *Server) saveActivationImport(writer http.ResponseWriter, request *http.Request, initial bool) {
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
	// Do not let a slow anonymous upload hold the configuration/import locks.
	activationImportMutex.Lock()
	defer activationImportMutex.Unlock()
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := s.readRawConfig()
	if err != nil {
		if initial {
			http.Error(writer, "设备授权状态无法确认，请联系管理员", http.StatusServiceUnavailable)
			return
		}
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if initial {
		// Recheck under both import/config locks: a successful concurrent import
		// must close the anonymous entry before any subsequent write.
		if !s.initialActivationAllowed(cfg) {
			http.Error(writer, "设备已有授权记录，更换授权需要管理员验证", http.StatusForbidden)
			return
		}
		if body.File != "" || activation.File != "" || !activation.IsEnabled() || strings.TrimSpace(activation.Signature) == "" || activation.LicenseVersion < 2 || activation.LicenseSequence < 1 {
			http.Error(writer, "首次激活必须使用管理平台签发的完整授权文件，不允许指定文件路径", http.StatusBadRequest)
			return
		}
	}
	activationPath := body.File
	if activationPath == "" {
		activationPath = cfg.Activation.File
	}
	if activationPath == "" {
		activationPath = "activation.local.json"
	}
	activationPath = s.activationPath(activationPath)
	if activation.DeviceSecret == "***" {
		if previousRaw, readErr := os.ReadFile(activationPath); readErr == nil {
			var previous config.ActivationConfig
			if json.Unmarshal(previousRaw, &previous) == nil {
				activation.DeviceSecret = previous.DeviceSecret
			}
		}
	}
	identity := hardware.ReadIdentity()
	if activation.IsEnabled() && activation.HardwareID == "" && strings.TrimSpace(activation.Signature) == "" {
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
	if err := config.ValidateActivationPackage(activation, identity); err != nil {
		http.Error(writer, "activation validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if activation.Signature != "" {
		if err := config.CheckLicenseReplay(s.configPath, activation); err != nil {
			http.Error(writer, err.Error(), http.StatusConflict)
			return
		}
		candidate := cfg
		candidate.Activation = activation
		candidate.Activation.File = ""
		candidateRaw, marshalErr := json.Marshal(candidate)
		if marshalErr != nil {
			http.Error(writer, marshalErr.Error(), 500)
			return
		}
		checked, parseErr := config.Parse(candidateRaw)
		if parseErr != nil || !checked.License.Valid {
			http.Error(writer, fmt.Sprintf("authorization preflight failed: %v %s", parseErr, checked.License.Reason), 400)
			return
		}
	}
	if request.Method == http.MethodPost {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "sn": activation.SN, "keyId": activation.KeyID, "licenseSequence": activation.LicenseSequence, "expiresAt": activation.ExpiresAt})
		return
	}
	activationRaw, err := json.MarshalIndent(activation, "", "  ")
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	// Signed imports use immutable versioned files. Never overwrite a working
	// license, even if a caller provides the currently installed filename.
	if activation.Signature != "" {
		digest := sha256.Sum256(activationRaw)
		activationPath = filepath.Join(filepath.Dir(s.configPath), fmt.Sprintf("activation-%d-%x.json", activation.LicenseSequence, digest[:12]))
	}
	previousConfig, err := os.ReadFile(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), 500)
		return
	}
	if err := config.AtomicPrivateWrite(activationPath, append(activationRaw, '\n')); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg.Activation = config.ActivationConfig{Enabled: activation.Enabled, File: activationPath}
	if initial {
		if activation.HardwareProfile != nil {
			if err := config.ApplyInitialHardwareProfile(&cfg, activation.ProductModel, *activation.HardwareProfile); err != nil {
				_ = os.Remove(activationPath)
				http.Error(writer, "产品硬件配置初始化失败: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		// First signed activation opts into the platform connection, even if the
		// factory's unlicensed configuration explicitly disabled it. Replacement
		// imports must preserve an administrator's subsequent choice to stay offline.
		enabled := true
		cfg.CloudConnectionEnabled = &enabled
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	activatedCfg, err := config.Load(s.configPath)
	if err != nil || (activation.Signature != "" && !activatedCfg.License.Valid) {
		restoreErr := config.AtomicPrivateWrite(s.configPath, previousConfig)
		http.Error(writer, fmt.Sprintf("activation not applied; rollback attempted: %v %s (rollback error: %v)", err, activatedCfg.License.Reason, restoreErr), http.StatusBadRequest)
		return
	}
	signatureRequired, _ := strconv.ParseBool(strings.TrimSpace(config.ActivationSignatureRequired))
	if signatureRequired {
		if !activatedCfg.License.Valid {
			http.Error(writer, "activation remains restricted: "+activatedCfg.License.Reason, http.StatusBadRequest)
			return
		}
		if err := scheduleServiceRestart(); err != nil {
			http.Error(writer, "activation saved but restart scheduling failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok": true, "file": activationPath, "identity": identity, "restartRequired": true,
		})
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

// activationPath makes relative activation filenames independent from the
// process working directory. On EG100 the supervisor starts the process from
// /root, while the gateway configuration lives under /userdata/weikong/config.
func (s *Server) activationPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(filepath.Dir(s.configPath), filepath.Base(path))
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

package web
import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)
type Server struct {
	auth       *authManager
	cfg        config.ListenerConfig
	store      *state.Store
	runtime    *gatewayruntime.Manager
	configPath string
	onConfig   func(config.Config)
}

func New(cfg config.ListenerConfig, store *state.Store, runtime *gatewayruntime.Manager, configPath string, onConfig func(config.Config)) *Server {
	return &Server{cfg: cfg, store: store, runtime: runtime, configPath: configPath, onConfig: onConfig, auth: newAuthManager()}
}

func (s *Server) Run(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}
	listen := s.cfg.Listen
	if listen == "" {
		listen = "0.0.0.0:8088"
	}

	server := &http.Server{
		Addr:              listen,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("gateway web status listening on http://%s", listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("gateway web server stopped: %v", err)
	}
}

func (s *Server) routes() http.Handler {
	protected := http.NewServeMux()
	protected.HandleFunc("/", s.index)
	protected.HandleFunc("/api/status", s.status)
	protected.HandleFunc("/api/config", s.configFile)
	protected.HandleFunc("/api/activation", s.activationFile)
	protected.HandleFunc("/api/collect-now", s.collectNow)
	protected.HandleFunc("/api/mqtt/test-publish", s.testMQTTPublish)
	protected.HandleFunc("/api/pdf-points/preview", s.previewPDFPoints)
	protected.HandleFunc("/api/s7-points/scan", s.scanS7Points)
	protected.HandleFunc("/api/s7-connection/test", s.testS7Connection)
	protected.HandleFunc("/api/connection/test", s.testConnection)
	protected.HandleFunc("/api/opcua/browse", s.browseOPCUA)
	protected.HandleFunc("/api/network/interfaces", s.networkInterfaces)
	protected.HandleFunc("/api/network/apply", s.applyNetworkInterface)
	protected.HandleFunc("/api/storage", s.storageStatus)

	mux := http.NewServeMux()
	mux.HandleFunc("/login", s.login)
	mux.HandleFunc("/logout", s.logout)
	mux.Handle("/", s.requireAuth(protected))
	return mux
}
func (s *Server) status(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(s.store.Snapshot())
}

func (s *Server) configFile(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.getConfig(writer)
	case http.MethodPut:
		s.saveConfig(writer, request)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfig(writer http.ResponseWriter) {
	raw, err := os.ReadFile(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]string{"content": string(raw)})
}

func (s *Server) saveConfig(writer http.ResponseWriter, request *http.Request) {
	var body struct {
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
	cfg, err := config.Parse([]byte(body.Content))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.onConfig != nil {
		s.onConfig(cfg)
	} else {
		s.runtime.UpdateConfig(cfg)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "restartRequired": false})
}

func (s *Server) collectNow(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.runtime.CollectOnce(request.Context())
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(s.store.Snapshot())
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
	identity := hardware.ReadIdentity()
	if activation.HardwareID == "" {
		activation.HardwareID = identity.ID
	}
	if activation.HardwareID == "" || activation.SN == "" || activation.DeviceSecret == "" || activation.Broker == "" {
		http.Error(writer, "activation requires hardwareId, sn, deviceSecret and broker", http.StatusBadRequest)
		return
	}
	if identity.Available && !strings.EqualFold(activation.HardwareID, identity.ID) {
		http.Error(writer, "activation hardwareId does not match this gateway", http.StatusBadRequest)
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
	if s.onConfig != nil {
		go s.onConfig(activatedCfg)
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
	if !cfg.MQTT.IsEnabled() {
		http.Error(writer, "manual mqtt channel is disabled", http.StatusBadRequest)
		return
	}
	cfg.MQTT.ClientID = cfg.MQTT.ClientID + "_test_" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	client := cloud.NewManualMQTT(cfg, nil)
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
func (s *Server) index(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(indexHTML))
}

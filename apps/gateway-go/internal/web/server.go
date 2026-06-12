package web

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

type Server struct {
	cfg        config.ListenerConfig
	store      *state.Store
	runtime    *gatewayruntime.Manager
	configPath string
	onConfig   func(config.Config)
}

func New(cfg config.ListenerConfig, store *state.Store, runtime *gatewayruntime.Manager, configPath string, onConfig func(config.Config)) *Server {
	return &Server{cfg: cfg, store: store, runtime: runtime, configPath: configPath, onConfig: onConfig}
}

func (s *Server) Run(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}
	listen := s.cfg.Listen
	if listen == "" {
		listen = "0.0.0.0:8088"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/status", s.status)
	mux.HandleFunc("/api/config", s.configFile)
	mux.HandleFunc("/api/collect-now", s.collectNow)
	mux.HandleFunc("/api/pdf-points/preview", s.previewPDFPoints)
	mux.HandleFunc("/api/s7-points/scan", s.scanS7Points)
	mux.HandleFunc("/api/s7-connection/test", s.testS7Connection)
	mux.HandleFunc("/api/connection/test", s.testConnection)

	server := &http.Server{
		Addr:              listen,
		Handler:           mux,
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

func (s *Server) index(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(indexHTML))
}

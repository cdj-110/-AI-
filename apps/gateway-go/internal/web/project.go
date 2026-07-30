package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
)

const maxProjectImportBytes = 32 * 1024 * 1024

func (s *Server) projectFile(writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/project/export":
		if !s.hasPermission(request, "config.view") && !s.hasPermission(request, "config.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		s.exportProject(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/api/project/import":
		if !s.hasPermission(request, "config.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		s.importProject(writer, request)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) exportProject(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	project := config.RedactSecrets(cfg)
	raw, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	raw = append(raw, '\n')
	filename := fmt.Sprintf("weikong-project-%s-%s.json", safeProjectName(cfg.GatewayKey), time.Now().Format("20060102-150405"))
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write(raw)
}

func (s *Server) importProject(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := readProjectImport(writer, request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := config.Parse(raw)
	if err != nil {
		http.Error(writer, "parse project failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if previous, previousErr := config.Load(s.configPath); previousErr == nil {
		config.PreserveMaskedSecrets(previous, &cfg)
		cfg.ApplyDefaults()
		if err := cfg.ValidateNewGlobalIdentifierDuplicates(previous); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	} else if err := cfg.ValidateGlobalDeviceKeys(); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := edgecompute.Validate(cfg); err != nil {
		http.Error(writer, "validate edge computing failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.saveAndApplyConfig(cfg); err != nil {
		http.Error(writer, "apply project failed and previous configuration was restored: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok":              true,
		"gatewayKey":      cfg.GatewayKey,
		"resources":       len(cfg.Resources),
		"channels":        len(cfg.Channels),
		"devices":         len(cfg.Devices),
		"points":          len(cfg.FlattenPoints()),
		"restartRequired": false,
	})
}

func readProjectImport(writer http.ResponseWriter, request *http.Request) ([]byte, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxProjectImportBytes)
	contentType := request.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := request.ParseMultipartForm(maxProjectImportBytes); err != nil {
			return nil, err
		}
		for _, field := range []string{"file", "project"} {
			file, _, err := request.FormFile(field)
			if err == nil {
				defer file.Close()
				return io.ReadAll(file)
			}
		}
		return nil, fmt.Errorf("project file is required")
	}

	raw, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &body); err == nil && body.Content != "" {
		return []byte(body.Content), nil
	}
	return raw, nil
}

func (s *Server) applyConfig(cfg config.Config) error {
	if s.store != nil {
		s.store.ResetMQTTChannels(cfg)
	}
	if s.onConfig != nil {
		return s.onConfig(cfg)
	}
	if s.runtime != nil {
		s.runtime.UpdateConfig(cfg)
	}
	return nil
}

func (s *Server) saveAndApplyConfig(cfg config.Config) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	previous, previousErr := config.Load(s.configPath)
	if err := config.Save(s.configPath, cfg); err != nil {
		return err
	}
	if err := s.applyConfig(cfg); err != nil {
		if previousErr == nil {
			_ = config.Save(s.configPath, previous)
			_ = s.applyConfig(previous)
		}
		return err
	}
	return nil
}

func safeProjectName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "gateway"
	}
	var builder strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	clean := strings.Trim(builder.String(), "-_")
	if clean == "" {
		return "gateway"
	}
	return clean
}

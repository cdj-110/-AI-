package web

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestExportProjectDownloadsCurrentConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	raw := []byte(`{"gatewayKey":"gw-01","mqtt":{"enabled":false},"devices":[{"deviceKey":"dev-01","name":"Device 1","protocol":"modbus-tcp","address":"127.0.0.1:502","points":[]}]}`)
	if err := os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}

	server := &Server{configPath: configPath}
	response := httptest.NewRecorder()
	server.exportProject(response, httptest.NewRequest(http.MethodGet, "/api/project/export", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Header().Get("Content-Disposition"), "weikong-project-gw-01-") {
		t.Fatalf("unexpected content disposition: %q", response.Header().Get("Content-Disposition"))
	}
	if response.Body.String() != string(raw) {
		t.Fatalf("export body = %s, want %s", response.Body.String(), string(raw))
	}
}

func TestImportProjectSavesAndAppliesConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"gatewayKey":"old","mqtt":{"enabled":false}}`), 0600); err != nil {
		t.Fatal(err)
	}

	imported := `{"gatewayKey":"new","mqtt":{"enabled":false},"devices":[{"deviceKey":"dev-01","name":"Device 1","protocol":"modbus-tcp","address":"127.0.0.1:502","points":[{"name":"Temperature","metric":"temp","dataType":"float32","function":3,"register":0}]}]}`
	var applied config.Config
	server := &Server{
		configPath: configPath,
		onConfig: func(cfg config.Config) {
			applied = cfg
		},
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "project.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fileWriter.Write([]byte(imported)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/project/import", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.importProject(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if applied.GatewayKey != "new" || len(applied.Points) != 1 {
		t.Fatalf("applied config = %#v", applied)
	}
	savedRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved config.Config
	if err := json.Unmarshal(savedRaw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.GatewayKey != "new" || len(saved.Devices) != 1 {
		t.Fatalf("saved config = %#v", saved)
	}
}

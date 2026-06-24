package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
)

func TestSaveActivationDoesNotWaitForConfigApply(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	activationPath := filepath.Join(tempDir, "activation.json")
	if err := os.WriteFile(configPath, []byte(`{"gatewayKey":"test","mqtt":{"enabled":false}}`), 0600); err != nil {
		t.Fatal(err)
	}

	hardwareID := hardware.ReadIdentity().ID
	if hardwareID == "" {
		hardwareID = "TEST-HARDWARE"
	}
	activation := fmt.Sprintf(`{"enabled":true,"hardwareId":%q,"sn":"TEST-SN","deviceSecret":"secret","broker":"tcp://127.0.0.1:1883"}`, hardwareID)
	body, err := json.Marshal(map[string]string{"file": activationPath, "content": activation})
	if err != nil {
		t.Fatal(err)
	}

	applyBlocked := make(chan struct{})
	server := &Server{
		configPath: configPath,
		onConfig: func(config.Config) {
			<-applyBlocked
		},
	}
	request := httptest.NewRequest(http.MethodPut, "/api/activation", bytes.NewReader(body))
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.saveActivation(response, request)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		close(applyBlocked)
		t.Fatal("activation save waited for config apply")
	}
	close(applyBlocked)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

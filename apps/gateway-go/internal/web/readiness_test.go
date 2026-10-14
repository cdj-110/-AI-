package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestReadinessDoesNotExposeGatewayDetails(t *testing.T) {
	store := state.New(config.Config{GatewayKey: "secret-gateway"})
	server := &Server{store: store, bootID: "test-boot"}
	response := httptest.NewRecorder()
	server.readiness(response, httptest.NewRequest(http.MethodGet, "/api/readyz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body readinessResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Ready || body.BootID != "test-boot" || body.Phase != "ready" {
		t.Fatalf("unexpected readiness: %#v", body)
	}
	if response.Body.String() == "" || containsAny(response.Body.String(), "secret-gateway", "pointCount", "lastCollectAt") {
		t.Fatalf("readiness leaked internal details: %s", response.Body.String())
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

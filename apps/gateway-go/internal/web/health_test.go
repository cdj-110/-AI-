package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestHealthEndpointIsPublicAndHealthy(t *testing.T) {
	server := New(config.ListenerConfig{}, state.New(config.Config{}), nil, "", nil)
	request := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()
	server.routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("health endpoint status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

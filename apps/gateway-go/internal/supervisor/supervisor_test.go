package supervisor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestOptionsFromConfigUsesLoopbackHealthAddress(t *testing.T) {
	cases := map[string]string{
		"":             "http://127.0.0.1:8088/api/healthz",
		":18088":       "http://127.0.0.1:18088/api/healthz",
		"0.0.0.0:8088": "http://127.0.0.1:8088/api/healthz",
		"[::]:8088":    "http://127.0.0.1:8088/api/healthz",
	}
	for listen, expected := range cases {
		cfg := config.Config{Web: config.ListenerConfig{Enabled: true, Listen: listen}}
		if actual := OptionsFromConfig("gateway", "config.json", cfg).HealthURL; actual != expected {
			t.Fatalf("listen %q produced health URL %q, want %q", listen, actual, expected)
		}
	}
}

func TestAllowRestartLimitsAttemptsWithinWindow(t *testing.T) {
	now := time.Now()
	var restarts []time.Time
	for index := 0; index < 3; index++ {
		if !allowRestart(&restarts, now.Add(time.Duration(index)*time.Second), 3, time.Minute) {
			t.Fatalf("restart %d was unexpectedly rejected", index+1)
		}
	}
	if allowRestart(&restarts, now.Add(4*time.Second), 3, time.Minute) {
		t.Fatal("fourth restart inside the window should be rejected")
	}
	if !allowRestart(&restarts, now.Add(2*time.Minute), 3, time.Minute) {
		t.Fatal("restart after the window should be accepted")
	}
}

func TestCheckHealthRequiresHealthyOKResponse(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"healthy":true}`))
	}))
	defer healthy.Close()
	client := &http.Client{Timeout: time.Second}
	if !checkHealth(context.Background(), client, healthy.URL) {
		t.Fatal("healthy response was rejected")
	}

	unhealthy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"healthy":false}`))
	}))
	defer unhealthy.Close()
	if checkHealth(context.Background(), client, unhealthy.URL) {
		t.Fatal("unhealthy response was accepted")
	}
}

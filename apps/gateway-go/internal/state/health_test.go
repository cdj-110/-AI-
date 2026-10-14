package state

import (
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestHealthTracksApplicationProgress(t *testing.T) {
	store := New(config.Config{})
	if health := store.Health(time.Second); !health.Healthy {
		t.Fatalf("new store should be healthy: %#v", health)
	}
	store.mu.Lock()
	store.lastProgressAt = time.Now().Add(-2 * time.Second)
	store.mu.Unlock()
	if health := store.Health(time.Second); health.Healthy || health.Reason == "" {
		t.Fatalf("stale progress should be unhealthy: %#v", health)
	}
	store.MarkProgress()
	if health := store.Health(time.Second); !health.Healthy {
		t.Fatalf("fresh progress should restore health: %#v", health)
	}
}

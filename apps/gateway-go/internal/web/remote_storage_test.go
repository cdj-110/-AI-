package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/remotestore"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

type remoteTestRuntime struct{ err error }

func (r remoteTestRuntime) Statuses() []remotestore.Status {
	return []remotestore.Status{{ID: "db", Pending: 3}}
}
func (r remoteTestRuntime) ValidateChange(config.RemoteStorageConfig) error { return r.err }

func TestRemoteStorageAPIAuthenticationRedactionAndSave(t *testing.T) {
	cfg := alarmWebConfig()
	cfg.GatewayKey = "preserve-gateway"
	target := config.RemoteStorageTarget{ID: "db", Name: "Test DB", Driver: "mysql", Host: "localhost", Database: "plant", Username: "writer", Password: "secret-to-mask", Points: []config.HistoryPointRef{{DeviceKey: "d1", Metric: "temperature"}}}
	target.ApplyDefaults()
	cfg.RemoteStorage = config.RemoteStorageConfig{Targets: []config.RemoteStorageTarget{target}}
	cfg.ApplyDefaults()
	path := filepath.Join(t.TempDir(), "config.json")
	if e := config.Save(path, cfg); e != nil {
		t.Fatal(e)
	}
	server := New(config.ListenerConfig{}, state.New(cfg), nil, path, func(config.Config) error { return nil }, nil)
	server.SetRemoteStorage(remoteTestRuntime{})
	invoke := func(method, route string, body any, revision string, permissions ...string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, route, bytes.NewReader(raw))
		authorizeConfigTestRequest(server, r, permissions...)
		r.Header.Set("If-Match", revision)
		w := httptest.NewRecorder()
		server.routes().ServeHTTP(w, r)
		return w
	}
	if w := invoke("GET", "/api/remote-storage", nil, ""); w.Code != 403 {
		t.Fatalf("unauthorized=%d", w.Code)
	}
	w := invoke("GET", "/api/remote-storage", nil, "", "history.view")
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret-to-mask") || strings.Contains(w.Body.String(), "writer") {
		t.Fatalf("read-only credentials: %d %s", w.Code, w.Body)
	}
	w = invoke("GET", "/api/remote-storage", nil, "", "history.manage", "history.view")
	var reply struct {
		Config   config.RemoteStorageConfig `json:"config"`
		Revision string                     `json:"revision"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &reply); e != nil {
		t.Fatal(e)
	}
	if reply.Config.Targets[0].Password != "***" || reply.Revision == "" {
		t.Fatal(w.Body.String())
	}
	if w = invoke("PUT", "/api/remote-storage", reply.Config, reply.Revision, "history.view"); w.Code != 403 {
		t.Fatal("viewer changed settings")
	}
	if w = invoke("POST", "/api/remote-storage/test", target, "", "history.view"); w.Code != 403 {
		t.Fatal("viewer tested connection")
	}
	if w = invoke("PUT", "/api/remote-storage", reply.Config, "old-version", "history.manage"); w.Code != 409 {
		t.Fatal("stale write accepted")
	}
	reply.Config.Targets[0].Name = "Updated"
	w = invoke("PUT", "/api/remote-storage", reply.Config, reply.Revision, "history.manage")
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	saved, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	if saved.RemoteStorage.Targets[0].Password != "secret-to-mask" || saved.GatewayKey != "preserve-gateway" || saved.RemoteStorage.Targets[0].Name != "Updated" {
		t.Fatal("save lost fields")
	}
	server.SetRemoteStorage(remoteTestRuntime{err: errors.New("pending records")})
	rev, _ := configFileRevision(path)
	if w = invoke("PUT", "/api/remote-storage", reply.Config, rev, "history.manage"); w.Code != 409 {
		t.Fatalf("pending %d %s", w.Code, w.Body)
	}
	server.SetRemoteStorage(remoteTestRuntime{})
	reply.Config.Targets[0].Points[0].Metric = "missing"
	if w = invoke("PUT", "/api/remote-storage", reply.Config, rev, "history.manage"); w.Code != 400 {
		t.Fatal("missing point accepted")
	}
}

package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func validRemoteTarget(driver string) RemoteStorageTarget {
	t := RemoteStorageTarget{ID: "db-test", Name: "数据库任务", Driver: driver, Host: "db.local", Database: "plant", Username: "writer", Password: "sensitive", Points: []HistoryPointRef{{DeviceKey: "d", Metric: "p"}}}
	t.ApplyDefaults()
	return t
}
func TestRemoteDefaultsAndValidation(t *testing.T) {
	for _, driver := range []string{"mysql", "postgresql", "sqlserver"} {
		target := validRemoteTarget(driver)
		if e := target.Validate(); e != nil {
			t.Fatal(e)
		}
		if target.TLSMode != "verify-full" || target.IntervalSeconds != 5 || target.TimeoutSeconds != 10 || target.Table != "gateway_samples" {
			t.Fatal(target)
		}
	}
	for name, edit := range map[string]func(*RemoteStorageTarget){"identifier": func(t *RemoteStorageTarget) { t.Table = "t;DROP TABLE x" }, "host": func(t *RemoteStorageTarget) { t.Host = "db;port=9" }, "port": func(t *RemoteStorageTarget) { t.Port = 65536 }, "tls": func(t *RemoteStorageTarget) { t.TLSMode = "skip-verify" }, "batch": func(t *RemoteStorageTarget) { t.BatchSize = 501 }, "points": func(t *RemoteStorageTarget) { t.Points = nil }, "duplicate": func(t *RemoteStorageTarget) { t.Points = append(t.Points, t.Points[0]) }, "driver": func(t *RemoteStorageTarget) { t.Driver = "domestic-not-implemented" }} {
		t.Run(name, func(t *testing.T) {
			target := validRemoteTarget("mysql")
			edit(&target)
			if target.Validate() == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	cfg := RemoteStorageConfig{StoragePath: filepath.Join(t.TempDir(), "queue"), StorageUUID: "test-volume", MaxSizeMB: 16, Targets: []RemoteStorageTarget{validRemoteTarget("mysql")}}
	if e := cfg.Validate(); e != nil {
		t.Fatal(e)
	}
	cfg.StorageUUID = ""
	if cfg.Validate() == nil {
		t.Fatal("unbound volume accepted")
	}
	var old Config
	if e := json.Unmarshal([]byte(`{"gatewayKey":"old"}`), &old); e != nil {
		t.Fatal(e)
	}
	old.ApplyDefaults()
	if e := old.RemoteStorage.Validate(); e != nil {
		t.Fatal("old config regression", e)
	}
}
func TestRemoteSecretsAndDestinationIdentity(t *testing.T) {
	old := Config{RemoteStorage: RemoteStorageConfig{Targets: []RemoteStorageTarget{validRemoteTarget("mysql")}}}
	redacted := RedactSecrets(old)
	if redacted.RemoteStorage.Targets[0].Password != "***" || old.RemoteStorage.Targets[0].Password != "sensitive" {
		t.Fatal("redaction mutated source or exposed secret")
	}
	raw, _ := json.Marshal(redacted)
	if strings.Contains(string(raw), "sensitive") {
		t.Fatal("secret exposed")
	}
	PreserveRemoteStorageSecrets(old.RemoteStorage, &redacted.RemoteStorage)
	if redacted.RemoteStorage.Targets[0].Password != "sensitive" {
		t.Fatal("masked update lost secret")
	}
	redacted = RedactSecrets(old)
	redacted.RemoteStorage.Targets[0].Host = "new-host"
	PreserveRemoteStorageSecrets(old.RemoteStorage, &redacted.RemoteStorage)
	if redacted.RemoteStorage.Targets[0].Password != "" {
		t.Fatal("password transferred to new destination")
	}
	before := old.RemoteStorage.Targets[0]
	after := before
	after.Password = "rotated"
	after.IntervalSeconds = 20
	if before.DestinationKey() != after.DestinationKey() {
		t.Fatal("credentials changed queue identity")
	}
	after.Table = "another"
	if before.DestinationKey() == after.DestinationKey() {
		t.Fatal("target change ignored")
	}
}

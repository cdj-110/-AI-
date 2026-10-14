package web

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestFactoryResetBackupIsRecoverableAndProtected(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.local.json")
	server := &Server{configPath: configPath}
	expected := config.Config{GatewayKey: "gw-backup", Security: config.SecurityConfig{Users: []config.UserConfig{{Username: "admin", PasswordHash: "secret-hash", Enabled: true}}}}
	info, err := server.createFactoryResetBackup(expected)
	if err != nil {
		t.Fatal(err)
	}
	if info.Storage != "local" || info.Path == "" || info.SHA256 == "" {
		t.Fatalf("unexpected backup info: %#v", info)
	}
	if err := verifyFactoryBackup(info.Path, info.SHA256); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0600 {
		t.Fatalf("backup mode = %o", stat.Mode().Perm())
	}
	file, err := os.Open(info.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var restored config.Config
	if err := json.NewDecoder(reader).Decode(&restored); err != nil {
		t.Fatal(err)
	}
	if restored.GatewayKey != expected.GatewayKey || restored.Security.Users[0].PasswordHash != "secret-hash" {
		t.Fatalf("backup did not preserve restorable config: %#v", restored)
	}
	if _, err := os.Stat(info.Path + ".manifest.json"); err != nil {
		t.Fatal(err)
	}
}

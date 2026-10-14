package web

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type factoryResetBackupInfo struct {
	Path      string    `json:"path"`
	Storage   string    `json:"storage"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Server) createFactoryResetBackup(cfg config.Config) (factoryResetBackupInfo, error) {
	root, storage := factoryBackupRoot(s.configPath)
	if err := os.MkdirAll(root, 0700); err != nil {
		return factoryResetBackupInfo{}, err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return factoryResetBackupInfo{}, err
	}
	raw = append(raw, '\n')
	var compressed bytes.Buffer
	archive := gzip.NewWriter(&compressed)
	archive.Header.ModTime = time.Time{}
	archive.Header.Name = "config.local.json"
	if _, err := archive.Write(raw); err != nil {
		return factoryResetBackupInfo{}, err
	}
	if err := archive.Close(); err != nil {
		return factoryResetBackupInfo{}, err
	}
	now := time.Now()
	name := "factory-reset-" + now.Format("20060102-150405.000") + ".json.gz"
	path := filepath.Join(root, name)
	if err := atomicWriteFile(path, compressed.Bytes(), 0600); err != nil {
		return factoryResetBackupInfo{}, err
	}
	sum := sha256.Sum256(compressed.Bytes())
	info := factoryResetBackupInfo{Path: path, Storage: storage, SHA256: hex.EncodeToString(sum[:]), CreatedAt: now}
	if err := verifyFactoryBackup(info.Path, info.SHA256); err != nil {
		_ = os.Remove(path)
		return factoryResetBackupInfo{}, fmt.Errorf("verify factory reset backup: %w", err)
	}
	manifest, err := json.MarshalIndent(map[string]interface{}{
		"format": "weikong-factory-reset-backup-v1", "gatewayKey": cfg.GatewayKey,
		"archive": filepath.Base(path), "sha256": info.SHA256, "createdAt": now,
	}, "", "  ")
	if err != nil {
		_ = os.Remove(path)
		return factoryResetBackupInfo{}, err
	}
	if err := atomicWriteFile(path+".manifest.json", append(manifest, '\n'), 0600); err != nil {
		_ = os.Remove(path)
		return factoryResetBackupInfo{}, err
	}
	return info, nil
}

func factoryBackupRoot(configPath string) (string, string) {
	if runtime.GOOS == "linux" && mountAvailable("/media/udisk1") {
		return "/media/udisk1/weikong-backups/factory-reset", "usb"
	}
	return filepath.Join(filepath.Dir(configPath), ".runtime", "factory-reset-backups"), "local"
}

func mountAvailable(target string) bool {
	content, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	needle := " " + strings.TrimSpace(target) + " "
	if !strings.Contains(string(content), needle) {
		return false
	}
	info, err := os.Stat(target)
	return err == nil && info.IsDir()
}

func verifyFactoryBackup(path, expectedHash string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(content)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(expectedHash)) {
		return fmt.Errorf("backup checksum mismatch")
	}
	return nil
}

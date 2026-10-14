package app

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cloud"
	"weikong-iot-platform/apps/gateway-go/internal/ota"
)

var otaTaskPattern = regexp.MustCompile(`^[a-fA-F0-9-]{36}$`)

const otaResultPath = "/userdata/weikong/config/ota-result.json"
const otaReleasePath = "/userdata/weikong/config/ota-release.json"

func (a *App) applyOTA(command cloud.OTACommand) cloud.OTAResult {
	result := cloud.OTAResult{TaskID: command.TaskID, HardwareID: a.cfg.Activation.HardwareID, Release: command.Release, Status: "FAILED"}
	if !otaTaskPattern.MatchString(command.TaskID) || command.HardwareID != a.cfg.Activation.HardwareID || command.SN != a.cfg.Activation.SN || !a.cfg.Activation.IsReady() {
		result.Message = "升级任务与本机身份不符"
		return result
	}
	if command.ExpiresAt.Before(time.Now()) || command.ExpiresAt.After(time.Now().Add(20*time.Minute)) {
		result.Message = "升级任务已过期或时间范围不合法"
		return result
	}
	if len(command.PackageSHA) != 64 {
		result.Message = "升级包摘要无效"
		return result
	}
	if _, err := hex.DecodeString(command.PackageSHA); err != nil {
		result.Message = "升级包摘要无效"
		return result
	}
	packageURL, err := url.Parse(command.PackageURL)
	cloudURL, cloudErr := url.Parse(a.cfg.Activation.CloudAPI)
	if err != nil || cloudErr != nil || packageURL.Scheme != "https" || cloudURL.Scheme != "https" || !strings.EqualFold(packageURL.Host, cloudURL.Host) || !strings.HasPrefix(packageURL.EscapedPath(), "/api/ota/download/") || packageURL.User != nil || packageURL.Fragment != "" {
		result.Message = "升级下载地址不属于授权云平台或未使用 HTTPS"
		return result
	}
	keys, err := ota.TrustedKeys()
	if err != nil {
		result.Message = err.Error()
		return result
	}
	currentRelease, releaseErr := installedOTARelease()
	if releaseErr != nil {
		result.Message = "本地升级版本记录损坏，拒绝安装"
		return result
	}
	if command.Release <= currentRelease {
		result.Message = "拒绝重复或降级安装"
		return result
	}
	lock := "/userdata/weikong/config/.ota-install-lock"
	if err := os.Mkdir(lock, 0700); err != nil {
		result.Message = "已有升级任务正在执行"
		return result
	}
	started := false
	defer func() {
		if !started {
			_ = os.Remove(lock)
		}
	}()
	// The OTA scratch space must be on removable storage, not the small /tmp.
	otaRoot := otaStorageRoot()
	if otaRoot == "" {
		result.Message = "升级暂存 U 盘未挂载"
		return result
	}
	if err := os.MkdirAll(otaRoot, 0700); err != nil {
		result.Message = err.Error()
		return result
	}
	if err := cleanupOTAStages(otaRoot); err != nil {
		result.Message = "无法清理旧升级暂存文件: " + err.Error()
		return result
	}
	stage := filepath.Join(otaRoot, command.TaskID)
	if err := os.Mkdir(stage, 0700); err != nil {
		result.Message = "升级任务已在处理或暂存目录不可用"
		return result
	}
	defer func() {
		if !started {
			_ = os.RemoveAll(stage)
		}
	}()
	path := filepath.Join(stage, "package.wku")
	if err := downloadOTAPackage(command.PackageURL, path, command.PackageSHA); err != nil {
		result.Message = err.Error()
		return result
	}
	manifest, err := ota.VerifyFile(path, keys)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	if manifest.Release != command.Release || manifest.Model != "EG100" {
		result.Message = "升级包型号或版本与任务不符"
		return result
	}
	binarySHA, err := prepareOTABinary(path, filepath.Join(stage, "new-gateway"))
	if err != nil {
		result.Message = err.Error()
		return result
	}
	// Keep the privileged installer on internal storage, not removable USB.
	scriptPath := filepath.Join("/userdata/weikong/config", ".ota-install-"+command.TaskID+".sh")
	script, err := os.OpenFile(scriptPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	_, writeErr := script.Write([]byte(otaInstallScript(stage, command, binarySHA)))
	closeErr := script.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(scriptPath)
		result.Message = "无法写入升级安装程序"
		return result
	}
	process := exec.Command("/bin/sh", scriptPath)
	if err := process.Start(); err != nil {
		_ = os.Remove(scriptPath)
		result.Message = err.Error()
		return result
	}
	go func() { _ = process.Wait() }()
	started = true
	result.Status = "INSTALLING"
	result.Message = "升级包已验证，网关即将重启安装"
	return result
}

func installedOTARelease() (int64, error) {
	var state struct {
		Release int64 `json:"release"`
	}
	raw, err := os.ReadFile(otaReleasePath)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if json.Unmarshal(raw, &state) != nil || state.Release < 1 {
		return 0, errors.New("invalid OTA release state")
	}
	return state.Release, nil
}

func otaStorageRoot() string {
	raw, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return ""
	}
	return selectOTAStorageRoot(string(raw))
}

func selectOTAStorageRoot(mounts string) string {
	var fallback string
	for _, line := range strings.Split(mounts, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		switch parts[1] {
		case "/media/udisk1":
			// EG100 mounts the large data partition here. Prefer it over the
			// small /mnt/udisk service partition even when /proc/mounts lists
			// the service partition first.
			return path.Join(parts[1], "weikong-ota")
		case "/mnt/udisk":
			fallback = path.Join(parts[1], "weikong-ota")
		}
	}
	return fallback
}

// cleanupOTAStages removes completed and failed OTA work directories before a
// new download. The install lock is already held by the caller, so no active
// installer can be using one of these directories at the same time. Files or
// directories outside the strict task-id naming scheme are left untouched.
func cleanupOTAStages(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !otaTaskPattern.MatchString(entry.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func downloadOTAPackage(address, target, expectedSHA string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("OTA redirects are disabled") }}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("升级包下载失败: HTTP %d", response.StatusCode)
	}
	if response.ContentLength > ota.MaxArtifactSize+ota.MaxManifestSize+8 {
		return errors.New("升级包过大")
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	count, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, ota.MaxArtifactSize+ota.MaxManifestSize+9))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if count > ota.MaxArtifactSize+ota.MaxManifestSize+8 || hex.EncodeToString(hash.Sum(nil)) != expectedSHA {
		return errors.New("升级包完整性校验失败")
	}
	return nil
}

func prepareOTABinary(packagePath, target string) (string, error) {
	prepared, created := false, false
	defer func() {
		if created && !prepared {
			_ = os.Remove(target)
		}
	}()
	source, err := os.Open(packagePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	var header [8]byte
	if _, err := io.ReadFull(source, header[:]); err != nil {
		return "", err
	}
	length := binary.BigEndian.Uint32(header[4:])
	if _, err := source.Seek(int64(length), io.SeekCurrent); err != nil {
		return "", err
	}
	decompressor, err := gzip.NewReader(source)
	if err != nil {
		return "", err
	}
	defer decompressor.Close()
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	created = true
	hash := sha256.New()
	count, copyErr := io.Copy(io.MultiWriter(destination, hash), io.LimitReader(decompressor, 128<<20+1))
	closeErr := destination.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if count < 1 || count > 128<<20 {
		return "", errors.New("解压后的网关程序过大")
	}
	if err := os.Chmod(target, 0755); err != nil {
		return "", err
	}
	// The EG100 USB filesystem is noexec and its rootfs cannot hold two gateway
	// binaries at once. Validate the signed artifact here; the executable's
	// -check-config runs after the old binary is backed up and replaced.
	artifact, err := elf.Open(target)
	if err != nil {
		return "", fmt.Errorf("升级程序不是有效 ELF: %w", err)
	}
	defer artifact.Close()
	if artifact.Class != elf.ELFCLASS32 || artifact.Machine != elf.EM_ARM || (artifact.Type != elf.ET_EXEC && artifact.Type != elf.ET_DYN) {
		return "", errors.New("升级程序不是 EG100 ARMv7 可执行文件")
	}
	prepared = true
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func otaInstallScript(stage string, command cloud.OTACommand, expectedBinarySHA string) string {
	success, _ := json.Marshal(cloud.OTAResult{TaskID: command.TaskID, HardwareID: command.HardwareID, Release: command.Release, Status: "SUCCEEDED", Message: "网关应用升级成功"})
	rollback, _ := json.Marshal(cloud.OTAResult{TaskID: command.TaskID, HardwareID: command.HardwareID, Release: command.Release, Status: "ROLLED_BACK", Message: "新版本未通过启动检查，已恢复旧版本"})
	failure, _ := json.Marshal(cloud.OTAResult{TaskID: command.TaskID, HardwareID: command.HardwareID, Release: command.Release, Status: "FAILED", Message: "安装前准备失败，旧程序未替换"})
	// Stage and task names have been validated and contain no shell metacharacters.
	return fmt.Sprintf(`#!/bin/sh
set -eu
sleep 3
STAGE='%s'
EXPECTED='%s'
LIVE=/userdata/weikong/bin/weikong-gateway
SCRIPT=/userdata/weikong/config/.ota-install-%s.sh
SERVICE=/etc/init.d/S99weikong-gateway
BACKUP="$STAGE/previous-gateway"
NEW="$STAGE/new-gateway"
CONFIG=/userdata/weikong/config/config.local.json
LOCK=/userdata/weikong/config/.ota-install-lock
trap 'rmdir "$LOCK" 2>/dev/null || true; rm -f "$SCRIPT"; rm -rf "$STAGE"' EXIT
if [ "$(sha256sum "$NEW" | cut -d ' ' -f 1)" != "$EXPECTED" ] || ! cp "$LIVE" "$BACKUP" || [ "$(sha256sum "$LIVE" | cut -d ' ' -f 1)" != "$(sha256sum "$BACKUP" | cut -d ' ' -f 1)" ]; then
  printf '%%s' '%s' > %s.tmp
  mv -f %s.tmp %s
  exit 1
fi
if ! "$SERVICE" stop; then
  printf '%%s' '%s' > %s.tmp
  mv -f %s.tmp %s
  exit 1
fi
if ! rm -f "$LIVE" || ! cp "$NEW" "$LIVE" || ! chmod 755 "$LIVE" || [ "$(sha256sum "$LIVE" | cut -d ' ' -f 1)" != "$EXPECTED" ] || ! env LD_LIBRARY_PATH=/userdata/weikong/lib "$LIVE" -config "$CONFIG" -check-config; then
  cp "$BACKUP" "$LIVE"
  chmod 755 "$LIVE"
  "$SERVICE" start || true
  printf '%%s' '%s' > %s.tmp
  mv -f %s.tmp %s
  exit 1
fi
if "$SERVICE" start; then
  sleep 10
fi
READY=1
for CHECK in 1 2 3; do
  if ! "$SERVICE" status >/dev/null 2>&1 || [ -z "$(pidof weikong-gateway 2>/dev/null || true)" ]; then
    READY=0
    break
  fi
  sleep 5
done
if [ "$READY" -eq 1 ]; then
  printf '%%s' '%s' > /userdata/weikong/config/ota-release.json.tmp
  mv -f /userdata/weikong/config/ota-release.json.tmp /userdata/weikong/config/ota-release.json
  printf '%%s' '%s' > %s.tmp
  mv -f %s.tmp %s
else
  "$SERVICE" stop || true
  cp "$BACKUP" "$LIVE"
  chmod 755 "$LIVE"
  printf '%%s' '%s' > %s.tmp
  mv -f %s.tmp %s
  "$SERVICE" start
fi
`, stage, expectedBinarySHA, command.TaskID, failure, otaResultPath, otaResultPath, otaResultPath, failure, otaResultPath, otaResultPath, otaResultPath, rollback, otaResultPath, otaResultPath, otaResultPath, fmt.Sprintf(`{"release":%d}`, command.Release), success, otaResultPath, otaResultPath, otaResultPath, rollback, otaResultPath, otaResultPath, otaResultPath)
}

func (a *App) otaReceiptLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		a.cloudMu.Lock()
		var activation *cloud.Client
		for _, client := range a.clouds {
			if client != nil && client.Name() == "activation" {
				activation = client
				break
			}
		}
		a.cloudMu.Unlock()
		if activation != nil && activation.IsConnected() {
			activation.PublishPendingOTAResult()
		}
	}
}

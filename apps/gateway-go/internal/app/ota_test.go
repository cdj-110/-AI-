package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cloud"
)

func TestOTAInstallerScriptIsWellFormed(t *testing.T) {
	command := cloud.OTACommand{TaskID: "11111111-1111-1111-1111-111111111111", HardwareID: "ABC123", Release: 2026092101, ExpiresAt: time.Now()}
	script := otaInstallScript("/mnt/udisk/weikong-ota/"+command.TaskID, command, strings.Repeat("a", 64))
	if strings.Contains(script, "%!") {
		t.Fatal("installer format arguments do not match")
	}
	if !strings.Contains(script, "ROLLED_BACK") || !strings.Contains(script, "SUCCEEDED") || !strings.Contains(script, "FAILED") {
		t.Fatal("installer outcomes are missing")
	}
	if !strings.Contains(script, `NEW="$STAGE/new-gateway"`) || strings.Contains(script, `"$LIVE.ota-new"`) {
		t.Fatal("installer must stage the new binary on USB, not the constrained rootfs")
	}
	if !strings.Contains(script, `sha256sum "$BACKUP"`) || !strings.Contains(script, `-check-config`) {
		t.Fatal("installer must verify the backup and check the new configuration")
	}
	if !strings.Contains(script, `EXPECTED='`+strings.Repeat("a", 64)+`'`) || !strings.Contains(script, `sha256sum "$NEW"`) || !strings.Contains(script, `sha256sum "$LIVE"`) {
		t.Fatal("installer must verify staged and installed binary hashes")
	}
	path := filepath.Join(t.TempDir(), "install.sh")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("shell unavailable")
	}
	if output, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("invalid installer shell: %v: %s", err, output)
	}
}

func TestCleanupOTAStagesRemovesOnlyTaskDirectories(t *testing.T) {
	root := t.TempDir()
	task := "11111111-1111-4111-8111-111111111111"
	if err := os.Mkdir(filepath.Join(root, task), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "not-a-task"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := cleanupOTAStages(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, task)); !os.IsNotExist(err) {
		t.Fatalf("task stage was not removed: %v", err)
	}
	for _, name := range []string{"keep.txt", "not-a-task"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("unrelated entry %q was changed: %v", name, err)
		}
	}
}

func TestSelectOTAStorageRootPrefersLargeDataPartition(t *testing.T) {
	mounts := strings.Join([]string{
		"/dev/sda2 /mnt/udisk ext4 rw 0 0",
		"/dev/sda4 /media/udisk1 ext4 rw 0 0",
	}, "\n")
	if got, want := selectOTAStorageRoot(mounts), "/media/udisk1/weikong-ota"; got != want {
		t.Fatalf("ota storage root = %q, want %q", got, want)
	}
}

func TestSelectOTAStorageRootFallsBackToServicePartition(t *testing.T) {
	if got, want := selectOTAStorageRoot("/dev/sda2 /mnt/udisk ext4 rw 0 0\n"), "/mnt/udisk/weikong-ota"; got != want {
		t.Fatalf("ota storage root = %q, want %q", got, want)
	}
}

package hardware

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Identity struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Available bool   `json:"available"`
	Message   string `json:"message,omitempty"`
}

func ReadIdentity() Identity {
	readers := identityReaders()
	for _, reader := range readers {
		value, source, err := reader()
		if err != nil || strings.TrimSpace(value) == "" {
			continue
		}
		return Identity{
			ID:        fingerprint(value),
			Source:    source,
			Available: true,
		}
	}
	return Identity{
		Source:    "unavailable",
		Available: false,
		Message:   "未读取到硬件唯一 ID，请确认系统是否开放 DMI/CPU/eMMC 标识读取权限",
	}
}

type identityReader func() (string, string, error)

func identityReaders() []identityReader {
	if runtime.GOOS == "windows" {
		return []identityReader{
			readWindowsCIM("Win32_ComputerSystemProduct", "UUID", "windows.computer_system_uuid"),
			readWindowsCIM("Win32_BIOS", "SerialNumber", "windows.bios_serial"),
			readWindowsCIM("Win32_BaseBoard", "SerialNumber", "windows.baseboard_serial"),
		}
	}
	return []identityReader{
		readFile("/sys/block/mmcblk0/device/cid", "linux.emmc_cid"),
		readFile("/sys/class/dmi/id/product_uuid", "linux.product_uuid"),
		readFile("/sys/class/dmi/id/product_serial", "linux.product_serial"),
		readFile("/sys/class/dmi/id/board_serial", "linux.board_serial"),
		readCPUInfoSerial,
		readFile("/etc/machine-id", "linux.machine_id"),
	}
}

func readFile(path string, source string) identityReader {
	return func() (string, string, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", source, err
		}
		return strings.TrimSpace(string(raw)), source, nil
	}
}

func readCPUInfoSerial() (string, string, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", "linux.cpu_serial", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "serial") {
			continue
		}
		return strings.TrimSpace(parts[1]), "linux.cpu_serial", nil
	}
	return "", "linux.cpu_serial", scanner.Err()
}

func readWindowsCIM(className string, property string, source string) identityReader {
	return func() (string, string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		command := fmt.Sprintf("(Get-CimInstance -ClassName %s).%s", className, property)
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		err := cmd.Run()
		if err != nil {
			return "", source, err
		}
		value := strings.TrimSpace(stdout.String())
		if isPlaceholder(value) {
			return "", source, nil
		}
		return value, source, nil
	}
}

func isPlaceholder(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "" ||
		normalized == "to be filled by o.e.m." ||
		normalized == "default string" ||
		normalized == "none" ||
		normalized == "unknown" ||
		normalized == "system serial number" ||
		normalized == "00000000-0000-0000-0000-000000000000"
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return strings.ToUpper(hex.EncodeToString(sum[:16]))
}

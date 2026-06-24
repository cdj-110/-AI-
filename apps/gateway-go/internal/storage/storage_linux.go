//go:build linux

package storage

import (
	"bufio"
	"os"
	"strings"
	"syscall"
)

func detectPlatform() ([]Device, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var devices []Device
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		devicePath, mountPath, fileSystem := fields[0], unescapeMount(fields[1]), fields[2]
		kind := removableKind(devicePath)
		if kind == "" {
			continue
		}
		var stats syscall.Statfs_t
		if err := syscall.Statfs(mountPath, &stats); err != nil {
			continue
		}
		devices = append(devices, Device{
			Device: devicePath, MountPath: mountPath, FileSystem: fileSystem, Kind: kind,
			TotalBytes: uint64(stats.Blocks) * uint64(stats.Bsize),
			FreeBytes:  uint64(stats.Bavail) * uint64(stats.Bsize),
		})
	}
	return devices, scanner.Err()
}

func removableKind(device string) string {
	if strings.HasPrefix(device, "/dev/mmcblk") {
		return "TF"
	}
	if strings.HasPrefix(device, "/dev/sd") {
		return "USB"
	}
	return ""
}

func unescapeMount(value string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\134`, `\`).Replace(value)
}

//go:build linux

package storage

import (
	"bufio"
	"io"
	"os"
	"strings"
	"syscall"
)

func detectPlatform() ([]Device, error) {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	devices := parseMountInfo(file)
	var available []Device
	for _, device := range devices {
		var stats syscall.Statfs_t
		if err := syscall.Statfs(device.MountPath, &stats); err != nil {
			continue
		}
		device.TotalBytes = uint64(stats.Blocks) * uint64(stats.Bsize)
		device.FreeBytes = uint64(stats.Bavail) * uint64(stats.Bsize)
		available = append(available, device)
	}
	return available, nil
}

func parseMountInfo(reader io.Reader) []Device {
	var devices []Device
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), " - ", 2)
		if len(parts) != 2 {
			continue
		}
		fields, filesystem := strings.Fields(parts[0]), strings.Fields(parts[1])
		if len(fields) < 6 || len(filesystem) < 3 {
			continue
		}
		devicePath, mountPath, fileSystem := unescapeMount(filesystem[1]), unescapeMount(fields[4]), filesystem[0]
		kind := removableKind(devicePath)
		if kind == "" {
			continue
		}
		devices = append(devices, Device{
			Device: devicePath, MountPath: mountPath, FileSystem: fileSystem, Kind: kind,
			UUID: filesystemUUID(fields[2]), MountID: fields[0],
			ReadOnly: strings.Contains(","+fields[5]+",", ",ro,") || strings.Contains(","+filesystem[2]+",", ",ro,"),
		})
	}
	return devices
}

func filesystemUUID(majorMinor string) string {
	data, err := os.ReadFile("/run/udev/data/b" + majorMinor)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "E:ID_FS_UUID=") {
			return strings.TrimPrefix(line, "E:ID_FS_UUID=")
		}
	}
	return ""
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

func pathSpace(path string) (uint64, uint64, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(path, &stats); err != nil {
		return 0, 0, err
	}
	blockSize := uint64(stats.Bsize)
	return uint64(stats.Blocks) * blockSize, uint64(stats.Bavail) * blockSize, nil
}

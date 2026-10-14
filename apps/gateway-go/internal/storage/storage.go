package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrSpaceUnsupported = errors.New("filesystem capacity inspection is not supported on this platform")

type Device struct {
	Device     string `json:"device"`
	MountPath  string `json:"mountPath"`
	FileSystem string `json:"fileSystem"`
	Kind       string `json:"kind"`
	TotalBytes uint64 `json:"totalBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
	UUID       string `json:"uuid,omitempty"`
	MountID    string `json:"-"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
}

func Detect() ([]Device, error) {
	return detectPlatform()
}

func Find(mountPath string) (Device, bool) {
	return Resolve(mountPath, "")
}

// Resolve follows a bound filesystem UUID even if hotplug assigns a different
// mount directory. A missing bound volume must never fall back to that directory.
func Resolve(mountPath, uuid string) (Device, bool) {
	devices, err := Detect()
	if err != nil {
		return Device{}, false
	}
	return resolveDevice(devices, mountPath, uuid)
}

func resolveDevice(devices []Device, mountPath, uuid string) (Device, bool) {
	resolved, err := filepath.EvalSymlinks(mountPath)
	if err == nil {
		mountPath = resolved
	}
	uuid = strings.TrimSpace(uuid)
	for _, device := range devices {
		if (uuid != "" && device.UUID == uuid) || (uuid == "" && device.MountPath == filepath.Clean(mountPath)) {
			return device, true
		}
	}
	return Device{}, false
}

// Guard latches a lost mount generation. Even the same device name and UUID
// after replug require a new database connection, never reuse of the old inode.
func Guard(device Device) func() bool {
	return mountGuard(device, Detect)
}

func mountGuard(device Device, detect func() ([]Device, error)) func() bool {
	var mu sync.Mutex
	lost := false
	return func() bool {
		mu.Lock()
		defer mu.Unlock()
		if lost {
			return false
		}
		devices, err := detect()
		if err == nil {
			for _, current := range devices {
				if current.MountPath == device.MountPath && current.Device == device.Device && current.UUID == device.UUID && current.MountID == device.MountID && !current.ReadOnly {
					return true
				}
			}
		}
		lost = true
		return false
	}
}

// Space returns the total and currently available bytes for the filesystem
// containing path. It is used for the EG100 local history database guard.
func Space(path string) (totalBytes uint64, freeBytes uint64, err error) {
	probe := filepath.Clean(path)
	for {
		if _, statErr := os.Stat(probe); statErr == nil {
			return pathSpace(probe)
		} else if !os.IsNotExist(statErr) {
			return 0, 0, statErr
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return 0, 0, os.ErrNotExist
		}
		probe = parent
	}
}

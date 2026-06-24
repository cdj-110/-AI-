package storage

type Device struct {
	Device     string `json:"device"`
	MountPath  string `json:"mountPath"`
	FileSystem string `json:"fileSystem"`
	Kind       string `json:"kind"`
	TotalBytes uint64 `json:"totalBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
}

func Detect() ([]Device, error) {
	return detectPlatform()
}

func Find(mountPath string) (Device, bool) {
	devices, err := Detect()
	if err != nil {
		return Device{}, false
	}
	for _, device := range devices {
		if device.MountPath == mountPath {
			return device, true
		}
	}
	return Device{}, false
}

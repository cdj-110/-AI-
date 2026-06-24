//go:build !linux

package storage

func detectPlatform() ([]Device, error) {
	return nil, nil
}

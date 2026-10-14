//go:build !linux

package storage

func detectPlatform() ([]Device, error) {
	return nil, nil
}

func pathSpace(string) (uint64, uint64, error) {
	return 0, 0, ErrSpaceUnsupported
}

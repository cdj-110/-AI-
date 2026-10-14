//go:build !linux

package supervisor

import (
	"errors"
	"time"
)

func openHardwareWatchdog(_ string, _ time.Duration) (hardwareWatchdog, error) {
	return nil, errors.New("hardware watchdog is only supported on Linux")
}

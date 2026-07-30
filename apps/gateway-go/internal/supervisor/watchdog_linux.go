//go:build linux

package supervisor

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const watchdogSetTimeout = 0xc0045706

type linuxWatchdog struct {
	file *os.File
}

func openHardwareWatchdog(path string, timeout time.Duration) (hardwareWatchdog, error) {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	seconds := int32(timeout / time.Second)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), watchdogSetTimeout, uintptr(unsafe.Pointer(&seconds)))
	if errno != 0 {
		_ = file.Close()
		return nil, fmt.Errorf("set timeout: %w", errno)
	}
	return &linuxWatchdog{file: file}, nil
}

func (w *linuxWatchdog) Feed() error {
	_, err := w.file.Write([]byte{0})
	return err
}

func (w *linuxWatchdog) Disarm() error {
	if w.file == nil {
		return nil
	}
	_, writeErr := w.file.Write([]byte{'V'})
	closeErr := w.file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

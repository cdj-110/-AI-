//go:build linux

package collector

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Exercise the actual serial driver: a same-DI reply arrives after timeout,
// across close/reopen; subsequent reads contain wrong-DI and fragmented replies.
func TestDLT645RealSerialLateReplyAndFollowingPoint(t *testing.T) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Skipf("PTY unavailable: %v", err)
	}
	master := os.NewFile(uintptr(fd), "dlt-response-master")
	defer master.Close()
	var lock, number uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&lock))); errno != 0 {
		t.Fatal(errno)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&number))); errno != 0 {
		t.Fatal(errno)
	}
	address := fmt.Sprintf("/dev/pts/%d", number)
	slave, err := os.OpenFile(address, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	CloseDLT645Connections()
	defer CloseDLT645Connections()
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 3; i++ {
			raw := make([]byte, 20)
			if _, err := io.ReadFull(master, raw); err != nil {
				done <- err
				return
			}
			request, _, complete, err := parseDLT645Stream(raw)
			if err != nil || !complete {
				done <- fmt.Errorf("request parse: %v", err)
				return
			}
			if i == 0 {
				time.Sleep(170 * time.Millisecond)
				late := responseDLTFrame(0x91, append(request.Data, 0x99, 0x99, 0, 0))
				if _, err := master.Write(late); err != nil {
					done <- err
					return
				}
				continue
			}
			wrong := append([]byte(nil), request.Data...)
			wrong[0]++
			noise := responseDLTFrame(0x91, append(wrong, 0x99, 0, 0, 0))
			if _, err := master.Write(noise); err != nil {
				done <- err
				return
			}
			good := responseDLTFrame(0x91, append(request.Data, 0x23, 0x01, 0, 0))
			if _, err := master.Write(good[:9]); err != nil {
				done <- err
				return
			}
			time.Sleep(10 * time.Millisecond)
			if _, err := master.Write(good[9:]); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	p := scheduledDLTPoint()
	p.Address = address
	p.DLT645.WakeupBytes = 4
	p.DLT645.ResponseTimeoutMilliseconds = 120
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, di := range []string{"00000000", "00000400"} {
		p.DLT645.DataIdentifier = di
		value, err := (DLT645{}).ReadPoint(ctx, p)
		if err != nil || value.Value != float64(123) {
			t.Fatalf("DI=%s value=%v err=%v", di, value.Value, err)
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

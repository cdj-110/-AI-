//go:build linux

package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

// End-to-end regression: three real serial transactions exceed the schedule
// deadline, but each response arrives within its own configured timeout.
func TestDLT645SlowSerialRoundOutlivesSamplingInterval(t *testing.T) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Skipf("PTY unavailable: %v", err)
	}
	master := os.NewFile(uintptr(fd), "dlt645-test-master")
	defer master.Close()
	var lock, number uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&lock))); errno != 0 {
		t.Fatal(errno)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&number))); errno != 0 {
		t.Fatal(errno)
	}
	address := fmt.Sprintf("/dev/pts/%d", number)
	// Keep the slave open until the collector connects, avoiding PTY EIO.
	slave, err := os.OpenFile(address, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	collector.CloseDLT645Connections()
	defer collector.CloseDLT645Connections()
	serverDone := make(chan error, 1)
	go func() {
		for i := 0; i < 3; i++ {
			request := make([]byte, 20)
			if _, err := io.ReadFull(master, request); err != nil {
				serverDone <- err
				return
			}
			time.Sleep(120 * time.Millisecond)
			// Strip the four FE bytes; echo address and encoded DI, append BCD 123.
			response := append([]byte{}, request[4:12]...)
			response = append(response, 0x91, 8)
			response = append(response, request[14:18]...)
			response = append(response, 0x56, 0x34, 0x33, 0x33)
			var checksum byte
			for _, b := range response {
				checksum += b
			}
			response = append(response, checksum, 0x16)
			if _, err := master.Write(response); err != nil {
				serverDone <- err
				return
			}
		}
		serverDone <- nil
	}()
	cfg := config.Config{CollectIntervalSeconds: 1}
	for i := 0; i < 3; i++ {
		cfg.Points = append(cfg.Points, config.PointConfig{DeviceKey: "meter", ChannelKey: "serial", Metric: fmt.Sprintf("p%d", i), Protocol: "dlt645", Address: address, BaudRate: 9600, DataBits: 8, StopBits: 1, Parity: "none", Scale: 1,
			DLT645: &config.DLT645Config{Version: "2007", MeterAddress: "000000000001", DataIdentifier: "00000000", ValueFormat: "bcd", DataLength: 4, ResponseTimeoutMilliseconds: 500, Retries: 1, WakeupBytes: 4, Access: "read"}})
	}
	store := state.New(cfg)
	manager := NewManager(cfg, store)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	batch := manager.collectPointSet(ctx, cfg.Points, 100*time.Millisecond, true)
	if len(batch.Points["meter"]) != 3 {
		t.Fatalf("round lost points: %+v; statuses=%+v", batch, store.PointStatuses())
	}
	if time.Since(start) < 360*time.Millisecond {
		t.Fatal("simulated serial timing not exercised")
	}
	for _, status := range store.PointStatuses() {
		if status.Error != "" {
			t.Fatalf("false error: %+v", status)
		}
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("simulator did not finish")
	}
}

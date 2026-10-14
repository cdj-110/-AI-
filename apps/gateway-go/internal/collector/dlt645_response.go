package collector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/goburrow/serial"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// Read replies carry a DI; write acknowledgements and read-address replies do
// not use this shape. Exceptional responses are handled before this matcher.
func matchDLT645ReadResponse(version string, request, response dlt645Frame) error {
	diLength := 4
	if version == "1997" {
		diLength = 2
	}
	switch request.Control & 0x1f {
	case 0x01, 0x02, 0x11, 0x12:
	default:
		return nil
	}
	if len(request.Data) < diLength || len(response.Data) < diLength {
		return errors.New("已忽略：读响应缺少完整的数据标识")
	}
	if !bytes.Equal(request.Data[:diLength], response.Data[:diLength]) {
		return fmt.Errorf("已忽略：响应数据标识不匹配（请求 DI=%s，收到 DI=%s），继续等待当前点位", dlt645DIText(request.Data[:diLength]), dlt645DIText(response.Data[:diLength]))
	}
	return nil
}

func dlt645DIText(di []byte) string {
	reversed := append([]byte(nil), di...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return fmt.Sprintf("%X", reversed)
}

// Caller holds the physical bus lock. Reopening a port alone cannot remove
// replies still in flight at the meter/USB adapter. Drain for one previous
// response window, then require 100ms silence. Noise may extend this by at most
// one second; on failure keep the quarantine and never send into the noise.
// This is recovery time, not part of the next request's response deadline.
// DL/T 645 has no transaction ID: an arbitrarily late reply for the SAME DI
// cannot be distinguished from a new reply. This bounded guard reduces that
// ambiguity; it cannot promise freshness for unbounded device delays.
func drainDLT645Residual(ctx context.Context, bus *dlt645Bus, point config.PointConfig, version string, control byte) error {
	if bus.resyncWindow <= 0 {
		return nil
	}
	start := time.Now()
	minimum := start.Add(bus.resyncWindow)
	deadline := minimum.Add(time.Second)
	lastByte := start
	chunk := make([]byte, 256)
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("清理串口残留报文时已取消，尚未发送新请求: %w", err)
		}
		now := time.Now()
		if !now.Before(minimum) && now.Sub(lastByte) >= 100*time.Millisecond {
			bus.resyncWindow = 0
			bus.resyncSensitive = false
			return nil
		}
		if !now.Before(deadline) {
			failure := errors.New("串口残留报文持续到达，未达到静默条件；尚未发送新请求")
			recordDLT645Frame(point, version, "rx", nil, control, failure)
			return failure
		}
		n, err := bus.port.Read(chunk)
		if n > 0 {
			lastByte = time.Now()
			raw := chunk[:n]
			if bus.resyncSensitive {
				raw = nil // A partial write echo can contain credentials.
			}
			recordDLT645Frame(point, version, "rx", raw, control, errors.New("已丢弃：恢复等待期间收到的迟到或残留报文，尚未发送新请求"))
		}
		if err != nil && !errors.Is(err, serial.ErrTimeout) && !errors.Is(err, io.EOF) {
			return fmt.Errorf("清理串口残留报文失败，尚未发送新请求: %w", err)
		}
		// Some drivers/test ports return immediately on an empty read.
		if n == 0 {
			timer := time.NewTimer(time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
}

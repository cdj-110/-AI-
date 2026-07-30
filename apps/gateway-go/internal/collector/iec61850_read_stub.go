//go:build !iec61850_mms || !cgo

package collector

import (
	"context"
	"fmt"
	"net"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func readIEC61850Point(ctx context.Context, point config.PointConfig, objectRef string, fc string) (interface{}, error) {
	return nil, fmt.Errorf("IEC61850 MMS read is not built in; rebuild with CGO_ENABLED=1 and -tags iec61850_mms after installing libiec61850")
}

func testIEC61850Association(ctx context.Context, address string) error {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("IEC61850 TCP connection failed %s: %w", address, err)
	}
	_ = conn.Close()
	return fmt.Errorf("IEC61850 TCP port is reachable, but this build cannot verify the MMS association")
}

func CloseIEC61850Connections() {}

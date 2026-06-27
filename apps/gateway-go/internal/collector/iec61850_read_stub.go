//go:build !iec61850_mms || !cgo

package collector

import (
	"context"
	"fmt"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func readIEC61850Point(ctx context.Context, point config.PointConfig, objectRef string, fc string) (interface{}, error) {
	return nil, fmt.Errorf("IEC61850 MMS read is not built in; rebuild with CGO_ENABLED=1 and -tags iec61850_mms after installing libiec61850")
}

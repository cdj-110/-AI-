//go:build !linux

package collector

import (
	"context"
	"fmt"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func readIEC61850GOOSEPoint(context.Context, config.PointConfig) (interface{}, error) {
	return nil, fmt.Errorf("IEC61850 GOOSE raw Ethernet collection is currently supported on Linux gateways")
}

func CloseIEC61850GOOSEConnections() {}

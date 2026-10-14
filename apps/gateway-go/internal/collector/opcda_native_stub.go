//go:build !opcda || !cgo

package collector

import (
	"context"
	"fmt"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func readOPCDAPoints(_ context.Context, points []config.PointConfig) []PointReadResult {
	results := make([]PointReadResult, len(points))
	for index := range points {
		results[index] = PointReadResult{
			Index: index,
			Err:   fmt.Errorf("OPC DA native runtime is not built in; rebuild with CGO_ENABLED=1 and -tags opcda after installing libwk_opcda"),
		}
	}
	return results
}

func closeOPCDASessions() {}

func closeOPCDASessionsForDevices(_ []string) {}

func closeOPCDABrowseSession(_ string) {}

func testOPCDAConnection(_ context.Context, _ config.PointConfig) error {
	return fmt.Errorf("OPC DA native runtime is not built in; rebuild with CGO_ENABLED=1 and -tags opcda after installing libwk_opcda")
}

func browseOPCDANodes(_ context.Context, _ config.PointConfig, _ string, _ int) ([]OPCDABrowseNode, error) {
	return nil, fmt.Errorf("OPC DA native runtime is not built in; rebuild with CGO_ENABLED=1 and -tags opcda after installing libwk_opcda")
}

func writeOPCDAPoint(_ context.Context, _ config.PointConfig, _ interface{}) error {
	return fmt.Errorf("OPC DA native runtime is not built in; rebuild with CGO_ENABLED=1 and -tags opcda after installing libwk_opcda")
}

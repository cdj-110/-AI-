//go:build !linux

package forward

import (
	"context"
	"fmt"
)

func (m *Manager) serveIEC61850GOOSE(context.Context) error {
	return fmt.Errorf("IEC61850 GOOSE 原始以太网转发当前仅支持 Linux 网关")
}

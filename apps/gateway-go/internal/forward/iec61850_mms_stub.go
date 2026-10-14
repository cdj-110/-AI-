//go:build !iec61850_mms || !cgo

package forward

import (
	"context"
	"fmt"
)

func (m *Manager) serveIEC61850MMS(context.Context, string) error {
	return fmt.Errorf("IEC61850 MMS 转发未编译，请使用 iec61850_mms 构建标签和 libiec61850 重新构建")
}

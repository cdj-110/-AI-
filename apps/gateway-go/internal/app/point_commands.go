package app

import (
	"context"
	"fmt"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/cloud"
)

func (a *App) applyPointCommand(command cloud.PointCommand) cloud.PointCommandResult {
	return a.pointCommands.Execute(command, func() error {
		cfg := a.runtime.Config()
		if cfg.License.Restricted || !cfg.Activation.AllowsFeature("cloud") || !cfg.Activation.AllowsFeature("collection") || !cfg.CloudConnectionIsEnabled() {
			return fmt.Errorf("网关未授权或未启用云平台连接，拒绝下发")
		}
		if !command.ExpiresAt.After(time.Now()) {
			return fmt.Errorf("指令已过期，未执行")
		}
		allowed := false
		for _, point := range a.runtime.WritablePoints() {
			if point["deviceKey"] == command.DeviceKey && point["metric"] == command.Metric {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("点位未启用或不支持写入")
		}
		ctx, cancel := context.WithDeadline(context.Background(), command.ExpiresAt)
		defer cancel()
		ctx, cancelWrite := context.WithTimeout(ctx, 5*time.Second)
		defer cancelWrite()
		// Always include the child key: identical metrics in other children are
		// not candidates, and the normal runtime write validation stays in force.
		_, err := a.runtime.WritePoint(ctx, command.DeviceKey, command.Metric, command.Value)
		return err
	})
}

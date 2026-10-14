package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"weikong-iot-platform/apps/gateway-go/internal/cloud"
)

var scheduleRemoteMaintenance = scheduleRemoteMaintenanceAction

func (a *App) applyMaintenance(command cloud.MaintenanceCommand) cloud.MaintenanceResult {
	if command.HardwareID != a.cfg.Activation.HardwareID {
		return cloud.MaintenanceResult{TaskID: command.TaskID, HardwareID: a.cfg.Activation.HardwareID, Action: command.Action, Status: "FAILED", Message: "维护任务与本机身份不符"}
	}
	return a.maintenanceCommands.Execute(command, scheduleRemoteMaintenance)
}

func scheduleRemoteMaintenanceAction(action string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("本地 Windows 测试环境禁止远程重启")
	}
	var script string
	switch action {
	case cloud.MaintenanceServiceRestart:
		if _, err := os.Stat("/etc/init.d/S99weikong-gateway"); err != nil {
			return fmt.Errorf("网关服务启动脚本不可用")
		}
		script = "sleep 3; exec /etc/init.d/S99weikong-gateway restart"
	case cloud.MaintenanceGatewayReboot:
		if _, err := exec.LookPath("reboot"); err != nil {
			return fmt.Errorf("网关系统不支持 reboot 命令")
		}
		script = "sleep 3; exec reboot"
	default:
		return fmt.Errorf("不支持的远程维护操作")
	}
	command := exec.Command("sh", "-c", script)
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return fmt.Errorf("安排远程维护失败: %w", err)
	}
	return command.Process.Release()
}

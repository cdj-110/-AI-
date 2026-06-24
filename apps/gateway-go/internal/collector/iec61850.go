package collector

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type IEC61850 struct{}

type IEC61850ConnectionResult struct {
	Address string
}

func (IEC61850) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	objectRef := strings.TrimSpace(point.ObjectRef)
	if objectRef == "" {
		return model.PointValue{}, fmt.Errorf("IEC61850 对象引用不能为空，例如 LD0/LLN0.Mod.stVal")
	}
	return model.PointValue{}, fmt.Errorf("IEC61850 MMS 读取暂未启用：已完成配置入口和连接测试，请接入 MMS 协议栈后读取 %s[%s]", objectRef, valueOrDefault(point.FC, "ST"))
}

func TestIEC61850Connection(ctx context.Context, address string) (IEC61850ConnectionResult, error) {
	address = iec61850Address(address)
	result := IEC61850ConnectionResult{Address: address}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return result, fmt.Errorf("IEC61850 连接失败 %s：%w", address, err)
	}
	_ = conn.Close()
	return result, nil
}

func iec61850Address(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return "127.0.0.1:102"
	}
	if strings.Contains(address, ":") {
		return address
	}
	return address + ":102"
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

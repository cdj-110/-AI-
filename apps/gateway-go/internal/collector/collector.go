package collector

import (
	"context"
	"fmt"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type Collector interface {
	ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error)
}

type RegisterRangeReader interface {
	ReadRegisterRange(ctx context.Context, point config.PointConfig, start uint16, quantity uint16) ([]byte, error)
}

func New(protocol string) (Collector, error) {
	switch protocol {
	case "modbus-tcp":
		return ModbusTCP{}, nil
	case "modbus-rtu":
		return ModbusRTU{}, nil
	case "siemens-s7":
		return SiemensS7{}, nil
	case "iec104":
		return IEC104{}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol %s", protocol)
	}
}

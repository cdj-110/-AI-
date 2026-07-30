package collector

import (
	"context"
	"errors"
	"fmt"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

// ErrCollectionDeferred means collection did not reach the device because a
// higher-priority operation (normally a control write) occupied the connection.
// Callers should retain the previous value instead of reporting a link fault.
var ErrCollectionDeferred = errors.New("collection deferred by a higher-priority operation")

type Collector interface {
	ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error)
}

type PointWriter interface {
	WritePoint(ctx context.Context, point config.PointConfig, value interface{}) error
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
	case "opcua":
		return OPCUA{}, nil
	case "iec61850-goose":
		return IEC61850GOOSE{}, nil
	default:
		if collector, ok := newOptionalCollector(protocol); ok {
			return collector, nil
		}
		return nil, fmt.Errorf("unsupported protocol %s", protocol)
	}
}

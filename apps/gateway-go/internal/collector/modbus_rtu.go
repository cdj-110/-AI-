package collector

import (
	"context"
	"time"

	"github.com/goburrow/modbus"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/mapper"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type ModbusRTU struct{}

func (ModbusRTU) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	handler := modbus.NewRTUClientHandler(point.Address)
	handler.SlaveId = point.SlaveID
	handler.Timeout = 3 * time.Second
	if err := handler.Connect(); err != nil {
		return model.PointValue{}, err
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	raw, err := readByFunction(ctx, client, point)
	if err != nil {
		return model.PointValue{}, err
	}
	value, err := mapper.Decode(point, raw)
	if err != nil {
		return model.PointValue{}, err
	}
	return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
}

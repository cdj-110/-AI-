package collector

import (
	"context"
	"fmt"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

// IEC61850GOOSE reads data set members from a persistent layer-2 GOOSE
// subscription. Point GooseIndex is zero based and follows the data set order.
type IEC61850GOOSE struct{}

func (IEC61850GOOSE) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	if strings.TrimSpace(point.Address) == "" {
		return model.PointValue{}, fmt.Errorf("IEC61850 GOOSE interface is required")
	}
	if strings.TrimSpace(point.GoCBRef) == "" {
		return model.PointValue{}, fmt.Errorf("IEC61850 GOOSE goCbRef is required")
	}
	value, err := readIEC61850GOOSEPoint(ctx, point)
	if err != nil {
		return model.PointValue{}, err
	}
	return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
}

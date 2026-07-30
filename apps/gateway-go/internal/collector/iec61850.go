package collector

import (
	"context"
	"fmt"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type IEC61850 struct{}

type IEC61850ConnectionResult struct {
	Address string
}

type IEC61850BrowseNode struct {
	Name      string               `json:"name"`
	ObjectRef string               `json:"objectRef"`
	FC        string               `json:"fc"`
	DataType  string               `json:"dataType,omitempty"`
	Leaf      bool                 `json:"leaf"`
	Children  []IEC61850BrowseNode `json:"children,omitempty"`
}

type IEC61850ProbeResult struct {
	ObjectRef string `json:"objectRef"`
	FC        string `json:"fc"`
	Exists    bool   `json:"exists"`
}

func (IEC61850) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	objectRef := strings.TrimSpace(point.ObjectRef)
	if objectRef == "" {
		return model.PointValue{}, fmt.Errorf("IEC61850 objectRef is required, for example LD0/LLN0.Mod.stVal")
	}
	value, err := readIEC61850Point(ctx, point, objectRef, valueOrDefault(point.FC, "ST"))
	if err != nil {
		return model.PointValue{}, err
	}
	return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: value}, nil
}

func TestIEC61850Connection(ctx context.Context, address string) (IEC61850ConnectionResult, error) {
	address = iec61850Address(address)
	result := IEC61850ConnectionResult{Address: address}
	return result, testIEC61850Association(ctx, address)
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

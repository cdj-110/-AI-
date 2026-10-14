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

const IEC61850BatchSize = 32

func (IEC61850) BatchKey(point config.PointConfig) string {
	return iec61850Address(point.Address)
}

func (IEC61850) ReadPoints(ctx context.Context, points []config.PointConfig) []PointReadResult {
	return readIEC61850Points(ctx, points)
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

// iec61850MMSReference converts an IEC 61850 object reference such as
// LD0/LLN0.Mod.stVal[ST] to its MMS domain and variable names
// LD0 / LLN0$ST$Mod$stVal. All variables in one MMS multiple-read request
// must belong to the same domain.
func iec61850MMSReference(objectRef, fc string) (string, string, error) {
	objectRef = strings.TrimSpace(objectRef)
	slash := strings.IndexByte(objectRef, '/')
	if slash <= 0 || slash == len(objectRef)-1 {
		return "", "", fmt.Errorf("IEC61850 objectRef %q is invalid", objectRef)
	}
	domain := objectRef[:slash]
	path := objectRef[slash+1:]
	if len(domain) > 64 || len(path) > 64 || strings.ContainsAny(path, "()") {
		return "", "", fmt.Errorf("IEC61850 objectRef %q cannot be batch read", objectRef)
	}
	fc = strings.ToUpper(strings.TrimSpace(fc))
	if len(fc) != 2 {
		return "", "", fmt.Errorf("IEC61850 functional constraint %q is invalid", fc)
	}
	dot := strings.IndexByte(path, '.')
	if dot < 1 {
		return domain, path + "$" + fc, nil
	}
	item := path[:dot] + "$" + fc + "$" + strings.ReplaceAll(path[dot+1:], ".", "$")
	return domain, item, nil
}

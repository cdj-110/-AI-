package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestDLT645LanesSharePhysicalBusAcrossMeters(t *testing.T) {
	cfg := config.Config{CollectIntervalSeconds: 2, Points: []config.PointConfig{
		{DeviceKey: "a", ChannelKey: "c1", Metric: "p1", Protocol: "dlt645", Address: "/dev/ttyS2"},
		{DeviceKey: "c", ChannelKey: "c3", Metric: "p3", Protocol: "dlt645", Address: "/dev/ttyS3"},
		{DeviceKey: "b", ChannelKey: "c2", Metric: "p2", Protocol: "dlt645", Address: "/dev/ttyS2"},
	}}
	lanes, order := buildCollectionLanes(cfg)
	if len(order) != 2 || len(lanes["dlt645::/dev/ttyS2"].Points) != 2 {
		t.Fatalf("lanes: %+v", lanes)
	}
}
func TestDLT645TaskIgnoresExpiredScheduleButHonorsCaller(t *testing.T) {
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	parent, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	points := []config.PointConfig{{DeviceKey: "a", Metric: "p", Protocol: "dlt645", Address: "/nonexistent/dlt645-test", DLT645: &config.DLT645Config{Version: "2007", MeterAddress: "000000000001", DataIdentifier: "00000000"}}}
	results := collectPointsWithDLTContext(expired, parent, points)
	if len(results) != 1 || results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "打开 DL/T 645 串口") {
		t.Fatalf("serial task inherited expired schedule: %+v", results)
	}
}

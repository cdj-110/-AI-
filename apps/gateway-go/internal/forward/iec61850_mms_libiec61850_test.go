//go:build iec61850_mms && cgo

package forward

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestIEC61850MMSForwardServerCanBeBrowsedAndRead(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	address := fmt.Sprintf("127.0.0.1:%d", port)

	cfg := config.Config{
		GatewayKey: "gw",
		Devices: []config.DeviceConfig{{
			DeviceKey: "source", Name: "source", Protocol: "modbus-tcp", Address: "127.0.0.1:502",
			Points: []config.PointConfig{{
				DeviceKey: "source", Name: "temperature", Metric: "temperature", Protocol: "modbus-tcp",
				Function: 3, Quantity: 2, DataType: "float32", Scale: 1,
			}},
		}},
		ForwardSlave: config.FeatureConfig{
			Enabled:        true,
			IEC61850Listen: address,
		},
		ForwardDevices: []config.ForwardDeviceConfig{{
			DeviceKey: "mms-forward", Protocol: ProtocolIEC61850MMS, IEDName: "WEIKONG", LogicalDevice: "LD1",
			Points: []config.ForwardPointConfig{{
				Name: "temperature", Metric: "temperature", SourceDeviceKey: "source", SourceMetric: "temperature",
				DataType: "float32", ObjectRef: "WEIKONGLD1/GGIO1.Temperature.mag.f", FC: "MX",
			}},
		}},
	}
	cfg.ApplyDefaults()
	store := state.New(cfg)
	store.SetPointValue("source", "temperature", 23.5)
	manager := NewManager(store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Update(ctx, cfg)
	defer manager.Stop()
	defer collector.CloseIEC61850Connections()

	var nodes []collector.IEC61850BrowseNode
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		nodes, err = collector.BrowseIEC61850Nodes(context.Background(), address, "", true)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("browse IEC61850 MMS forward server: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("IEC61850 MMS forward server returned no model nodes")
	}
	refs := map[string]bool{}
	var collectRefs func([]collector.IEC61850BrowseNode)
	collectRefs = func(items []collector.IEC61850BrowseNode) {
		for _, item := range items {
			refs[item.ObjectRef] = true
			collectRefs(item.Children)
		}
	}
	collectRefs(nodes)
	for _, want := range []string{
		"WEIKONGLD1/LLN0",
		"WEIKONGLD1/LPHD1",
		"WEIKONGLD1/GGIO1.Temperature.mag.f",
		"WEIKONGLD1/GGIO1.Temperature.q",
		"WEIKONGLD1/GGIO1.Temperature.t",
	} {
		if !refs[want] {
			t.Errorf("IEC61850 model missing %s", want)
		}
	}

	driver, err := collector.New("iec61850")
	if err != nil {
		t.Fatal(err)
	}
	value, err := driver.ReadPoint(context.Background(), config.PointConfig{
		DeviceKey: "client", Metric: "temperature", Protocol: "iec61850", Address: address,
		ObjectRef: "WEIKONGLD1/GGIO1.Temperature.mag.f", FC: "MX", DataType: "float32", Scale: 1,
	})
	if err != nil {
		t.Fatalf("read forwarded IEC61850 value: %v", err)
	}
	if got, ok := value.Value.(float64); !ok || got != 23.5 {
		t.Fatalf("forwarded IEC61850 value = %#v, want 23.5", value.Value)
	}
	if _, err := collector.TestIEC61850Connection(context.Background(), address); err != nil {
		t.Fatalf("full MMS connection test failed: %v", err)
	}
}

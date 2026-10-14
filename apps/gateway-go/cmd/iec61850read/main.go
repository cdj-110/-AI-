package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
)

func main() {
	address := flag.String("address", "127.0.0.1:102", "IEC61850 server address")
	objectRef := flag.String("object", "", "IEC61850 object reference")
	fc := flag.String("fc", "MX", "functional constraint")
	timeout := flag.Duration("timeout", 10*time.Second, "read timeout")
	flag.Parse()

	if *objectRef == "" {
		log.Fatal("object reference is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	value, err := gatewayruntime.ReadPoint(ctx, config.PointConfig{
		Protocol:  "iec61850",
		Address:   *address,
		ObjectRef: *objectRef,
		FC:        *fc,
		DataType:  "auto",
		Scale:     1,
	})
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}

	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Fatalf("encode result failed: %v", err)
	}
	fmt.Fprintln(os.Stdout, string(payload))
}

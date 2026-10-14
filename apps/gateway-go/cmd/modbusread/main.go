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
	protocol := flag.String("protocol", "modbus-tcp", "modbus-tcp or modbus-rtu")
	address := flag.String("address", "127.0.0.1:502", "device address")
	slaveID := flag.Int("slave", 1, "slave id")
	function := flag.Int("function", 3, "modbus function code")
	register := flag.Int("register", 0, "register/coil address")
	quantity := flag.Int("quantity", 1, "quantity")
	dataType := flag.String("type", "uint16", "data type")
	timeout := flag.Duration("timeout", 5*time.Second, "read timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	value, err := gatewayruntime.ReadPoint(ctx, config.PointConfig{
		Protocol:  *protocol,
		Address:   *address,
		SlaveID:   uint8(*slaveID),
		Function:  uint8(*function),
		Register:  uint16(*register),
		Quantity:  uint16(*quantity),
		DataType:  *dataType,
		ByteOrder: "big",
		WordOrder: "normal",
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

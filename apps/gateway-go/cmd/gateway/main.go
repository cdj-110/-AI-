package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"weikong-iot-platform/apps/gateway-go/internal/app"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func main() {
	configPath := flag.String("config", "config.local.json", "gateway config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.New(cfg, *configPath).Run(ctx); err != nil {
		log.Fatalf("gateway stopped: %v", err)
	}
}

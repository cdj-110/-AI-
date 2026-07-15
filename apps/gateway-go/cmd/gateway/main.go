package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/app"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const restartDelayEnv = "GATEWAY_RESTART_DELAY_MS"

func main() {
	if delayText := os.Getenv(restartDelayEnv); delayText != "" {
		_ = os.Unsetenv(restartDelayEnv)
		if delayMS, err := strconv.Atoi(delayText); err == nil && delayMS > 0 {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
		}
	}

	configPath := flag.String("config", "config.local.json", "gateway config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(cfg, *configPath)
	if err != nil {
		log.Fatalf("initialize gateway failed: %v", err)
	}
	if err := application.Run(ctx); err != nil {
		log.Fatalf("gateway stopped: %v", err)
	}
}

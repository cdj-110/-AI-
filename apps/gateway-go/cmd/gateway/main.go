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
	"weikong-iot-platform/apps/gateway-go/internal/supervisor"
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
	supervise := flag.Bool("supervise", false, "run the gateway under the software and hardware watchdog supervisor")
	checkConfig := flag.Bool("check-config", false, "validate the configuration and exit")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	if *checkConfig {
		log.Printf("configuration is valid: %s", *configPath)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *supervise && cfg.Watchdog.Enabled {
		executable, err := os.Executable()
		if err != nil {
			log.Fatalf("resolve gateway executable failed: %v", err)
		}
		if err := supervisor.Run(ctx, supervisor.OptionsFromConfig(executable, *configPath, cfg)); err != nil {
			log.Fatalf("gateway supervisor stopped: %v", err)
		}
		return
	}
	if *supervise {
		log.Printf("watchdog supervisor is disabled by configuration; running gateway directly")
	}

	application, err := app.New(cfg, *configPath)
	if err != nil {
		log.Fatalf("initialize gateway failed: %v", err)
	}
	if err := application.Run(ctx); err != nil {
		log.Fatalf("gateway stopped: %v", err)
	}
}

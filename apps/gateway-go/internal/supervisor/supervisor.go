package supervisor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const supervisedEnv = "GATEWAY_SUPERVISED"

type Options struct {
	Executable        string
	ConfigPath        string
	HealthURL         string
	HealthTLSInsecure bool
	HardwareEnabled   bool
	HardwareDevice    string
	HardwareTimeout   time.Duration
	FeedInterval      time.Duration
	HealthInterval    time.Duration
	HealthTimeout     time.Duration
	StartupGrace      time.Duration
	FailureThreshold  int
	RestartLimit      int
	RestartWindow     time.Duration
	SafeModeFile      string
	Stdout            io.Writer
	Stderr            io.Writer
}

func OptionsFromConfig(executable, configPath string, cfg config.Config) Options {
	watchdog := cfg.Watchdog
	watchdog.ApplyDefaults()
	listen := cfg.Web.Listen
	if listen == "" {
		listen = "127.0.0.1:8088"
	} else if strings.HasPrefix(listen, "0.0.0.0:") {
		listen = "127.0.0.1:" + strings.TrimPrefix(listen, "0.0.0.0:")
	} else if strings.HasPrefix(listen, ":") {
		listen = "127.0.0.1" + listen
	} else if strings.HasPrefix(listen, "[::]:") {
		listen = "127.0.0.1:" + strings.TrimPrefix(listen, "[::]:")
	}
	scheme := "http"
	if cfg.Web.TLSCertFile != "" && cfg.Web.TLSKeyFile != "" {
		scheme = "https"
	}
	return Options{
		Executable:        executable,
		ConfigPath:        configPath,
		HealthURL:         scheme + "://" + listen + "/api/healthz",
		HealthTLSInsecure: scheme == "https",
		HardwareEnabled:   watchdog.HardwareEnabled,
		HardwareDevice:    watchdog.Device,
		HardwareTimeout:   time.Duration(watchdog.HardwareTimeoutSec) * time.Second,
		FeedInterval:      time.Duration(watchdog.FeedIntervalSec) * time.Second,
		HealthInterval:    time.Duration(watchdog.HealthCheckSec) * time.Second,
		HealthTimeout:     time.Duration(watchdog.HealthTimeoutSec) * time.Second,
		StartupGrace:      time.Duration(watchdog.StartupGraceSec) * time.Second,
		FailureThreshold:  watchdog.FailureThreshold,
		RestartLimit:      watchdog.RestartLimit,
		RestartWindow:     time.Duration(watchdog.RestartWindowSec) * time.Second,
		SafeModeFile:      watchdog.SafeModeFile,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
	}
}

type healthResponse struct {
	Healthy bool `json:"healthy"`
}

type childProcess struct {
	command *exec.Cmd
	done    chan error
}

type hardwareWatchdog interface {
	Feed() error
	Disarm() error
}

func Run(ctx context.Context, options Options) error {
	if err := validateOptions(options); err != nil {
		return err
	}
	if _, err := os.Stat(options.SafeModeFile); err == nil {
		log.Printf("watchdog safe mode active file=%s; hardware watchdog and automatic restarts are disabled", options.SafeModeFile)
		return runChildOnce(ctx, options)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read watchdog safe mode file: %w", err)
	}

	var hardware hardwareWatchdog
	if options.HardwareEnabled {
		opened, err := openHardwareWatchdog(options.HardwareDevice, options.HardwareTimeout)
		if err != nil {
			return fmt.Errorf("enable hardware watchdog: %w", err)
		}
		hardware = opened
		defer func() {
			if err := hardware.Disarm(); err != nil {
				log.Printf("disarm hardware watchdog failed: %v", err)
			}
		}()
		log.Printf("hardware watchdog enabled device=%s timeout=%s", options.HardwareDevice, options.HardwareTimeout)
	}

	child, err := startChild(options)
	if err != nil {
		return err
	}
	log.Printf("gateway child started pid=%d", child.command.Process.Pid)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if options.HealthTLSInsecure {
		// This connection never leaves loopback; certificate validation remains
		// enforced for browser and remote clients.
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402
	}
	client := &http.Client{Timeout: options.HealthTimeout, Transport: transport}
	ticker := time.NewTicker(options.HealthInterval)
	defer ticker.Stop()
	startupUntil := time.Now().Add(options.StartupGrace)
	lastFeed := time.Time{}
	failures := 0
	var restarts []time.Time

	feed := func(now time.Time) error {
		if hardware == nil || (!lastFeed.IsZero() && now.Sub(lastFeed) < options.FeedInterval) {
			return nil
		}
		if err := hardware.Feed(); err != nil {
			return fmt.Errorf("feed hardware watchdog: %w", err)
		}
		lastFeed = now
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			stopChild(child, 10*time.Second)
			return nil
		case err := <-child.done:
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("gateway child exited: %v", err)
			if !allowRestart(&restarts, time.Now(), options.RestartLimit, options.RestartWindow) {
				return enterSafeMode(ctx, options, hardware, "gateway exited too often")
			}
			child, err = startChild(options)
			if err != nil {
				return enterSafeMode(ctx, options, hardware, "gateway restart failed: "+err.Error())
			}
			log.Printf("gateway child restarted pid=%d", child.command.Process.Pid)
			startupUntil = time.Now().Add(options.StartupGrace)
			failures = 0
		case now := <-ticker.C:
			if now.Before(startupUntil) {
				if err := feed(now); err != nil {
					stopChild(child, 10*time.Second)
					return err
				}
				continue
			}
			if checkHealth(ctx, client, options.HealthURL) {
				failures = 0
				if err := feed(now); err != nil {
					stopChild(child, 10*time.Second)
					return err
				}
				continue
			}
			failures++
			log.Printf("gateway health check failed count=%d/%d", failures, options.FailureThreshold)
			if failures < options.FailureThreshold {
				continue
			}
			stopChild(child, 10*time.Second)
			if !allowRestart(&restarts, now, options.RestartLimit, options.RestartWindow) {
				return enterSafeMode(ctx, options, hardware, "gateway health check failed too often")
			}
			child, err = startChild(options)
			if err != nil {
				return enterSafeMode(ctx, options, hardware, "gateway restart failed: "+err.Error())
			}
			log.Printf("gateway child restarted after failed health check pid=%d", child.command.Process.Pid)
			startupUntil = time.Now().Add(options.StartupGrace)
			failures = 0
		}
	}
}

func validateOptions(options Options) error {
	if options.Executable == "" || options.ConfigPath == "" {
		return errors.New("watchdog executable and config path are required")
	}
	if options.HealthURL == "" || options.HealthInterval <= 0 || options.HealthTimeout <= 0 || options.StartupGrace <= 0 {
		return errors.New("watchdog health URL and positive timing values are required")
	}
	if options.FailureThreshold <= 0 || options.RestartLimit <= 0 || options.RestartWindow <= 0 {
		return errors.New("watchdog failure and restart limits must be positive")
	}
	if options.HardwareEnabled && (options.HardwareDevice == "" || options.HardwareTimeout <= options.FeedInterval*2) {
		return errors.New("hardware watchdog timeout must be more than twice the feed interval")
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	return nil
}

func runChildOnce(ctx context.Context, options Options) error {
	child, err := startChild(options)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		stopChild(child, 10*time.Second)
		return nil
	case err := <-child.done:
		return err
	}
}

func startChild(options Options) (*childProcess, error) {
	command := exec.Command(options.Executable, "-config", options.ConfigPath)
	command.Env = append(os.Environ(), supervisedEnv+"=1")
	command.Stdout = options.Stdout
	command.Stderr = options.Stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start gateway child: %w", err)
	}
	child := &childProcess{command: command, done: make(chan error, 1)}
	go func() { child.done <- command.Wait() }()
	return child, nil
}

func stopChild(child *childProcess, timeout time.Duration) {
	if child == nil || child.command == nil || child.command.Process == nil {
		return
	}
	_ = child.command.Process.Signal(os.Interrupt)
	select {
	case <-child.done:
		return
	case <-time.After(timeout):
		_ = child.command.Process.Kill()
		select {
		case <-child.done:
		case <-time.After(time.Second):
		}
	}
}

func checkHealth(parent context.Context, client *http.Client, healthURL string) bool {
	request, err := http.NewRequestWithContext(parent, http.MethodGet, healthURL, nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var health healthResponse
	return json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&health) == nil && health.Healthy
}

func allowRestart(restarts *[]time.Time, now time.Time, limit int, window time.Duration) bool {
	cutoff := now.Add(-window)
	kept := (*restarts)[:0]
	for _, restartedAt := range *restarts {
		if restartedAt.After(cutoff) {
			kept = append(kept, restartedAt)
		}
	}
	if len(kept) >= limit {
		*restarts = kept
		return false
	}
	*restarts = append(kept, now)
	return true
}

func enterSafeMode(ctx context.Context, options Options, hardware hardwareWatchdog, reason string) error {
	if err := os.MkdirAll(filepath.Dir(options.SafeModeFile), 0755); err != nil {
		return fmt.Errorf("create watchdog safe mode directory: %w", err)
	}
	content := time.Now().Format(time.RFC3339) + " " + reason + "\n"
	if err := os.WriteFile(options.SafeModeFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write watchdog safe mode file: %w", err)
	}
	log.Printf("watchdog entered safe mode: %s", reason)
	if hardware != nil {
		log.Printf("hardware watchdog will no longer be fed; waiting for board reset")
		<-ctx.Done()
		return nil
	}
	return errors.New(reason)
}

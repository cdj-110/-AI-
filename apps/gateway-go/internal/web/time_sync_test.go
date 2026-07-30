package web

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateTimeSyncConfig(t *testing.T) {
	enabled := true
	valid := timeSyncConfig{Enabled: &enabled, Timezone: "Asia/Shanghai", PrimaryServer: "ntp.example.com", SecondaryServer: "192.168.1.10"}
	if err := validateTimeSyncConfig(valid); err != nil {
		t.Fatalf("valid time sync configuration rejected: %v", err)
	}
	invalid := valid
	invalid.Timezone = "../etc/passwd"
	if err := validateTimeSyncConfig(invalid); err == nil {
		t.Fatal("unsafe timezone should be rejected")
	}
	invalid = valid
	invalid.PrimaryServer = "https://ntp.example.com"
	if err := validateTimeSyncConfig(invalid); err == nil {
		t.Fatal("URL must not be accepted as an NTP server")
	}
}

func TestRenderChronyConfigReplacesExistingSources(t *testing.T) {
	enabled := true
	cfg := timeSyncConfig{Enabled: &enabled, Timezone: "Asia/Shanghai", PrimaryServer: "ntp1.example.com", SecondaryServer: "192.168.1.10"}
	rendered := renderChronyConfig("pool pool.ntp.org iburst\ndriftfile /tmp/drift\nmakestep 1.0 3\n", cfg)
	if strings.Contains(rendered, "pool pool.ntp.org") {
		t.Fatal("existing source was not removed")
	}
	if !strings.Contains(rendered, "server ntp1.example.com iburst") || !strings.Contains(rendered, "server 192.168.1.10 iburst") {
		t.Fatalf("managed sources missing:\n%s", rendered)
	}
	if !strings.Contains(rendered, "driftfile /tmp/drift") || !strings.Contains(rendered, "makestep 1.0 3") {
		t.Fatalf("non-source settings were not preserved:\n%s", rendered)
	}
}

func TestRenderChronyConfigCanDisableSources(t *testing.T) {
	disabled := false
	cfg := timeSyncConfig{Enabled: &disabled, Timezone: "UTC", PrimaryServer: "pool.ntp.org"}
	rendered := renderChronyConfig("server old.example.com iburst\nrtcsync\n", cfg)
	if strings.Contains(rendered, "server old.example.com") || strings.Contains(rendered, "server pool.ntp.org") {
		t.Fatalf("disabled configuration retained active sources:\n%s", rendered)
	}
	if !strings.Contains(rendered, "# NTP synchronization disabled") {
		t.Fatalf("disabled marker missing:\n%s", rendered)
	}
}

func TestFriendlyNTPProbeErrorHidesChronyDiagnostics(t *testing.T) {
	message := friendlyNTPProbeError(
		"pool.ntp.org",
		errors.New("exit status 1"),
		"chronyd version 4.6.1 starting\nDisabled control of system clock\nTimeout reached",
	)
	if !strings.Contains(message, "UDP 123") {
		t.Fatalf("expected actionable timeout message, got %q", message)
	}
	if strings.Contains(message, "Disabled control") || strings.Contains(message, "chronyd version") {
		t.Fatalf("raw chrony diagnostics leaked to the user: %q", message)
	}
}

func TestSaveAndLoadTimeSyncConfig(t *testing.T) {
	enabled := true
	path := t.TempDir() + "/time-sync.json"
	expected := timeSyncConfig{
		Enabled:         &enabled,
		Timezone:        "Asia/Shanghai",
		PrimaryServer:   "192.168.1.10",
		SecondaryServer: "192.168.1.11",
	}
	if err := saveTimeSyncConfig(path, expected); err != nil {
		t.Fatalf("save time sync configuration: %v", err)
	}
	actual, err := loadTimeSyncConfig(path)
	if err != nil {
		t.Fatalf("load time sync configuration: %v", err)
	}
	if !actual.isEnabled() || actual.Timezone != expected.Timezone ||
		actual.PrimaryServer != expected.PrimaryServer || actual.SecondaryServer != expected.SecondaryServer {
		t.Fatalf("unexpected loaded configuration: %#v", actual)
	}
}

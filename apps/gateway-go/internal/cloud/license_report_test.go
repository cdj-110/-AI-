package cloud

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestLicenseReportWhitelistAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	cfg := config.Config{License: config.LicenseStatus{Valid: true}, Activation: config.ActivationConfig{SN: "WK-TEST", HardwareID: strings.Repeat("A", 32), ChipUSID: strings.Repeat("B", 16), LicenseVersion: 2, LicenseSequence: 4, KeyID: "key-1", Signature: "private-signature", DeviceSecret: "private-secret", Broker: "private-broker", CloudAPI: "private-api", RemoteAccessEndpoint: "private-endpoint", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339), Features: []string{"collection", "cloud"}}}
	report := licenseReport(cfg, now)
	if !report.Valid || report.Restricted || report.LicenseSequence != 4 || report.LicenseID != fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.Activation.Signature))) {
		t.Fatal("invalid report")
	}
	raw, _ := json.Marshal(report)
	for _, secret := range []string{"private-signature", "private-secret", "private-broker", "private-api", "private-endpoint", "deviceSecret", "signature", "broker", "cloudApi", "remoteAccessEndpoint"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("report leaked %s", secret)
		}
	}
	report.Features[0] = "modified"
	if cfg.Activation.Features[0] != "collection" {
		t.Fatal("report mutated license")
	}
	expired := licenseReport(cfg, now.Add(time.Hour))
	if expired.Valid || !expired.Restricted {
		t.Fatal("expired license still valid")
	}
	cfg.Activation.ExpiresAt = "invalid"
	if licenseReport(cfg, now).Valid {
		t.Fatal("bad expiry accepted")
	}
	cfg.Activation.ExpiresAt = ""
	cfg.License = config.LicenseStatus{Valid: true}
	perpetual := licenseReport(cfg, now.AddDate(100, 0, 0))
	if !perpetual.Valid || perpetual.Restricted || perpetual.ExpiresAt != "" {
		t.Fatal("perpetual license reported expired")
	}
	cfg.License = config.LicenseStatus{Restricted: true}
	cfg.Activation.ExpiresAt = ""
	if licenseReport(cfg, now).Valid {
		t.Fatal("restricted license reported valid")
	}
}

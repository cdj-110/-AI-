package cloud

import (
	"crypto/sha256"
	"fmt"
	"time"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// Only public metadata is sent, never the activation package or credentials.
type LicenseReport struct {
	SN              string   `json:"sn"`
	HardwareID      string   `json:"hardwareId"`
	ChipUSID        string   `json:"chipUsid"`
	LicenseVersion  int      `json:"licenseVersion"`
	LicenseSequence int64    `json:"licenseSequence"`
	KeyID           string   `json:"keyId"`
	LicenseID       string   `json:"licenseId"`
	IssuedAt        string   `json:"issuedAt"`
	ExpiresAt       string   `json:"expiresAt"`
	Features        []string `json:"features"`
	Valid           bool     `json:"valid"`
	Restricted      bool     `json:"restricted"`
}

func licenseReport(cfg config.Config, now time.Time) LicenseReport {
	a := cfg.Activation
	report := LicenseReport{SN: a.SN, HardwareID: a.HardwareID, ChipUSID: a.ChipUSID, LicenseVersion: a.LicenseVersion, LicenseSequence: a.LicenseSequence, KeyID: a.KeyID, IssuedAt: a.IssuedAt, ExpiresAt: a.ExpiresAt, Features: append([]string{}, a.Features...), Valid: cfg.License.Valid, Restricted: cfg.License.Restricted}
	if a.Signature != "" {
		report.LicenseID = fmt.Sprintf("%x", sha256.Sum256([]byte(a.Signature)))
	}
	if a.ExpiresAt != "" {
		expiry, err := time.Parse(time.RFC3339, a.ExpiresAt)
		if err != nil || !now.Before(expiry) {
			report.Valid = false
			report.Restricted = true
		}
	}
	return report
}

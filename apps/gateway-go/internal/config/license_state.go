package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var licenseStateMutex sync.Mutex

type licenseReceipt struct {
	Sequence   int64  `json:"sequence"`
	Digest     string `json:"digest"`
	HardwareID string `json:"hardwareId"`
	SN         string `json:"sn"`
}

func (c Config) CloudConnectionIsEnabled() bool {
	if c.CloudConnectionEnabled != nil {
		return *c.CloudConnectionEnabled
	}
	return c.Activation.IsEnabled()
}

func receiptFor(a ActivationConfig) (licenseReceipt, error) {
	payload, err := activationPayload(a)
	if err != nil {
		return licenseReceipt{}, err
	}
	digest := sha256.Sum256(payload)
	return licenseReceipt{a.LicenseSequence, hex.EncodeToString(digest[:]), strings.ToUpper(a.HardwareID), a.SN}, nil
}

// The receipt is a crash-safe software high-water mark, not a hardware
// monotonic counter. Root/full-disk rollback still needs secure boot/storage.
func checkLicense(configPath string, a ActivationConfig) (licenseReceipt, error) {
	next, err := receiptFor(a)
	if err != nil {
		return next, err
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(configPath), ".license-state.json"))
	if os.IsNotExist(err) {
		return next, nil
	}
	if err != nil {
		return next, fmt.Errorf("read license replay state: %w", err)
	}
	var previous licenseReceipt
	if json.Unmarshal(raw, &previous) != nil || previous.Sequence < 0 || len(previous.Digest) != 64 || previous.HardwareID == "" || previous.SN == "" {
		return next, fmt.Errorf("license replay state is damaged; contact administrator")
	}
	if next.HardwareID != previous.HardwareID || next.SN != previous.SN {
		return next, fmt.Errorf("license identity differs from the installed license")
	}
	if next.Sequence < previous.Sequence || (next.Sequence == previous.Sequence && next.Digest != previous.Digest) {
		return next, fmt.Errorf("old or conflicting authorization rejected (installed sequence %d)", previous.Sequence)
	}
	return next, nil
}

func CheckLicenseReplay(configPath string, a ActivationConfig) error {
	licenseStateMutex.Lock()
	defer licenseStateMutex.Unlock()
	_, err := checkLicense(configPath, a)
	return err
}

func AcceptLicense(configPath string, a ActivationConfig) error {
	licenseStateMutex.Lock()
	defer licenseStateMutex.Unlock()
	next, err := checkLicense(configPath, a)
	if err != nil {
		return err
	}
	// Legacy files can be renewed until the first sequenced license is installed.
	if a.LicenseVersion < 2 {
		return nil
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(configPath), ".license-state.json")
	if previous, readErr := os.ReadFile(path); readErr == nil && string(previous) == string(raw) {
		return nil
	}
	return AtomicPrivateWrite(path, raw)
}

// AtomicPrivateWrite never truncates the previously working file.
func AtomicPrivateWrite(path string, raw []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".license-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	// Directory fsync is supported on the Linux appliance; Windows may reject it.
	if directory, openErr := os.Open(filepath.Dir(path)); openErr == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

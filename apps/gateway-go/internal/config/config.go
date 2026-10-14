package config

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/hardware"
	"weikong-iot-platform/apps/gateway-go/internal/securechip"
)

const CurrentSchemaVersion = 2

// ActivationBindingRequired is set to true through production build ldflags.
// Development and unit-test builds keep it false so an explicit
// activation.enabled=false remains available outside appliance releases.
var ActivationBindingRequired = "false"

// ActivationSignatureRequired and ActivationSignaturePublicKey are injected
// by production builds. The private signing key never ships with a gateway.
// ActivationSignaturePublicKey accepts either a base64 encoded raw Ed25519
// public key or a base64 encoded PKIX public key.
var ActivationSignatureRequired = "false"
var ActivationSignaturePublicKey = ""

// JSON keyId -> base64 Ed25519 public key, embedded by production builds.
var ActivationTrustedPublicKeys = ""
var SecureChipRequired = "false"

// DefaultHistoryStoragePath is the persistent local data partition on EG100.
// Removable storage remains opt-in by explicitly selecting its mount path.
const DefaultHistoryStoragePath = "/userdata/weikong/data"

// NormalizeProtocol accepts legacy spellings while keeping the canonical
// protocol identifiers consistent with the rest of the gateway.
func NormalizeProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "opc_da", "opc-da":
		return "opcda"
	case "dlt-645", "dl/t645", "dl/t 645", "dl_t_645":
		return "dlt645"
	default:
		return value
	}
}

const (
	OPCDASecurityModern          = "modern"
	OPCDASecurityPacketIntegrity = "packet_integrity"
	OPCDASecurityPacketPrivacy   = "packet_privacy"
	OPCDASecurityLegacyConnect   = "legacy_connect"
)

// NormalizeOPCDASecurityMode preserves V1.0 authenticationLevel projects while
// making the V1.1 security policy explicit. Modern never falls back to Connect.
func NormalizeOPCDASecurityMode(mode string, legacyAuthenticationLevel string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "modern", "auto", "modern_auto":
		return OPCDASecurityModern
	case "packet_integrity", "integrity":
		return OPCDASecurityPacketIntegrity
	case "packet_privacy", "privacy":
		return OPCDASecurityPacketPrivacy
	case "legacy_connect", "legacy", "connect":
		return OPCDASecurityLegacyConnect
	}
	if normalized != "" {
		return normalized
	}
	switch strings.ToLower(strings.TrimSpace(legacyAuthenticationLevel)) {
	case "connect", "legacy_connect":
		return OPCDASecurityLegacyConnect
	case "packet_privacy", "privacy":
		return OPCDASecurityPacketPrivacy
	case "packet_integrity", "integrity":
		return OPCDASecurityPacketIntegrity
	default:
		return OPCDASecurityModern
	}
}

func OPCDAAuthenticationLevel(securityMode string) string {
	switch securityMode {
	case OPCDASecurityLegacyConnect:
		return "connect"
	case OPCDASecurityPacketPrivacy:
		return "packet_privacy"
	default:
		return "packet_integrity"
	}
}

type Config struct {
	SchemaVersion               int                   `json:"schemaVersion"`
	GatewayKey                  string                `json:"gatewayKey"`
	CollectIntervalSeconds      int                   `json:"collectIntervalSeconds"`
	CollectIntervalMilliseconds int                   `json:"collectIntervalMilliseconds,omitempty"`
	CacheFile                   string                `json:"cacheFile"`
	OfflineCache                OfflineCacheConfig    `json:"offlineCache,omitempty"`
	HistoryStorage              HistoryStorageConfig  `json:"historyStorage,omitempty"`
	RemoteStorage               RemoteStorageConfig   `json:"remoteStorage,omitempty"`
	Activation                  ActivationConfig      `json:"activation,omitempty"`
	ProductModel                string                `json:"productModel,omitempty"`
	HardwareProfile             *HardwareProfile      `json:"hardwareProfile,omitempty"`
	CloudConnectionEnabled      *bool                 `json:"cloudConnectionEnabled,omitempty"`
	MQTT                        MQTTConfig            `json:"mqtt"`
	MQTTChannels                []MQTTConfig          `json:"mqttChannels,omitempty"`
	RemoteAccess                RemoteAccessConfig    `json:"remoteAccess,omitempty"`
	Resources                   []ResourceConfig      `json:"resources,omitempty"`
	Channels                    []ChannelConfig       `json:"channels,omitempty"`
	SerialPorts                 []SerialPort          `json:"serialPorts,omitempty"`
	NetworkPorts                []NetworkPort         `json:"networkPorts,omitempty"`
	WiFi                        WiFiConfig            `json:"wifi,omitempty"`
	Cellular                    CellularConfig        `json:"cellular,omitempty"`
	Devices                     []DeviceConfig        `json:"devices,omitempty"`
	Points                      []PointConfig         `json:"points,omitempty"`
	ForwardDevices              []ForwardDeviceConfig `json:"forwardDevices,omitempty"`
	EdgeComputing               EdgeComputingConfig   `json:"edgeComputing,omitempty"`
	Alarms                      AlarmConfig           `json:"alarms,omitempty"`
	ForwardSlave                FeatureConfig         `json:"forwardSlave"`
	Web                         ListenerConfig        `json:"web"`
	Security                    SecurityConfig        `json:"security,omitempty"`
	Watchdog                    WatchdogConfig        `json:"watchdog,omitempty"`
	License                     LicenseStatus         `json:"-"`
}

type LicenseStatus struct {
	Valid      bool   `json:"valid"`
	Restricted bool   `json:"restricted"`
	Reason     string `json:"reason,omitempty"`
}

type WatchdogConfig struct {
	Enabled            bool   `json:"enabled"`
	HardwareEnabled    bool   `json:"hardwareEnabled"`
	Device             string `json:"device,omitempty"`
	HardwareTimeoutSec int    `json:"hardwareTimeoutSeconds,omitempty"`
	FeedIntervalSec    int    `json:"feedIntervalSeconds,omitempty"`
	HealthCheckSec     int    `json:"healthCheckSeconds,omitempty"`
	HealthTimeoutSec   int    `json:"healthTimeoutSeconds,omitempty"`
	StartupGraceSec    int    `json:"startupGraceSeconds,omitempty"`
	FailureThreshold   int    `json:"failureThreshold,omitempty"`
	RestartLimit       int    `json:"restartLimit,omitempty"`
	RestartWindowSec   int    `json:"restartWindowSeconds,omitempty"`
	SafeModeFile       string `json:"safeModeFile,omitempty"`
}

func (w *WatchdogConfig) ApplyDefaults() {
	if w.Device == "" {
		w.Device = "/dev/watchdog"
	}
	if w.HardwareTimeoutSec <= 0 {
		w.HardwareTimeoutSec = 90
	}
	if w.FeedIntervalSec <= 0 {
		w.FeedIntervalSec = 10
	}
	if w.HealthCheckSec <= 0 {
		w.HealthCheckSec = 5
	}
	if w.HealthTimeoutSec <= 0 {
		w.HealthTimeoutSec = 3
	}
	if w.StartupGraceSec <= 0 {
		w.StartupGraceSec = 45
	}
	if w.FailureThreshold <= 0 {
		w.FailureThreshold = 4
	}
	if w.RestartLimit <= 0 {
		w.RestartLimit = 3
	}
	if w.RestartWindowSec <= 0 {
		w.RestartWindowSec = 300
	}
	if w.SafeModeFile == "" {
		w.SafeModeFile = "/userdata/weikong/data/watchdog-safe-mode"
	}
}

type OfflineCacheConfig struct {
	Enabled     bool   `json:"enabled"`
	MaxSizeMB   int    `json:"maxSizeMB"`
	StoragePath string `json:"storagePath,omitempty"`
}

type HistoryStorageConfig struct {
	StorageUUID         string               `json:"storageUUID,omitempty"`
	Enabled             bool                 `json:"enabled"`
	MaxSizeMB           int                  `json:"maxSizeMB"`
	RetentionDays       int                  `json:"retentionDays,omitempty"`
	StoragePath         string               `json:"storagePath,omitempty"`
	BackupEnabled       bool                 `json:"backupEnabled,omitempty"`
	BackupIntervalHours int                  `json:"backupIntervalHours,omitempty"`
	BackupKeep          int                  `json:"backupKeep,omitempty"`
	BackupPath          string               `json:"backupPath,omitempty"`
	EncryptBackups      bool                 `json:"encryptBackups,omitempty"`
	AggregationEnabled  bool                 `json:"aggregationEnabled,omitempty"`
	Tasks               []HistoryStorageTask `json:"tasks,omitempty"`
	// Legacy single-policy fields are retained so existing config.local.json
	// files continue to load. New configurations use Tasks.
	SelectionMode       string            `json:"selectionMode,omitempty"`
	Points              []HistoryPointRef `json:"points,omitempty"`
	SaveMode            string            `json:"saveMode,omitempty"`
	SaveIntervalSeconds int               `json:"saveIntervalSeconds,omitempty"`
	ChangeDeadband      float64           `json:"changeDeadband,omitempty"`
}

type HistoryStorageTask struct {
	TaskKey             string            `json:"taskKey"`
	Name                string            `json:"name"`
	Enabled             *bool             `json:"enabled,omitempty"`
	SelectionMode       string            `json:"selectionMode,omitempty"`
	Points              []HistoryPointRef `json:"points,omitempty"`
	SaveMode            string            `json:"saveMode,omitempty"`
	SaveIntervalSeconds int               `json:"saveIntervalSeconds,omitempty"`
	ChangeDeadband      float64           `json:"changeDeadband,omitempty"`
	StartAt             *time.Time        `json:"startAt,omitempty"`
	EndAt               *time.Time        `json:"endAt,omitempty"`
}

func (t HistoryStorageTask) IsEnabled() bool { return t.Enabled == nil || *t.Enabled }

func (t HistoryStorageTask) StatusAt(now time.Time) string {
	if !t.IsEnabled() {
		return "disabled"
	}
	if t.StartAt != nil && now.Before(*t.StartAt) {
		return "pending"
	}
	if t.EndAt != nil && !now.Before(*t.EndAt) {
		return "completed"
	}
	return "running"
}

func (h *HistoryStorageConfig) ApplyDefaults() {
	missingStoragePath := strings.TrimSpace(h.StoragePath) == ""
	if missingStoragePath {
		h.StoragePath = DefaultHistoryStoragePath
	}
	h.StorageUUID = strings.TrimSpace(h.StorageUUID)
	if h.UsesLocalStorage() {
		h.StorageUUID = ""
	}
	if h.MaxSizeMB == 0 || missingStoragePath && h.MaxSizeMB == 256 {
		// EG100 local storage is intentionally bounded conservatively. Users may
		// increase this after checking the device's remaining capacity. The old
		// default was 256 MB with no path; migrate that unusable combination too.
		h.MaxSizeMB = 16
	}
	if h.BackupIntervalHours <= 0 {
		h.BackupIntervalHours = 24
	}
	if h.BackupKeep <= 0 {
		h.BackupKeep = 3
	}
	if strings.TrimSpace(h.SelectionMode) == "" {
		h.SelectionMode = "all"
	} else {
		h.SelectionMode = strings.ToLower(strings.TrimSpace(h.SelectionMode))
	}
	if strings.TrimSpace(h.SaveMode) == "" {
		h.SaveMode = "interval"
	} else {
		h.SaveMode = strings.ToLower(strings.TrimSpace(h.SaveMode))
	}
	if h.SaveIntervalSeconds <= 0 {
		h.SaveIntervalSeconds = 1
	}
	for index := range h.Tasks {
		task := &h.Tasks[index]
		task.TaskKey = strings.TrimSpace(task.TaskKey)
		task.Name = strings.TrimSpace(task.Name)
		task.SelectionMode = strings.ToLower(strings.TrimSpace(task.SelectionMode))
		if task.SelectionMode == "" {
			task.SelectionMode = "selected"
		}
		task.SaveMode = strings.ToLower(strings.TrimSpace(task.SaveMode))
		if task.SaveMode == "" {
			task.SaveMode = "interval"
		}
		if task.SaveIntervalSeconds <= 0 {
			task.SaveIntervalSeconds = 1
		}
	}
}

func (h HistoryStorageConfig) UsesLocalStorage() bool {
	return filepath.Clean(strings.TrimSpace(h.StoragePath)) == filepath.Clean(DefaultHistoryStoragePath)
}

func (h HistoryStorageConfig) EffectiveTasks() []HistoryStorageTask {
	if len(h.Tasks) > 0 {
		return h.Tasks
	}
	if !h.Enabled {
		return nil
	}
	enabled := true
	return []HistoryStorageTask{{
		TaskKey: "legacy-default", Name: "长期历史任务", Enabled: &enabled,
		SelectionMode: h.SelectionMode, Points: h.Points, SaveMode: h.SaveMode,
		SaveIntervalSeconds: h.SaveIntervalSeconds, ChangeDeadband: h.ChangeDeadband,
	}}
}

type HistoryPointRef struct {
	DeviceKey string `json:"deviceKey"`
	Metric    string `json:"metric"`
}

type MQTTConfig struct {
	Name                        string `json:"name,omitempty"`
	Enabled                     *bool  `json:"enabled,omitempty"`
	Broker                      string `json:"broker"`
	ClientID                    string `json:"clientId"`
	Username                    string `json:"username"`
	Password                    string `json:"password"`
	TopicTemplate               string `json:"topicTemplate,omitempty"`
	PayloadMode                 string `json:"payloadMode,omitempty"`
	PayloadTemplate             string `json:"payloadTemplate,omitempty"`
	SubscribeTopic              string `json:"subscribeTopic,omitempty"`
	PublishIntervalMilliseconds int    `json:"publishIntervalMilliseconds,omitempty"`
	CAFile                      string `json:"caFile,omitempty"`
	CertFile                    string `json:"certFile,omitempty"`
	KeyFile                     string `json:"keyFile,omitempty"`
}

type ActivationConfig struct {
	Enabled              *bool            `json:"enabled,omitempty"`
	File                 string           `json:"file,omitempty"`
	LicenseVersion       int              `json:"licenseVersion,omitempty"`
	KeyID                string           `json:"keyId,omitempty"`
	LicenseSequence      int64            `json:"licenseSequence,omitempty"`
	HardwareID           string           `json:"hardwareId,omitempty"`
	ChipUSID             string           `json:"chipUsid,omitempty"`
	SN                   string           `json:"sn,omitempty"`
	DeviceSecret         string           `json:"deviceSecret,omitempty"`
	Broker               string           `json:"broker,omitempty"`
	CloudAPI             string           `json:"cloudApi,omitempty"`
	RemoteAccessEndpoint string           `json:"remoteAccessEndpoint,omitempty"`
	IssuedAt             string           `json:"issuedAt,omitempty"`
	ExpiresAt            string           `json:"expiresAt,omitempty"`
	Features             []string         `json:"features,omitempty"`
	ProductModel         string           `json:"productModel,omitempty"`
	HardwareProfile      *HardwareProfile `json:"hardwareProfile,omitempty"`
	Nonce                string           `json:"nonce,omitempty"`
	Signature            string           `json:"signature,omitempty"`
}

// activationSignedPayload is intentionally kept in the same field order as
// the management platform signer. JSON produced from this struct is the exact
// byte sequence covered by the Ed25519 signature.
type activationSignedPayload struct {
	Enabled              bool             `json:"enabled"`
	LicenseVersion       int              `json:"licenseVersion"`
	KeyID                string           `json:"keyId,omitempty"`
	LicenseSequence      int64            `json:"licenseSequence,omitempty"`
	HardwareID           string           `json:"hardwareId"`
	ChipUSID             string           `json:"chipUsid"`
	SN                   string           `json:"sn"`
	DeviceSecret         string           `json:"deviceSecret"`
	Broker               string           `json:"broker"`
	CloudAPI             string           `json:"cloudApi"`
	RemoteAccessEndpoint string           `json:"remoteAccessEndpoint"`
	IssuedAt             string           `json:"issuedAt"`
	ExpiresAt            string           `json:"expiresAt"`
	Features             []string         `json:"features"`
	ProductModel         string           `json:"productModel,omitempty"`
	HardwareProfile      *HardwareProfile `json:"hardwareProfile,omitempty"`
	Nonce                string           `json:"nonce"`
}

// HardwareProfile is the immutable hardware capability snapshot copied from
// the management platform's product catalog when the license is issued.
type HardwareProfile struct {
	EthernetPortCount int  `json:"ethernetPortCount"`
	RS232PortCount    int  `json:"rs232PortCount"`
	RS485PortCount    int  `json:"rs485PortCount"`
	CANPortCount      int  `json:"canPortCount"`
	Supports4G        bool `json:"supports4G"`
	SupportsWiFi      bool `json:"supportsWifi"`
}

// RemoteAccessConfig controls the outbound-only management tunnel. Username
// and password may be omitted to reuse the active cloud device credential.
type RemoteAccessConfig struct {
	Enabled                 bool   `json:"enabled"`
	Endpoint                string `json:"endpoint,omitempty"`
	Username                string `json:"username,omitempty"`
	Password                string `json:"password,omitempty"`
	LocalURL                string `json:"localUrl,omitempty"`
	InsecureSkipVerify      bool   `json:"insecureSkipVerify,omitempty"`
	LocalInsecureSkipVerify bool   `json:"localInsecureSkipVerify,omitempty"`
}

func (r *RemoteAccessConfig) ApplyDefaults() {
	if r.LocalURL == "" {
		r.LocalURL = "http://127.0.0.1:8088"
	}
}

func (r RemoteAccessConfig) Equal(other RemoteAccessConfig) bool { return r == other }

func (a ActivationConfig) Equal(other ActivationConfig) bool {
	return a.IsEnabled() == other.IsEnabled() &&
		a.File == other.File &&
		a.LicenseVersion == other.LicenseVersion &&
		a.KeyID == other.KeyID && a.LicenseSequence == other.LicenseSequence &&
		a.HardwareID == other.HardwareID &&
		a.ChipUSID == other.ChipUSID &&
		a.SN == other.SN &&
		a.DeviceSecret == other.DeviceSecret &&
		a.Broker == other.Broker &&
		a.CloudAPI == other.CloudAPI &&
		a.RemoteAccessEndpoint == other.RemoteAccessEndpoint &&
		a.IssuedAt == other.IssuedAt &&
		a.ExpiresAt == other.ExpiresAt &&
		strings.Join(a.Features, "\x00") == strings.Join(other.Features, "\x00") &&
		a.ProductModel == other.ProductModel &&
		hardwareProfilesEqual(a.HardwareProfile, other.HardwareProfile) &&
		a.Nonce == other.Nonce &&
		a.Signature == other.Signature
}

type SerialPort struct {
	Name           string `json:"name"`
	Port           string `json:"port"`
	ElectricalType string `json:"electricalType,omitempty"`
	BaudRate       int    `json:"baudRate"`
	DataBits       int    `json:"dataBits"`
	StopBits       int    `json:"stopBits"`
	Parity         string `json:"parity"`
	Enabled        bool   `json:"enabled"`
}

type NetworkPort struct {
	Name         string   `json:"name"`
	Interface    string   `json:"interface,omitempty"`
	Mode         string   `json:"mode"`
	IPAddress    string   `json:"ipAddress,omitempty"`
	PrefixLength int      `json:"prefixLength,omitempty"`
	Gateway      string   `json:"gateway,omitempty"`
	DNS          []string `json:"dns,omitempty"`
	Address      string   `json:"address,omitempty"`
	Enabled      bool     `json:"enabled"`
}

type WiFiConfig struct {
	Enabled      bool     `json:"enabled"`
	Interface    string   `json:"interface,omitempty"`
	SSID         string   `json:"ssid,omitempty"`
	Password     string   `json:"password,omitempty"`
	Mode         string   `json:"mode,omitempty"`
	IPAddress    string   `json:"ipAddress,omitempty"`
	PrefixLength int      `json:"prefixLength,omitempty"`
	Gateway      string   `json:"gateway,omitempty"`
	DNS          []string `json:"dns,omitempty"`
}

type CellularConfig struct {
	Enabled   *bool  `json:"enabled,omitempty"`
	Interface string `json:"interface,omitempty"`
}

type EdgeComputingConfig struct {
	Enabled bool                     `json:"enabled"`
	Groups  []EdgeComputeGroupConfig `json:"groups,omitempty"`
}

type AlarmConfig struct {
	PopupDurationSeconds *int              `json:"popupDurationSeconds,omitempty"`
	Rules                []AlarmRuleConfig `json:"rules,omitempty"`
	HistoryLimit         int               `json:"historyLimit,omitempty"`
	RetentionLimit       int               `json:"retentionLimit,omitempty"`
	StoragePath          string            `json:"storagePath,omitempty"`
}

type AlarmRuleConfig struct {
	// Runtime-only grouping metadata; never accepted from or saved to configuration.
	ParentRuleID            string             `json:"-"`
	ParentRuleName          string             `json:"-"`
	LimitKey                string             `json:"-"`
	LimitName               string             `json:"-"`
	Limits                  *AlarmLimitsConfig `json:"limits,omitempty"`
	RuleID                  string             `json:"ruleId"`
	Name                    string             `json:"name"`
	Enabled                 *bool              `json:"enabled,omitempty"`
	DeviceKey               string             `json:"deviceKey"`
	Metric                  string             `json:"metric"`
	Type                    string             `json:"type"`
	Operator                string             `json:"operator,omitempty"`
	Threshold               *float64           `json:"threshold,omitempty"`
	BooleanValue            *bool              `json:"booleanValue,omitempty"`
	Quality                 string             `json:"quality,omitempty"`
	Hysteresis              float64            `json:"hysteresis,omitempty"`
	TriggerDelaySeconds     int                `json:"triggerDelaySeconds,omitempty"`
	RecoveryDelaySeconds    int                `json:"recoveryDelaySeconds,omitempty"`
	AllowManualRecovery     bool               `json:"allowManualRecovery,omitempty"`
	PopupEnabled            *bool              `json:"popupEnabled,omitempty"`
	SoundWhenPopupDisabled  bool               `json:"soundWhenPopupDisabled,omitempty"`
	RetainAfterAcknowledged bool               `json:"retainAfterAcknowledged,omitempty"`
	Level                   string             `json:"level"`
	Message                 string             `json:"message,omitempty"`
}

func (r AlarmRuleConfig) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }

// Absent values retain popup notifications for existing rules.
func (r AlarmRuleConfig) IsPopupEnabled() bool { return r.PopupEnabled == nil || *r.PopupEnabled }

// Zero keeps the popup visible; an absent setting preserves the old 10s default.
func (a AlarmConfig) PopupDuration() int {
	if a.PopupDurationSeconds == nil {
		return 10
	}
	return *a.PopupDurationSeconds
}

func (a *AlarmConfig) ApplyDefaults() {
	if a.HistoryLimit <= 0 {
		a.HistoryLimit = 500
	}
	if a.RetentionLimit <= 0 {
		a.RetentionLimit = 10000
	}
	if a.StoragePath == "" {
		a.StoragePath = ".runtime/alarm-events.db"
	} else if strings.EqualFold(filepath.Ext(a.StoragePath), ".jsonl") {
		a.StoragePath = strings.TrimSuffix(a.StoragePath, filepath.Ext(a.StoragePath)) + ".db"
	}
	for index := range a.Rules {
		rule := &a.Rules[index]
		rule.DeviceKey = strings.TrimSpace(rule.DeviceKey)
		rule.Metric = strings.TrimSpace(rule.Metric)
		rule.Type = strings.ToLower(strings.TrimSpace(rule.Type))
		if rule.Type == "" {
			rule.Type = "threshold"
		}
		if rule.RuleID == "" {
			rule.RuleID = fmt.Sprintf("alarm-%s-%s-%03d", sanitizeAlarmKey(rule.DeviceKey), sanitizeAlarmKey(rule.Metric), index+1)
		}
		if rule.Name == "" {
			rule.Name = rule.Metric + " 告警"
		}
		if rule.Enabled == nil {
			enabled := true
			rule.Enabled = &enabled
		}
		if rule.Operator == "" && rule.Type == "threshold" {
			rule.Operator = ">"
		}
		if rule.Quality == "" && rule.Type == "quality" {
			rule.Quality = "BAD"
		}
		rule.Quality = strings.ToUpper(strings.TrimSpace(rule.Quality))
		if rule.Level == "" {
			rule.Level = "WARNING"
		}
		rule.Level = strings.ToUpper(strings.TrimSpace(rule.Level))
		if rule.Limits != nil {
			rule.Limits.ApplyDefaults()
		}
	}
}

func sanitizeAlarmKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			result.WriteRune(character)
		} else {
			result.WriteByte('-')
		}
	}
	cleaned := strings.Trim(result.String(), "-_")
	if cleaned == "" {
		return "point"
	}
	return cleaned
}

type EdgeComputeGroupConfig struct {
	GroupKey         string                    `json:"groupKey"`
	Name             string                    `json:"name"`
	Enabled          *bool                     `json:"enabled,omitempty"`
	HeartbeatSeconds int                       `json:"heartbeatSeconds,omitempty"`
	Points           []EdgeComputedPointConfig `json:"points,omitempty"`
}

type EdgeComputedPointConfig struct {
	Name       string                   `json:"name"`
	Metric     string                   `json:"metric"`
	Enabled    *bool                    `json:"enabled,omitempty"`
	DataType   string                   `json:"dataType"`
	Unit       string                   `json:"unit,omitempty"`
	Decimals   int                      `json:"decimals,omitempty"`
	Expression string                   `json:"expression"`
	Inputs     []EdgeComputeInputConfig `json:"inputs,omitempty"`
}

type EdgeComputeInputConfig struct {
	Alias           string `json:"alias"`
	SourceDeviceKey string `json:"sourceDeviceKey"`
	SourceMetric    string `json:"sourceMetric"`
}

func (g EdgeComputeGroupConfig) IsEnabled() bool  { return g.Enabled == nil || *g.Enabled }
func (p EdgeComputedPointConfig) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

func (e *EdgeComputingConfig) ApplyDefaults() {
	for groupIndex := range e.Groups {
		group := &e.Groups[groupIndex]
		if group.GroupKey == "" {
			group.GroupKey = fmt.Sprintf("edge-group-%03d", groupIndex+1)
		}
		if group.Name == "" {
			group.Name = group.GroupKey
		}
		if group.Enabled == nil {
			enabled := true
			group.Enabled = &enabled
		}
		for pointIndex := range group.Points {
			point := &group.Points[pointIndex]
			if point.Metric == "" {
				point.Metric = fmt.Sprintf("computed_%d", pointIndex+1)
			}
			if point.Name == "" {
				point.Name = point.Metric
			}
			if point.Enabled == nil {
				enabled := true
				point.Enabled = &enabled
			}
			if point.DataType == "" {
				point.DataType = "float64"
			}
		}
	}
}

// ComputedStatusPoints exposes enabled derived points to the shared runtime
// state without adding them to the physical collection list.
func (c Config) ComputedStatusPoints() []PointConfig {
	if !c.EdgeComputing.Enabled {
		return nil
	}
	var points []PointConfig
	for _, group := range c.EdgeComputing.Groups {
		if !group.IsEnabled() {
			continue
		}
		for _, computed := range group.Points {
			if !computed.IsEnabled() {
				continue
			}
			points = append(points, PointConfig{
				DeviceKey: group.GroupKey,
				Name:      computed.Name,
				Metric:    computed.Metric,
				Protocol:  "edge-compute",
				Address:   group.GroupKey,
				DataType:  computed.DataType,
				Quantity:  1,
				Scale:     1,
				Unit:      computed.Unit,
				Decimals:  computed.Decimals,
			})
		}
	}
	return points
}

func (c Config) StatusPoints() []PointConfig {
	points := append([]PointConfig(nil), c.Points...)
	return append(points, c.ComputedStatusPoints()...)
}

func (c CellularConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

type ResourceConfig struct {
	ResourceKey string       `json:"resourceKey"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Enabled     bool         `json:"enabled"`
	Serial      *SerialPort  `json:"serial,omitempty"`
	Network     *NetworkPort `json:"network,omitempty"`
}

type ChannelConfig struct {
	ChannelKey                  string         `json:"channelKey"`
	ResourceKey                 string         `json:"resourceKey,omitempty"`
	Name                        string         `json:"name"`
	Type                        string         `json:"type"`
	Role                        string         `json:"role,omitempty"`
	Protocol                    string         `json:"protocol"`
	ForwardProtocol             string         `json:"forwardProtocol,omitempty"`
	CollectIntervalSeconds      int            `json:"collectIntervalSeconds,omitempty"`
	CollectIntervalMilliseconds int            `json:"collectIntervalMilliseconds,omitempty"`
	IEC104                      IEC104Config   `json:"iec104,omitempty"`
	InterfaceName               string         `json:"interfaceName,omitempty"`
	Enabled                     bool           `json:"enabled"`
	Serial                      *SerialPort    `json:"serial,omitempty"`
	Network                     *NetworkPort   `json:"network,omitempty"`
	Devices                     []DeviceConfig `json:"devices,omitempty"`
}

type IEC104Config struct {
	Configured                          bool   `json:"configured,omitempty"`
	AcquisitionMode                     string `json:"acquisitionMode,omitempty"`
	GeneralInterrogationOnStart         bool   `json:"generalInterrogationOnStart"`
	GeneralInterrogationIntervalSeconds int    `json:"generalInterrogationIntervalSeconds"`
	ClockSyncOnStart                    bool   `json:"clockSyncOnStart"`
	ClockSyncIntervalSeconds            int    `json:"clockSyncIntervalSeconds"`
	CounterInterrogationOnStart         bool   `json:"counterInterrogationOnStart"`
	CounterInterrogationIntervalSeconds int    `json:"counterInterrogationIntervalSeconds"`
}

// DLT645Config is used by devices and their flattened points. Device-wide
// serial/protocol settings and point-specific data-item settings intentionally
// share one object so legacy flat point projects remain self-contained.
type DLT645Config struct {
	Version                     string `json:"version,omitempty"`
	MeterAddress                string `json:"meterAddress,omitempty"`
	ResponseTimeoutMilliseconds int    `json:"responseTimeoutMilliseconds,omitempty"`
	Retries                     int    `json:"retries"`
	WakeupBytes                 int    `json:"wakeupBytes"`
	WritePassword               string `json:"writePassword,omitempty"`
	OperatorCode                string `json:"operatorCode,omitempty"`
	ItemKey                     string `json:"itemKey,omitempty"`
	DataIdentifier              string `json:"dataIdentifier,omitempty"`
	ValueFormat                 string `json:"valueFormat,omitempty"`
	DataLength                  int    `json:"dataLength,omitempty"`
	Access                      string `json:"access,omitempty"`
	retriesConfigured           bool
	wakeupBytesConfigured       bool
}

func (d *DLT645Config) UnmarshalJSON(data []byte) error {
	type plain DLT645Config
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*d = DLT645Config(decoded)
	_, d.retriesConfigured = fields["retries"]
	_, d.wakeupBytesConfigured = fields["wakeupBytes"]
	return nil
}

func (d *DLT645Config) ApplyDeviceDefaults() {
	d.Version = strings.ToLower(strings.TrimSpace(d.Version))
	if d.Version == "" {
		d.Version = "auto"
	}
	d.MeterAddress = strings.TrimSpace(d.MeterAddress)
	if d.ResponseTimeoutMilliseconds <= 0 {
		d.ResponseTimeoutMilliseconds = 1500
	}
	if !d.retriesConfigured && d.Retries == 0 {
		d.Retries = 1
	} else if d.Retries < 0 {
		d.Retries = 0
	}
	if !d.wakeupBytesConfigured && d.WakeupBytes == 0 {
		d.WakeupBytes = 4
	} else if d.WakeupBytes < 0 {
		d.WakeupBytes = 0
	}
}

func (d *DLT645Config) ApplyPointDefaults() {
	d.ApplyDeviceDefaults()
	d.DataIdentifier = strings.ToUpper(strings.TrimSpace(d.DataIdentifier))
	d.ValueFormat = strings.ToLower(strings.TrimSpace(d.ValueFormat))
	if d.ValueFormat == "" {
		d.ValueFormat = "bcd"
	}
	d.Access = strings.ToLower(strings.TrimSpace(d.Access))
	if d.Access == "readwrite" {
		d.Access = "read_write"
	}
	if d.Access == "" {
		d.Access = "read"
	}
}

type PointConfig struct {
	DeviceKey                   string        `json:"deviceKey"`
	ChannelKey                  string        `json:"channelKey,omitempty"`
	CollectIntervalSeconds      int           `json:"collectIntervalSeconds,omitempty"`
	CollectIntervalMilliseconds int           `json:"collectIntervalMilliseconds,omitempty"`
	Name                        string        `json:"name"`
	Metric                      string        `json:"metric"`
	Protocol                    string        `json:"protocol"`
	Address                     string        `json:"address"`
	PointType                   string        `json:"pointType,omitempty"`
	SlaveID                     byte          `json:"slaveId"`
	CommonAddress               uint16        `json:"commonAddress,omitempty"`
	IOA                         uint32        `json:"ioa,omitempty"`
	IEC104                      *IEC104Config `json:"iec104,omitempty"`
	DLT645                      *DLT645Config `json:"dlt645,omitempty"`
	Area                        string        `json:"area,omitempty"`
	DBNumber                    uint16        `json:"dbNumber,omitempty"`
	Rack                        uint8         `json:"rack,omitempty"`
	Slot                        uint8         `json:"slot,omitempty"`
	LocalTSAP                   string        `json:"localTsap,omitempty"`
	RemoteTSAP                  string        `json:"remoteTsap,omitempty"`
	ObjectRef                   string        `json:"objectRef,omitempty"`
	FC                          string        `json:"fc,omitempty"`
	GoCBRef                     string        `json:"goCbRef,omitempty"`
	DataSetRef                  string        `json:"dataSetRef,omitempty"`
	GooseIndex                  int           `json:"gooseIndex,omitempty"`
	AppID                       uint16        `json:"appId,omitempty"`
	DestinationMAC              string        `json:"destinationMac,omitempty"`
	VLANID                      uint16        `json:"vlanId,omitempty"`
	VLANPriority                uint8         `json:"vlanPriority,omitempty"`
	NodeID                      string        `json:"nodeId,omitempty"`
	ItemID                      string        `json:"itemId,omitempty"`
	AccessPath                  string        `json:"accessPath,omitempty"`
	Access                      string        `json:"access,omitempty"`
	RequestedDataType           string        `json:"requestedDataType,omitempty"`
	Active                      *bool         `json:"active,omitempty"`
	Enabled                     *bool         `json:"enabled,omitempty"`
	ProgID                      string        `json:"progId,omitempty"`
	CLSID                       string        `json:"clsid,omitempty"`
	Domain                      string        `json:"domain,omitempty"`
	Workstation                 string        `json:"workstation,omitempty"`
	SecurityMode                string        `json:"securityMode,omitempty"`
	Authentication              string        `json:"authentication,omitempty"`
	AuthenticationLevel         string        `json:"authenticationLevel,omitempty"`
	ConnectTimeoutMilliseconds  int           `json:"connectTimeoutMilliseconds,omitempty"`
	RequestTimeoutMilliseconds  int           `json:"requestTimeoutMilliseconds,omitempty"`
	ReadMode                    string        `json:"readMode,omitempty"`
	ReadSource                  string        `json:"readSource,omitempty"`
	BatchSize                   int           `json:"batchSize,omitempty"`
	AutoReconnect               *bool         `json:"autoReconnect,omitempty"`
	GroupName                   string        `json:"groupName,omitempty"`
	RequestedUpdateRateMS       int           `json:"requestedUpdateRateMilliseconds,omitempty"`
	Deadband                    float64       `json:"deadband,omitempty"`
	LocaleID                    uint32        `json:"localeId,omitempty"`
	LocalBindIP                 string        `json:"localBindIp,omitempty"`
	Username                    string        `json:"username,omitempty"`
	Password                    string        `json:"password,omitempty"`
	BaudRate                    int           `json:"baudRate,omitempty"`
	DataBits                    int           `json:"dataBits,omitempty"`
	StopBits                    int           `json:"stopBits,omitempty"`
	Parity                      string        `json:"parity,omitempty"`
	Function                    uint8         `json:"function"`
	Register                    uint16        `json:"register"`
	Quantity                    uint16        `json:"quantity"`
	DataType                    string        `json:"dataType"`
	ByteOrder                   string        `json:"byteOrder"`
	WordOrder                   string        `json:"wordOrder"`
	BitIndex                    *uint8        `json:"bitIndex"`
	Scale                       float64       `json:"scale"`
	Offset                      float64       `json:"offset"`
	Unit                        string        `json:"unit,omitempty"`
	Decimals                    int           `json:"decimals"`
}

func (p PointConfig) IsEnabled() bool {
	return p.Enabled == nil || *p.Enabled
}

type DeviceConfig struct {
	DeviceKey                   string        `json:"deviceKey"`
	ChannelKey                  string        `json:"channelKey,omitempty"`
	Name                        string        `json:"name"`
	Enabled                     *bool         `json:"enabled,omitempty"`
	CollectIntervalSeconds      int           `json:"collectIntervalSeconds,omitempty"`
	CollectIntervalMilliseconds int           `json:"collectIntervalMilliseconds,omitempty"`
	PLCModel                    string        `json:"plcModel,omitempty"`
	InterfaceType               string        `json:"interfaceType,omitempty"`
	InterfaceName               string        `json:"interfaceName,omitempty"`
	Protocol                    string        `json:"protocol"`
	DLT645                      *DLT645Config `json:"dlt645,omitempty"`
	Address                     string        `json:"address"`
	ProgID                      string        `json:"progId,omitempty"`
	CLSID                       string        `json:"clsid,omitempty"`
	Domain                      string        `json:"domain,omitempty"`
	Workstation                 string        `json:"workstation,omitempty"`
	SecurityMode                string        `json:"securityMode,omitempty"`
	Authentication              string        `json:"authentication,omitempty"`
	AuthenticationLevel         string        `json:"authenticationLevel,omitempty"`
	ConnectTimeoutMilliseconds  int           `json:"connectTimeoutMilliseconds,omitempty"`
	RequestTimeoutMilliseconds  int           `json:"requestTimeoutMilliseconds,omitempty"`
	ReadMode                    string        `json:"readMode,omitempty"`
	ReadSource                  string        `json:"readSource,omitempty"`
	BatchSize                   int           `json:"batchSize,omitempty"`
	AutoReconnect               *bool         `json:"autoReconnect,omitempty"`
	GroupName                   string        `json:"groupName,omitempty"`
	RequestedUpdateRateMS       int           `json:"requestedUpdateRateMilliseconds,omitempty"`
	Deadband                    float64       `json:"deadband,omitempty"`
	LocaleID                    uint32        `json:"localeId,omitempty"`
	LocalBindIP                 string        `json:"localBindIp,omitempty"`
	SlaveID                     byte          `json:"slaveId"`
	CommonAddress               uint16        `json:"commonAddress,omitempty"`
	ReconnectIntervalSeconds    int           `json:"reconnectIntervalSeconds"`
	Rack                        uint8         `json:"rack,omitempty"`
	Slot                        uint8         `json:"slot,omitempty"`
	LocalTSAP                   string        `json:"localTsap,omitempty"`
	RemoteTSAP                  string        `json:"remoteTsap,omitempty"`
	IEDName                     string        `json:"iedName,omitempty"`
	GoCBRef                     string        `json:"goCbRef,omitempty"`
	DataSetRef                  string        `json:"dataSetRef,omitempty"`
	AppID                       uint16        `json:"appId,omitempty"`
	DestinationMAC              string        `json:"destinationMac,omitempty"`
	VLANID                      uint16        `json:"vlanId,omitempty"`
	VLANPriority                uint8         `json:"vlanPriority,omitempty"`
	TimeAllowedToLiveMS         uint32        `json:"timeAllowedToLiveMilliseconds,omitempty"`
	ConfRev                     uint32        `json:"confRev,omitempty"`
	Username                    string        `json:"username,omitempty"`
	Password                    string        `json:"password,omitempty"`
	Points                      []PointConfig `json:"points"`
}

func (d DeviceConfig) IsEnabled() bool {
	return d.Enabled == nil || *d.Enabled
}

// ForwardDeviceConfig describes a logical downstream device exposed by a
// forwarding server. Its points reference collected points instead of being
// collected independently.
type ForwardDeviceConfig struct {
	DeviceKey           string               `json:"deviceKey"`
	ChannelKey          string               `json:"channelKey,omitempty"`
	Name                string               `json:"name"`
	Protocol            string               `json:"protocol"`
	Enabled             *bool                `json:"enabled,omitempty"`
	UnitID              byte                 `json:"unitId,omitempty"`
	CommonAddress       uint16               `json:"commonAddress,omitempty"`
	IEDName             string               `json:"iedName,omitempty"`
	LogicalDevice       string               `json:"logicalDevice,omitempty"`
	InterfaceName       string               `json:"interfaceName,omitempty"`
	GoCBRef             string               `json:"goCbRef,omitempty"`
	DataSetRef          string               `json:"dataSetRef,omitempty"`
	AppID               uint16               `json:"appId,omitempty"`
	DestinationMAC      string               `json:"destinationMac,omitempty"`
	VLANID              uint16               `json:"vlanId,omitempty"`
	VLANPriority        uint8                `json:"vlanPriority,omitempty"`
	TimeAllowedToLiveMS uint32               `json:"timeAllowedToLiveMilliseconds,omitempty"`
	ConfRev             uint32               `json:"confRev,omitempty"`
	Points              []ForwardPointConfig `json:"points,omitempty"`
}

type ForwardPointConfig struct {
	Name            string  `json:"name"`
	Metric          string  `json:"metric"`
	SourceDeviceKey string  `json:"sourceDeviceKey"`
	SourceMetric    string  `json:"sourceMetric"`
	PointType       string  `json:"pointType,omitempty"`
	Function        uint8   `json:"function,omitempty"`
	Register        uint16  `json:"register,omitempty"`
	IOA             uint32  `json:"ioa,omitempty"`
	Quantity        uint16  `json:"quantity,omitempty"`
	DataType        string  `json:"dataType,omitempty"`
	ByteOrder       string  `json:"byteOrder,omitempty"`
	WordOrder       string  `json:"wordOrder,omitempty"`
	Scale           float64 `json:"scale,omitempty"`
	Offset          float64 `json:"offset,omitempty"`
	Unit            string  `json:"unit,omitempty"`
	Decimals        int     `json:"decimals"`
	ObjectRef       string  `json:"objectRef,omitempty"`
	FC              string  `json:"fc,omitempty"`
}

func (d ForwardDeviceConfig) IsEnabled() bool {
	return d.Enabled == nil || *d.Enabled
}

func (d *ForwardDeviceConfig) ApplyDefaults(index int) {
	if d.DeviceKey == "" {
		d.DeviceKey = fmt.Sprintf("forward-device-%03d", index+1)
	}
	if d.Name == "" {
		d.Name = d.DeviceKey
	}
	if d.UnitID == 0 {
		d.UnitID = 1
	}
	if d.CommonAddress == 0 {
		d.CommonAddress = 1
	}
	if d.Protocol == "iec61850-mms-server" {
		if d.IEDName == "" {
			d.IEDName = "WEIKONG"
		}
		d.IEDName = sanitizeIEC61850Identifier(d.IEDName, "WEIKONG")
		if d.LogicalDevice == "" {
			d.LogicalDevice = fmt.Sprintf("LD%d", index+1)
		}
		d.LogicalDevice = sanitizeIEC61850Identifier(d.LogicalDevice, fmt.Sprintf("LD%d", index+1))
	}
	if d.Protocol == "iec61850-goose-publisher" {
		if d.AppID == 0 {
			d.AppID = 0x1000
		}
		if d.DestinationMAC == "" {
			d.DestinationMAC = "01:0c:cd:01:00:01"
		}
		if d.VLANPriority > 7 {
			d.VLANPriority = 4
		}
		if d.TimeAllowedToLiveMS == 0 {
			d.TimeAllowedToLiveMS = 2000
		}
		if d.ConfRev == 0 {
			d.ConfRev = 1
		}
		if d.IEDName == "" {
			d.IEDName = "WEIKONG"
		}
		d.IEDName = sanitizeIEC61850Identifier(d.IEDName, "WEIKONG")
		if d.LogicalDevice == "" {
			d.LogicalDevice = fmt.Sprintf("LD%d", index+1)
		}
		d.LogicalDevice = sanitizeIEC61850Identifier(d.LogicalDevice, fmt.Sprintf("LD%d", index+1))
		if d.GoCBRef == "" {
			d.GoCBRef = d.IEDName + d.LogicalDevice + "/LLN0$GO$gcb01"
		}
		if d.DataSetRef == "" {
			d.DataSetRef = d.IEDName + d.LogicalDevice + "/LLN0$Dataset01"
		}
	}
	for pointIndex := range d.Points {
		point := &d.Points[pointIndex]
		if point.Name == "" {
			point.Name = point.SourceMetric
		}
		if point.Metric == "" {
			point.Metric = point.SourceMetric
		}
		if point.DataType == "" {
			point.DataType = "uint16"
		}
		if point.Quantity == 0 {
			point.Quantity = defaultQuantity(point.DataType)
		}
		if point.Scale == 0 {
			point.Scale = 1
		}
		if point.Function == 0 && d.Protocol == "modbus-tcp-slave" {
			point.Function = 3
		}
		if point.IOA == 0 {
			point.IOA = uint32(point.Register)
		}
		if d.Protocol == "iec61850-mms-server" {
			kind := strings.ToLower(strings.TrimSpace(point.DataType))
			if point.FC == "" {
				if kind == "bool" || kind == "string" {
					point.FC = "ST"
				} else {
					point.FC = "MX"
				}
			}
			point.FC = strings.ToUpper(strings.TrimSpace(point.FC))
			if point.ObjectRef == "" {
				objectName := sanitizeIEC61850Identifier(point.Metric, fmt.Sprintf("Point%d", pointIndex+1))
				leaf := ".mag.f"
				if kind == "bool" || kind == "string" {
					leaf = ".stVal"
				}
				point.ObjectRef = d.IEDName + d.LogicalDevice + "/GGIO1." + objectName + leaf
			}
		}
	}
}

func sanitizeIEC61850Identifier(value string, fallback string) string {
	value = strings.TrimSpace(value)
	var result strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' {
			result.WriteRune(character)
		}
	}
	sanitized := result.String()
	if sanitized == "" {
		sanitized = fallback
	}
	if sanitized[0] >= '0' && sanitized[0] <= '9' {
		sanitized = "P" + sanitized
	}
	return sanitized
}

type FeatureConfig struct {
	Enabled        bool   `json:"enabled"`
	Listen         string `json:"listen"`
	ModbusListen   string `json:"modbusListen,omitempty"`
	IEC104Listen   string `json:"iec104Listen,omitempty"`
	IEC61850Listen string `json:"iec61850Listen,omitempty"`
}

type ListenerConfig struct {
	Enabled     bool   `json:"enabled"`
	Listen      string `json:"listen"`
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	TLSKeyFile  string `json:"tlsKeyFile,omitempty"`
}

type SecurityConfig struct {
	Users []UserConfig `json:"users,omitempty"`
	Roles []RoleConfig `json:"roles,omitempty"`
}

type UserConfig struct {
	Username           string   `json:"username"`
	DisplayName        string   `json:"displayName,omitempty"`
	PasswordHash       string   `json:"passwordHash,omitempty"`
	RoleKey            string   `json:"roleKey"`
	Enabled            bool     `json:"enabled"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	MFAEnabled         bool     `json:"mfaEnabled,omitempty"`
	TOTPSecret         string   `json:"totpSecret,omitempty"`
	RecoveryCodeHashes []string `json:"recoveryCodeHashes,omitempty"`
	TOTPLastCounter    int64    `json:"totpLastCounter,omitempty"`
}

type RoleConfig struct {
	RoleKey     string   `json:"roleKey"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	BuiltIn     bool     `json:"builtIn,omitempty"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var source Config
	if err := json.Unmarshal(raw, &source); err != nil {
		return Config{}, err
	}
	if source.Activation.File != "" && !filepath.IsAbs(source.Activation.File) {
		source.Activation.File = filepath.Join(filepath.Dir(path), source.Activation.File)
		raw, err = json.Marshal(source)
		if err != nil {
			return Config{}, err
		}
	}
	cfg, err := Parse(raw)
	if err == nil && cfg.License.Valid && cfg.Activation.Signature != "" {
		if replayErr := AcceptLicense(path, cfg.Activation); replayErr != nil {
			cfg.License = LicenseStatus{Restricted: true, Reason: replayErr.Error()}
		}
	}
	return cfg, err
}

func Parse(raw []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.SchemaVersion < 0 || cfg.SchemaVersion > CurrentSchemaVersion {
		return Config{}, fmt.Errorf("unsupported config schemaVersion %d; this gateway supports up to %d", cfg.SchemaVersion, CurrentSchemaVersion)
	}
	if err := cfg.ApplyActivation(); err != nil {
		return Config{}, err
	}
	if cfg.Activation.IsReady() {
		cfg.GatewayKey = cfg.Activation.SN
		if cfg.Activation.RemoteAccessEndpoint != "" {
			cfg.RemoteAccess.Endpoint = cfg.Activation.RemoteAccessEndpoint
		}
	} else if cfg.GatewayKey == "" && cfg.Activation.SN != "" {
		cfg.GatewayKey = cfg.Activation.SN
	}
	if cfg.GatewayKey == "" {
		return Config{}, fmt.Errorf("gatewayKey is required")
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	if err := validateForwardListenAddress("Modbus TCP", cfg.ForwardSlave.ModbusListen); err != nil {
		return Config{}, err
	}
	if err := validateForwardListenAddress("IEC104", cfg.ForwardSlave.IEC104Listen); err != nil {
		return Config{}, err
	}
	if err := validateForwardListenAddress("IEC61850 MMS", cfg.ForwardSlave.IEC61850Listen); err != nil {
		return Config{}, err
	}
	if err := cfg.validateIEC61850Forwarding(); err != nil {
		return Config{}, err
	}
	if cfg.OfflineCache.Enabled && (cfg.OfflineCache.MaxSizeMB < 12 || cfg.OfflineCache.MaxSizeMB > 16) {
		return Config{}, fmt.Errorf("offline cache maxSizeMB must be between 12 and 16")
	}
	for _, channel := range cfg.ManualMQTTChannels() {
		if channel.IsEnabled() && (channel.Broker == "" || channel.ClientID == "" || channel.Username == "") {
			return Config{}, fmt.Errorf("mqtt broker/clientId/username are required")
		}
	}
	for _, point := range cfg.Points {
		if point.Protocol == "iec104" && (point.IOA == 0 || point.IOA > 0xFFFFFF) {
			return Config{}, fmt.Errorf("IEC104 point %s IOA must be between 1 and 16777215", point.Metric)
		}
	}
	return cfg, nil
}

// ValidateNewGlobalIdentifierDuplicates allows duplicate identifiers already
// present in a legacy project, while preventing a save from introducing or
// increasing a collision. This keeps old forwarding references stable.
func (c Config) ValidateNewGlobalIdentifierDuplicates(previous Config) error {
	currentCounts := c.globalDeviceKeyCounts()
	previousCounts := previous.globalDeviceKeyCounts()
	for key, count := range currentCounts {
		if key == "" {
			return fmt.Errorf("设备标识不能为空")
		}
		if count > 1 && count > previousCounts[key] {
			return fmt.Errorf("设备标识 %s 重复，请使用全局唯一标识", key)
		}
	}
	currentMetrics := c.globalPointMetricCounts()
	previousMetrics := previous.globalPointMetricCounts()
	for metric, count := range currentMetrics {
		if metric == "" {
			if count > previousMetrics[metric] {
				return fmt.Errorf("点位标识符不能为空")
			}
			continue
		}
		if count > 1 && count > previousMetrics[metric] {
			return fmt.Errorf("点位标识符 %s 重复，请使用全局唯一标识符", metric)
		}
	}
	return nil
}

func (c Config) globalDeviceKeyCounts() map[string]int {
	counts := map[string]int{}
	add := func(value string) { counts[strings.ToLower(strings.TrimSpace(value))]++ }
	for _, device := range c.Devices {
		add(device.DeviceKey)
	}
	for _, device := range c.ForwardDevices {
		add(device.DeviceKey)
	}
	for _, group := range c.EdgeComputing.Groups {
		add(group.GroupKey)
	}
	return counts
}

func (c Config) globalPointMetricCounts() map[string]int {
	counts := map[string]int{}
	add := func(value string) { counts[strings.ToLower(strings.TrimSpace(value))]++ }
	if len(c.Devices) > 0 {
		for _, device := range c.Devices {
			for _, point := range device.Points {
				add(point.Metric)
			}
		}
	} else {
		for _, point := range c.Points {
			add(point.Metric)
		}
	}
	// Forward points are transparent aliases of collected points. Their metric
	// intentionally remains identical to SourceMetric and therefore does not
	// consume another entry in the global source/derived metric namespace.
	for _, group := range c.EdgeComputing.Groups {
		for _, point := range group.Points {
			add(point.Metric)
		}
	}
	return counts
}

// ValidateGlobalDeviceKeys keeps the identifier namespace shared by collected
// devices, forwarding devices, and edge-compute virtual devices. Comparisons
// are case-insensitive so identifiers remain unambiguous in APIs and trees.
func (c Config) ValidateGlobalDeviceKeys() error {
	seen := map[string]string{}
	add := func(deviceKey, owner string) error {
		deviceKey = strings.TrimSpace(deviceKey)
		if deviceKey == "" {
			return fmt.Errorf("%s 的设备标识不能为空", owner)
		}
		normalized := strings.ToLower(deviceKey)
		if previous, exists := seen[normalized]; exists {
			return fmt.Errorf("设备标识 %s 重复：%s 与 %s 使用了同一标识", deviceKey, previous, owner)
		}
		seen[normalized] = owner
		return nil
	}
	for index, device := range c.Devices {
		if err := add(device.DeviceKey, fmt.Sprintf("采集设备 %d", index+1)); err != nil {
			return err
		}
	}
	for index, device := range c.ForwardDevices {
		if err := add(device.DeviceKey, fmt.Sprintf("转发设备 %d", index+1)); err != nil {
			return err
		}
	}
	for index, group := range c.EdgeComputing.Groups {
		if err := add(group.GroupKey, fmt.Sprintf("边缘计算组 %d", index+1)); err != nil {
			return err
		}
	}
	return nil
}

func validateForwardListenAddress(protocol, address string) error {
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s forward listen address %q is invalid: %w", protocol, address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s forward listen port must be between 1 and 65535", protocol)
	}
	return nil
}

func (c Config) validateIEC61850Forwarding() error {
	var iedName string
	objectRefs := map[string]string{}
	for _, device := range c.ForwardDevices {
		if !device.IsEnabled() || !strings.EqualFold(device.Protocol, "iec61850-mms-server") {
			continue
		}
		if iedName == "" {
			iedName = device.IEDName
		} else if !strings.EqualFold(iedName, device.IEDName) {
			return fmt.Errorf("IEC61850 MMS 转发设备必须使用相同的 IED 名称，当前同时存在 %s 和 %s", iedName, device.IEDName)
		}
		for _, point := range device.Points {
			fc := strings.ToUpper(strings.TrimSpace(point.FC))
			if fc != "ST" && fc != "MX" {
				return fmt.Errorf("IEC61850 MMS 转发点 %s 的 FC 必须为 ST 或 MX", point.Metric)
			}
			objectRef := strings.ToLower(strings.TrimSpace(point.ObjectRef))
			if objectRef == "" {
				return fmt.Errorf("IEC61850 MMS 转发点 %s 缺少对象引用", point.Metric)
			}
			if previous, exists := objectRefs[objectRef]; exists {
				return fmt.Errorf("IEC61850 MMS 对象引用 %s 重复，点位 %s 与 %s 冲突", point.ObjectRef, previous, point.Metric)
			}
			objectRefs[objectRef] = point.Metric
			dataType := strings.ToLower(strings.TrimSpace(point.DataType))
			switch dataType {
			case "bool", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "float32", "float64", "string":
			default:
				return fmt.Errorf("IEC61850 MMS 转发点 %s 不支持数据类型 %s", point.Metric, point.DataType)
			}
			if dataType == "bool" || dataType == "string" {
				if fc != "ST" || !strings.HasSuffix(objectRef, ".stval") {
					return fmt.Errorf("IEC61850 MMS 转发点 %s 的布尔/字符串值必须使用 stVal[ST]", point.Metric)
				}
			} else if fc != "MX" || !strings.HasSuffix(objectRef, ".mag.f") {
				return fmt.Errorf("IEC61850 MMS 转发点 %s 的数值必须使用 mag.f[MX]", point.Metric)
			}
		}
	}
	return nil
}

func (c Config) validateIEC61850GOOSEForwarding() error {
	for _, device := range c.ForwardDevices {
		if !device.IsEnabled() || !strings.EqualFold(device.Protocol, "iec61850-goose-publisher") {
			continue
		}
		if strings.TrimSpace(device.InterfaceName) == "" {
			return fmt.Errorf("IEC61850 GOOSE 转发设备 %s 缺少网卡名称", device.DeviceKey)
		}
		if strings.TrimSpace(device.GoCBRef) == "" || strings.TrimSpace(device.DataSetRef) == "" {
			return fmt.Errorf("IEC61850 GOOSE 转发设备 %s 必须配置 GoCBRef 和 DataSetRef", device.DeviceKey)
		}
		mac, err := net.ParseMAC(device.DestinationMAC)
		if err != nil || len(mac) != 6 {
			return fmt.Errorf("IEC61850 GOOSE 转发设备 %s 的目标 MAC %q 无效", device.DeviceKey, device.DestinationMAC)
		}
		if device.VLANPriority > 7 {
			return fmt.Errorf("IEC61850 GOOSE 转发设备 %s 的 VLAN 优先级必须为 0-7", device.DeviceKey)
		}
		if len(device.Points) == 0 {
			return fmt.Errorf("IEC61850 GOOSE 转发设备 %s 至少需要一个数据集点位", device.DeviceKey)
		}
		for _, point := range device.Points {
			switch strings.ToLower(strings.TrimSpace(point.DataType)) {
			case "bool", "int8", "uint8", "int16", "uint16", "int32", "uint32", "float32", "float64", "string":
			default:
				return fmt.Errorf("IEC61850 GOOSE 转发点 %s 不支持数据类型 %s", point.Metric, point.DataType)
			}
		}
	}
	return nil
}

func (c *Config) ApplyActivation() error {
	bindingRequired, _ := strconv.ParseBool(strings.TrimSpace(ActivationBindingRequired))
	signatureRequired, _ := strconv.ParseBool(strings.TrimSpace(ActivationSignatureRequired))
	secureChipRequired, _ := strconv.ParseBool(strings.TrimSpace(SecureChipRequired))
	strict := bindingRequired || signatureRequired || secureChipRequired
	c.License = LicenseStatus{Valid: !strict, Restricted: false}
	reject := func(err error) error {
		if !strict {
			return err
		}
		c.License = LicenseStatus{Valid: false, Restricted: true, Reason: err.Error()}
		return nil
	}
	if c.Activation.File == "" && c.Activation.Enabled != nil && !*c.Activation.Enabled {
		if strict {
			return reject(fmt.Errorf("this gateway build requires an active signed hardware license"))
		}
		return nil
	}
	if c.Activation.File != "" {
		raw, err := os.ReadFile(c.Activation.File)
		if err != nil {
			return reject(fmt.Errorf("load activation file failed: %w", err))
		}
		var fileActivation ActivationConfig
		if err := json.Unmarshal(raw, &fileActivation); err != nil {
			return reject(fmt.Errorf("parse activation file failed: %w", err))
		}
		// A local cloud switch must not override signed enabled or metadata.
		if c.CloudConnectionEnabled == nil && c.Activation.Enabled != nil {
			value := *c.Activation.Enabled
			c.CloudConnectionEnabled = &value
		}
		fileActivation.File = c.Activation.File
		c.Activation = fileActivation
	}
	if c.Activation.SN == "" || c.Activation.DeviceSecret == "" {
		if bindingRequired {
			return reject(fmt.Errorf("this gateway build requires a complete activation file"))
		}
		if strict {
			return reject(fmt.Errorf("this gateway build requires a complete activation file"))
		}
		return nil
	}
	identity := hardware.ReadIdentity()
	if err := ValidateActivationPackage(c.Activation, identity); err != nil {
		return reject(err)
	}
	if c.Activation.HardwareProfile != nil {
		c.ProductModel = strings.TrimSpace(c.Activation.ProductModel)
		profile := *c.Activation.HardwareProfile
		c.HardwareProfile = &profile
	}
	if c.Activation.HardwareID == "" {
		c.Activation.HardwareID = identity.ID
	}
	c.License = LicenseStatus{Valid: true}
	return nil
}

// ValidateActivationPackage validates the platform signature, license window
// and immutable hardware binding without mutating the supplied activation.
// It is also used by the activation upload endpoint before replacing the
// currently working file.
func ValidateActivationPackage(activation ActivationConfig, identity hardware.Identity) error {
	bindingRequired, _ := strconv.ParseBool(strings.TrimSpace(ActivationBindingRequired))
	signatureRequired, _ := strconv.ParseBool(strings.TrimSpace(ActivationSignatureRequired))
	secureChipRequired, _ := strconv.ParseBool(strings.TrimSpace(SecureChipRequired))
	if err := validateActivationSignature(activation, signatureRequired, time.Now()); err != nil {
		return err
	}
	if activation.HardwareProfile != nil {
		if strings.TrimSpace(activation.ProductModel) == "" {
			return fmt.Errorf("activation hardware profile requires productModel")
		}
		if err := activation.HardwareProfile.Validate(); err != nil {
			return fmt.Errorf("activation hardware profile is invalid: %w", err)
		}
	}
	if err := validateActivationBinding(activation, identity, bindingRequired); err != nil {
		return err
	}
	return validateSecureChipBinding(activation, secureChipRequired)
}

func validateSecureChipBinding(activation ActivationConfig, required bool) error {
	expectedUSID := strings.ToUpper(strings.TrimSpace(activation.ChipUSID))
	if expectedUSID == "" && !required {
		return nil
	}
	if expectedUSID == "" {
		return fmt.Errorf("this gateway build requires an RJGT102-bound activation")
	}
	identity, err := securechip.Authenticate()
	if err != nil {
		return fmt.Errorf("RJGT102 authentication failed: %w", err)
	}
	if !strings.EqualFold(expectedUSID, identity.USID) {
		return fmt.Errorf("activation chipUsid does not match this gateway")
	}
	return nil
}

func activationPayload(activation ActivationConfig) ([]byte, error) {
	features := append([]string(nil), activation.Features...)
	if features == nil {
		features = []string{}
	}
	payload := activationSignedPayload{
		Enabled: activation.IsEnabled(), LicenseVersion: activation.LicenseVersion,
		KeyID: strings.TrimSpace(activation.KeyID), LicenseSequence: activation.LicenseSequence,
		HardwareID: strings.ToUpper(strings.TrimSpace(activation.HardwareID)),
		ChipUSID:   strings.ToUpper(strings.TrimSpace(activation.ChipUSID)),
		SN:         strings.TrimSpace(activation.SN), DeviceSecret: activation.DeviceSecret,
		Broker: strings.TrimSpace(activation.Broker), CloudAPI: strings.TrimSpace(activation.CloudAPI),
		RemoteAccessEndpoint: strings.TrimSpace(activation.RemoteAccessEndpoint),
		IssuedAt:             strings.TrimSpace(activation.IssuedAt), ExpiresAt: strings.TrimSpace(activation.ExpiresAt),
		Features: features, ProductModel: strings.TrimSpace(activation.ProductModel), HardwareProfile: activation.HardwareProfile,
		Nonce: strings.TrimSpace(activation.Nonce),
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(output.Bytes(), []byte("\n")), nil
}

func validateActivationSignature(activation ActivationConfig, required bool, now time.Time) error {
	publicKeyValue := strings.TrimSpace(ActivationSignaturePublicKey)
	if activation.LicenseVersion == 2 {
		var keys map[string]string
		if err := json.Unmarshal([]byte(ActivationTrustedPublicKeys), &keys); err != nil {
			return fmt.Errorf("trusted activation key ring is not configured")
		}
		publicKeyValue = strings.TrimSpace(keys[activation.KeyID])
		if publicKeyValue == "" {
			return fmt.Errorf("activation keyId is not trusted")
		}
		if activation.KeyID == "" || activation.LicenseSequence < 1 || activation.LicenseSequence > 9007199254740991 {
			return fmt.Errorf("activation keyId or licenseSequence is invalid")
		}
	} else if activation.LicenseVersion != 0 && activation.LicenseVersion != 1 {
		return fmt.Errorf("unsupported activation licenseVersion")
	} else if activation.KeyID != "" || activation.LicenseSequence != 0 {
		return fmt.Errorf("legacy activation must not contain unsigned version metadata")
	}
	signatureValue := strings.TrimSpace(activation.Signature)
	if publicKeyValue == "" || signatureValue == "" {
		if signatureValue != "" || activation.LicenseVersion == 2 {
			return fmt.Errorf("activation signature or trusted key is missing")
		}
		if required {
			return fmt.Errorf("this gateway build requires a signed activation file")
		}
		return nil
	}
	if activation.LicenseVersion < 1 || strings.TrimSpace(activation.IssuedAt) == "" || strings.TrimSpace(activation.Nonce) == "" {
		return fmt.Errorf("signed activation metadata is incomplete")
	}
	issuedAt, err := time.Parse(time.RFC3339, activation.IssuedAt)
	if err != nil {
		return fmt.Errorf("activation issuedAt is invalid: %w", err)
	}
	if issuedAt.After(now.Add(5 * time.Minute)) {
		return fmt.Errorf("activation is not valid yet")
	}
	if strings.TrimSpace(activation.ExpiresAt) != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339, activation.ExpiresAt)
		if parseErr != nil {
			return fmt.Errorf("activation expiresAt is invalid: %w", parseErr)
		}
		if !now.Before(expiresAt) {
			return fmt.Errorf("activation has expired")
		}
	}
	publicKey, err := decodeActivationPublicKey(publicKeyValue)
	if err != nil {
		return fmt.Errorf("activation public key is invalid: %w", err)
	}
	signature, err := decodeActivationBase64(signatureValue)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("activation signature is invalid")
	}
	payload, err := activationPayload(activation)
	if err != nil {
		return fmt.Errorf("build activation signature payload failed: %w", err)
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("activation signature verification failed")
	}
	return nil
}

func decodeActivationPublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := decodeActivationBase64(value)
	if err != nil {
		return nil, err
	}
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}
	parsed, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, err
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not Ed25519")
	}
	return publicKey, nil
}

func decodeActivationBase64(value string) ([]byte, error) {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("invalid base64")
}

func validateActivationBinding(activation ActivationConfig, identity hardware.Identity, required bool) error {
	if !activation.IsEnabled() {
		if required {
			return fmt.Errorf("this gateway build requires a hardware-bound activation")
		}
		return nil
	}
	if activation.SN == "" || activation.DeviceSecret == "" {
		if required {
			return fmt.Errorf("this gateway build requires a complete activation file")
		}
		return nil
	}
	if !identity.Available || strings.TrimSpace(identity.ID) == "" {
		if !required {
			return nil
		}
		return fmt.Errorf("cannot verify activation because the hardware identity is unavailable")
	}
	if activation.HardwareID != "" && !strings.EqualFold(strings.TrimSpace(activation.HardwareID), identity.ID) {
		return fmt.Errorf("activation hardwareId does not match this gateway")
	}
	return nil
}

func (a *ActivationConfig) Merge(other ActivationConfig) {
	if a.KeyID == "" {
		a.KeyID = other.KeyID
	}
	if a.LicenseSequence == 0 {
		a.LicenseSequence = other.LicenseSequence
	}
	if a.Enabled == nil {
		a.Enabled = other.Enabled
	}
	if a.HardwareID == "" {
		a.HardwareID = other.HardwareID
	}
	if a.LicenseVersion == 0 {
		a.LicenseVersion = other.LicenseVersion
	}
	if a.ChipUSID == "" {
		a.ChipUSID = other.ChipUSID
	}
	if a.SN == "" {
		a.SN = other.SN
	}
	if a.DeviceSecret == "" {
		a.DeviceSecret = other.DeviceSecret
	}
	if a.Broker == "" {
		a.Broker = other.Broker
	}
	if a.CloudAPI == "" {
		a.CloudAPI = other.CloudAPI
	}
	if a.RemoteAccessEndpoint == "" {
		a.RemoteAccessEndpoint = other.RemoteAccessEndpoint
	}
	if a.IssuedAt == "" {
		a.IssuedAt = other.IssuedAt
	}
	if a.ExpiresAt == "" {
		a.ExpiresAt = other.ExpiresAt
	}
	if len(a.Features) == 0 {
		a.Features = append([]string(nil), other.Features...)
	}
	if a.ProductModel == "" {
		a.ProductModel = other.ProductModel
	}
	if a.HardwareProfile == nil && other.HardwareProfile != nil {
		copy := *other.HardwareProfile
		a.HardwareProfile = &copy
	}
	if a.Nonce == "" {
		a.Nonce = other.Nonce
	}
	if a.Signature == "" {
		a.Signature = other.Signature
	}
}

func Save(path string, cfg Config) error {
	cfg.applyDefaults(false)
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.Activation.File != "" {
		cfg.Activation = ActivationConfig{Enabled: cfg.Activation.Enabled, File: cfg.Activation.File}
	}
	if len(cfg.Devices) > 0 {
		cfg.Points = nil
	}
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if _, readErr := os.Stat(path); readErr == nil {
		if err := copyPrivateAtomic(path+".previous", path); err != nil {
			return fmt.Errorf("backup previous config: %w", err)
		}
	}

	temp, err := os.CreateTemp(dir, ".gateway-config-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func copyPrivateAtomic(path string, sourcePath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".gateway-private-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := io.Copy(temp, source); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func writePrivateAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".gateway-private-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// Validate enforces the normalized runtime model, including points nested
// under devices. Older validation only inspected the legacy top-level points.
func (c Config) Validate() error {
	if err := c.RemoteStorage.Validate(); err != nil {
		return err
	}
	if c.HardwareProfile != nil {
		if strings.TrimSpace(c.ProductModel) == "" {
			return fmt.Errorf("hardwareProfile requires productModel")
		}
		if err := c.HardwareProfile.Validate(); err != nil {
			return fmt.Errorf("hardwareProfile is invalid: %w", err)
		}
	}
	if c.SchemaVersion != 0 && c.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("unsupported config schemaVersion %d; expected %d", c.SchemaVersion, CurrentSchemaVersion)
	}
	// Duplicate identifiers from older projects remain loadable. Save/import
	// entry points compare against the previous revision and reject newly
	// introduced collisions.
	if (c.Web.TLSCertFile == "") != (c.Web.TLSKeyFile == "") {
		return fmt.Errorf("web TLS certificate and key must be configured together")
	}
	if c.RemoteAccess.Enabled {
		endpoint := strings.ToLower(strings.TrimSpace(c.RemoteAccess.Endpoint))
		if !strings.HasPrefix(endpoint, "ws://") && !strings.HasPrefix(endpoint, "wss://") {
			return fmt.Errorf("remote access endpoint must use ws:// or wss://")
		}
		localURL := strings.ToLower(strings.TrimSpace(c.RemoteAccess.LocalURL))
		if !strings.HasPrefix(localURL, "http://127.0.0.1") && !strings.HasPrefix(localURL, "https://127.0.0.1") && !strings.HasPrefix(localURL, "http://localhost") && !strings.HasPrefix(localURL, "https://localhost") {
			return fmt.Errorf("remote access localUrl must target localhost")
		}
	}
	for _, channel := range c.ManualMQTTChannels() {
		if (channel.CertFile == "") != (channel.KeyFile == "") {
			return fmt.Errorf("MQTT channel %s client certificate and key must be configured together", channel.Name)
		}
		if channel.PublishIntervalMilliseconds < 100 || channel.PublishIntervalMilliseconds > 86400000 {
			return fmt.Errorf("MQTT channel %s publish interval must be between 100 and 86400000 milliseconds", channel.Name)
		}
	}
	if err := c.validateIEC61850Forwarding(); err != nil {
		return err
	}
	if err := c.validateIEC61850GOOSEForwarding(); err != nil {
		return err
	}
	if err := c.validateAlarmRules(); err != nil {
		return err
	}
	if err := c.validateSerialBusProtocols(); err != nil {
		return err
	}
	metrics := map[string]string{}
	return c.forEachCollectionPoint(func(point PointConfig) error {
		metric := strings.TrimSpace(point.Metric)
		if metric == "" {
			return fmt.Errorf("point metric is required for device %s", point.DeviceKey)
		}
		normalized := strings.ToLower(metric)
		if owner, exists := metrics[normalized]; exists {
			return fmt.Errorf("point metric %s is duplicated by %s and %s", metric, owner, point.DeviceKey)
		}
		metrics[normalized] = point.DeviceKey
		if strings.TrimSpace(point.DeviceKey) == "" {
			return fmt.Errorf("point %s deviceKey is required", metric)
		}
		if strings.TrimSpace(point.Protocol) == "" {
			return fmt.Errorf("point %s protocol is required", metric)
		}
		switch point.Protocol {
		case "modbus-tcp", "modbus-rtu":
			if point.Address == "" {
				return fmt.Errorf("Modbus point %s address is required", metric)
			}
			if point.Function != 1 && point.Function != 2 && point.Function != 3 && point.Function != 4 {
				return fmt.Errorf("Modbus point %s function must be 1, 2, 3 or 4", metric)
			}
			if point.Quantity == 0 || point.Quantity > 125 {
				return fmt.Errorf("Modbus point %s quantity must be between 1 and 125", metric)
			}
		case "dlt645":
			if strings.TrimSpace(point.Address) == "" {
				return fmt.Errorf("DL/T 645 点位 %s 缺少串口", metric)
			}
			if point.DLT645 == nil {
				return fmt.Errorf("DL/T 645 点位 %s 缺少协议配置", metric)
			}
			if !validDLT645Address(point.DLT645.MeterAddress) {
				return fmt.Errorf("DL/T 645 点位 %s 的表地址必须是 12 位十进制数字且不能是广播地址", metric)
			}
			if point.DLT645.Version != "auto" && point.DLT645.Version != "2007" && point.DLT645.Version != "1997" {
				return fmt.Errorf("DL/T 645 点位 %s 的版本必须为 auto、2007 或 1997", metric)
			}
			if point.DLT645.ItemKey == "" && !validDLT645DI(point.DLT645.DataIdentifier) {
				return fmt.Errorf("DL/T 645 点位 %s 的数据标识必须是 4 位或 8 位十六进制", metric)
			}
			if point.DLT645.ItemKey == "" && point.DLT645.Version == "2007" && len(point.DLT645.DataIdentifier) != 8 {
				return fmt.Errorf("DL/T 645-2007 点位 %s 的数据标识必须是 8 位十六进制", metric)
			}
			if point.DLT645.ItemKey == "" && point.DLT645.Version == "1997" && len(point.DLT645.DataIdentifier) != 4 {
				return fmt.Errorf("DL/T 645-1997 点位 %s 的数据标识必须是 4 位十六进制", metric)
			}
			if point.DLT645.Access != "read" && point.DLT645.Access != "write" && point.DLT645.Access != "read_write" {
				return fmt.Errorf("DL/T 645 点位 %s 的读写类型必须为 read、write 或 read_write", metric)
			}
			if point.DLT645.ResponseTimeoutMilliseconds < 100 || point.DLT645.ResponseTimeoutMilliseconds > 10000 {
				return fmt.Errorf("DL/T 645 点位 %s 的响应超时必须在 100～10000 毫秒之间", metric)
			}
			if point.DLT645.Retries < 0 || point.DLT645.Retries > 3 || point.DLT645.WakeupBytes < 0 || point.DLT645.WakeupBytes > 8 {
				return fmt.Errorf("DL/T 645 点位 %s 的重试次数或前导字节数超出范围", metric)
			}
		case "iec104":
			if point.Address == "" {
				return fmt.Errorf("IEC104 point %s address is required", metric)
			}
			if point.IOA == 0 || point.IOA > 0xFFFFFF {
				return fmt.Errorf("IEC104 point %s IOA must be between 1 and 16777215", metric)
			}
		case "siemens-s7":
			if point.Address == "" {
				return fmt.Errorf("S7 point %s address is required", metric)
			}
		case "opcua":
			if point.Address == "" || point.NodeID == "" {
				return fmt.Errorf("OPC UA point %s endpoint and nodeId are required", metric)
			}
		case "opcda":
			if strings.TrimSpace(point.Address) == "" {
				return fmt.Errorf("OPC DA point %s server address is required", metric)
			}
			if strings.TrimSpace(point.CLSID) == "" && strings.TrimSpace(point.ProgID) == "" {
				return fmt.Errorf("OPC DA point %s requires clsid or progId", metric)
			}
			if strings.TrimSpace(point.ItemID) == "" {
				return fmt.Errorf("OPC DA point %s itemId is required", metric)
			}
			if point.SecurityMode != OPCDASecurityModern &&
				point.SecurityMode != OPCDASecurityPacketIntegrity &&
				point.SecurityMode != OPCDASecurityPacketPrivacy &&
				point.SecurityMode != OPCDASecurityLegacyConnect {
				return fmt.Errorf("OPC DA point %s securityMode must be modern, packet_integrity, packet_privacy or legacy_connect", metric)
			}
			if point.Access != "read" && point.Access != "write" && point.Access != "read_write" {
				return fmt.Errorf("OPC DA point %s access must be read, write or read_write", metric)
			}
			if point.BatchSize < 1 || point.BatchSize > 10000 {
				return fmt.Errorf("OPC DA point %s batchSize must be between 1 and 10000", metric)
			}
			if point.ConnectTimeoutMilliseconds < 100 || point.ConnectTimeoutMilliseconds > 300000 {
				return fmt.Errorf("OPC DA point %s connect timeout must be between 100 and 300000 milliseconds", metric)
			}
			if point.RequestTimeoutMilliseconds < 100 || point.RequestTimeoutMilliseconds > 300000 {
				return fmt.Errorf("OPC DA point %s request timeout must be between 100 and 300000 milliseconds", metric)
			}
			if point.ReadMode != "sync" && point.ReadMode != "subscription" {
				return fmt.Errorf("OPC DA point %s readMode must be sync or subscription", metric)
			}
			if point.ReadSource != "device" && point.ReadSource != "cache" {
				return fmt.Errorf("OPC DA point %s readSource must be device or cache", metric)
			}
		case "iec61850":
			if point.Address == "" || point.ObjectRef == "" {
				return fmt.Errorf("IEC61850 point %s address and objectRef are required", metric)
			}
		case "iec61850-goose":
			if strings.TrimSpace(point.Address) == "" || strings.TrimSpace(point.GoCBRef) == "" {
				return fmt.Errorf("IEC61850 GOOSE point %s interface and goCbRef are required", metric)
			}
			mac, err := net.ParseMAC(point.DestinationMAC)
			if err != nil || len(mac) != 6 {
				return fmt.Errorf("IEC61850 GOOSE point %s destination MAC %q is invalid", metric, point.DestinationMAC)
			}
			if point.VLANPriority > 7 {
				return fmt.Errorf("IEC61850 GOOSE point %s VLAN priority must be between 0 and 7", metric)
			}
		default:
			return fmt.Errorf("point %s uses unsupported protocol %q", metric, point.Protocol)
		}
		return nil
	})
}

// validateSerialBusProtocols keeps one physical RS-485 port on one collection
// protocol. Two collectors reading the same serial bus concurrently would
// interleave requests and make otherwise valid devices appear unstable.
func (c Config) validateSerialBusProtocols() error {
	resources := make(map[string]ResourceConfig, len(c.Resources))
	for _, resource := range c.Resources {
		resources[resource.ResourceKey] = resource
	}
	type busProtocol struct {
		protocol string
		label    string
	}
	buses := map[string]busProtocol{}
	channelProtocols := map[string]string{}
	for _, channel := range c.Channels {
		if channel.Role == "forward" {
			continue
		}
		resource, hasResource := resources[channel.ResourceKey]
		isSerial := channel.Type == "serial" || (hasResource && resource.Type == "serial")
		if !isSerial {
			continue
		}
		protocol := NormalizeProtocol(channel.Protocol)
		if protocol != "modbus-rtu" && protocol != "dlt645" {
			return fmt.Errorf("串口资源 %s 仅支持 Modbus RTU 或 DL/T 645 采集协议", resourceLabel(resource, channel))
		}
		busKey := channel.ResourceKey
		if hasResource && resource.Serial != nil && strings.TrimSpace(resource.Serial.Port) != "" {
			busKey = strings.TrimSpace(resource.Serial.Port)
		} else if channel.Serial != nil && strings.TrimSpace(channel.Serial.Port) != "" {
			busKey = strings.TrimSpace(channel.Serial.Port)
		} else if busKey == "" {
			busKey = channel.InterfaceName
		}
		label := resourceLabel(resource, channel)
		if previous, exists := buses[busKey]; exists && previous.protocol != protocol {
			return fmt.Errorf("RS485 总线 %s 不能同时使用 %s 和 %s，请在串口资源中统一采集协议", label, serialProtocolLabel(previous.protocol), serialProtocolLabel(protocol))
		}
		buses[busKey] = busProtocol{protocol: protocol, label: label}
		channelProtocols[channel.ChannelKey] = protocol
	}
	for _, device := range c.Devices {
		expected, belongsToSerialBus := channelProtocols[device.ChannelKey]
		if !belongsToSerialBus {
			continue
		}
		actual := NormalizeProtocol(device.Protocol)
		if actual != expected {
			return fmt.Errorf("串口设备 %s 的协议为 %s，与所属 RS485 总线协议 %s 不一致", device.Name, serialProtocolLabel(actual), serialProtocolLabel(expected))
		}
	}
	return nil
}

func resourceLabel(resource ResourceConfig, channel ChannelConfig) string {
	if strings.TrimSpace(resource.Name) != "" {
		return resource.Name
	}
	if strings.TrimSpace(resource.ResourceKey) != "" {
		return resource.ResourceKey
	}
	if strings.TrimSpace(channel.InterfaceName) != "" {
		return channel.InterfaceName
	}
	return channel.ChannelKey
}

func serialProtocolLabel(protocol string) string {
	switch NormalizeProtocol(protocol) {
	case "modbus-rtu":
		return "Modbus RTU"
	case "dlt645":
		return "DL/T 645"
	default:
		return protocol
	}
}

func validDLT645Address(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 12 || value == "999999999999" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validDLT645DI(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 4 && len(value) != 8 {
		return false
	}
	_, err := strconv.ParseUint(value, 16, 32)
	return err == nil
}

func (c Config) validateAlarmRules() error {
	points := map[string]struct{}{}
	physicalPoints := append([]PointConfig(nil), c.Points...)
	for _, device := range c.Devices {
		physicalPoints = append(physicalPoints, device.Points...)
	}
	for _, point := range append(physicalPoints, c.ComputedStatusPoints()...) {
		points[strings.ToLower(strings.TrimSpace(point.DeviceKey))+"::"+strings.ToLower(strings.TrimSpace(point.Metric))] = struct{}{}
	}
	seen := map[string]struct{}{}
	for index, rule := range c.Alarms.Rules {
		label := fmt.Sprintf("告警规则 %d", index+1)
		if strings.TrimSpace(rule.RuleID) == "" {
			return fmt.Errorf("%s 的规则标识不能为空", label)
		}
		normalizedID := strings.ToLower(strings.TrimSpace(rule.RuleID))
		if _, exists := seen[normalizedID]; exists {
			return fmt.Errorf("告警规则标识 %s 重复", rule.RuleID)
		}
		seen[normalizedID] = struct{}{}
		pointKey := strings.ToLower(strings.TrimSpace(rule.DeviceKey)) + "::" + strings.ToLower(strings.TrimSpace(rule.Metric))
		if _, exists := points[pointKey]; !exists {
			return fmt.Errorf("%s 关联的点位 %s/%s 不存在", label, rule.DeviceKey, rule.Metric)
		}
		switch rule.Type {
		case "threshold":
			if rule.Limits != nil {
				if err := rule.Limits.Validate(); err != nil {
					return fmt.Errorf("%s：%w", label, err)
				}
			} else if rule.Threshold == nil {
				return fmt.Errorf("%s 缺少阈值", label)
			}
			if rule.Operator != ">" && rule.Operator != ">=" && rule.Operator != "<" && rule.Operator != "<=" {
				return fmt.Errorf("%s 的比较条件必须为 >、>=、< 或 <=", label)
			}
		case "boolean":
			if rule.BooleanValue == nil {
				return fmt.Errorf("%s 缺少布尔目标值", label)
			}
		case "quality":
			if rule.Quality != "BAD" && rule.Quality != "UNCERTAIN" && rule.Quality != "OFFLINE" {
				return fmt.Errorf("%s 的质量条件必须为 BAD、UNCERTAIN 或 OFFLINE", label)
			}
		case "stale":
		default:
			return fmt.Errorf("%s 的类型必须为 threshold、boolean、quality 或 stale", label)
		}
		if rule.Limits != nil && rule.Type != "threshold" {
			return fmt.Errorf("%s：四限告警仅适用于数值阈值", label)
		}
		if rule.Hysteresis < 0 {
			return fmt.Errorf("%s 的回差不能小于 0", label)
		}
		if rule.TriggerDelaySeconds < 0 || rule.TriggerDelaySeconds > 86400 || rule.RecoveryDelaySeconds < 0 || rule.RecoveryDelaySeconds > 86400 {
			return fmt.Errorf("%s 的触发和恢复延时必须在 0～86400 秒之间", label)
		}
		if rule.Level != "INFO" && rule.Level != "WARNING" && rule.Level != "CRITICAL" {
			return fmt.Errorf("%s 的等级必须为 INFO、WARNING 或 CRITICAL", label)
		}
	}
	if c.Alarms.HistoryLimit < 1 || c.Alarms.HistoryLimit > 10000 {
		return fmt.Errorf("告警历史内存缓存条数必须在 1～10000 之间")
	}
	if err := c.Alarms.ValidateRuntimeRuleIDs(); err != nil {
		return err
	}
	if c.Alarms.PopupDuration() < 0 || c.Alarms.PopupDuration() > 3600 {
		return fmt.Errorf("告警弹窗显示时长必须为 0～3600 秒，0 表示不自动关闭")
	}
	if c.Alarms.RetentionLimit < c.Alarms.HistoryLimit || c.Alarms.RetentionLimit > 1000000 {
		return fmt.Errorf("告警历史数据库保留条数必须在内存缓存条数～1000000 之间")
	}
	return nil
}

func (c Config) forEachCollectionPoint(visit func(PointConfig) error) error {
	if len(c.Devices) == 0 {
		for _, point := range c.Points {
			if !point.IsEnabled() {
				continue
			}
			if err := visit(point); err != nil {
				return err
			}
		}
		return nil
	}
	for _, device := range c.Devices {
		for _, point := range device.Points {
			if !point.IsEnabled() {
				continue
			}
			if err := visit(point); err != nil {
				return err
			}
		}
	}
	return nil
}

// RedactSecrets returns a detached copy suitable for read-only APIs and
// portable project exports. The marker is restored from the current config
// when a masked document is saved.
func RedactSecrets(source Config) Config {
	raw, _ := json.Marshal(source)
	var result Config
	_ = json.Unmarshal(raw, &result)
	// User management has a dedicated API. Password hashes must never be
	// exposed through the general configuration or project export APIs.
	result.Security = SecurityConfig{}
	for i := range result.RemoteStorage.Targets {
		if result.RemoteStorage.Targets[i].Password != "" {
			result.RemoteStorage.Targets[i].Password = "***"
		}
	}
	if result.MQTT.Password != "" {
		result.MQTT.Password = "***"
	}
	for i := range result.MQTTChannels {
		if result.MQTTChannels[i].Password != "" {
			result.MQTTChannels[i].Password = "***"
		}
	}
	if result.WiFi.Password != "" {
		result.WiFi.Password = "***"
	}
	if result.Activation.DeviceSecret != "" {
		result.Activation.DeviceSecret = "***"
	}
	for i := range result.Devices {
		if result.Devices[i].Password != "" {
			result.Devices[i].Password = "***"
		}
		if result.Devices[i].DLT645 != nil {
			if result.Devices[i].DLT645.WritePassword != "" {
				result.Devices[i].DLT645.WritePassword = "***"
			}
			if result.Devices[i].DLT645.OperatorCode != "" {
				result.Devices[i].DLT645.OperatorCode = "***"
			}
		}
		for j := range result.Devices[i].Points {
			if result.Devices[i].Points[j].Password != "" {
				result.Devices[i].Points[j].Password = "***"
			}
			if result.Devices[i].Points[j].DLT645 != nil {
				if result.Devices[i].Points[j].DLT645.WritePassword != "" {
					result.Devices[i].Points[j].DLT645.WritePassword = "***"
				}
				if result.Devices[i].Points[j].DLT645.OperatorCode != "" {
					result.Devices[i].Points[j].DLT645.OperatorCode = "***"
				}
			}
		}
	}
	for i := range result.Points {
		if result.Points[i].Password != "" {
			result.Points[i].Password = "***"
		}
		if result.Points[i].DLT645 != nil {
			if result.Points[i].DLT645.WritePassword != "" {
				result.Points[i].DLT645.WritePassword = "***"
			}
			if result.Points[i].DLT645.OperatorCode != "" {
				result.Points[i].DLT645.OperatorCode = "***"
			}
		}
	}
	return result
}

func PreserveMaskedSecrets(current Config, next *Config) {
	if next == nil {
		return
	}
	PreserveRemoteStorageSecrets(current.RemoteStorage, &next.RemoteStorage)
	if len(next.Security.Users) == 0 {
		next.Security = current.Security
	}
	if next.MQTT.Password == "***" {
		next.MQTT.Password = current.MQTT.Password
	}
	currentByClientID := map[string]string{}
	clientIDCount := map[string]int{}
	currentByName := map[string]string{}
	nameCount := map[string]int{}
	for _, channel := range current.MQTTChannels {
		if channel.ClientID != "" {
			currentByClientID[channel.ClientID] = channel.Password
			clientIDCount[channel.ClientID]++
		}
		if channel.Name != "" {
			currentByName[channel.Name] = channel.Password
			nameCount[channel.Name]++
		}
	}
	for i := range next.MQTTChannels {
		if next.MQTTChannels[i].Password == "***" {
			name := next.MQTTChannels[i].Name
			if name != "" && nameCount[name] == 1 {
				next.MQTTChannels[i].Password = currentByName[name]
				continue
			}
			clientID := next.MQTTChannels[i].ClientID
			if clientID != "" && clientIDCount[clientID] == 1 {
				next.MQTTChannels[i].Password = currentByClientID[clientID]
				continue
			}
			if i < len(current.MQTTChannels) {
				next.MQTTChannels[i].Password = current.MQTTChannels[i].Password
			} else {
				next.MQTTChannels[i].Password = ""
			}
		}
	}
	if next.WiFi.Password == "***" {
		next.WiFi.Password = current.WiFi.Password
	}
	if next.Activation.DeviceSecret == "***" {
		next.Activation.DeviceSecret = current.Activation.DeviceSecret
	}
	currentDevices := map[string]DeviceConfig{}
	for _, device := range current.Devices {
		currentDevices[device.DeviceKey] = device
	}
	for i := range next.Devices {
		previous := currentDevices[next.Devices[i].DeviceKey]
		if next.Devices[i].Password == "***" {
			next.Devices[i].Password = previous.Password
		}
		if next.Devices[i].DLT645 != nil && previous.DLT645 != nil {
			if next.Devices[i].DLT645.WritePassword == "***" {
				next.Devices[i].DLT645.WritePassword = previous.DLT645.WritePassword
			}
			if next.Devices[i].DLT645.OperatorCode == "***" {
				next.Devices[i].DLT645.OperatorCode = previous.DLT645.OperatorCode
			}
		}
		previousPoints := map[string]PointConfig{}
		for _, point := range previous.Points {
			previousPoints[point.Metric] = point
		}
		for j := range next.Devices[i].Points {
			if next.Devices[i].Points[j].Password == "***" {
				next.Devices[i].Points[j].Password = previousPoints[next.Devices[i].Points[j].Metric].Password
			}
		}
	}
}

func (c Config) CollectInterval() time.Duration {
	interval := intervalDuration(c.CollectIntervalMilliseconds, c.CollectIntervalSeconds)
	for _, channel := range c.Channels {
		candidate := intervalDuration(channel.CollectIntervalMilliseconds, channel.CollectIntervalSeconds)
		if candidate > 0 && (interval <= 0 || candidate < interval) {
			interval = candidate
		}
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return interval
}

func intervalDuration(milliseconds, seconds int) time.Duration {
	if milliseconds > 0 {
		return time.Duration(milliseconds) * time.Millisecond
	}
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func (p PointConfig) CollectInterval(fallback time.Duration) time.Duration {
	if interval := intervalDuration(p.CollectIntervalMilliseconds, p.CollectIntervalSeconds); interval > 0 {
		return interval
	}
	if fallback > 0 {
		return fallback
	}
	return 5 * time.Second
}

func (c *Config) ApplyDefaults() {
	c.applyDefaults(true)
}

func (c *Config) applyDefaults(flatten bool) {
	if c.SchemaVersion == 0 {
		c.SchemaVersion = CurrentSchemaVersion
	}
	hadDevices := len(c.Devices) > 0
	if c.CollectIntervalMilliseconds <= 0 && c.CollectIntervalSeconds <= 0 {
		c.CollectIntervalSeconds = 5
	}
	if c.CacheFile == "" {
		c.CacheFile = ".runtime/gateway-spool.jsonl"
	}
	if c.OfflineCache.MaxSizeMB == 0 {
		c.OfflineCache.MaxSizeMB = 16
	}
	c.HistoryStorage.ApplyDefaults()
	c.RemoteStorage.ApplyDefaults()
	c.RemoteAccess.ApplyDefaults()
	c.ForwardSlave.ApplyDefaults()
	c.MQTT.ApplyDefaults()
	for i := range c.MQTTChannels {
		c.MQTTChannels[i].ApplyDefaults()
	}
	if len(c.SerialPorts) == 0 {
		c.SerialPorts = []SerialPort{
			{Name: "Serial1", Port: "COM1", Enabled: true},
			{Name: "Serial2", Port: "COM2", Enabled: true},
		}
	}
	if len(c.NetworkPorts) == 0 {
		c.NetworkPorts = []NetworkPort{
			{Name: "网口1", Interface: "eth0", Mode: "static", PrefixLength: 24, Enabled: true},
			{Name: "网口2", Interface: "eth1", Mode: "static", PrefixLength: 24, Enabled: true},
		}
	}
	for i := range c.SerialPorts {
		c.SerialPorts[i].ApplyDefaults()
	}
	for i := range c.NetworkPorts {
		c.NetworkPorts[i].ApplyDefaults()
	}
	c.WiFi.ApplyDefaults()
	c.Cellular.ApplyDefaults()
	c.Watchdog.ApplyDefaults()
	c.ApplyResourceDefaults()
	c.ApplyChannelDefaults()
	c.ApplyForwardDefaults()
	c.EdgeComputing.ApplyDefaults()
	c.Alarms.ApplyDefaults()
	for i := range c.ForwardDevices {
		c.ForwardDevices[i].ApplyDefaults(i)
		if c.ForwardDevices[i].Protocol == "iec61850-goose-publisher" && c.ForwardDevices[i].ChannelKey != "" {
			if channel, ok := c.channel(c.ForwardDevices[i].ChannelKey); ok {
				if channel.Network != nil && channel.Network.Interface != "" {
					c.ForwardDevices[i].InterfaceName = channel.Network.Interface
				} else if c.ForwardDevices[i].InterfaceName == "" {
					c.ForwardDevices[i].InterfaceName = channel.InterfaceName
				}
			}
		}
	}
	if len(c.Devices) > 0 {
		hadDevices = true
	}
	for i := range c.Points {
		c.Points[i].ApplyDefaults()
	}
	if len(c.Devices) == 0 && len(c.Points) > 0 {
		c.Devices = devicesFromFlatPoints(c.Points)
	}
	if len(c.Devices) > 0 {
		for i := range c.Devices {
			c.Devices[i].ApplyDefaults()
			if c.Devices[i].ChannelKey != "" {
				if channel, ok := c.channel(c.Devices[i].ChannelKey); ok {
					applyChannelConnection(&c.Devices[i], channel)
				}
			}
			if c.Devices[i].InterfaceType == "" {
				if c.Devices[i].Protocol == "modbus-rtu" || c.Devices[i].Protocol == "dlt645" {
					c.Devices[i].InterfaceType = "serial"
				} else {
					c.Devices[i].InterfaceType = "network"
				}
			}
			if c.Devices[i].InterfaceName == "" {
				if c.Devices[i].InterfaceType == "serial" && len(c.SerialPorts) > 0 {
					c.Devices[i].InterfaceName = c.SerialPorts[0].Name
				}
				if c.Devices[i].InterfaceType == "network" && len(c.NetworkPorts) > 0 {
					c.Devices[i].InterfaceName = c.NetworkPorts[0].Name
				}
			}
			if hadDevices && (c.Devices[i].Protocol == "modbus-rtu" || c.Devices[i].Protocol == "dlt645") {
				if serialPort, ok := c.serialPort(c.Devices[i].InterfaceName); ok && serialPort.Enabled {
					c.Devices[i].Address = serialPort.Port
					for pointIndex := range c.Devices[i].Points {
						applyDeviceConnection(&c.Devices[i].Points[pointIndex], c.Devices[i])
						applySerialConnection(&c.Devices[i].Points[pointIndex], serialPort)
					}
				}
			}
		}
		if flatten {
			c.Points = c.FlattenPoints()
		}
	}
}

func (c *Config) ApplyResourceDefaults() {
	if len(c.Resources) == 0 && c.HardwareProfile == nil {
		if len(c.Channels) > 0 {
			c.Resources = c.resourcesFromChannels()
		} else {
			c.Resources = c.resourcesFromLegacyConfig()
		}
	}
	c.SerialPorts = nil
	c.NetworkPorts = nil
	for i := range c.Resources {
		resource := &c.Resources[i]
		if resource.ResourceKey == "" {
			resource.ResourceKey = fmt.Sprintf("resource-%03d", i+1)
		}
		if resource.Name == "" {
			resource.Name = resource.ResourceKey
		}
		if resource.Type == "" {
			if resource.Serial != nil {
				resource.Type = "serial"
			} else {
				resource.Type = "network"
			}
		}
		if resource.Type == "serial" {
			serial := SerialPort{Name: resource.Name, Enabled: resource.Enabled}
			if resource.Serial != nil {
				serial = *resource.Serial
			}
			if serial.Name == "" {
				serial.Name = resource.Name
			}
			if serial.Port == "" {
				serial.Port = serial.Name
			}
			serial.Enabled = resource.Enabled
			serial.ApplyDefaults()
			resource.Serial = &serial
			c.SerialPorts = append(c.SerialPorts, serial)
		} else {
			network := NetworkPort{Name: resource.Name, Enabled: resource.Enabled}
			if resource.Network != nil {
				network = *resource.Network
			}
			if network.Name == "" {
				network.Name = resource.Name
			}
			network.Enabled = resource.Enabled
			network.ApplyDefaults()
			resource.Network = &network
			c.NetworkPorts = append(c.NetworkPorts, network)
		}
	}
}

func (c *Config) ApplyChannelDefaults() {
	if len(c.Channels) == 0 {
		c.Channels = c.channelsFromLegacyConfig()
		return
	}
	usedChannelKeys := map[string]bool{}
	for i := range c.Channels {
		channel := &c.Channels[i]
		channel.Protocol = NormalizeProtocol(channel.Protocol)
		originalChannelKey := channel.ChannelKey
		if channel.ChannelKey == "" {
			channel.ChannelKey = fmt.Sprintf("channel-%03d", i+1)
		}
		if channel.Name == "" {
			channel.Name = channel.ChannelKey
		}
		if channel.Role == "" {
			channel.Role = "collect"
			hasCollectionDevice := false
			for _, device := range c.Devices {
				if device.ChannelKey == channel.ChannelKey {
					hasCollectionDevice = true
					break
				}
			}
			if !hasCollectionDevice && channel.ForwardProtocol != "" && channel.ForwardProtocol != "none" {
				channel.Role = "forward"
			}
		}
		if channel.Role == "forward" {
			channel.Protocol = "none"
			if channel.ForwardProtocol == "" || channel.ForwardProtocol == "none" {
				channel.ForwardProtocol = "modbus-tcp-slave"
			}
		} else if channel.Protocol == "" || channel.Protocol == "none" {
			channel.Protocol = "modbus-tcp"
		}
		if channel.ForwardProtocol == "" {
			channel.ForwardProtocol = "none"
		}
		if channel.CollectIntervalMilliseconds <= 0 && channel.CollectIntervalSeconds <= 0 {
			channel.CollectIntervalMilliseconds = c.CollectIntervalMilliseconds
			channel.CollectIntervalSeconds = c.CollectIntervalSeconds
		}
		if channel.CollectIntervalMilliseconds <= 0 && channel.CollectIntervalSeconds <= 0 {
			channel.CollectIntervalSeconds = 5
		}
		if channel.Protocol == "iec104" {
			channel.IEC104.ApplyDefaults()
		}
		if usedChannelKeys[channel.ChannelKey] {
			channel.ChannelKey = uniqueChannelKey(channel.ChannelKey, i+1, usedChannelKeys)
			c.remapDevicesForRenamedChannel(originalChannelKey, channel.ChannelKey, channel.Protocol)
		}
		usedChannelKeys[channel.ChannelKey] = true
		if channel.ResourceKey == "" {
			channel.ResourceKey = c.defaultResourceKey(channel.Type, channel.Protocol)
		}
		if channel.Type == "" {
			if channel.Protocol == "modbus-rtu" || channel.Protocol == "dlt645" {
				channel.Type = "serial"
			} else {
				channel.Type = "network"
			}
		}
		if resource, ok := c.resource(channel.ResourceKey); ok {
			channel.Type = resource.Type
			channel.InterfaceName = resource.Name
			if resource.Type == "serial" && resource.Serial != nil {
				serial := *resource.Serial
				channel.Serial = &serial
			}
			if resource.Type == "network" && resource.Network != nil {
				network := *resource.Network
				if channel.Network != nil && channel.Network.Address != "" {
					network.Address = channel.Network.Address
				}
				channel.Network = &network
			}
		} else if channel.InterfaceName == "" {
			channel.InterfaceName = channel.Name
		}
		if channel.Type == "serial" {
			serial := SerialPort{Name: channel.InterfaceName, Enabled: channel.Enabled}
			if channel.Serial != nil {
				serial = *channel.Serial
				if serial.Name == "" {
					serial.Name = channel.InterfaceName
				}
				serial.Enabled = channel.Enabled
				serial.ApplyDefaults()
				channel.Serial = &serial
			}
			if serial.Name == "" {
				serial.Name = channel.Name
			}
			if _, ok := c.serialPort(serial.Name); !ok && channel.Serial != nil {
				c.SerialPorts = append(c.SerialPorts, serial)
			}
		} else {
			network := NetworkPort{Name: channel.InterfaceName, Enabled: channel.Enabled}
			if channel.Network != nil {
				network = *channel.Network
				if network.Name == "" {
					network.Name = channel.InterfaceName
				}
				network.Enabled = channel.Enabled
				network.ApplyDefaults()
				channel.Network = &network
			}
			if network.Name == "" {
				network.Name = channel.Name
			}
			if _, ok := c.networkPort(network.Name); !ok && channel.Network != nil {
				c.NetworkPorts = append(c.NetworkPorts, network)
			}
		}
		for deviceIndex := range channel.Devices {
			device := channel.Devices[deviceIndex]
			device.ChannelKey = channel.ChannelKey
			applyChannelConnection(&device, *channel)
			c.Devices = append(c.Devices, device)
		}
	}
}

func uniqueChannelKey(base string, index int, used map[string]bool) string {
	if base == "" {
		base = "channel"
	}
	for suffix := 1; ; suffix++ {
		candidate := fmt.Sprintf("%s-%02d", base, suffix)
		if index > 0 {
			candidate = fmt.Sprintf("%s-%02d", base, index+suffix)
		}
		if !used[candidate] {
			return candidate
		}
	}
}

func (c *Config) remapDevicesForRenamedChannel(oldKey string, newKey string, protocol string) {
	if oldKey == "" || newKey == "" || oldKey == newKey {
		return
	}
	for i := range c.Devices {
		device := &c.Devices[i]
		if device.ChannelKey != oldKey {
			continue
		}
		if protocol != "" && device.Protocol != "" && device.Protocol != protocol {
			continue
		}
		device.ChannelKey = newKey
		for pointIndex := range device.Points {
			device.Points[pointIndex].ChannelKey = newKey
		}
	}
	for i := range c.ForwardDevices {
		if c.ForwardDevices[i].ChannelKey == oldKey {
			c.ForwardDevices[i].ChannelKey = newKey
		}
	}
}

func (i *IEC104Config) ApplyDefaults() {
	if i.AcquisitionMode == "" && i.GeneralInterrogationIntervalSeconds > 0 {
		i.AcquisitionMode = "periodic"
	} else if i.AcquisitionMode != "periodic" {
		i.AcquisitionMode = "auto"
	}
	if !i.Configured && !i.GeneralInterrogationOnStart && i.GeneralInterrogationIntervalSeconds == 0 && !i.ClockSyncOnStart && i.ClockSyncIntervalSeconds == 0 && !i.CounterInterrogationOnStart && i.CounterInterrogationIntervalSeconds == 0 {
		i.GeneralInterrogationOnStart = true
		if i.AcquisitionMode == "periodic" {
			i.GeneralInterrogationIntervalSeconds = 5
		}
		return
	}
	if i.ClockSyncIntervalSeconds < 0 {
		i.ClockSyncIntervalSeconds = 0
	}
	if i.GeneralInterrogationIntervalSeconds < 0 {
		i.GeneralInterrogationIntervalSeconds = 0
	}
	if i.CounterInterrogationIntervalSeconds < 0 {
		i.CounterInterrogationIntervalSeconds = 0
	}
	if i.AcquisitionMode == "auto" {
		i.GeneralInterrogationIntervalSeconds = 0
	} else if i.GeneralInterrogationIntervalSeconds == 0 {
		i.GeneralInterrogationIntervalSeconds = 5
	}
}

func (c *Config) ApplyForwardDefaults() {
	c.ForwardSlave.ApplyDefaults()
	for _, device := range c.ForwardDevices {
		if device.IsEnabled() && device.Protocol != "" && device.Protocol != "none" {
			c.ForwardSlave.Enabled = true
			return
		}
	}
	for _, channel := range c.Channels {
		if channel.ForwardProtocol != "" && channel.ForwardProtocol != "none" {
			c.ForwardSlave.Enabled = true
			return
		}
	}
}

func (c Config) channelsFromLegacyConfig() []ChannelConfig {
	var channels []ChannelConfig
	for _, resource := range c.Resources {
		protocol := "modbus-tcp"
		if resource.Type == "serial" {
			protocol = "modbus-rtu"
		}
		channels = append(channels, ChannelConfig{
			ChannelKey:                  resource.ResourceKey + "-" + protocol,
			ResourceKey:                 resource.ResourceKey,
			Name:                        resource.Name + " " + protocol,
			Type:                        resource.Type,
			Protocol:                    protocol,
			CollectIntervalSeconds:      c.CollectIntervalSeconds,
			CollectIntervalMilliseconds: c.CollectIntervalMilliseconds,
			InterfaceName:               resource.Name,
			Enabled:                     resource.Enabled,
			Serial:                      resource.Serial,
			Network:                     resource.Network,
		})
	}
	return channels
}

func (f *FeatureConfig) ApplyDefaults() {
	if f.Listen == "" {
		f.Listen = "0.0.0.0:1502"
	}
	if f.ModbusListen == "" {
		f.ModbusListen = f.Listen
	}
	if f.IEC104Listen == "" {
		f.IEC104Listen = "0.0.0.0:2404"
	}
	if f.IEC61850Listen == "" {
		f.IEC61850Listen = "0.0.0.0:102"
	}
}

func (c Config) resourcesFromLegacyConfig() []ResourceConfig {
	var resources []ResourceConfig
	for _, serial := range c.SerialPorts {
		resources = append(resources, ResourceConfig{
			ResourceKey: serial.Name,
			Name:        serial.Name,
			Type:        "serial",
			Enabled:     serial.Enabled,
			Serial:      &serial,
		})
	}
	for _, network := range c.NetworkPorts {
		resources = append(resources, ResourceConfig{
			ResourceKey: network.Name,
			Name:        network.Name,
			Type:        "network",
			Enabled:     network.Enabled,
			Network:     &network,
		})
	}
	return resources
}

func (c Config) resourcesFromChannels() []ResourceConfig {
	var resources []ResourceConfig
	seen := map[string]bool{}
	for index, channel := range c.Channels {
		key := channel.ResourceKey
		if key == "" {
			key = channel.InterfaceName
		}
		if key == "" {
			key = channel.Name
		}
		if key == "" {
			key = fmt.Sprintf("resource-%03d", index+1)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		resourceType := channel.Type
		if resourceType == "" {
			if channel.Protocol == "modbus-rtu" {
				resourceType = "serial"
			} else {
				resourceType = "network"
			}
		}
		name := channel.InterfaceName
		if name == "" {
			name = channel.Name
		}
		if name == "" {
			name = key
		}
		resource := ResourceConfig{
			ResourceKey: key,
			Name:        name,
			Type:        resourceType,
			Enabled:     channel.Enabled,
		}
		if resourceType == "serial" {
			if channel.Serial != nil {
				serial := *channel.Serial
				resource.Serial = &serial
			}
		} else if channel.Network != nil {
			network := *channel.Network
			resource.Network = &network
		}
		resources = append(resources, resource)
	}
	return resources
}

func (c Config) resource(key string) (ResourceConfig, bool) {
	for _, resource := range c.Resources {
		if resource.ResourceKey == key {
			return resource, true
		}
	}
	return ResourceConfig{}, false
}

func (c Config) defaultResourceKey(channelType string, protocol string) string {
	wantType := channelType
	if wantType == "" {
		if protocol == "modbus-rtu" {
			wantType = "serial"
		} else {
			wantType = "network"
		}
	}
	for _, resource := range c.Resources {
		if resource.Type == wantType {
			return resource.ResourceKey
		}
	}
	return ""
}

func (c Config) channel(key string) (ChannelConfig, bool) {
	for _, channel := range c.Channels {
		if channel.ChannelKey == key {
			return channel, true
		}
	}
	return ChannelConfig{}, false
}

func (c Config) serialPort(name string) (SerialPort, bool) {
	for _, serialPort := range c.SerialPorts {
		if serialPort.Name == name {
			return serialPort, true
		}
	}
	return SerialPort{}, false
}

func (c Config) networkPort(name string) (NetworkPort, bool) {
	for _, networkPort := range c.NetworkPorts {
		if networkPort.Name == name {
			return networkPort, true
		}
	}
	return NetworkPort{}, false
}

func (p *SerialPort) ApplyDefaults() {
	if p.Name == "" {
		p.Name = p.Port
	}
	if p.BaudRate == 0 {
		p.BaudRate = 9600
	}
	if p.DataBits == 0 {
		p.DataBits = 8
	}
	if p.StopBits == 0 {
		p.StopBits = 1
	}
	if p.Parity == "" {
		p.Parity = "none"
	}
}

func (p *NetworkPort) ApplyDefaults() {
	if p.Name == "" {
		p.Name = p.Interface
	}
	if p.Interface == "" {
		switch p.Name {
		case "net1", "网口1":
			p.Interface = "eth0"
		case "net2", "网口2":
			p.Interface = "eth1"
		}
	}
	if p.Mode == "" || p.Mode == "tcp-client" || p.Mode == "tcp-server" {
		p.Mode = "static"
	}
	if p.PrefixLength == 0 {
		p.PrefixLength = 24
	}
}

func (w *WiFiConfig) ApplyDefaults() {
	if w.Interface == "" {
		w.Interface = "wlan0"
	}
	if w.Mode == "" {
		w.Mode = "dhcp"
	}
	if w.PrefixLength == 0 {
		w.PrefixLength = 24
	}
}

func (c *CellularConfig) ApplyDefaults() {
	if c.Interface == "" {
		c.Interface = "usbeth0"
	}
}

func (m *MQTTConfig) ApplyDefaults() {
	if m.Name == "" {
		m.Name = "手动 MQTT"
	}
	if m.TopicTemplate == "" {
		m.TopicTemplate = "attributes"
	}
	if m.PayloadMode == "" {
		m.PayloadMode = "flat"
	}
	if m.PublishIntervalMilliseconds <= 0 {
		m.PublishIntervalMilliseconds = 1000
	}
}

func (m MQTTConfig) IsEnabled() bool {
	if m.Enabled != nil {
		return *m.Enabled
	}
	return m.Broker != "" || m.ClientID != "" || m.Username != "" || m.Password != ""
}

func (m MQTTConfig) Equal(other MQTTConfig) bool {
	return m.IsEnabled() == other.IsEnabled() &&
		m.Name == other.Name &&
		m.Broker == other.Broker &&
		m.ClientID == other.ClientID &&
		m.Username == other.Username &&
		m.Password == other.Password &&
		m.TopicTemplate == other.TopicTemplate &&
		m.PayloadMode == other.PayloadMode &&
		m.PayloadTemplate == other.PayloadTemplate &&
		m.SubscribeTopic == other.SubscribeTopic &&
		m.PublishIntervalMilliseconds == other.PublishIntervalMilliseconds &&
		m.CAFile == other.CAFile &&
		m.CertFile == other.CertFile &&
		m.KeyFile == other.KeyFile
}

func (c Config) ManualMQTTChannels() []MQTTConfig {
	if len(c.MQTTChannels) > 0 {
		channels := make([]MQTTConfig, 0, len(c.MQTTChannels))
		for index, channel := range c.MQTTChannels {
			if channel.Name == "" {
				channel.Name = fmt.Sprintf("手动 MQTT %d", index+1)
			}
			channel.ApplyDefaults()
			channels = append(channels, channel)
		}
		return channels
	}
	if !c.MQTT.IsEnabled() {
		return nil
	}
	channel := c.MQTT
	if channel.Name == "" {
		channel.Name = "手动 MQTT 1"
	}
	channel.ApplyDefaults()
	return []MQTTConfig{channel}
}

func (c Config) ManualMQTTEnabled() bool {
	for _, channel := range c.ManualMQTTChannels() {
		if channel.IsEnabled() {
			return true
		}
	}
	return false
}

func EqualMQTTChannels(left, right []MQTTConfig) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !left[index].Equal(right[index]) {
			return false
		}
	}
	return true
}

func (a ActivationConfig) IsEnabled() bool {
	if a.Enabled != nil {
		return *a.Enabled
	}
	return a.File != "" || (a.SN != "" && a.DeviceSecret != "")
}

func (a ActivationConfig) IsReady() bool {
	return a.IsEnabled() && a.HardwareID != "" && a.SN != "" && a.DeviceSecret != ""
}

// AllowsFeature keeps legacy development activations compatible while making
// a signed feature list authoritative when one is present.
func (a ActivationConfig) AllowsFeature(feature string) bool {
	if len(a.Features) == 0 {
		return true
	}
	for _, current := range a.Features {
		if strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(feature)) {
			return true
		}
	}
	return false
}

func (c Config) FlattenPoints() []PointConfig {
	if len(c.Devices) == 0 {
		points := append([]PointConfig(nil), c.Points...)
		active := points[:0]
		for i := range points {
			points[i].ApplyDefaults()
			if points[i].IsEnabled() {
				active = append(active, points[i])
			}
		}
		return active
	}
	var points []PointConfig
	for _, device := range c.Devices {
		if !device.IsEnabled() {
			continue
		}
		channel, hasChannel := c.channel(device.ChannelKey)
		for _, point := range device.Points {
			if !point.IsEnabled() {
				continue
			}
			applyDeviceConnection(&point, device)
			if hasChannel {
				applyChannelPointDefaults(&point, channel)
			}
			point.ApplyDefaults()
			points = append(points, point)
		}
	}
	return points
}

func (d *DeviceConfig) ApplyDefaults() {
	d.Protocol = NormalizeProtocol(d.Protocol)
	if d.Protocol == "" {
		d.Protocol = "modbus-tcp"
	}
	if d.Protocol == "siemens-s7" && d.PLCModel == "" {
		d.PLCModel = "s7-200-smart"
	}
	if d.SlaveID == 0 {
		d.SlaveID = 1
	}
	if d.ReconnectIntervalSeconds <= 0 {
		d.ReconnectIntervalSeconds = 30
	}
	if d.Protocol == "opcda" {
		d.SecurityMode = NormalizeOPCDASecurityMode(d.SecurityMode, d.AuthenticationLevel)
		if d.Authentication == "" {
			d.Authentication = "ntlm"
		}
		d.AuthenticationLevel = OPCDAAuthenticationLevel(d.SecurityMode)
		if d.ConnectTimeoutMilliseconds <= 0 {
			d.ConnectTimeoutMilliseconds = 10000
		}
		if d.RequestTimeoutMilliseconds <= 0 {
			d.RequestTimeoutMilliseconds = 10000
		}
		if d.ReadMode == "" {
			d.ReadMode = "sync"
		}
		if d.ReadSource == "" {
			d.ReadSource = "device"
		}
		if d.BatchSize <= 0 {
			d.BatchSize = 100
		}
		if d.AutoReconnect == nil {
			enabled := true
			d.AutoReconnect = &enabled
		}
		if d.GroupName == "" {
			d.GroupName = "WK_OPCDA"
		}
		if d.RequestedUpdateRateMS <= 0 {
			d.RequestedUpdateRateMS = 1000
		}
		if d.LocaleID == 0 {
			d.LocaleID = 0x0409
		}
	}
	if d.Protocol == "dlt645" {
		if d.DLT645 == nil {
			d.DLT645 = &DLT645Config{}
		}
		d.DLT645.ApplyDeviceDefaults()
	}
	if d.Protocol == "iec104" && d.CommonAddress == 0 {
		d.CommonAddress = uint16(d.SlaveID)
	}
	if d.Protocol == "iec61850-goose" {
		if d.AppID == 0 {
			d.AppID = 0x1000
		}
		if d.DestinationMAC == "" {
			d.DestinationMAC = "01:0c:cd:01:00:01"
		}
		if d.VLANPriority > 7 {
			d.VLANPriority = 4
		}
	}
	for i := range d.Points {
		applyDeviceConnection(&d.Points[i], *d)
		d.Points[i].ApplyDefaults()
	}
}

func applyDeviceConnection(point *PointConfig, device DeviceConfig) {
	point.DeviceKey = device.DeviceKey
	point.ChannelKey = device.ChannelKey
	point.Protocol = device.Protocol
	if point.CollectIntervalSeconds <= 0 && point.CollectIntervalMilliseconds <= 0 {
		if device.CollectIntervalMilliseconds > 0 {
			point.CollectIntervalMilliseconds = device.CollectIntervalMilliseconds
		} else if device.CollectIntervalSeconds > 0 {
			point.CollectIntervalSeconds = device.CollectIntervalSeconds
		}
	}
	if device.DLT645 != nil {
		if point.DLT645 == nil {
			point.DLT645 = &DLT645Config{}
		}
		point.DLT645.Version = device.DLT645.Version
		point.DLT645.MeterAddress = device.DLT645.MeterAddress
		point.DLT645.ResponseTimeoutMilliseconds = device.DLT645.ResponseTimeoutMilliseconds
		point.DLT645.Retries = device.DLT645.Retries
		point.DLT645.WakeupBytes = device.DLT645.WakeupBytes
		point.DLT645.WritePassword = device.DLT645.WritePassword
		point.DLT645.OperatorCode = device.DLT645.OperatorCode
	}
	point.Address = device.Address
	point.ProgID = device.ProgID
	point.CLSID = device.CLSID
	point.Domain = device.Domain
	point.Workstation = device.Workstation
	point.SecurityMode = device.SecurityMode
	point.Authentication = device.Authentication
	point.AuthenticationLevel = device.AuthenticationLevel
	point.ConnectTimeoutMilliseconds = device.ConnectTimeoutMilliseconds
	point.RequestTimeoutMilliseconds = device.RequestTimeoutMilliseconds
	point.ReadMode = device.ReadMode
	point.ReadSource = device.ReadSource
	point.BatchSize = device.BatchSize
	point.AutoReconnect = device.AutoReconnect
	point.GroupName = device.GroupName
	point.RequestedUpdateRateMS = device.RequestedUpdateRateMS
	point.Deadband = device.Deadband
	point.LocaleID = device.LocaleID
	point.LocalBindIP = device.LocalBindIP
	point.SlaveID = device.SlaveID
	if device.Protocol == "iec104" {
		point.CommonAddress = device.CommonAddress
		if point.CommonAddress == 0 {
			point.CommonAddress = uint16(device.SlaveID)
		}
	}
	point.Rack = device.Rack
	point.Slot = device.Slot
	point.LocalTSAP = device.LocalTSAP
	point.RemoteTSAP = device.RemoteTSAP
	point.Username = device.Username
	point.Password = device.Password
	point.GoCBRef = device.GoCBRef
	point.DataSetRef = device.DataSetRef
	point.AppID = device.AppID
	point.DestinationMAC = device.DestinationMAC
	point.VLANID = device.VLANID
	point.VLANPriority = device.VLANPriority
}

func applyChannelConnection(device *DeviceConfig, channel ChannelConfig) {
	device.ChannelKey = channel.ChannelKey
	device.InterfaceType = channel.Type
	device.InterfaceName = channel.InterfaceName
	if device.Protocol == "" {
		device.Protocol = channel.Protocol
	}
	if channel.Type == "serial" && channel.Serial != nil {
		device.Address = channel.Serial.Port
		return
	}
	if channel.Type == "network" && device.Address == "" && channel.Network != nil && channel.Network.Address != "" {
		device.Address = channel.Network.Address
	}
	if channel.Type == "network" && device.Protocol == "iec61850-goose" && channel.Network != nil && channel.Network.Interface != "" {
		device.Address = channel.Network.Interface
	}
}

func applyChannelPointDefaults(point *PointConfig, channel ChannelConfig) {
	point.ChannelKey = channel.ChannelKey
	if point.CollectIntervalSeconds <= 0 && point.CollectIntervalMilliseconds <= 0 {
		if channel.CollectIntervalMilliseconds > 0 {
			point.CollectIntervalMilliseconds = channel.CollectIntervalMilliseconds
		} else if channel.CollectIntervalSeconds > 0 {
			point.CollectIntervalSeconds = channel.CollectIntervalSeconds
		}
	}
	if channel.Protocol == "iec104" {
		options := channel.IEC104
		options.ApplyDefaults()
		point.IEC104 = &options
	}
}

func applySerialConnection(point *PointConfig, serialPort SerialPort) {
	point.Address = serialPort.Port
	point.BaudRate = serialPort.BaudRate
	point.DataBits = serialPort.DataBits
	point.StopBits = serialPort.StopBits
	point.Parity = serialPort.Parity
}

func (p *PointConfig) ApplyDefaults() {
	p.Protocol = NormalizeProtocol(p.Protocol)
	if p.Protocol == "opcda" {
		p.SecurityMode = NormalizeOPCDASecurityMode(p.SecurityMode, p.AuthenticationLevel)
		if p.Access == "readwrite" {
			// Accept browse/import data produced before the enum was standardized.
			p.Access = "read_write"
		} else if p.Access == "" || p.Access == "unknown" {
			p.Access = "read"
		}
		if p.RequestedDataType == "" {
			p.RequestedDataType = "auto"
		}
		if p.Authentication == "" {
			p.Authentication = "ntlm"
		}
		p.AuthenticationLevel = OPCDAAuthenticationLevel(p.SecurityMode)
		if p.ConnectTimeoutMilliseconds <= 0 {
			p.ConnectTimeoutMilliseconds = 10000
		}
		if p.RequestTimeoutMilliseconds <= 0 {
			p.RequestTimeoutMilliseconds = 10000
		}
		if p.ReadMode == "" {
			p.ReadMode = "sync"
		}
		if p.ReadSource == "" {
			p.ReadSource = "device"
		}
		if p.BatchSize <= 0 {
			p.BatchSize = 100
		}
		if p.AutoReconnect == nil {
			enabled := true
			p.AutoReconnect = &enabled
		}
		if p.GroupName == "" {
			p.GroupName = "WK_OPCDA"
		}
		if p.RequestedUpdateRateMS <= 0 {
			p.RequestedUpdateRateMS = 1000
		}
		if p.LocaleID == 0 {
			p.LocaleID = 0x0409
		}
	}
	if p.Protocol == "iec104" {
		if p.CommonAddress == 0 {
			p.CommonAddress = uint16(p.SlaveID)
		}
		if p.CommonAddress == 0 {
			p.CommonAddress = 1
		}
		if p.IOA == 0 && p.Register != 0 {
			p.IOA = uint32(p.Register)
		}
		if p.IEC104 != nil {
			p.IEC104.ApplyDefaults()
		}
	}
	if p.Protocol == "modbus-rtu" {
		if p.BaudRate == 0 {
			p.BaudRate = 9600
		}
		if p.DataBits == 0 {
			p.DataBits = 8
		}
		if p.StopBits == 0 {
			p.StopBits = 1
		}
		if p.Parity == "" {
			p.Parity = "none"
		}
	}
	if p.Protocol == "dlt645" {
		if p.DLT645 == nil {
			p.DLT645 = &DLT645Config{}
		}
		p.DLT645.ApplyPointDefaults()
		if p.BaudRate == 0 {
			p.BaudRate = 2400
		}
		if p.DataBits == 0 {
			p.DataBits = 8
		}
		if p.StopBits == 0 {
			p.StopBits = 1
		}
		if p.Parity == "" {
			p.Parity = "even"
		}
	}
	if p.Protocol == "siemens-s7" && p.Area == "" {
		p.Area = "DB"
	}
	if p.Protocol == "siemens-s7" && p.Area == "DB" && p.DBNumber == 0 {
		p.DBNumber = 1
	}
	if p.Protocol == "siemens-s7" && p.Area == "V" && p.DBNumber == 0 {
		p.DBNumber = 1
	}
	if p.Protocol == "iec61850" && p.FC == "" {
		p.FC = "ST"
	}
	if p.Protocol == "iec61850-goose" {
		if p.DataType == "" || p.DataType == "auto" {
			p.DataType = "bool"
		}
		if p.AppID == 0 {
			p.AppID = 0x1000
		}
		if p.DestinationMAC == "" {
			p.DestinationMAC = "01:0c:cd:01:00:01"
		}
	}
	if p.Protocol == "iec104" && p.PointType == "" {
		if p.DataType == "single" || p.DataType == "double" {
			p.PointType = "遥信"
		} else {
			p.PointType = "遥测"
		}
	}
	if p.Quantity == 0 {
		p.Quantity = defaultQuantity(p.DataType)
	}
	if p.ByteOrder == "" {
		p.ByteOrder = "big"
	}
	if p.WordOrder == "" {
		p.WordOrder = "normal"
	}
	if p.Scale == 0 {
		p.Scale = 1
	}
	if p.Decimals < 0 {
		p.Decimals = 0
	}
	if p.Decimals > 9 {
		p.Decimals = 9
	}
}

func defaultQuantity(dataType string) uint16 {
	switch dataType {
	case "uint32", "int32", "float32":
		return 2
	default:
		return 1
	}
}

func devicesFromFlatPoints(points []PointConfig) []DeviceConfig {
	indexes := map[string]int{}
	var devices []DeviceConfig
	for _, point := range points {
		stationAddress := fmt.Sprint(point.SlaveID)
		if point.Protocol == "iec104" {
			stationAddress = fmt.Sprint(point.CommonAddress)
		}
		if point.Protocol == "dlt645" && point.DLT645 != nil {
			stationAddress = point.DLT645.MeterAddress
		}
		key := point.DeviceKey + "::" + point.Protocol + "::" + point.Address + "::" + stationAddress
		index, ok := indexes[key]
		if !ok {
			devices = append(devices, DeviceConfig{
				DeviceKey:                   point.DeviceKey,
				CollectIntervalSeconds:      point.CollectIntervalSeconds,
				CollectIntervalMilliseconds: point.CollectIntervalMilliseconds,
				Protocol:                    point.Protocol,
				DLT645:                      point.DLT645,
				Address:                     point.Address,
				ProgID:                      point.ProgID,
				CLSID:                       point.CLSID,
				Domain:                      point.Domain,
				Workstation:                 point.Workstation,
				SecurityMode:                point.SecurityMode,
				Authentication:              point.Authentication,
				AuthenticationLevel:         point.AuthenticationLevel,
				ConnectTimeoutMilliseconds:  point.ConnectTimeoutMilliseconds,
				RequestTimeoutMilliseconds:  point.RequestTimeoutMilliseconds,
				ReadMode:                    point.ReadMode,
				ReadSource:                  point.ReadSource,
				BatchSize:                   point.BatchSize,
				AutoReconnect:               point.AutoReconnect,
				GroupName:                   point.GroupName,
				RequestedUpdateRateMS:       point.RequestedUpdateRateMS,
				Deadband:                    point.Deadband,
				LocaleID:                    point.LocaleID,
				LocalBindIP:                 point.LocalBindIP,
				Username:                    point.Username,
				Password:                    point.Password,
				SlaveID:                     point.SlaveID,
				CommonAddress:               point.CommonAddress,
			})
			index = len(devices) - 1
			indexes[key] = index
		}
		devices[index].Points = append(devices[index].Points, point)
	}
	return devices
}

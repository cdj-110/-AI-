package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"
)

// Drivers/dialects live outside config so new domestic adapters can reuse the
// same durable queue, task selection, lifecycle and UI contract.
type RemoteStorageConfig struct {
	StoragePath string                `json:"storagePath"`
	StorageUUID string                `json:"storageUUID,omitempty"`
	MaxSizeMB   int                   `json:"maxSizeMB"` // Per-target durable queue limit.
	Targets     []RemoteStorageTarget `json:"targets"`
}
type RemoteStorageTarget struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Enabled         bool              `json:"enabled"`
	Driver          string            `json:"driver"`
	Host            string            `json:"host"`
	Port            int               `json:"port"`
	Database        string            `json:"database"`
	Schema          string            `json:"schema,omitempty"`
	Table           string            `json:"table"`
	Username        string            `json:"username"`
	Password        string            `json:"password"`
	TLSMode         string            `json:"tlsMode"` // verify-full or disabled; never skip certificate checks.
	CAFile          string            `json:"caFile,omitempty"`
	CreateTable     bool              `json:"createTable"`
	IntervalSeconds int               `json:"intervalSeconds"`
	BatchSize       int               `json:"batchSize"`
	TimeoutSeconds  int               `json:"timeoutSeconds"`
	Points          []HistoryPointRef `json:"points"`
}

var remoteID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`)
var sqlIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,47}$`)

func (c *RemoteStorageConfig) ApplyDefaults() {
	if c.StoragePath == "" {
		c.StoragePath = DefaultHistoryStoragePath
	}
	if c.MaxSizeMB == 0 {
		c.MaxSizeMB = 16
	}
	for i := range c.Targets {
		c.Targets[i].ApplyDefaults()
	}
}
func (t *RemoteStorageTarget) ApplyDefaults() {
	if t.Port == 0 {
		t.Port = map[string]int{"mysql": 3306, "sqlserver": 1433, "postgresql": 5432}[t.Driver]
	}
	if t.Table == "" {
		t.Table = "gateway_samples"
	}
	if t.Schema == "" {
		t.Schema = map[string]string{"sqlserver": "dbo", "postgresql": "public"}[t.Driver]
	}
	if t.TLSMode == "" {
		t.TLSMode = "verify-full"
	}
	if t.IntervalSeconds == 0 {
		t.IntervalSeconds = 5
	}
	if t.BatchSize == 0 {
		t.BatchSize = 100
	}
	if t.TimeoutSeconds == 0 {
		t.TimeoutSeconds = 10
	}
}
func (t RemoteStorageTarget) ValidateConnection() error {
	if !remoteID.MatchString(t.ID) || len(t.Name) == 0 || len(t.Name) > 160 {
		return fmt.Errorf("远程存储任务编号或名称无效")
	}
	if t.Driver != "mysql" && t.Driver != "sqlserver" && t.Driver != "postgresql" {
		return fmt.Errorf("不支持的远程数据库类型")
	}
	if t.Host == "" || strings.ContainsAny(t.Host, "/\\?@;= \t\r\n") || t.Port < 1 || t.Port > 65535 {
		return fmt.Errorf("数据库主机或端口无效")
	}
	if strings.Contains(t.Host, ":") && net.ParseIP(t.Host) == nil {
		return fmt.Errorf("IPv6 地址不应包含方括号或端口")
	}
	if len(t.Database) == 0 || len(t.Database) > 128 || len(t.Username) == 0 || len(t.Username) > 128 || len(t.Password) > 1024 || strings.ContainsAny(t.Database+t.Username+t.Password, "\x00\r\n") {
		return fmt.Errorf("数据库名称或凭据格式无效")
	}
	if !sqlIdentifier.MatchString(t.Table) || (t.Schema != "" && !sqlIdentifier.MatchString(t.Schema)) {
		return fmt.Errorf("表名和模式名只允许英文字母、数字和下划线，不能以数字开头")
	}
	if t.Driver == "mysql" && t.Schema != "" {
		return fmt.Errorf("MySQL 不需要单独填写模式名")
	}
	if t.TLSMode != "verify-full" && t.TLSMode != "disabled" {
		return fmt.Errorf("TLS 模式必须为 verify-full 或 disabled")
	}
	if t.CAFile != "" && (!filepath.IsAbs(t.CAFile) || strings.ContainsAny(t.CAFile, "\x00\r\n")) {
		return fmt.Errorf("CA 文件必须为绝对路径")
	}
	if t.IntervalSeconds < 1 || t.IntervalSeconds > 86400 || t.BatchSize < 1 || t.BatchSize > 500 || t.TimeoutSeconds < 2 || t.TimeoutSeconds > 60 {
		return fmt.Errorf("存储周期、批量大小或超时时间超出范围")
	}
	return nil
}
func (t RemoteStorageTarget) Validate() error {
	if err := t.ValidateConnection(); err != nil {
		return err
	}
	if len(t.Points) == 0 || len(t.Points) > 2000 {
		return fmt.Errorf("每个任务需选择 1～2000 个点位")
	}
	seen := map[string]bool{}
	for _, p := range t.Points {
		key := p.DeviceKey + "\x00" + p.Metric
		if p.DeviceKey == "" || p.Metric == "" || strings.ContainsRune(p.DeviceKey+p.Metric, 0) || len(p.DeviceKey) > 256 || len(p.Metric) > 256 || seen[key] {
			return fmt.Errorf("点位选择为空、重复或标识过长")
		}
		seen[key] = true
	}
	return nil
}
func (c RemoteStorageConfig) Validate() error {
	if len(c.Targets) == 0 {
		return nil
	}
	if len(c.Targets) > 4 || c.MaxSizeMB < 4 || c.MaxSizeMB > 1024 {
		return fmt.Errorf("远程存储最多 4 个任务，每任务缓存 4～1024 MB")
	}
	if c.StoragePath != DefaultHistoryStoragePath && !filepath.IsAbs(c.StoragePath) {
		return fmt.Errorf("缓存路径必须为绝对路径")
	}
	if filepath.Clean(c.StoragePath) != filepath.Clean(DefaultHistoryStoragePath) && c.StorageUUID == "" {
		return fmt.Errorf("外部缓存存储必须绑定已挂载存储设备 UUID")
	}
	seen := map[string]bool{}
	for _, t := range c.Targets {
		if seen[t.ID] {
			return fmt.Errorf("远程存储任务编号重复")
		}
		seen[t.ID] = true
		if err := t.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Cache identity excludes credentials/timing, but never allow pending samples
// to silently cross destinations after an address/database/table change.
func (t RemoteStorageTarget) DestinationKey() string {
	raw, _ := json.Marshal([]any{t.Driver, strings.ToLower(t.Host), t.Port, t.Database, t.Schema, t.Table})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func PreserveRemoteStorageSecrets(previous RemoteStorageConfig, next *RemoteStorageConfig) {
	for i := range next.Targets {
		t := &next.Targets[i]
		if t.Password != "***" {
			continue
		}
		t.Password = ""
		for _, old := range previous.Targets {
			if t.ID == old.ID && t.DestinationKey() == old.DestinationKey() && t.Username == old.Username {
				t.Password = old.Password
				break
			}
		}
	}
}

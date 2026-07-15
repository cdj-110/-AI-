package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/hardware"
)

type Config struct {
	GatewayKey             string                `json:"gatewayKey"`
	CollectIntervalSeconds int                   `json:"collectIntervalSeconds"`
	CacheFile              string                `json:"cacheFile"`
	OfflineCache           OfflineCacheConfig    `json:"offlineCache,omitempty"`
	HistoryStorage         HistoryStorageConfig  `json:"historyStorage,omitempty"`
	Activation             ActivationConfig      `json:"activation,omitempty"`
	MQTT                   MQTTConfig            `json:"mqtt"`
	MQTTChannels           []MQTTConfig          `json:"mqttChannels,omitempty"`
	Resources              []ResourceConfig      `json:"resources,omitempty"`
	Channels               []ChannelConfig       `json:"channels,omitempty"`
	SerialPorts            []SerialPort          `json:"serialPorts,omitempty"`
	NetworkPorts           []NetworkPort         `json:"networkPorts,omitempty"`
	WiFi                   WiFiConfig            `json:"wifi,omitempty"`
	Cellular               CellularConfig        `json:"cellular,omitempty"`
	Devices                []DeviceConfig        `json:"devices,omitempty"`
	Points                 []PointConfig         `json:"points,omitempty"`
	ForwardDevices         []ForwardDeviceConfig `json:"forwardDevices,omitempty"`
	EdgeComputing          EdgeComputingConfig   `json:"edgeComputing,omitempty"`
	ForwardSlave           FeatureConfig         `json:"forwardSlave"`
	Web                    ListenerConfig        `json:"web"`
	Security               SecurityConfig        `json:"security,omitempty"`
}

type OfflineCacheConfig struct {
	Enabled     bool   `json:"enabled"`
	MaxSizeMB   int    `json:"maxSizeMB"`
	StoragePath string `json:"storagePath,omitempty"`
}

type HistoryStorageConfig struct {
	Enabled     bool   `json:"enabled"`
	MaxSizeMB   int    `json:"maxSizeMB"`
	StoragePath string `json:"storagePath,omitempty"`
}

type MQTTConfig struct {
	Name            string `json:"name,omitempty"`
	Enabled         *bool  `json:"enabled,omitempty"`
	Broker          string `json:"broker"`
	ClientID        string `json:"clientId"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	TopicTemplate   string `json:"topicTemplate,omitempty"`
	PayloadMode     string `json:"payloadMode,omitempty"`
	PayloadTemplate string `json:"payloadTemplate,omitempty"`
	SubscribeTopic  string `json:"subscribeTopic,omitempty"`
}

type ActivationConfig struct {
	Enabled      *bool  `json:"enabled,omitempty"`
	File         string `json:"file,omitempty"`
	HardwareID   string `json:"hardwareId,omitempty"`
	SN           string `json:"sn,omitempty"`
	DeviceSecret string `json:"deviceSecret,omitempty"`
	Broker       string `json:"broker,omitempty"`
}

func (a ActivationConfig) Equal(other ActivationConfig) bool {
	return a.IsEnabled() == other.IsEnabled() &&
		a.File == other.File &&
		a.HardwareID == other.HardwareID &&
		a.SN == other.SN &&
		a.DeviceSecret == other.DeviceSecret &&
		a.Broker == other.Broker
}

type SerialPort struct {
	Name     string `json:"name"`
	Port     string `json:"port"`
	BaudRate int    `json:"baudRate"`
	DataBits int    `json:"dataBits"`
	StopBits int    `json:"stopBits"`
	Parity   string `json:"parity"`
	Enabled  bool   `json:"enabled"`
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
	ChannelKey             string         `json:"channelKey"`
	ResourceKey            string         `json:"resourceKey,omitempty"`
	Name                   string         `json:"name"`
	Type                   string         `json:"type"`
	Role                   string         `json:"role,omitempty"`
	Protocol               string         `json:"protocol"`
	ForwardProtocol        string         `json:"forwardProtocol,omitempty"`
	CollectIntervalSeconds int            `json:"collectIntervalSeconds,omitempty"`
	IEC104                 IEC104Config   `json:"iec104,omitempty"`
	InterfaceName          string         `json:"interfaceName,omitempty"`
	Enabled                bool           `json:"enabled"`
	Serial                 *SerialPort    `json:"serial,omitempty"`
	Network                *NetworkPort   `json:"network,omitempty"`
	Devices                []DeviceConfig `json:"devices,omitempty"`
}

type IEC104Config struct {
	Configured                          bool `json:"configured,omitempty"`
	GeneralInterrogationOnStart         bool `json:"generalInterrogationOnStart"`
	GeneralInterrogationIntervalSeconds int  `json:"generalInterrogationIntervalSeconds"`
	ClockSyncOnStart                    bool `json:"clockSyncOnStart"`
	ClockSyncIntervalSeconds            int  `json:"clockSyncIntervalSeconds"`
	CounterInterrogationOnStart         bool `json:"counterInterrogationOnStart"`
	CounterInterrogationIntervalSeconds int  `json:"counterInterrogationIntervalSeconds"`
}

type PointConfig struct {
	DeviceKey              string        `json:"deviceKey"`
	ChannelKey             string        `json:"channelKey,omitempty"`
	CollectIntervalSeconds int           `json:"collectIntervalSeconds,omitempty"`
	Name                   string        `json:"name"`
	Metric                 string        `json:"metric"`
	Protocol               string        `json:"protocol"`
	Address                string        `json:"address"`
	PointType              string        `json:"pointType,omitempty"`
	SlaveID                byte          `json:"slaveId"`
	CommonAddress          uint16        `json:"commonAddress,omitempty"`
	IOA                    uint32        `json:"ioa,omitempty"`
	IEC104                 *IEC104Config `json:"iec104,omitempty"`
	Area                   string        `json:"area,omitempty"`
	DBNumber               uint16        `json:"dbNumber,omitempty"`
	Rack                   uint8         `json:"rack,omitempty"`
	Slot                   uint8         `json:"slot,omitempty"`
	LocalTSAP              string        `json:"localTsap,omitempty"`
	RemoteTSAP             string        `json:"remoteTsap,omitempty"`
	ObjectRef              string        `json:"objectRef,omitempty"`
	FC                     string        `json:"fc,omitempty"`
	NodeID                 string        `json:"nodeId,omitempty"`
	Username               string        `json:"username,omitempty"`
	Password               string        `json:"password,omitempty"`
	BaudRate               int           `json:"baudRate,omitempty"`
	DataBits               int           `json:"dataBits,omitempty"`
	StopBits               int           `json:"stopBits,omitempty"`
	Parity                 string        `json:"parity,omitempty"`
	Function               uint8         `json:"function"`
	Register               uint16        `json:"register"`
	Quantity               uint16        `json:"quantity"`
	DataType               string        `json:"dataType"`
	ByteOrder              string        `json:"byteOrder"`
	WordOrder              string        `json:"wordOrder"`
	BitIndex               *uint8        `json:"bitIndex"`
	Scale                  float64       `json:"scale"`
	Offset                 float64       `json:"offset"`
	Unit                   string        `json:"unit,omitempty"`
	Decimals               int           `json:"decimals,omitempty"`
}

type DeviceConfig struct {
	DeviceKey                string        `json:"deviceKey"`
	ChannelKey               string        `json:"channelKey,omitempty"`
	Name                     string        `json:"name"`
	PLCModel                 string        `json:"plcModel,omitempty"`
	InterfaceType            string        `json:"interfaceType,omitempty"`
	InterfaceName            string        `json:"interfaceName,omitempty"`
	Protocol                 string        `json:"protocol"`
	Address                  string        `json:"address"`
	SlaveID                  byte          `json:"slaveId"`
	CommonAddress            uint16        `json:"commonAddress,omitempty"`
	ReconnectIntervalSeconds int           `json:"reconnectIntervalSeconds"`
	Rack                     uint8         `json:"rack,omitempty"`
	Slot                     uint8         `json:"slot,omitempty"`
	LocalTSAP                string        `json:"localTsap,omitempty"`
	RemoteTSAP               string        `json:"remoteTsap,omitempty"`
	IEDName                  string        `json:"iedName,omitempty"`
	Username                 string        `json:"username,omitempty"`
	Password                 string        `json:"password,omitempty"`
	Points                   []PointConfig `json:"points"`
}

// ForwardDeviceConfig describes a logical downstream device exposed by a
// forwarding server. Its points reference collected points instead of being
// collected independently.
type ForwardDeviceConfig struct {
	DeviceKey     string               `json:"deviceKey"`
	ChannelKey    string               `json:"channelKey,omitempty"`
	Name          string               `json:"name"`
	Protocol      string               `json:"protocol"`
	Enabled       *bool                `json:"enabled,omitempty"`
	UnitID        byte                 `json:"unitId,omitempty"`
	CommonAddress uint16               `json:"commonAddress,omitempty"`
	Points        []ForwardPointConfig `json:"points,omitempty"`
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
	Decimals        int     `json:"decimals,omitempty"`
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
	}
}

type FeatureConfig struct {
	Enabled      bool   `json:"enabled"`
	Listen       string `json:"listen"`
	ModbusListen string `json:"modbusListen,omitempty"`
	IEC104Listen string `json:"iec104Listen,omitempty"`
}

type ListenerConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
}

type SecurityConfig struct {
	Users []UserConfig `json:"users,omitempty"`
	Roles []RoleConfig `json:"roles,omitempty"`
}

type UserConfig struct {
	Username     string `json:"username"`
	DisplayName  string `json:"displayName,omitempty"`
	PasswordHash string `json:"passwordHash,omitempty"`
	RoleKey      string `json:"roleKey"`
	Enabled      bool   `json:"enabled"`
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
	return Parse(raw)
}

func Parse(raw []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.ApplyActivation(); err != nil {
		return Config{}, err
	}
	if cfg.Activation.IsReady() {
		cfg.GatewayKey = cfg.Activation.SN
	} else if cfg.GatewayKey == "" && cfg.Activation.SN != "" {
		cfg.GatewayKey = cfg.Activation.SN
	}
	if cfg.GatewayKey == "" {
		return Config{}, fmt.Errorf("gatewayKey is required")
	}
	cfg.ApplyDefaults()
	if err := validateForwardListenAddress("Modbus TCP", cfg.ForwardSlave.ModbusListen); err != nil {
		return Config{}, err
	}
	if err := validateForwardListenAddress("IEC104", cfg.ForwardSlave.IEC104Listen); err != nil {
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
	for _, device := range c.ForwardDevices {
		for _, point := range device.Points {
			add(point.Metric)
		}
	}
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

func (c *Config) ApplyActivation() error {
	if c.Activation.Enabled != nil && !*c.Activation.Enabled {
		return nil
	}
	if c.Activation.File != "" {
		raw, err := os.ReadFile(c.Activation.File)
		if err != nil {
			return fmt.Errorf("load activation file failed: %w", err)
		}
		var fileActivation ActivationConfig
		if err := json.Unmarshal(raw, &fileActivation); err != nil {
			return fmt.Errorf("parse activation file failed: %w", err)
		}
		c.Activation.Merge(fileActivation)
	}
	if c.Activation.SN == "" || c.Activation.DeviceSecret == "" {
		return nil
	}
	hardwareID := c.Activation.HardwareID
	if hardwareID == "" {
		identity := hardware.ReadIdentity()
		hardwareID = identity.ID
	}
	if hardwareID == "" {
		return fmt.Errorf("activation requires hardwareId or readable hardware identity")
	}
	c.Activation.HardwareID = hardwareID
	return nil
}

func (a *ActivationConfig) Merge(other ActivationConfig) {
	if a.Enabled == nil {
		a.Enabled = other.Enabled
	}
	if a.HardwareID == "" {
		a.HardwareID = other.HardwareID
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
}

func Save(path string, cfg Config) error {
	cfg.ApplyDefaults()
	if cfg.Activation.File != "" {
		cfg.Activation = ActivationConfig{Enabled: cfg.Activation.Enabled, File: cfg.Activation.File}
	}
	if len(cfg.Devices) > 0 {
		cfg.Points = nil
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	temp, err := os.CreateTemp(dir, ".gateway-config-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(raw, '\n')); err != nil {
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

func (c Config) CollectInterval() time.Duration {
	seconds := c.CollectIntervalSeconds
	for _, channel := range c.Channels {
		if channel.CollectIntervalSeconds > 0 && (seconds <= 0 || channel.CollectIntervalSeconds < seconds) {
			seconds = channel.CollectIntervalSeconds
		}
	}
	if seconds <= 0 {
		seconds = 5
	}
	return time.Duration(seconds) * time.Second
}

func (c *Config) ApplyDefaults() {
	hadDevices := len(c.Devices) > 0
	if c.CollectIntervalSeconds <= 0 {
		c.CollectIntervalSeconds = 5
	}
	if c.CacheFile == "" {
		c.CacheFile = ".runtime/gateway-spool.jsonl"
	}
	if c.OfflineCache.MaxSizeMB == 0 {
		c.OfflineCache.MaxSizeMB = 16
	}
	if c.HistoryStorage.MaxSizeMB == 0 {
		c.HistoryStorage.MaxSizeMB = 256
	}
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
	c.ApplyResourceDefaults()
	c.ApplyChannelDefaults()
	c.ApplyForwardDefaults()
	c.EdgeComputing.ApplyDefaults()
	for i := range c.ForwardDevices {
		c.ForwardDevices[i].ApplyDefaults(i)
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
				if c.Devices[i].Protocol == "modbus-rtu" {
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
			if hadDevices && c.Devices[i].Protocol == "modbus-rtu" {
				if serialPort, ok := c.serialPort(c.Devices[i].InterfaceName); ok && serialPort.Enabled {
					c.Devices[i].Address = serialPort.Port
					for pointIndex := range c.Devices[i].Points {
						applyDeviceConnection(&c.Devices[i].Points[pointIndex], c.Devices[i])
						applySerialConnection(&c.Devices[i].Points[pointIndex], serialPort)
					}
				}
			}
		}
		c.Points = c.FlattenPoints()
	}
}

func (c *Config) ApplyResourceDefaults() {
	if len(c.Resources) == 0 {
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
		if channel.CollectIntervalSeconds <= 0 {
			channel.CollectIntervalSeconds = c.CollectIntervalSeconds
		}
		if channel.CollectIntervalSeconds <= 0 {
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
			if channel.Protocol == "modbus-rtu" {
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
	if !i.Configured && !i.GeneralInterrogationOnStart && i.GeneralInterrogationIntervalSeconds == 0 && !i.ClockSyncOnStart && i.ClockSyncIntervalSeconds == 0 && !i.CounterInterrogationOnStart && i.CounterInterrogationIntervalSeconds == 0 {
		i.GeneralInterrogationOnStart = true
		i.ClockSyncOnStart = true
		i.ClockSyncIntervalSeconds = 3600
		i.CounterInterrogationOnStart = true
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
			ChannelKey:             resource.ResourceKey + "-" + protocol,
			ResourceKey:            resource.ResourceKey,
			Name:                   resource.Name + " " + protocol,
			Type:                   resource.Type,
			Protocol:               protocol,
			CollectIntervalSeconds: c.CollectIntervalSeconds,
			InterfaceName:          resource.Name,
			Enabled:                resource.Enabled,
			Serial:                 resource.Serial,
			Network:                resource.Network,
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
		m.SubscribeTopic == other.SubscribeTopic
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

func (c Config) FlattenPoints() []PointConfig {
	if len(c.Devices) == 0 {
		points := append([]PointConfig(nil), c.Points...)
		for i := range points {
			points[i].ApplyDefaults()
		}
		return points
	}
	var points []PointConfig
	for _, device := range c.Devices {
		channel, hasChannel := c.channel(device.ChannelKey)
		for _, point := range device.Points {
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
	if d.Protocol == "iec104" && d.CommonAddress == 0 {
		d.CommonAddress = uint16(d.SlaveID)
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
	point.Address = device.Address
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
}

func applyChannelPointDefaults(point *PointConfig, channel ChannelConfig) {
	point.ChannelKey = channel.ChannelKey
	if channel.CollectIntervalSeconds > 0 {
		point.CollectIntervalSeconds = channel.CollectIntervalSeconds
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
		key := point.DeviceKey + "::" + point.Protocol + "::" + point.Address + "::" + stationAddress
		index, ok := indexes[key]
		if !ok {
			devices = append(devices, DeviceConfig{
				DeviceKey:     point.DeviceKey,
				Protocol:      point.Protocol,
				Address:       point.Address,
				SlaveID:       point.SlaveID,
				CommonAddress: point.CommonAddress,
			})
			index = len(devices) - 1
			indexes[key] = index
		}
		devices[index].Points = append(devices[index].Points, point)
	}
	return devices
}

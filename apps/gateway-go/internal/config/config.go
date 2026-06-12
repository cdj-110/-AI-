package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	GatewayKey             string         `json:"gatewayKey"`
	CollectIntervalSeconds int            `json:"collectIntervalSeconds"`
	CacheFile              string         `json:"cacheFile"`
	MQTT                   MQTTConfig     `json:"mqtt"`
	SerialPorts            []SerialPort   `json:"serialPorts,omitempty"`
	NetworkPorts           []NetworkPort  `json:"networkPorts,omitempty"`
	Devices                []DeviceConfig `json:"devices,omitempty"`
	Points                 []PointConfig  `json:"points,omitempty"`
	ForwardSlave           FeatureConfig  `json:"forwardSlave"`
	Web                    ListenerConfig `json:"web"`
}

type MQTTConfig struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
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
	Name    string `json:"name"`
	Address string `json:"address"`
	Mode    string `json:"mode"`
	Enabled bool   `json:"enabled"`
}

type PointConfig struct {
	DeviceKey  string  `json:"deviceKey"`
	Name       string  `json:"name"`
	Metric     string  `json:"metric"`
	Protocol   string  `json:"protocol"`
	Address    string  `json:"address"`
	SlaveID    byte    `json:"slaveId"`
	Area       string  `json:"area,omitempty"`
	DBNumber   uint16  `json:"dbNumber,omitempty"`
	Rack       uint8   `json:"rack,omitempty"`
	Slot       uint8   `json:"slot,omitempty"`
	LocalTSAP  string  `json:"localTsap,omitempty"`
	RemoteTSAP string  `json:"remoteTsap,omitempty"`
	Function   uint8   `json:"function"`
	Register   uint16  `json:"register"`
	Quantity   uint16  `json:"quantity"`
	DataType   string  `json:"dataType"`
	ByteOrder  string  `json:"byteOrder"`
	WordOrder  string  `json:"wordOrder"`
	BitIndex   *uint8  `json:"bitIndex"`
	Scale      float64 `json:"scale"`
	Offset     float64 `json:"offset"`
}

type DeviceConfig struct {
	DeviceKey     string        `json:"deviceKey"`
	Name          string        `json:"name"`
	InterfaceType string        `json:"interfaceType,omitempty"`
	InterfaceName string        `json:"interfaceName,omitempty"`
	Protocol      string        `json:"protocol"`
	Address       string        `json:"address"`
	SlaveID       byte          `json:"slaveId"`
	Rack          uint8         `json:"rack,omitempty"`
	Slot          uint8         `json:"slot,omitempty"`
	LocalTSAP     string        `json:"localTsap,omitempty"`
	RemoteTSAP    string        `json:"remoteTsap,omitempty"`
	Points        []PointConfig `json:"points"`
}

type FeatureConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
}

type ListenerConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
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
	if cfg.GatewayKey == "" {
		return Config{}, fmt.Errorf("gatewayKey is required")
	}
	cfg.ApplyDefaults()
	if cfg.MQTT.IsEnabled() && (cfg.MQTT.Broker == "" || cfg.MQTT.ClientID == "" || cfg.MQTT.Username == "") {
		return Config{}, fmt.Errorf("mqtt broker/clientId/username are required")
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg.ApplyDefaults()
	if len(cfg.Devices) > 0 {
		cfg.Points = nil
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(raw, '\n'), 0644)
}

func (c Config) CollectInterval() time.Duration {
	return time.Duration(c.CollectIntervalSeconds) * time.Second
}

func (c *Config) ApplyDefaults() {
	if c.CollectIntervalSeconds <= 0 {
		c.CollectIntervalSeconds = 5
	}
	if c.CacheFile == "" {
		c.CacheFile = ".runtime/gateway-spool.jsonl"
	}
	c.MQTT.ApplyDefaults()
	if len(c.SerialPorts) == 0 {
		c.SerialPorts = []SerialPort{
			{Name: "Serial1", Port: "COM1", Enabled: true},
			{Name: "Serial2", Port: "COM2", Enabled: true},
		}
	}
	if len(c.NetworkPorts) == 0 {
		c.NetworkPorts = []NetworkPort{
			{Name: "net1", Address: "192.168.1.50:502", Mode: "tcp-client", Enabled: true},
			{Name: "net2", Address: "192.168.1.51:502", Mode: "tcp-client", Enabled: true},
		}
	}
	for i := range c.SerialPorts {
		c.SerialPorts[i].ApplyDefaults()
	}
	for i := range c.NetworkPorts {
		c.NetworkPorts[i].ApplyDefaults()
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
		}
		c.Points = c.FlattenPoints()
	}
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
		p.Name = p.Address
	}
	if p.Mode == "" {
		p.Mode = "tcp-client"
	}
}

func (m *MQTTConfig) ApplyDefaults() {
	if m.Enabled == nil {
		enabled := true
		m.Enabled = &enabled
	}
}

func (m MQTTConfig) IsEnabled() bool {
	return m.Enabled != nil && *m.Enabled
}

func (m MQTTConfig) Equal(other MQTTConfig) bool {
	return m.IsEnabled() == other.IsEnabled() &&
		m.Broker == other.Broker &&
		m.ClientID == other.ClientID &&
		m.Username == other.Username &&
		m.Password == other.Password
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
		for _, point := range device.Points {
			point.DeviceKey = device.DeviceKey
			if point.Protocol == "" {
				point.Protocol = device.Protocol
			}
			if point.Address == "" {
				point.Address = device.Address
			}
			if point.SlaveID == 0 {
				point.SlaveID = device.SlaveID
			}
			if point.Rack == 0 {
				point.Rack = device.Rack
			}
			if point.Slot == 0 {
				point.Slot = device.Slot
			}
			if point.LocalTSAP == "" {
				point.LocalTSAP = device.LocalTSAP
			}
			if point.RemoteTSAP == "" {
				point.RemoteTSAP = device.RemoteTSAP
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
	if d.SlaveID == 0 {
		d.SlaveID = 1
	}
	for i := range d.Points {
		d.Points[i].DeviceKey = d.DeviceKey
		if d.Points[i].Protocol == "" {
			d.Points[i].Protocol = d.Protocol
		}
		if d.Points[i].Address == "" {
			d.Points[i].Address = d.Address
		}
		if d.Points[i].SlaveID == 0 {
			d.Points[i].SlaveID = d.SlaveID
		}
		if d.Points[i].Rack == 0 {
			d.Points[i].Rack = d.Rack
		}
		if d.Points[i].Slot == 0 {
			d.Points[i].Slot = d.Slot
		}
		if d.Points[i].LocalTSAP == "" {
			d.Points[i].LocalTSAP = d.LocalTSAP
		}
		if d.Points[i].RemoteTSAP == "" {
			d.Points[i].RemoteTSAP = d.RemoteTSAP
		}
		d.Points[i].ApplyDefaults()
	}
}

func (p *PointConfig) ApplyDefaults() {
	if p.Protocol == "siemens-s7" && p.Area == "" {
		p.Area = "DB"
	}
	if p.Protocol == "siemens-s7" && p.Area == "DB" && p.DBNumber == 0 {
		p.DBNumber = 1
	}
	if p.Protocol == "siemens-s7" && p.Area == "V" && p.DBNumber == 0 {
		p.DBNumber = 1
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
		key := point.DeviceKey + "::" + point.Protocol + "::" + point.Address + "::" + fmt.Sprint(point.SlaveID)
		index, ok := indexes[key]
		if !ok {
			devices = append(devices, DeviceConfig{
				DeviceKey: point.DeviceKey,
				Protocol:  point.Protocol,
				Address:   point.Address,
				SlaveID:   point.SlaveID,
			})
			index = len(devices) - 1
			indexes[key] = index
		}
		devices[index].Points = append(devices[index].Points, point)
	}
	return devices
}

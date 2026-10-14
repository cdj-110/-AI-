package cloud

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

var attributeTemplateTokenPattern = regexp.MustCompile(`\{attribute(?:\.[^{}\s",:\[\]]+){0,2}\}`)
var exactAttributePointTemplatePattern = regexp.MustCompile(`^\{attribute\.[^{}\s",:\[\]]+\.([^{}\s",:\[\]]+)\}$`)
var ErrNoManualPayload = errors.New("manual payload has no matching values")

type Client struct {
	name                  string
	manual                bool
	gatewayKey            string
	hardwareID            string
	clientID              string
	username              string
	topicTemplate         string
	payloadMode           string
	payloadTemplate       string
	subscribeTopic        string
	publishInterval       time.Duration
	mqtt                  mqtt.Client
	setupErr              error
	attributeMu           sync.RWMutex
	attributeHandler      func(topic string, payload []byte)
	remoteConfigMu        sync.RWMutex
	remoteConfigHandler   func(RemoteConfigCommand) RemoteConfigResult
	remoteConfigSnapshot  func() interface{}
	licenseConfig         *config.Config
	pointCommandMu        sync.RWMutex
	pointCommandHandler   func(PointCommand) PointCommandResult
	otaMu                 sync.RWMutex
	otaHandler            func(OTACommand) OTAResult
	otaResultPath         string
	maintenanceMu         sync.RWMutex
	maintenanceHandler    func(MaintenanceCommand) MaintenanceResult
	maintenanceResultPath string
}

type RemoteConfigCommand struct {
	TaskID  string          `json:"taskId"`
	Version int             `json:"version"`
	Config  json.RawMessage `json:"config"`
}

type RemoteConfigResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type MQTTOptions struct {
	Name            string
	Broker          string
	ClientID        string
	Username        string
	Password        string
	GatewayKey      string
	HardwareID      string
	TopicTemplate   string
	PayloadMode     string
	PayloadTemplate string
	SubscribeTopic  string
	Manual          bool
	CAFile          string
	CertFile        string
	KeyFile         string
	PublishInterval time.Duration
}

func NewManualMQTT(cfg config.Config, onConnectionChanged func(bool)) *Client {
	return NewManualMQTTChannel("manual", cfg.GatewayKey, cfg.MQTT, onConnectionChanged)
}

func NewManualMQTTChannel(name string, gatewayKey string, channel config.MQTTConfig, onConnectionChanged func(bool)) *Client {
	return NewMQTT(MQTTOptions{
		Name:            name,
		Broker:          channel.Broker,
		ClientID:        channel.ClientID,
		Username:        channel.Username,
		Password:        channel.Password,
		GatewayKey:      gatewayKey,
		TopicTemplate:   channel.TopicTemplate,
		PayloadMode:     channel.PayloadMode,
		PayloadTemplate: channel.PayloadTemplate,
		SubscribeTopic:  channel.SubscribeTopic,
		Manual:          true,
		CAFile:          channel.CAFile,
		CertFile:        channel.CertFile,
		KeyFile:         channel.KeyFile,
		PublishInterval: time.Duration(channel.PublishIntervalMilliseconds) * time.Millisecond,
	}, onConnectionChanged)
}

func NewActivationMQTT(cfg config.Config, onConnectionChanged func(bool)) *Client {
	client := NewMQTT(MQTTOptions{
		Name:       "activation",
		Broker:     cfg.Activation.Broker,
		ClientID:   "factory_" + cfg.Activation.HardwareID,
		Username:   "factory:" + cfg.Activation.HardwareID,
		Password:   cfg.Activation.DeviceSecret,
		GatewayKey: cfg.Activation.SN,
		HardwareID: cfg.Activation.HardwareID,
	}, onConnectionChanged)
	client.licenseConfig = &cfg
	return client
}

func NewMQTT(opts MQTTOptions, onConnectionChanged func(bool)) *Client {
	clientOptions := mqtt.NewClientOptions().
		AddBroker(opts.Broker).
		SetClientID(opts.ClientID).
		SetUsername(opts.Username).
		SetPassword(opts.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetResumeSubs(true).
		SetConnectTimeout(5 * time.Second).
		SetKeepAlive(30 * time.Second)
	tlsConfig, tlsErr := mqttTLSConfig(opts)
	if tlsErr == nil && tlsConfig != nil {
		clientOptions.SetTLSConfig(tlsConfig)
	}
	client := &Client{
		name:            opts.Name,
		manual:          opts.Manual,
		gatewayKey:      opts.GatewayKey,
		hardwareID:      opts.HardwareID,
		clientID:        opts.ClientID,
		username:        opts.Username,
		topicTemplate:   defaultString(opts.TopicTemplate, "attributes"),
		payloadMode:     defaultString(opts.PayloadMode, "flat"),
		payloadTemplate: opts.PayloadTemplate,
		subscribeTopic:  opts.SubscribeTopic,
		publishInterval: defaultDuration(opts.PublishInterval, time.Second),
		setupErr:        tlsErr,
	}
	clientOptions.SetOnConnectHandler(func(mqtt.Client) {
		log.Printf("mqtt connected channel=%s", opts.Name)
		if onConnectionChanged != nil {
			onConnectionChanged(true)
		}
		if err := client.subscribeAttributesNow(); err != nil {
			log.Printf("resubscribe mqtt attributes failed channel=%s: %v", opts.Name, err)
		}
		if err := client.subscribeRemoteConfigNow(false); err != nil {
			log.Printf("resubscribe remote config failed channel=%s: %v", opts.Name, err)
		}
		if err := client.subscribePointCommandsNow(); err != nil {
			log.Printf("resubscribe point commands failed channel=%s: %v", opts.Name, err)
		}
		if err := client.subscribeOTANow(); err != nil {
			log.Printf("resubscribe OTA failed channel=%s: %v", opts.Name, err)
		}
		if err := client.subscribeMaintenanceNow(); err != nil {
			log.Printf("resubscribe maintenance failed channel=%s: %v", opts.Name, err)
		}
		go client.PublishPendingOTAResult()
		go client.PublishPendingMaintenanceResult()
		// Do not block Paho's connection callback while waiting for a publish.
		if !client.manual && client.hardwareID != "" {
			go func() {
				if err := client.PublishGatewayHeartbeat(); err != nil {
					log.Printf("publish authorization heartbeat failed: %v", err)
				}
			}()
		}
	})
	if onConnectionChanged != nil {
		clientOptions.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			log.Printf("mqtt connection lost channel=%s: %v", opts.Name, err)
			onConnectionChanged(false)
		})
	}
	client.mqtt = mqtt.NewClient(clientOptions)
	return client
}

func (c *Client) PublishInterval() time.Duration {
	return c.publishInterval
}

func mqttTLSConfig(opts MQTTOptions) (*tls.Config, error) {
	if opts.CAFile == "" && opts.CertFile == "" && opts.KeyFile == "" {
		return nil, nil
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if opts.CAFile != "" {
		raw, err := os.ReadFile(opts.CAFile)
		if err != nil {
			return nil, err
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("mqtt CA file contains no certificates")
		}
		config.RootCAs = roots
	}
	if opts.CertFile != "" || opts.KeyFile != "" {
		if opts.CertFile == "" || opts.KeyFile == "" {
			return nil, fmt.Errorf("mqtt client certificate and key must be configured together")
		}
		certificate, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}

func (c *Client) SubscribeAttributes(handler func(topic string, payload []byte)) error {
	if strings.TrimSpace(c.subscribeTopic) == "" {
		return nil
	}
	c.attributeMu.Lock()
	c.attributeHandler = handler
	c.attributeMu.Unlock()
	return c.subscribeAttributesNow()
}

func (c *Client) subscribeAttributesNow() error {
	c.attributeMu.RLock()
	handler := c.attributeHandler
	c.attributeMu.RUnlock()
	if handler == nil || strings.TrimSpace(c.subscribeTopic) == "" {
		return nil
	}
	if !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt is not connected")
	}
	topic := c.RenderSubscribeTopic()
	token := c.mqtt.Subscribe(topic, 1, func(_ mqtt.Client, message mqtt.Message) {
		handler(message.Topic(), append([]byte(nil), message.Payload()...))
	})
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe attributes timeout")
	}
	if err := token.Error(); err != nil {
		return err
	}
	log.Printf("mqtt attributes subscribed channel=%s topic=%s", c.name, topic)
	return nil
}

func (c *Client) RenderSubscribeTopic() string {
	topic := strings.TrimSpace(c.subscribeTopic)
	topic = strings.ReplaceAll(topic, "{gatewayKey}", c.gatewayKey)
	topic = strings.ReplaceAll(topic, "{clientId}", c.clientID)
	return topic
}

func (c *Client) SubscribeRemoteConfig(handler func(RemoteConfigCommand) RemoteConfigResult, snapshot func() interface{}) error {
	c.remoteConfigMu.Lock()
	c.remoteConfigHandler = handler
	c.remoteConfigSnapshot = snapshot
	c.remoteConfigMu.Unlock()
	return c.subscribeRemoteConfigNow(true)
}

func (c *Client) subscribeRemoteConfigNow(reportCurrent bool) error {
	c.remoteConfigMu.RLock()
	handler := c.remoteConfigHandler
	snapshot := c.remoteConfigSnapshot
	c.remoteConfigMu.RUnlock()
	if handler == nil || snapshot == nil {
		return nil
	}
	if !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt is not connected")
	}
	setTopic := fmt.Sprintf("weikong/gateways/%s/config/set", c.gatewayKey)
	getTopic := fmt.Sprintf("weikong/gateways/%s/config/get", c.gatewayKey)
	report := func() {
		payload := map[string]interface{}{"config": snapshot(), "reportedAt": time.Now().Format(time.RFC3339Nano)}
		if err := c.publish(fmt.Sprintf("weikong/gateways/%s/config/reported", c.gatewayKey), payload); err != nil {
			fmt.Printf("publish current config failed: %v\n", err)
		}
	}
	token := c.mqtt.SubscribeMultiple(map[string]byte{setTopic: 1, getTopic: 1}, func(_ mqtt.Client, message mqtt.Message) {
		if message.Topic() == getTopic {
			report()
			return
		}
		var command RemoteConfigCommand
		result := RemoteConfigResult{Status: "FAILED"}
		if err := json.Unmarshal(message.Payload(), &command); err != nil {
			result.Message = "parse remote config command failed: " + err.Error()
		} else if command.TaskID == "" || len(command.Config) == 0 {
			result.Message = "remote config taskId/config is required"
		} else {
			result = handler(command)
		}
		reply := map[string]interface{}{
			"taskId":      command.TaskID,
			"version":     command.Version,
			"status":      result.Status,
			"message":     result.Message,
			"completedAt": time.Now().Format(time.RFC3339Nano),
		}
		if err := c.publish(fmt.Sprintf("weikong/gateways/%s/config/reply", c.gatewayKey), reply); err != nil {
			fmt.Printf("publish remote config reply failed: %v\n", err)
		}
		if result.Status == "APPLIED" {
			report()
		}
	})
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe remote config timeout")
	}
	if err := token.Error(); err != nil {
		return err
	}
	log.Printf("remote config subscribed channel=%s topics=%s,%s", c.name, setTopic, getTopic)
	if reportCurrent {
		report()
	}
	return nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) IsManual() bool {
	return c.manual
}

func (c *Client) Connect() error {
	if c.setupErr != nil {
		return fmt.Errorf("mqtt TLS configuration: %w", c.setupErr)
	}
	token := c.mqtt.Connect()
	if token.WaitTimeout(6*time.Second) && token.Error() != nil {
		return token.Error()
	}
	if token.Error() == nil && !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt connect timeout")
	}
	return nil
}

func (c *Client) Disconnect() {
	c.mqtt.Disconnect(250)
}

func (c *Client) IsConnected() bool {
	return c.mqtt.IsConnected()
}

func (c *Client) PublishGatewayHeartbeat() error {
	payload := map[string]interface{}{"timestamp": time.Now().Format(time.RFC3339Nano)}
	if c.hardwareID != "" {
		if c.licenseConfig != nil {
			payload["authorization"] = licenseReport(*c.licenseConfig, time.Now())
		}
		if err := c.publish(fmt.Sprintf("weikong/factory/%s/heartbeat", c.hardwareID), payload); err != nil {
			return err
		}
		if err := c.publish(fmt.Sprintf("weikong/devices/%s/heartbeat", c.gatewayKey), payload); err != nil {
			// 出厂未绑定阶段，平台只允许 factory 心跳；绑定完成后正式设备主题会恢复可用。
			return nil
		}
		return nil
	}
	return c.publish(fmt.Sprintf("weikong/devices/%s/heartbeat", c.gatewayKey), payload)
}

func (c *Client) PublishChildHeartbeat(childKey string) error {
	payload := map[string]interface{}{"timestamp": time.Now().Format(time.RFC3339Nano)}
	return c.publish(fmt.Sprintf("weikong/gateways/%s/children/%s/heartbeat", c.gatewayKey, childKey), payload)
}

func (c *Client) PublishTelemetry(reading model.Reading) error {
	metrics := sanitizeMetrics(reading.Metrics)
	if len(metrics) == 0 {
		return nil
	}
	payload := map[string]interface{}{
		"ts": reading.Time.Format(time.RFC3339Nano),
		"d": map[string]interface{}{
			"gateway": map[string]interface{}{
				"Val": metrics,
			},
		},
	}
	return c.publish(fmt.Sprintf("weikong/gateways/%s/children/%s/telemetry", c.gatewayKey, reading.DeviceKey), payload)
}

func (c *Client) PublishTopology(topology interface{}) error {
	return c.publish(fmt.Sprintf("weikong/gateways/%s/topology/reported", c.gatewayKey), topology)
}

// PublishAlarmEvent exports a gateway-side alarm lifecycle transition. The
// payload stays versioned and keeps the gateway identity outside the event so
// cloud ingest can validate it against the MQTT topic.
func (c *Client) PublishAlarmEvent(event interface{}) error {
	payload := map[string]interface{}{
		"schemaVersion": 1,
		"eventType":     "ALARM",
		"gatewayKey":    c.gatewayKey,
		"reportedAt":    time.Now().Format(time.RFC3339Nano),
		"event":         event,
	}
	return c.publish(fmt.Sprintf("weikong/gateways/%s/events", c.gatewayKey), payload)
}

func (c *Client) PublishAttributes(metrics map[string]interface{}) error {
	payload, err := c.RenderManualPayload(metrics, nil)
	if err != nil {
		return err
	}
	return c.publishQoS(c.RenderManualTopic(), payload, 0)
}

func (c *Client) PublishManual(grouped map[string]map[string]interface{}) error {
	flat := map[string]interface{}{}
	for _, metrics := range grouped {
		for key, value := range metrics {
			flat[key] = value
		}
	}
	payload, err := c.RenderManualPayload(flat, grouped)
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		return ErrNoManualPayload
	}
	// Manual MQTT channels carry high-rate latest-value telemetry. QoS 0
	// avoids retaining messages and waiting for PUBACK on lightweight or
	// non-compliant brokers. Platform control/config traffic still uses the
	// default QoS 1 path below.
	return c.publishQoS(c.RenderManualTopic(), payload, 0)
}

func (c *Client) RenderManualTopic() string {
	topic := c.topicTemplate
	if topic == "" {
		topic = "attributes"
	}
	topic = strings.ReplaceAll(topic, "{gatewayKey}", c.gatewayKey)
	topic = strings.ReplaceAll(topic, "{clientId}", c.clientID)
	topic = strings.ReplaceAll(topic, "{username}", c.username)
	topic = strings.ReplaceAll(topic, "{ts}", time.Now().Format(time.RFC3339Nano))
	return topic
}

func (c *Client) RenderManualPayload(metrics map[string]interface{}, grouped map[string]map[string]interface{}) (map[string]interface{}, error) {
	attributes := sanitizeMetrics(metrics)
	grouped = normalizeTelemetryGroups(grouped)
	if c.payloadMode == "grouped" {
		return map[string]interface{}{
			"ts":         time.Now().Format(time.RFC3339Nano),
			"gatewayKey": c.gatewayKey,
			"devices":    grouped,
		}, nil
	}
	if c.payloadMode != "custom" {
		return attributes, nil
	}
	template := strings.TrimSpace(c.payloadTemplate)
	if template == "" {
		return attributes, nil
	}
	template = normalizeAttributePointTemplate(template)
	rendered := renderPayloadTemplate(template, map[string]interface{}{
		"gatewayKey": c.gatewayKey,
		"clientId":   c.clientID,
		"username":   c.username,
		"ts":         time.Now().Format(time.RFC3339Nano),
		"eventTime":  time.Now().UTC().Format("20060102T150405Z"),
		"attributes": attributes,
		"metrics":    attributes,
		"devices":    grouped,
	})
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(rendered), &payload); err != nil {
		return nil, fmt.Errorf("parse rendered payload template failed: %w", err)
	}
	return payload, nil
}

func normalizeAttributePointTemplate(template string) string {
	match := exactAttributePointTemplatePattern.FindStringSubmatch(strings.TrimSpace(template))
	if len(match) != 2 {
		return template
	}
	key, _ := json.Marshal(match[1])
	return "{" + string(key) + ":" + match[0] + "}"
}

func sanitizeMetrics(metrics map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{}, len(metrics))
	for key, value := range metrics {
		if strings.TrimSpace(key) == "" {
			continue
		}
		sanitized[key] = normalizeTelemetryValue(value)
	}
	return sanitized
}

func (c *Client) publish(topic string, payload interface{}) error {
	return c.publishQoS(topic, payload, 1)
}

func (c *Client) publishQoS(topic string, payload interface{}, qos byte) error {
	if !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt is not connected")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	token := c.mqtt.Publish(topic, qos, false, raw)
	if !token.WaitTimeout(2 * time.Second) {
		return fmt.Errorf("mqtt publish timeout")
	}
	if token.Error() != nil {
		return token.Error()
	}
	return nil
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func renderPayloadTemplate(template string, values map[string]interface{}) string {
	rendered := template
	rendered = replaceAttributeTemplateTokens(rendered, values)
	for key, value := range values {
		token := "{" + key + "}"
		stringToken := "\"" + token + "\""
		if text, ok := value.(string); ok {
			quoted, _ := json.Marshal(text)
			rendered = strings.ReplaceAll(rendered, stringToken, string(quoted))
		}
		raw, _ := json.Marshal(value)
		rendered = strings.ReplaceAll(rendered, token, string(raw))
	}
	return rendered
}

func replaceAttributeTemplateTokens(template string, values map[string]interface{}) string {
	rendered := template
	grouped, _ := values["devices"].(map[string]map[string]interface{})
	attributes, _ := values["attributes"].(map[string]interface{})
	tokens := attributeTemplateTokenPattern.FindAllString(template, -1)
	for _, token := range tokens {
		value := attributeTemplateValue(token, attributes, grouped)
		if text, ok := value.(string); ok {
			quoted, _ := json.Marshal(text)
			rendered = strings.ReplaceAll(rendered, `"`+token+`"`, string(quoted))
		}
		raw, _ := json.Marshal(value)
		rendered = strings.ReplaceAll(rendered, token, string(raw))
	}
	return rendered
}

func attributeTemplateValue(token string, attributes map[string]interface{}, grouped map[string]map[string]interface{}) interface{} {
	path := strings.TrimSuffix(strings.TrimPrefix(token, "{attribute"), "}")
	path = strings.TrimPrefix(path, ".")
	var value interface{} = attributes
	if path != "" {
		parts := strings.Split(path, ".")
		if len(parts) >= 1 {
			value = grouped[parts[0]]
		}
		if len(parts) >= 2 {
			deviceValues, _ := value.(map[string]interface{})
			value = deviceValues[parts[1]]
		}
	}
	return value
}

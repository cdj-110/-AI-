package cloud

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

var attributeTemplateTokenPattern = regexp.MustCompile(`\{attribute(?:\.[^{}\s",:\[\]]+){0,2}\}`)
var exactAttributePointTemplatePattern = regexp.MustCompile(`^\{attribute\.[^{}\s",:\[\]]+\.([^{}\s",:\[\]]+)\}$`)

type Client struct {
	name            string
	manual          bool
	gatewayKey      string
	hardwareID      string
	clientID        string
	username        string
	topicTemplate   string
	payloadMode     string
	payloadTemplate string
	subscribeTopic  string
	mqtt            mqtt.Client
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
	}, onConnectionChanged)
}

func NewActivationMQTT(cfg config.Config, onConnectionChanged func(bool)) *Client {
	return NewMQTT(MQTTOptions{
		Name:       "activation",
		Broker:     cfg.Activation.Broker,
		ClientID:   "factory_" + cfg.Activation.HardwareID,
		Username:   "factory:" + cfg.Activation.HardwareID,
		Password:   cfg.Activation.DeviceSecret,
		GatewayKey: cfg.Activation.SN,
		HardwareID: cfg.Activation.HardwareID,
	}, onConnectionChanged)
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
	if onConnectionChanged != nil {
		clientOptions.SetOnConnectHandler(func(mqtt.Client) {
			onConnectionChanged(true)
		})
		clientOptions.SetConnectionLostHandler(func(_ mqtt.Client, _ error) {
			onConnectionChanged(false)
		})
	}

	return &Client{
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
		mqtt:            mqtt.NewClient(clientOptions),
	}
}

func (c *Client) SubscribeAttributes(handler func(topic string, payload []byte)) error {
	if strings.TrimSpace(c.subscribeTopic) == "" {
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
	return token.Error()
}

func (c *Client) RenderSubscribeTopic() string {
	topic := strings.TrimSpace(c.subscribeTopic)
	topic = strings.ReplaceAll(topic, "{gatewayKey}", c.gatewayKey)
	topic = strings.ReplaceAll(topic, "{clientId}", c.clientID)
	return topic
}

func (c *Client) SubscribeRemoteConfig(handler func(RemoteConfigCommand) RemoteConfigResult, snapshot func() interface{}) error {
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
	report()
	return nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) IsManual() bool {
	return c.manual
}

func (c *Client) Connect() error {
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

func (c *Client) PublishAttributes(metrics map[string]interface{}) error {
	payload, err := c.RenderManualPayload(metrics, nil)
	if err != nil {
		return err
	}
	return c.publish(c.RenderManualTopic(), payload)
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
		return nil
	}
	return c.publish(c.RenderManualTopic(), payload)
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
		sanitized[key] = value
	}
	return sanitized
}

func (c *Client) publish(topic string, payload interface{}) error {
	if !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt is not connected")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	token := c.mqtt.Publish(topic, 1, false, raw)
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

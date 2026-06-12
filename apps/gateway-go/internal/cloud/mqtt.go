package cloud

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type Client struct {
	gatewayKey string
	mqtt       mqtt.Client
}

func NewMQTT(cfg config.Config, onConnectionChanged func(bool)) *Client {
	options := mqtt.NewClientOptions().
		AddBroker(cfg.MQTT.Broker).
		SetClientID(cfg.MQTT.ClientID).
		SetUsername(cfg.MQTT.Username).
		SetPassword(cfg.MQTT.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectTimeout(5 * time.Second).
		SetKeepAlive(30 * time.Second)
	if onConnectionChanged != nil {
		options.SetOnConnectHandler(func(mqtt.Client) {
			onConnectionChanged(true)
		})
		options.SetConnectionLostHandler(func(_ mqtt.Client, _ error) {
			onConnectionChanged(false)
		})
	}

	return &Client{
		gatewayKey: cfg.GatewayKey,
		mqtt:       mqtt.NewClient(options),
	}
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

package cloud

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type OTACommand struct {
	TaskID     string    `json:"taskId"`
	HardwareID string    `json:"hardwareId"`
	SN         string    `json:"sn"`
	PackageURL string    `json:"packageUrl"`
	PackageSHA string    `json:"packageSha256"`
	Release    int64     `json:"release"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type OTAResult struct {
	TaskID     string `json:"taskId"`
	HardwareID string `json:"hardwareId"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	Release    int64  `json:"release"`
}

func (c *Client) SubscribeOTA(handler func(OTACommand) OTAResult, resultPath string) error {
	c.otaMu.Lock()
	c.otaHandler, c.otaResultPath = handler, resultPath
	c.otaMu.Unlock()
	return c.subscribeOTANow()
}

func (c *Client) subscribeOTANow() error {
	c.otaMu.RLock()
	handler := c.otaHandler
	c.otaMu.RUnlock()
	if handler == nil || c.hardwareID == "" {
		return nil
	}
	if !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt is not connected")
	}
	topic := fmt.Sprintf("weikong/factory/%s/ota/set", c.hardwareID)
	token := c.mqtt.Subscribe(topic, 1, func(_ mqtt.Client, message mqtt.Message) {
		if message.Retained() || len(message.Payload()) > 4096 {
			return
		}
		var command OTACommand
		if json.Unmarshal(message.Payload(), &command) != nil {
			return
		}
		go func() {
			result := handler(command)
			if err := c.PublishOTAResult(result); err != nil {
				log.Printf("publish OTA result failed: %v", err)
			}
		}()
	})
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe OTA timeout")
	}
	return token.Error()
}

func (c *Client) PublishOTAResult(result OTAResult) error {
	if c.hardwareID == "" {
		return fmt.Errorf("not an activation channel")
	}
	return c.publish(fmt.Sprintf("weikong/factory/%s/ota/reply", c.hardwareID), result)
}

func (c *Client) PublishPendingOTAResult() {
	c.otaMu.RLock()
	path := c.otaResultPath
	c.otaMu.RUnlock()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var result OTAResult
	if json.Unmarshal(raw, &result) != nil || result.HardwareID != c.hardwareID {
		return
	}
	if err := c.PublishOTAResult(result); err == nil {
		_ = os.Remove(path)
	}
}

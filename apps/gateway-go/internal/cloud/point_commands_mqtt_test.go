package cloud

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type commandTestToken struct{ mqtt.Token }

func (commandTestToken) WaitTimeout(time.Duration) bool { return true }
func (commandTestToken) Error() error                   { return nil }

type commandTestMessage struct {
	mqtt.Message
	retained bool
	raw      []byte
}

func (m commandTestMessage) Retained() bool  { return m.retained }
func (m commandTestMessage) Payload() []byte { return m.raw }

type commandTestPublish struct {
	topic    string
	qos      byte
	retained bool
	payload  interface{}
}
type commandTestMQTT struct {
	mqtt.Client
	handler    mqtt.MessageHandler
	published  chan commandTestPublish
	subscribed string
}

func (*commandTestMQTT) IsConnected() bool { return true }
func (m *commandTestMQTT) Subscribe(topic string, qos byte, handler mqtt.MessageHandler) mqtt.Token {
	m.subscribed = topic
	m.handler = handler
	return commandTestToken{}
}
func (m *commandTestMQTT) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	m.published <- commandTestPublish{topic, qos, retained, payload}
	return commandTestToken{}
}

func TestPointCommandMQTTRejectsRetainedAndRepliesOnOwnTopic(t *testing.T) {
	transport := &commandTestMQTT{published: make(chan commandTestPublish, 2)}
	client := &Client{gatewayKey: "GW-A", mqtt: transport}
	var calls atomic.Int32
	if err := client.SubscribePointCommands(func(command PointCommand) PointCommandResult {
		calls.Add(1)
		return PointCommandResult{CommandID: command.CommandID, DeviceKey: command.DeviceKey, Metric: command.Metric, Status: "SUCCEEDED"}
	}); err != nil {
		t.Fatal(err)
	}
	if transport.subscribed != "weikong/gateways/GW-A/commands/set" {
		t.Fatal(transport.subscribed)
	}
	command := testPointCommand()
	raw, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	transport.handler(transport, commandTestMessage{retained: true, raw: raw})
	transport.handler(transport, commandTestMessage{raw: make([]byte, 8193)})
	transport.handler(transport, commandTestMessage{raw: []byte("broken")})
	if calls.Load() != 0 {
		t.Fatal("retained, oversized or invalid commands must not execute")
	}
	transport.handler(transport, commandTestMessage{raw: raw})
	select {
	case reply := <-transport.published:
		if reply.topic != "weikong/gateways/GW-A/commands/reply" || reply.qos != 1 || reply.retained {
			t.Fatal(reply)
		}
		var body PointCommandResult
		if err := json.Unmarshal(reply.payload.([]byte), &body); err != nil {
			t.Fatal(err)
		}
		if body.CommandID != command.CommandID || body.DeviceKey != command.DeviceKey || body.Metric != command.Metric || body.Status != "SUCCEEDED" {
			t.Fatal(body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing command execution receipt")
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
}

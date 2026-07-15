package cloud

import "testing"

func TestRenderManualTopicUsesTemplate(t *testing.T) {
	client := &Client{gatewayKey: "gw-1", topicTemplate: "gateways/{gatewayKey}/telemetry"}

	if got := client.RenderManualTopic(); got != "gateways/gw-1/telemetry" {
		t.Fatalf("unexpected topic: %s", got)
	}
}

func TestRenderManualTopicUsesHuaweiDeviceID(t *testing.T) {
	client := &Client{
		username:      "device-123",
		clientID:      "device-123_0_0_2026070307",
		topicTemplate: "$oc/devices/{username}/sys/properties/report",
	}

	if got := client.RenderManualTopic(); got != "$oc/devices/device-123/sys/properties/report" {
		t.Fatalf("unexpected topic: %s", got)
	}
}

func TestRenderManualPayloadCustomTemplate(t *testing.T) {
	client := &Client{
		gatewayKey:  "gw-1",
		username:    "device-123",
		clientID:    "client-123",
		payloadMode: "custom",
		payloadTemplate: `{
  "gatewayKey": "{gatewayKey}",
  "username": "{username}",
  "clientId": "{clientId}",
  "ts": "{ts}",
  "eventTime": "{eventTime}",
  "data": {attributes},
  "devices": {devices}
}`,
	}
	grouped := map[string]map[string]interface{}{
		"device-1": {"temperature": 23.5},
	}

	payload, err := client.RenderManualPayload(map[string]interface{}{"temperature": 23.5}, grouped)
	if err != nil {
		t.Fatalf("RenderManualPayload returned error: %v", err)
	}
	if payload["gatewayKey"] != "gw-1" {
		t.Fatalf("gatewayKey was not rendered correctly: %#v", payload["gatewayKey"])
	}
	if payload["username"] != "device-123" || payload["clientId"] != "client-123" {
		t.Fatalf("device identifiers were not rendered correctly: %#v", payload)
	}
	if _, ok := payload["ts"].(string); !ok {
		t.Fatalf("ts was not rendered as string: %#v", payload["ts"])
	}
	if eventTime, ok := payload["eventTime"].(string); !ok || len(eventTime) != len("20161219T114920Z") {
		t.Fatalf("eventTime was not rendered as Huawei UTC timestamp: %#v", payload["eventTime"])
	}
	data, ok := payload["data"].(map[string]interface{})
	if !ok || data["temperature"] != 23.5 {
		t.Fatalf("attributes were not embedded correctly: %#v", payload["data"])
	}
	devices, ok := payload["devices"].(map[string]interface{})
	if !ok || devices["device-1"] == nil {
		t.Fatalf("devices were not embedded correctly: %#v", payload["devices"])
	}
}

func TestRenderManualPayloadAttributePathTemplate(t *testing.T) {
	client := &Client{
		payloadMode: "custom",
		payloadTemplate: `{
  "all": {attribute},
  "device": {attribute.device-1},
  "point": {attribute.device-1.temperature},
  "status": "{attribute.device-1.status}"
}`,
	}
	grouped := map[string]map[string]interface{}{
		"device-1": {"temperature": 23.5, "humidity": 61, "status": "ok"},
		"device-2": {"pressure": 100.8},
	}

	payload, err := client.RenderManualPayload(map[string]interface{}{
		"temperature": 23.5,
		"humidity":    61,
		"pressure":    100.8,
		"status":      "ok",
	}, grouped)
	if err != nil {
		t.Fatalf("RenderManualPayload returned error: %v", err)
	}
	if payload["point"] != 23.5 {
		t.Fatalf("point attribute was not rendered correctly: %#v", payload["point"])
	}
	if payload["status"] != "ok" {
		t.Fatalf("string point attribute was not rendered correctly: %#v", payload["status"])
	}
	device, ok := payload["device"].(map[string]interface{})
	if !ok || device["humidity"] != float64(61) {
		t.Fatalf("device attribute group was not rendered correctly: %#v", payload["device"])
	}
	all, ok := payload["all"].(map[string]interface{})
	if !ok || all["pressure"] != 100.8 {
		t.Fatalf("all attributes were not rendered correctly: %#v", payload["all"])
	}
}

func TestRenderManualPayloadSingleAttributePointTemplateBecomesObject(t *testing.T) {
	client := &Client{
		payloadMode:     "custom",
		payloadTemplate: `{attribute.device-001.TS}`,
	}
	grouped := map[string]map[string]interface{}{
		"device-001": {"TS": 42},
	}

	payload, err := client.RenderManualPayload(map[string]interface{}{"TS": 42}, grouped)
	if err != nil {
		t.Fatalf("RenderManualPayload returned error: %v", err)
	}
	if payload["TS"] != float64(42) {
		t.Fatalf("single point template should become TS key/value object: %#v", payload)
	}
}

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestAISettingsArePrivateAndMasked(t *testing.T) {
	manager := newAIManager(filepath.Join(t.TempDir(), "config.json"))
	settings := defaultAISettings()
	settings.Enabled = true
	settings.DeepSeek.APIKey = "deepseek-secret"

	if err := manager.saveSettings(settings); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(manager.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatalf("AI settings permissions = %o, want no group/other access", info.Mode().Perm())
	}
	raw, err := os.ReadFile(manager.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "deepseek-secret") {
		t.Fatal("saved settings do not contain configured credentials")
	}
	masked := maskAISettings(settings)
	if masked.DeepSeek.APIKey != "***" {
		t.Fatalf("masked settings leaked API keys: %#v", masked)
	}
}

func TestAISettingsAllowDeepSeekOnlyFeatures(t *testing.T) {
	manager := newAIManager(filepath.Join(t.TempDir(), "config.json"))
	settings := defaultAISettings()
	settings.Enabled = true
	settings.DeepSeek.APIKey = "deepseek-secret"
	if err := manager.saveSettings(settings); err != nil {
		t.Fatalf("DeepSeek-only settings were rejected: %v", err)
	}
}

func TestGatewayAISystemPolicyRestrictsUnrelatedAnswers(t *testing.T) {
	for _, required := range []string{"只服务于当前网关", "完整对话上下文", "该问题不属于网关助手支持范围", "只读工具返回结果后", "不重复已完成步骤"} {
		if !strings.Contains(gatewayAISystemPolicy, required) {
			t.Fatalf("gateway AI policy is missing %q", required)
		}
	}
}

func TestAIChatFeedsDiagnosticResultBackToModel(t *testing.T) {
	var upstreamCalls int
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalls++
		var payload map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		messages, _ := payload["messages"].([]interface{})
		if upstreamCalls == 1 {
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{
				"content": "",
				"tool_calls": []interface{}{map[string]interface{}{
					"id": "call-diagnose", "type": "function",
					"function": map[string]interface{}{"name": "diagnose_connection", "arguments": `{"deviceKey":"device-001"}`},
				}},
			}}}})
			return
		}
		rawMessages, _ := json.Marshal(messages)
		if !bytes.Contains(rawMessages, []byte(`"role":"tool"`)) ||
			!bytes.Contains(rawMessages, []byte(`"tool_call_id":"call-diagnose"`)) ||
			!bytes.Contains(rawMessages, []byte("串口设备不单独抢占端口测试")) {
			http.Error(writer, "diagnostic result was not returned to the model", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{
			"content": "诊断完成：串口由通道共享，当前不应抢占端口。建议查看实时点位和报文监控。",
		}}}})
	}))
	defer upstream.Close()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	serial := config.SerialPort{Port: "COM2", BaudRate: 9600, DataBits: 8, StopBits: 1}
	cfg := config.Config{
		GatewayKey: "test-gateway",
		Channels: []config.ChannelConfig{{
			ChannelKey: "channel-001", Name: "串口采集", Type: "serial", Role: "collect",
			Protocol: "modbus-rtu", Enabled: true, Serial: &serial, CollectIntervalMilliseconds: 1000,
		}},
		Devices: []config.DeviceConfig{{
			DeviceKey: "device-001", ChannelKey: "channel-001", Name: "电表", Protocol: "modbus-rtu",
			Address: "COM2", Points: []config.PointConfig{{Metric: "voltage", Function: 3, Quantity: 1, DataType: "uint16"}},
		}},
	}
	rawConfig, _ := json.Marshal(cfg)
	if err := os.WriteFile(configPath, rawConfig, 0600); err != nil {
		t.Fatal(err)
	}
	manager := newAIManager(configPath)
	manager.httpClient = upstream.Client()
	settings := defaultAISettings()
	settings.Enabled = true
	settings.DeepSeek.BaseURL = upstream.URL
	settings.DeepSeek.APIKey = "test-key"
	if err := manager.saveSettings(settings); err != nil {
		t.Fatal(err)
	}
	server := &Server{configPath: configPath, ai: manager}
	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"诊断电表连接"}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/ai/chat", body)
	recorder := httptest.NewRecorder()

	server.aiChat(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("aiChat status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if upstreamCalls != 2 {
		t.Fatalf("upstream calls=%d, want 2", upstreamCalls)
	}
	if !strings.Contains(recorder.Body.String(), "诊断完成") {
		t.Fatalf("response did not contain interpreted diagnostic result: %s", recorder.Body.String())
	}
}

func TestApplyAIAddChannelCreatesValidatedCollectAndForwardChannels(t *testing.T) {
	cfg := config.Config{Resources: []config.ResourceConfig{{
		ResourceKey: "eth0", Name: "网口1", Type: "network", Enabled: true,
		Network: &config.NetworkPort{Interface: "eth0", Enabled: true},
	}}}
	collect := map[string]interface{}{
		"resourceKey": "eth0", "name": "Modbus采集", "role": "collect",
		"protocol": "modbus-tcp", "collectIntervalMilliseconds": 500,
	}
	if err := applyAIAddChannel(&cfg, collect); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Channels[0]; got.ChannelKey != "channel-001" || got.Protocol != "modbus-tcp" || got.ForwardProtocol != "" || got.CollectIntervalMilliseconds != 500 {
		t.Fatalf("collect channel = %#v", got)
	}
	forward := map[string]interface{}{
		"resourceKey": "eth0", "name": "104转发", "role": "forward",
		"protocol": "iec104-server", "collectIntervalMilliseconds": 1000,
	}
	if err := applyAIAddChannel(&cfg, forward); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Channels[1]; got.ChannelKey != "channel-002" || got.Protocol != "none" || got.ForwardProtocol != "iec104-server" {
		t.Fatalf("forward channel = %#v", got)
	}
}

func TestApplyAIAddChannelRejectsProtocolRoleMismatch(t *testing.T) {
	cfg := config.Config{Resources: []config.ResourceConfig{{ResourceKey: "eth0", Type: "network"}}}
	err := applyAIAddChannel(&cfg, map[string]interface{}{
		"resourceKey": "eth0", "name": "错误通道", "role": "collect",
		"protocol": "iec104-server", "collectIntervalMilliseconds": 1000,
	})
	if err == nil {
		t.Fatal("collect channel accepted a forwarding protocol")
	}
}

func TestAIDeviceAndChannelActions(t *testing.T) {
	serial := config.SerialPort{Port: "COM2", BaudRate: 9600, DataBits: 8, StopBits: 1}
	cfg := config.Config{Channels: []config.ChannelConfig{{
		ChannelKey: "channel-01", Name: "RTU", Type: "serial", Role: "collect",
		Protocol: "modbus-rtu", Enabled: true, Serial: &serial, CollectIntervalMilliseconds: 1000,
	}}}
	if err := applyAIAddDevice(&cfg, map[string]interface{}{
		"channelKey": "channel-01", "name": "电表", "address": "", "slaveId": 2,
		"commonAddress": 0, "rack": 0, "slot": 0,
	}); err != nil {
		t.Fatal(err)
	}
	device := cfg.Devices[0]
	if device.ChannelKey != "channel-01" || device.Protocol != "modbus-rtu" || device.Address != "COM2" || device.SlaveID != 2 {
		t.Fatalf("AI-created device = %#v", device)
	}
	if err := applyAIUpdateChannel(&cfg, map[string]interface{}{
		"channelKey": "channel-01", "name": "RTU快速采集", "collectIntervalMilliseconds": 250,
		"enabled": true, "acquisitionMode": "",
	}); err != nil {
		t.Fatal(err)
	}
	if cfg.Channels[0].Name != "RTU快速采集" || cfg.Channels[0].CollectIntervalMilliseconds != 250 {
		t.Fatalf("updated channel = %#v", cfg.Channels[0])
	}
	if err := applyAISetDeviceEnabled(&cfg, map[string]interface{}{"deviceKey": device.DeviceKey, "enabled": false}); err != nil {
		t.Fatal(err)
	}
	if cfg.Devices[0].IsEnabled() {
		t.Fatal("device should be disabled")
	}
	cfg.Devices[0].Points = []config.PointConfig{{Metric: "voltage", Function: 3, Quantity: 1}}
	if points := cfg.FlattenPoints(); len(points) != 0 {
		t.Fatalf("disabled device still contributes %d runtime points", len(points))
	}
}

func TestAICreateCollectionDeviceIsSingleConfigTransaction(t *testing.T) {
	cfg := config.Config{Resources: []config.ResourceConfig{{
		ResourceKey: "eth0", Name: "网口1", Type: "network", Enabled: true,
		Network: &config.NetworkPort{Interface: "eth0", Enabled: true},
	}}}
	err := applyAICreateCollectionDevice(&cfg, map[string]interface{}{
		"resourceKey": "eth0", "channelName": "104采集", "protocol": "iec104",
		"collectIntervalMilliseconds": 1000, "deviceName": "保护装置",
		"address": "192.168.1.10:2404", "slaveId": 1, "commonAddress": 2, "rack": 0, "slot": 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Channels) != 1 || len(cfg.Devices) != 1 {
		t.Fatalf("transaction created channels=%d devices=%d", len(cfg.Channels), len(cfg.Devices))
	}
	if cfg.Devices[0].ChannelKey != cfg.Channels[0].ChannelKey || cfg.Devices[0].Protocol != "iec104" || cfg.Devices[0].CommonAddress != 2 {
		t.Fatalf("transaction result channel=%#v device=%#v", cfg.Channels[0], cfg.Devices[0])
	}
}

func TestApplyAIBrowsedPointsUsesExplicitSelection(t *testing.T) {
	cfg := config.Config{Devices: []config.DeviceConfig{{
		DeviceKey: "opc-01", ChannelKey: "channel-01", Name: "OPC设备", Protocol: "opcua",
		Address: "opc.tcp://127.0.0.1:4840", Points: []config.PointConfig{},
	}}}
	candidates := []config.PointConfig{
		{Metric: "temperature", NodeID: "ns=2;s=temp", DataType: "float32", Quantity: 1, Scale: 1},
		{Metric: "pressure", NodeID: "ns=2;s=pressure", DataType: "float32", Quantity: 1, Scale: 1},
	}
	if err := applyAIBrowsedPoints(&cfg, map[string]interface{}{"deviceKey": "opc-01", "candidatePoints": candidates}, []string{"pressure"}); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Devices[0].Points) != 1 || cfg.Devices[0].Points[0].Metric != "pressure" || cfg.Devices[0].Points[0].Name != "OPC设备@pressure" {
		t.Fatalf("selected points = %#v", cfg.Devices[0].Points)
	}
}

func TestNormalizeAIDraftPreservesExistingSecretsAndConnectionFields(t *testing.T) {
	existing := config.DeviceConfig{
		DeviceKey: "opc-01", ChannelKey: "channel-01", Name: "Original",
		Protocol: "opcua", Address: "opc.tcp://192.168.1.10:4840",
		Username: "operator", Password: "secret", LocalTSAP: "0100",
		Points: []config.PointConfig{{Name: "Old", Metric: "old", NodeID: "ns=2;s=old"}},
	}
	current := config.Config{Devices: []config.DeviceConfig{existing}}
	channel := config.ChannelConfig{
		ChannelKey: "channel-01", Name: "OPC UA", Protocol: "opcua",
		Type: "network", Role: "collect", Enabled: true,
	}
	generated := config.DeviceConfig{
		Name: "Updated",
		// Deliberately attempt to inject different credentials. Update must
		// preserve the credentials already managed by the gateway.
		Username: "model-user", Password: "model-secret",
		Points: []config.PointConfig{{Name: "Temperature", Metric: "temp", NodeID: "ns=2;s=temp", DataType: "float32"}},
	}

	draft := normalizeAIDraft(current, channel, "update", existing.DeviceKey, generated)
	if len(draft.BlockingIssues) != 0 {
		t.Fatalf("blocking issues = %#v", draft.BlockingIssues)
	}
	if draft.Device.Username != "operator" || draft.Device.Password != "secret" {
		t.Fatalf("existing credentials were not preserved: %#v", draft.Device)
	}
	if draft.Device.Address != existing.Address || draft.Device.LocalTSAP != existing.LocalTSAP {
		t.Fatalf("existing connection fields were not preserved: %#v", draft.Device)
	}
	if len(draft.Device.Points) != 1 || draft.Device.Points[0].Password != "secret" {
		t.Fatalf("point connection credentials were not inherited: %#v", draft.Device.Points)
	}
}

func TestNormalizeAIDraftRejectsUnknownProtocolFieldsAndDeduplicatesMetric(t *testing.T) {
	current := config.Config{Devices: []config.DeviceConfig{{
		DeviceKey: "existing", Name: "Existing", Protocol: "modbus-tcp",
		Address: "127.0.0.1:502",
		Points:  []config.PointConfig{{Name: "P1", Metric: "temperature", Function: 3, DataType: "uint16"}},
	}}}
	channel := config.ChannelConfig{
		ChannelKey: "channel-01", Name: "Modbus TCP", Protocol: "modbus-tcp",
		Type: "network", Role: "collect", Enabled: true,
	}
	generated := config.DeviceConfig{
		DeviceKey: "new-device", Name: "New", Address: "192.168.1.20:502", SlaveID: 1,
		Username: "must-not-survive", Password: "must-not-survive",
		Points: []config.PointConfig{
			{Name: "Temperature", Metric: "temperature", Function: 3, DataType: "uint16"},
			{Name: "Missing function", Metric: "missing", DataType: "uint16"},
		},
	}

	draft := normalizeAIDraft(current, channel, "create", "", generated)
	if draft.Device.Username != "" || draft.Device.Password != "" {
		t.Fatal("model-generated credentials survived create normalization")
	}
	if draft.Device.Points[0].Metric == "temperature" {
		t.Fatal("global duplicate metric was not deterministically renamed")
	}
	if len(draft.BlockingIssues) == 0 {
		t.Fatal("missing Modbus function did not block the draft")
	}
}

func TestPDFTextIsExtractedLocallyWithoutCloudUpload(t *testing.T) {
	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "manual.pdf")
	raw := []byte("%PDF-1.7\nstream\nBT (Register 10 Temperature uint16 function 3 scale 0.1 unit Celsius) Tj ET\nendstream")
	if err := os.WriteFile(pdfPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	manager := newAIManager(filepath.Join(tempDir, "config.json"))
	text, err := manager.extractPDF(context.Background(), aiProviderSettings{}, pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Register 10", "Temperature", "uint16", "Celsius"} {
		if !strings.Contains(text, want) {
			t.Fatalf("extracted text = %q, missing %q", text, want)
		}
	}
}

func TestPDFProtocolSupportExcludesOPCDA(t *testing.T) {
	if supported, _ := aiPDFProtocolSupport("opcda"); supported {
		t.Fatal("OPC DA must use server browse instead of PDF generation")
	}
	for _, protocol := range []string{"modbus-tcp", "modbus-rtu", "iec104", "siemens-s7", "iec61850", "iec61850-goose", "opcua"} {
		if supported, reason := aiPDFProtocolSupport(protocol); !supported {
			t.Fatalf("protocol %s unexpectedly rejected: %s", protocol, reason)
		}
	}
}

func TestReceivePreprocessedPDFTextAndPages(t *testing.T) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for key, value := range map[string]string{
		"channelKey": "channel-01", "targetMode": "create", "sourceName": "manual.pdf",
		"extractedText": "Register 10 Temperature uint16 function 3 scale 0.1 unit Celsius",
	} {
		if err := form.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	page, err := form.CreateFormFile("page", "page-001.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := page.Write([]byte{0xff, 0xd8, 0xff, 0xd9}); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}

	manager := newAIManager(filepath.Join(t.TempDir(), "config.json"))
	server := &Server{ai: manager}
	request := httptest.NewRequest(http.MethodPost, "/api/ai/config-jobs", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	input, fields, err := server.receiveAIPDF(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer input.cleanup()
	if fields["channelKey"] != "channel-01" || !strings.Contains(input.DocumentText, "Temperature") || len(input.PagePaths) != 1 {
		t.Fatalf("preprocessed input = %#v, fields = %#v", input, fields)
	}
	if _, err := os.Stat(input.PagePaths[0]); err != nil {
		t.Fatalf("preprocessed page was not persisted for the job: %v", err)
	}
}

func TestDeepSeekDraftGenerationUsesJSONMode(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		responseFormat, _ := payload["response_format"].(map[string]interface{})
		if responseFormat["type"] != "json_object" {
			http.Error(writer, "JSON mode required", http.StatusBadRequest)
			return
		}
		content := `{"device":{"deviceKey":"device-01","name":"Meter","address":"192.168.1.20:502","slaveId":1,"points":[{"name":"Temperature","metric":"temp","function":3,"register":10,"quantity":1,"dataType":"uint16"}]},"warnings":["量程需要复核"],"uncertainFields":["Temperature：倍率未在文档中标明"]}`
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"choices": []interface{}{map[string]interface{}{"message": map[string]string{"content": content}}},
		})
	}))
	defer upstream.Close()

	manager := newAIManager(filepath.Join(t.TempDir(), "config.json"))
	manager.httpClient = upstream.Client()
	provider := aiProviderSettings{BaseURL: upstream.URL, APIKey: "deepseek-key", Model: "deepseek-v4-pro"}
	channel := config.ChannelConfig{ChannelKey: "channel-01", Name: "Modbus", Protocol: "modbus-tcp", Enabled: true}

	generated, err := manager.generateDeviceDraft(context.Background(), provider, channel, nil, "register table")
	if err != nil {
		t.Fatal(err)
	}
	device := generated.Device
	if device.DeviceKey != "device-01" || len(device.Points) != 1 || device.Points[0].Register != 10 {
		t.Fatalf("generated device = %#v", device)
	}
	if len(generated.Warnings) != 1 || len(generated.UncertainFields) != 1 {
		t.Fatalf("generated review metadata = %#v", generated)
	}
}

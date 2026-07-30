package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestAISettingsArePrivateAndMasked(t *testing.T) {
	manager := newAIManager(filepath.Join(t.TempDir(), "config.json"))
	settings := defaultAISettings()
	settings.Enabled = true
	settings.DeepSeek.APIKey = "deepseek-secret"
	settings.Bailian.APIKey = "bailian-secret"
	settings.Bailian.WorkspaceID = "workspace-a"

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
	if !strings.Contains(string(raw), "deepseek-secret") || !strings.Contains(string(raw), "bailian-secret") {
		t.Fatal("saved settings do not contain configured credentials")
	}
	masked := maskAISettings(settings)
	if masked.DeepSeek.APIKey != "***" || masked.Bailian.APIKey != "***" {
		t.Fatalf("masked settings leaked API keys: %#v", masked)
	}
}

func TestAISettingsAllowChatWithoutBailian(t *testing.T) {
	manager := newAIManager(filepath.Join(t.TempDir(), "config.json"))
	settings := defaultAISettings()
	settings.Enabled = true
	settings.DeepSeek.APIKey = "deepseek-secret"
	if err := manager.saveSettings(settings); err != nil {
		t.Fatalf("DeepSeek-only chat settings were rejected: %v", err)
	}
}

func TestGatewayAISystemPolicyRestrictsUnrelatedAnswers(t *testing.T) {
	for _, required := range []string{"只服务于当前网关", "完整对话上下文", "该问题不属于网关助手支持范围"} {
		if !strings.Contains(gatewayAISystemPolicy, required) {
			t.Fatalf("gateway AI policy is missing %q", required)
		}
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

func TestBailianPDFExtractionUsesTemporaryOSSURL(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var upstream *httptest.Server
	upstream = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		calls = append(calls, request.Method+" "+request.URL.Path)
		mu.Unlock()
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/uploads":
			if request.Header.Get("Authorization") != "Bearer bailian-key" ||
				request.URL.Query().Get("action") != "getPolicy" ||
				request.URL.Query().Get("model") != "qwen3.5-ocr" {
				http.Error(writer, "invalid upload policy request", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{
				"data": map[string]string{
					"policy": "policy", "signature": "signature",
					"upload_dir": "dashscope-instant/test", "upload_host": upstream.URL + "/oss",
					"oss_access_key_id": "access-key", "x_oss_object_acl": "private",
					"x_oss_forbid_overwrite": "true",
				},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/oss":
			if err := request.ParseMultipartForm(1024 * 1024); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			if request.FormValue("OSSAccessKeyId") != "access-key" ||
				request.FormValue("key") == "" ||
				request.FormValue("success_action_status") != "200" {
				http.Error(writer, "invalid OSS form", http.StatusBadRequest)
				return
			}
			file, _, err := request.FormFile("file")
			if err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			raw, _ := io.ReadAll(file)
			_ = file.Close()
			if !strings.HasPrefix(string(raw), "%PDF-") {
				http.Error(writer, "invalid PDF", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/responses":
			if request.Header.Get("Authorization") != "Bearer bailian-key" ||
				request.Header.Get("X-DashScope-OssResourceResolve") != "enable" {
				http.Error(writer, "missing response headers", http.StatusUnauthorized)
				return
			}
			var payload map[string]interface{}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			raw, _ := json.Marshal(payload)
			if !strings.Contains(string(raw), `"file_url":"oss://dashscope-instant/test/`) {
				http.Error(writer, "missing temporary OSS URL", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{
				"output": []interface{}{map[string]interface{}{"content": []interface{}{map[string]interface{}{"text": "寄存器 10 温度 uint16"}}}},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer upstream.Close()

	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "manual.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.7\nmock"), 0600); err != nil {
		t.Fatal(err)
	}
	manager := newAIManager(filepath.Join(tempDir, "config.json"))
	manager.httpClient = upstream.Client()
	provider := aiProviderSettings{BaseURL: upstream.URL + "/v1", APIKey: "bailian-key", Model: "qwen3.5-ocr"}

	text, err := manager.extractPDF(context.Background(), provider, pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if text != "寄存器 10 温度 uint16" {
		t.Fatalf("OCR text = %q", text)
	}
	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(calls, ",")
	for _, want := range []string{"GET /api/v1/uploads", "POST /oss", "POST /v1/responses"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("upstream calls = %q, missing %q", joined, want)
		}
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
		content := `{"device":{"deviceKey":"device-01","name":"Meter","address":"192.168.1.20:502","slaveId":1,"points":[{"name":"Temperature","metric":"temp","function":3,"register":10,"quantity":1,"dataType":"uint16"}]}}`
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"choices": []interface{}{map[string]interface{}{"message": map[string]string{"content": content}}},
		})
	}))
	defer upstream.Close()

	manager := newAIManager(filepath.Join(t.TempDir(), "config.json"))
	manager.httpClient = upstream.Client()
	provider := aiProviderSettings{BaseURL: upstream.URL, APIKey: "deepseek-key", Model: "deepseek-v4-pro"}
	channel := config.ChannelConfig{ChannelKey: "channel-01", Name: "Modbus", Protocol: "modbus-tcp", Enabled: true}

	device, err := manager.generateDeviceDraft(context.Background(), provider, channel, nil, "register table")
	if err != nil {
		t.Fatal(err)
	}
	if device.DeviceKey != "device-01" || len(device.Points) != 1 || device.Points[0].Register != 10 {
		t.Fatalf("generated device = %#v", device)
	}
}

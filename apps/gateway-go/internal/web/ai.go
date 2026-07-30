package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
)

const (
	maxAIPDFBytes         = 25 * 1024 * 1024
	maxOCRBytes           = 8 * 1024 * 1024
	aiJobTTL              = 30 * time.Minute
	aiJobTimeout          = 5 * time.Minute
	gatewayAISystemPolicy = `你是“微控工业网关专用助手”，只服务于当前网关。
允许回答和协助的范围仅包括：网关资源、串口与网口、采集和转发通道、设备与点位、Modbus、IEC104、IEC61850、OPC UA、S7、MQTT、边缘计算、网络与云连接、运行状态、日志、性能、故障诊断、系统维护，以及与这些功能直接相关的操作建议。
你需要结合完整对话上下文判断用户意图，所以“为什么”“继续”“怎么改”等追问应延续上一轮网关主题。
如果问题与上述网关范围无关，不提供该领域的知识、答案、创作或建议，只回复：该问题不属于网关助手支持范围。
只能依据给定的脱敏配置和状态回答；不得要求用户泄露密码、密钥或证书。
当用户要求执行工具列表支持的网关操作时，必须调用对应工具，不要声称自己是外部助手，也不要让用户改去 Web 配置页面。修改配置的工具只生成待确认项，用户确认后才真正执行；连接诊断等只读工具可以立即执行并返回真实结果。
缺少关键参数或存在多个合理选择时必须调用 request_gateway_input，让用户点选或填写；不得猜测 IP、端口、从站号、通道、设备、寄存器、IOA、NodeId 或对象引用。
同时缺少两个或更多参数时必须使用 request_gateway_form 一次性收集，不要连续提出单个问题。
多步任务每次只调用一个工具，严格按依赖顺序执行。用户要在网口上创建采集设备且没有合适通道时，优先调用 propose_create_collection_device，将创建通道和设备作为一次可回滚事务，不要拆成两个操作。`
)

type aiProviderSettings struct {
	BaseURL     string `json:"baseUrl"`
	APIKey      string `json:"apiKey"`
	Model       string `json:"model"`
	WorkspaceID string `json:"workspaceId,omitempty"`
}

type aiSettings struct {
	Enabled  bool               `json:"enabled"`
	DeepSeek aiProviderSettings `json:"deepseek"`
	Bailian  aiProviderSettings `json:"bailian"`
}

func defaultAISettings() aiSettings {
	return aiSettings{
		DeepSeek: aiProviderSettings{BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro"},
		Bailian: aiProviderSettings{
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			Model:   "qwen3.5-ocr",
		},
	}
}

func (settings *aiSettings) applyDefaults() {
	defaults := defaultAISettings()
	if strings.TrimSpace(settings.DeepSeek.BaseURL) == "" {
		settings.DeepSeek.BaseURL = defaults.DeepSeek.BaseURL
	}
	if strings.TrimSpace(settings.DeepSeek.Model) == "" {
		settings.DeepSeek.Model = defaults.DeepSeek.Model
	}
	if strings.TrimSpace(settings.Bailian.BaseURL) == "" {
		settings.Bailian.BaseURL = defaults.Bailian.BaseURL
	}
	settings.Bailian.Model = "qwen3.5-ocr"
}

func (settings aiSettings) validate() error {
	for name, provider := range map[string]aiProviderSettings{"DeepSeek": settings.DeepSeek, "百炼": settings.Bailian} {
		parsed, err := url.Parse(strings.TrimSpace(provider.BaseURL))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("%s API 地址必须是有效的 HTTPS 地址", name)
		}
	}
	if settings.Enabled {
		if strings.TrimSpace(settings.DeepSeek.APIKey) == "" {
			return errors.New("启用 AI 对话前必须配置 DeepSeek API Key")
		}
	}
	return nil
}

type aiDraft struct {
	ID             string              `json:"id"`
	BaseRevision   string              `json:"baseRevision"`
	ChannelKey     string              `json:"channelKey"`
	TargetMode     string              `json:"targetMode"`
	TargetDevice   string              `json:"targetDeviceKey,omitempty"`
	Device         config.DeviceConfig `json:"device"`
	Warnings       []string            `json:"warnings,omitempty"`
	BlockingIssues []string            `json:"blockingIssues,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
}

type aiJob struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	Stage        string    `json:"stage"`
	Progress     int       `json:"progress"`
	Message      string    `json:"message,omitempty"`
	Error        string    `json:"error,omitempty"`
	Draft        *aiDraft  `json:"draft,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ChannelKey   string    `json:"-"`
	TargetMode   string    `json:"-"`
	TargetDevice string    `json:"-"`
	FilePath     string    `json:"-"`
	Username     string    `json:"-"`
	RemoteIP     string    `json:"-"`
	cancel       context.CancelFunc
}

type aiManager struct {
	mu           sync.Mutex
	settingsPath string
	tempDir      string
	jobs         map[string]*aiJob
	drafts       map[string]*aiDraft
	actions      map[string]*aiChatAction
	busy         bool
	lastError    string
	httpClient   *http.Client
}

func newAIManager(configPath string) *aiManager {
	baseDir := filepath.Dir(configPath)
	runtimeDir := filepath.Join(baseDir, ".runtime")
	manager := &aiManager{
		settingsPath: filepath.Join(runtimeDir, "ai-settings.json"),
		tempDir:      filepath.Join(runtimeDir, "ai-temp"),
		jobs:         map[string]*aiJob{},
		drafts:       map[string]*aiDraft{},
		actions:      map[string]*aiChatAction{},
		httpClient:   &http.Client{Timeout: aiJobTimeout},
	}
	_ = os.MkdirAll(manager.tempDir, 0700)
	manager.cleanupTempFiles(true)
	return manager
}

func (manager *aiManager) cleanupTempFiles(removeAll bool) {
	entries, _ := os.ReadDir(manager.tempDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil && (removeAll || time.Since(info.ModTime()) > time.Hour) {
			_ = os.Remove(filepath.Join(manager.tempDir, entry.Name()))
		}
	}
}

func (manager *aiManager) start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				manager.mu.Lock()
				for _, job := range manager.jobs {
					if job.cancel != nil {
						job.cancel()
					}
				}
				manager.mu.Unlock()
				return
			case <-ticker.C:
				manager.cleanupTempFiles(false)
				manager.cleanupExpired()
			}
		}
	}()
}

func (manager *aiManager) cleanupExpired() {
	now := time.Now()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for id, job := range manager.jobs {
		if now.After(job.ExpiresAt) {
			if job.cancel != nil {
				job.cancel()
			}
			_ = os.Remove(job.FilePath)
			if job.Draft != nil {
				delete(manager.drafts, job.Draft.ID)
			}
			delete(manager.jobs, id)
		}
	}
	for id, action := range manager.actions {
		if now.After(action.ExpiresAt) {
			delete(manager.actions, id)
		}
	}
}

func (manager *aiManager) loadSettings() (aiSettings, error) {
	settings := defaultAISettings()
	raw, err := os.ReadFile(manager.settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return settings, err
	}
	settings.applyDefaults()
	return settings, nil
}

func (manager *aiManager) saveSettings(settings aiSettings) error {
	settings.applyDefaults()
	if err := settings.validate(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(manager.settingsPath), 0700); err != nil {
		return err
	}
	return atomicWriteFile(manager.settingsPath, append(raw, '\n'), 0600)
}

func maskAISettings(settings aiSettings) aiSettings {
	if settings.DeepSeek.APIKey != "" {
		settings.DeepSeek.APIKey = "***"
	}
	if settings.Bailian.APIKey != "" {
		settings.Bailian.APIKey = "***"
	}
	return settings
}

func (s *Server) aiSettingsFile(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		if !s.hasPermission(request, "ai.use") && !s.hasPermission(request, "ai.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		settings, err := s.ai.loadSettings()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		s.ai.mu.Lock()
		lastError := s.ai.lastError
		s.ai.mu.Unlock()
		writeJSON(writer, map[string]interface{}{
			"settings":  maskAISettings(settings),
			"lastError": lastError,
			"capabilities": map[string]bool{
				"chat": settings.Enabled && strings.TrimSpace(settings.DeepSeek.APIKey) != "",
				"pdf": settings.Enabled &&
					strings.TrimSpace(settings.DeepSeek.APIKey) != "" &&
					strings.TrimSpace(settings.Bailian.APIKey) != "" &&
					strings.TrimSpace(settings.Bailian.WorkspaceID) != "",
			},
		})
	case http.MethodPut:
		if !s.hasPermission(request, "ai.manage") {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		var next aiSettings
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024)).Decode(&next); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		current, _ := s.ai.loadSettings()
		if next.DeepSeek.APIKey == "***" {
			next.DeepSeek.APIKey = current.DeepSeek.APIKey
		}
		if next.Bailian.APIKey == "***" {
			next.Bailian.APIKey = current.Bailian.APIKey
		}
		if err := s.ai.saveSettings(next); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, map[string]bool{"ok": true})
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) testAISettings(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Provider string      `json:"provider"`
		Settings *aiSettings `json:"settings,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024)).Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	current, err := s.ai.loadSettings()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	settings := current
	if body.Settings != nil {
		settings = *body.Settings
		if settings.DeepSeek.APIKey == "***" {
			settings.DeepSeek.APIKey = current.DeepSeek.APIKey
		}
		if settings.Bailian.APIKey == "***" {
			settings.Bailian.APIKey = current.Bailian.APIKey
		}
		settings.applyDefaults()
	}
	provider := settings.DeepSeek
	if body.Provider == "bailian" {
		provider = settings.Bailian
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		http.Error(writer, "API Key 未配置", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 20*time.Second)
	defer cancel()
	endpoint := apiEndpoint(resolvedAIBaseURL(provider), "models")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	response, err := s.ai.httpClient.Do(req)
	if err != nil {
		message := "连接失败: " + err.Error()
		s.ai.setLastError(message)
		http.Error(writer, message, http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		result := upstreamError(response.StatusCode, message)
		s.ai.setLastError(result)
		http.Error(writer, result, http.StatusBadGateway)
		return
	}
	s.ai.setLastError("")
	writeJSON(writer, map[string]interface{}{"ok": true, "provider": body.Provider, "model": provider.Model})
}

type aiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiChatAction struct {
	ID           string                 `json:"id"`
	Tool         string                 `json:"tool"`
	Title        string                 `json:"title"`
	Summary      string                 `json:"summary"`
	Arguments    map[string]interface{} `json:"arguments"`
	Steps        []string               `json:"steps,omitempty"`
	BaseRevision string                 `json:"baseRevision"`
	Username     string                 `json:"-"`
	CreatedAt    time.Time              `json:"createdAt"`
	ExpiresAt    time.Time              `json:"expiresAt"`
}

type aiChatActionApplyRequest struct {
	SelectedKeys []string `json:"selectedKeys,omitempty"`
}

type aiChatQuestion struct {
	Question    string   `json:"question"`
	Reason      string   `json:"reason,omitempty"`
	Options     []string `json:"options"`
	AllowCustom bool     `json:"allowCustom"`
}

type aiChatFormField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Value       string   `json:"value,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type aiChatForm struct {
	Title  string            `json:"title"`
	Reason string            `json:"reason,omitempty"`
	Fields []aiChatFormField `json:"fields"`
}

type aiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type aiCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content   string       `json:"content"`
			ToolCalls []aiToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func (s *Server) aiChat(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	settings, err := s.ai.loadSettings()
	if err != nil || !settings.Enabled || settings.DeepSeek.APIKey == "" {
		http.Error(writer, "AI 助手未启用或 DeepSeek 未配置", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Messages []aiChatMessage `json:"messages"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 512*1024)).Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if len(body.Messages) == 0 || len(body.Messages) > 20 {
		http.Error(writer, "对话消息数量必须在 1 到 20 之间", http.StatusBadRequest)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	compact, pointCounts, totalPoints := compactConfigForUI(cfg)
	contextPayload := map[string]interface{}{
		"config":      compact,
		"pointCounts": pointCounts,
		"totalPoints": totalPoints,
	}
	if s.store != nil {
		snapshot := s.store.Snapshot()
		contextPayload["status"] = map[string]interface{}{
			"gatewayKey": snapshot.GatewayKey, "pointCount": snapshot.PointCount,
			"healthyCount": snapshot.HealthyCount, "errorCount": snapshot.ErrorCount,
			"pendingCount": snapshot.PendingCount, "staleCount": snapshot.StaleCount,
			"lastCollectAt": snapshot.LastCollectAt, "mqttConnected": snapshot.MQTTConnected,
			"errors": snapshot.Errors,
		}
	}
	contextJSON, _ := json.Marshal(contextPayload)
	messages := []aiChatMessage{{
		Role:    "system",
		Content: gatewayAISystemPolicy + "\n当前网关脱敏上下文 JSON:\n" + string(contextJSON),
	}}
	for _, message := range body.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			http.Error(writer, "消息角色无效", http.StatusBadRequest)
			return
		}
		message.Content = strings.TrimSpace(message.Content)
		if message.Content == "" || len(message.Content) > 32*1024 {
			http.Error(writer, "消息内容为空或过长", http.StatusBadRequest)
			return
		}
		messages = append(messages, message)
	}
	payload := map[string]interface{}{
		"model": settings.DeepSeek.Model, "messages": messages, "stream": false,
		"tool_choice": "auto", "tools": gatewayAITools(),
	}
	raw, _ := json.Marshal(payload)
	upstreamRequest, _ := http.NewRequestWithContext(request.Context(), http.MethodPost, apiEndpoint(settings.DeepSeek.BaseURL, "chat/completions"), bytes.NewReader(raw))
	upstreamRequest.Header.Set("Authorization", "Bearer "+settings.DeepSeek.APIKey)
	upstreamRequest.Header.Set("Content-Type", "application/json")
	response, err := s.ai.httpClient.Do(upstreamRequest)
	if err != nil {
		message := "DeepSeek 请求失败: " + err.Error()
		s.ai.setLastError(message)
		http.Error(writer, message, http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		result := upstreamError(response.StatusCode, message)
		s.ai.setLastError(result)
		http.Error(writer, result, http.StatusBadGateway)
		return
	}
	var completion aiCompletionResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&completion); err != nil {
		http.Error(writer, "DeepSeek 返回内容无法解析: "+err.Error(), http.StatusBadGateway)
		return
	}
	if len(completion.Choices) == 0 {
		http.Error(writer, "DeepSeek 未返回可用内容", http.StatusBadGateway)
		return
	}
	message := completion.Choices[0].Message
	var action *aiChatAction
	var question *aiChatQuestion
	var form *aiChatForm
	if len(message.ToolCalls) > 0 {
		var directResult string
		action, question, form, directResult, err = s.prepareAIChatTool(request, message.ToolCalls[0])
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if directResult != "" {
			message.Content = directResult
		}
		if strings.TrimSpace(message.Content) == "" {
			message.Content = "已根据你的要求生成待确认操作。请核对下方参数，确认后网关才会保存并热加载配置。"
		}
	}
	s.ai.setLastError("")
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache, no-store")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	if action != nil {
		rawAction, _ := json.Marshal(map[string]interface{}{"gatewayAction": action})
		_, _ = fmt.Fprintf(writer, "data: %s\n\n", rawAction)
	}
	if question != nil {
		rawQuestion, _ := json.Marshal(map[string]interface{}{"gatewayQuestion": question})
		_, _ = fmt.Fprintf(writer, "data: %s\n\n", rawQuestion)
	}
	if form != nil {
		rawForm, _ := json.Marshal(map[string]interface{}{"gatewayForm": form})
		_, _ = fmt.Fprintf(writer, "data: %s\n\n", rawForm)
	}
	rawContent, _ := json.Marshal(map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{"delta": map[string]string{"content": message.Content}}},
	})
	_, _ = fmt.Fprintf(writer, "data: %s\n\ndata: [DONE]\n\n", rawContent)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func gatewayAITools() []interface{} {
	tool := func(name, description string, properties map[string]interface{}, required []string) interface{} {
		return map[string]interface{}{"type": "function", "function": map[string]interface{}{
			"name": name, "description": description, "strict": true,
			"parameters": map[string]interface{}{
				"type": "object", "properties": properties, "required": required, "additionalProperties": false,
			},
		}}
	}
	return []interface{}{
		tool("request_gateway_input", "当执行网关操作缺少关键参数或存在多个合理选项时，向用户显示可选择或可填写的确认问题。不得自行猜测 IP、端口、从站号、通道、设备、地址范围等关键参数。", map[string]interface{}{
			"question": map[string]interface{}{"type": "string"}, "reason": map[string]interface{}{"type": "string"},
			"options":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "minItems": 2, "maxItems": 4},
			"allowCustom": map[string]interface{}{"type": "boolean"},
		}, []string{"question", "reason", "options", "allowCustom"}),
		tool("request_gateway_form", "当一个网关任务同时缺少两个或更多参数时，一次展示集中参数表单。字段值能从当前配置确定时应预填，不能确定时留空，禁止猜测。", map[string]interface{}{
			"title": map[string]interface{}{"type": "string"}, "reason": map[string]interface{}{"type": "string"},
			"fields": map[string]interface{}{
				"type": "array", "minItems": 2, "maxItems": 10,
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"key": map[string]interface{}{"type": "string"}, "label": map[string]interface{}{"type": "string"},
						"type":     map[string]interface{}{"type": "string", "enum": []string{"text", "number", "select"}},
						"required": map[string]interface{}{"type": "boolean"}, "value": map[string]interface{}{"type": "string"},
						"placeholder": map[string]interface{}{"type": "string"},
						"options":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "maxItems": 20},
					},
					"required": []string{"key", "label", "type", "required", "value", "placeholder", "options"}, "additionalProperties": false,
				},
			},
		}, []string{"title", "reason", "fields"}),
		tool("propose_create_collection_device", "在已有网口资源上原子创建一个采集通道及其首台设备。用户要求创建采集设备且尚无合适通道时优先使用此工具，不要分别调用新增通道和新增设备。", map[string]interface{}{
			"resourceKey": map[string]interface{}{"type": "string"}, "channelName": map[string]interface{}{"type": "string"},
			"protocol":                    map[string]interface{}{"type": "string", "enum": []string{"modbus-tcp", "iec104", "siemens-s7", "opcua", "iec61850"}},
			"collectIntervalMilliseconds": map[string]interface{}{"type": "integer", "minimum": 10, "maximum": 3600000},
			"deviceName":                  map[string]interface{}{"type": "string"}, "address": map[string]interface{}{"type": "string"},
			"slaveId":       map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 255},
			"commonAddress": map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 65535},
			"rack":          map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 255},
			"slot":          map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 255},
		}, []string{"resourceKey", "channelName", "protocol", "collectIntervalMilliseconds", "deviceName", "address", "slaveId", "commonAddress", "rack", "slot"}),
		tool("propose_add_channel", "在已有网口资源下新增采集或转发通道，生成待确认操作。", map[string]interface{}{
			"resourceKey": map[string]interface{}{"type": "string", "description": "目标网口资源标识，必须来自当前配置"},
			"name":        map[string]interface{}{"type": "string"}, "role": map[string]interface{}{"type": "string", "enum": []string{"collect", "forward"}},
			"protocol":                    map[string]interface{}{"type": "string", "enum": []string{"modbus-tcp", "iec104", "siemens-s7", "opcua", "iec61850", "iec61850-goose", "modbus-tcp-slave", "iec104-server", "iec61850-mms-server", "iec61850-goose-publisher"}},
			"collectIntervalMilliseconds": map[string]interface{}{"type": "integer", "minimum": 10, "maximum": 3600000},
		}, []string{"resourceKey", "name", "role", "protocol", "collectIntervalMilliseconds"}),
		tool("propose_add_device", "在已有采集通道下新增设备，生成待确认操作。串口设备的地址由通道决定。", map[string]interface{}{
			"channelKey": map[string]interface{}{"type": "string"}, "name": map[string]interface{}{"type": "string"},
			"address":       map[string]interface{}{"type": "string", "description": "TCP/OPC UA/S7/IEC61850 地址，串口设备传空字符串"},
			"slaveId":       map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 255},
			"commonAddress": map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 65535},
			"rack":          map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 255},
			"slot":          map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 255},
		}, []string{"channelKey", "name", "address", "slaveId", "commonAddress", "rack", "slot"}),
		tool("propose_update_channel", "修改已有通道名称、采集周期、启用状态或 IEC104 采集模式，生成待确认操作。", map[string]interface{}{
			"channelKey": map[string]interface{}{"type": "string"}, "name": map[string]interface{}{"type": "string"},
			"collectIntervalMilliseconds": map[string]interface{}{"type": "integer", "minimum": 10, "maximum": 3600000},
			"enabled":                     map[string]interface{}{"type": "boolean"},
			"acquisitionMode":             map[string]interface{}{"type": "string", "enum": []string{"auto", "periodic-general-interrogation", ""}},
		}, []string{"channelKey", "name", "collectIntervalMilliseconds", "enabled", "acquisitionMode"}),
		tool("propose_set_device_enabled", "启用或停用已有采集/转发设备，生成待确认操作。", map[string]interface{}{
			"deviceKey": map[string]interface{}{"type": "string"}, "enabled": map[string]interface{}{"type": "boolean"},
		}, []string{"deviceKey", "enabled"}),
		tool("browse_and_add_points", "连接已有设备，浏览或扫描可用点位并生成可多选的待添加点位。支持 OPC UA、IEC61850 和 S7。", map[string]interface{}{
			"deviceKey": map[string]interface{}{"type": "string"}, "search": map[string]interface{}{"type": "string"},
			"parentRef": map[string]interface{}{"type": "string", "description": "OPC UA NodeId 或 IEC61850 父对象引用；空字符串从根开始"},
			"recursive": map[string]interface{}{"type": "boolean"},
			"area":      map[string]interface{}{"type": "string", "enum": []string{"", "AUTO", "DB", "V", "M", "I", "Q"}},
			"dbNumber":  map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 65535},
			"start":     map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 65535},
			"end":       map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 65535},
			"dataType":  map[string]interface{}{"type": "string"},
		}, []string{"deviceKey", "search", "parentRef", "recursive", "area", "dbNumber", "start", "end", "dataType"}),
		tool("diagnose_connection", "立即对已有设备执行只读连接诊断和协议握手，不修改配置。", map[string]interface{}{
			"deviceKey": map[string]interface{}{"type": "string"},
		}, []string{"deviceKey"}),
	}
}

func (s *Server) prepareAIChatTool(request *http.Request, call aiToolCall) (*aiChatAction, *aiChatQuestion, *aiChatForm, string, error) {
	var arguments map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil {
		return nil, nil, nil, "", errors.New("模型生成的操作参数无效，请换一种方式描述")
	}
	if call.Function.Name == "request_gateway_input" {
		rawOptions, _ := arguments["options"].([]interface{})
		options := make([]string, 0, len(rawOptions))
		for _, raw := range rawOptions {
			if value := strings.TrimSpace(fmt.Sprint(raw)); value != "" {
				options = append(options, value)
			}
		}
		question := strings.TrimSpace(fmt.Sprint(arguments["question"]))
		if question == "" || len(options) < 2 {
			return nil, nil, nil, "", errors.New("模型生成的确认问题不完整")
		}
		return nil, &aiChatQuestion{
			Question: question, Reason: strings.TrimSpace(fmt.Sprint(arguments["reason"])),
			Options: options, AllowCustom: asBool(arguments["allowCustom"]),
		}, nil, "", nil
	}
	if call.Function.Name == "request_gateway_form" {
		rawFields, _ := arguments["fields"].([]interface{})
		fields := make([]aiChatFormField, 0, len(rawFields))
		for _, raw := range rawFields {
			item, _ := raw.(map[string]interface{})
			field := aiChatFormField{
				Key: strings.TrimSpace(fmt.Sprint(item["key"])), Label: strings.TrimSpace(fmt.Sprint(item["label"])),
				Type: strings.TrimSpace(fmt.Sprint(item["type"])), Required: asBool(item["required"]),
				Value: strings.TrimSpace(fmt.Sprint(item["value"])), Placeholder: strings.TrimSpace(fmt.Sprint(item["placeholder"])),
			}
			for _, option := range asInterfaceSlice(item["options"]) {
				if value := strings.TrimSpace(fmt.Sprint(option)); value != "" {
					field.Options = append(field.Options, value)
				}
			}
			if field.Key != "" && field.Label != "" && (field.Type == "text" || field.Type == "number" || field.Type == "select") {
				fields = append(fields, field)
			}
		}
		title := strings.TrimSpace(fmt.Sprint(arguments["title"]))
		if title == "" || len(fields) < 2 {
			return nil, nil, nil, "", errors.New("模型生成的参数表单不完整")
		}
		return nil, nil, &aiChatForm{Title: title, Reason: strings.TrimSpace(fmt.Sprint(arguments["reason"])), Fields: fields}, "", nil
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return nil, nil, nil, "", err
	}
	if call.Function.Name == "diagnose_connection" {
		result, err := s.diagnoseAIDeviceConnection(request.Context(), cfg, strings.TrimSpace(fmt.Sprint(arguments["deviceKey"])))
		return nil, nil, nil, result, err
	}
	title, summary := "", ""
	var steps []string
	switch call.Function.Name {
	case "propose_create_collection_device":
		resource, ok := findAIResource(cfg, strings.TrimSpace(fmt.Sprint(arguments["resourceKey"])))
		if !ok || resource.Type != "network" || !resource.Enabled {
			return nil, nil, nil, "", errors.New("请选择当前网关中已启用的网口资源")
		}
		protocol := strings.TrimSpace(fmt.Sprint(arguments["protocol"]))
		if err := validateAIChannelArguments("collect", protocol); err != nil {
			return nil, nil, nil, "", err
		}
		channelName := strings.TrimSpace(fmt.Sprint(arguments["channelName"]))
		deviceName := strings.TrimSpace(fmt.Sprint(arguments["deviceName"]))
		address := strings.TrimSpace(fmt.Sprint(arguments["address"]))
		if channelName == "" || deviceName == "" || address == "" {
			return nil, nil, nil, "", errors.New("通道名称、设备名称和设备地址不能为空")
		}
		arguments["channelName"], arguments["deviceName"], arguments["address"] = channelName, deviceName, address
		title = "创建采集通道和设备"
		summary = fmt.Sprintf("在 %s（%s）上创建“%s”，协议：%s，并新增设备“%s”（%s）", resource.Name, resource.ResourceKey, channelName, protocol, deviceName, address)
		steps = []string{
			fmt.Sprintf("创建 %s 采集通道“%s”", protocol, channelName),
			fmt.Sprintf("在新通道下创建设备“%s”", deviceName),
			"执行完整配置校验和标识符冲突检查",
			"保存配置并热加载；任一步失败时整体回滚",
		}
	case "propose_add_channel":
		resource, ok := findAIResource(cfg, strings.TrimSpace(fmt.Sprint(arguments["resourceKey"])))
		if !ok || resource.Type != "network" {
			return nil, nil, nil, "", errors.New("请选择当前网关中有效的网口资源")
		}
		role, protocol := strings.TrimSpace(fmt.Sprint(arguments["role"])), strings.TrimSpace(fmt.Sprint(arguments["protocol"]))
		if err := validateAIChannelArguments(role, protocol); err != nil {
			return nil, nil, nil, "", err
		}
		name := strings.TrimSpace(fmt.Sprint(arguments["name"]))
		if name == "" {
			return nil, nil, nil, "", errors.New("通道名称不能为空")
		}
		arguments["name"] = name
		title = "新增通道"
		summary = fmt.Sprintf("在 %s（%s）下新增“%s”，用途：%s，协议：%s，周期：%v ms", resource.Name, resource.ResourceKey, name, aiRoleLabel(role), protocol, arguments["collectIntervalMilliseconds"])
	case "propose_add_device":
		channel, ok := findAIChannel(cfg, strings.TrimSpace(fmt.Sprint(arguments["channelKey"])))
		if !ok || channel.Role == "forward" {
			return nil, nil, nil, "", errors.New("请选择当前网关中有效的采集通道")
		}
		name := strings.TrimSpace(fmt.Sprint(arguments["name"]))
		if name == "" {
			return nil, nil, nil, "", errors.New("设备名称不能为空")
		}
		arguments["name"] = name
		title = "新增设备"
		summary = fmt.Sprintf("在通道 %s（%s）下新增“%s”，协议：%s，地址：%s", channel.Name, channel.ChannelKey, name, channel.Protocol, valueOrDefault(strings.TrimSpace(fmt.Sprint(arguments["address"])), "由通道决定"))
	case "propose_update_channel":
		channel, ok := findAIAnyChannel(cfg, strings.TrimSpace(fmt.Sprint(arguments["channelKey"])))
		if !ok {
			return nil, nil, nil, "", errors.New("目标通道不存在")
		}
		title = "修改通道参数"
		summary = fmt.Sprintf("修改通道 %s（%s）：名称=%s，周期=%v ms，启用=%v", channel.Name, channel.ChannelKey, arguments["name"], arguments["collectIntervalMilliseconds"], arguments["enabled"])
	case "propose_set_device_enabled":
		deviceName, ok := findAIDeviceName(cfg, strings.TrimSpace(fmt.Sprint(arguments["deviceKey"])))
		if !ok {
			return nil, nil, nil, "", errors.New("目标设备不存在")
		}
		title = map[bool]string{true: "启用设备", false: "停用设备"}[asBool(arguments["enabled"])]
		summary = fmt.Sprintf("%s“%s”（%s）", title, deviceName, arguments["deviceKey"])
	case "browse_and_add_points":
		device, ok := findAIDevice(cfg, strings.TrimSpace(fmt.Sprint(arguments["deviceKey"])))
		if !ok {
			return nil, nil, nil, "", errors.New("目标采集设备不存在")
		}
		candidates, browseErr := browseAIPointCandidates(request.Context(), device, arguments)
		if browseErr != nil {
			return nil, nil, nil, "", browseErr
		}
		if len(candidates) == 0 {
			return nil, nil, nil, "", errors.New("没有浏览到符合条件的可添加点位")
		}
		arguments["candidatePoints"] = candidates
		title = "浏览并添加点位"
		summary = fmt.Sprintf("已从“%s”浏览到 %d 个候选点位，请勾选后确认添加", device.Name, len(candidates))
	default:
		return nil, nil, nil, "", fmt.Errorf("模型请求了不受支持的网关操作 %s", call.Function.Name)
	}
	revision, err := configFileRevision(s.configPath)
	if err != nil {
		return nil, nil, nil, "", err
	}
	user, _ := s.currentUser(request)
	now := time.Now()
	action := &aiChatAction{
		ID: newAIID("action"), Tool: call.Function.Name, Title: title, Summary: summary, Arguments: arguments,
		Steps: steps, BaseRevision: revision, Username: user.Username, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	s.ai.mu.Lock()
	s.ai.actions[action.ID] = action
	s.ai.mu.Unlock()
	return action, nil, nil, "", nil
}

func validateAIChannelArguments(role, protocol string) error {
	collect := map[string]bool{"modbus-tcp": true, "iec104": true, "siemens-s7": true, "opcua": true, "iec61850": true}
	forward := map[string]bool{"modbus-tcp-slave": true, "iec104-server": true, "iec61850-mms-server": true, "iec61850-goose-publisher": true}
	if role == "collect" && collect[protocol] || role == "forward" && forward[protocol] {
		return nil
	}
	return fmt.Errorf("通道用途 %s 与协议 %s 不匹配", role, protocol)
}

func findAIResource(cfg config.Config, resourceKey string) (config.ResourceConfig, bool) {
	for _, resource := range cfg.Resources {
		if resource.ResourceKey == resourceKey {
			return resource, true
		}
	}
	return config.ResourceConfig{}, false
}

func aiRoleLabel(role string) string {
	if role == "forward" {
		return "转发"
	}
	return "采集"
}

func asBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		result, _ := strconv.ParseBool(typed)
		return result
	default:
		return false
	}
}

func findAIDeviceName(cfg config.Config, key string) (string, bool) {
	if device, ok := findAIDevice(cfg, key); ok {
		return device.Name, true
	}
	for _, device := range cfg.ForwardDevices {
		if device.DeviceKey == key {
			return device.Name, true
		}
	}
	return "", false
}

func (s *Server) diagnoseAIDeviceConnection(ctx context.Context, cfg config.Config, deviceKey string) (string, error) {
	device, ok := findAIDevice(cfg, deviceKey)
	if !ok {
		return "", errors.New("目标采集设备不存在")
	}
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	prefix := fmt.Sprintf("设备：%s（%s）\n协议：%s\n地址：%s\n", device.Name, device.DeviceKey, device.Protocol, device.Address)
	switch device.Protocol {
	case "siemens-s7":
		result, err := collector.TestS7Connection(testCtx, config.PointConfig{
			Protocol: device.Protocol, Address: device.Address, Rack: device.Rack, Slot: device.Slot,
			LocalTSAP: device.LocalTSAP, RemoteTSAP: device.RemoteTSAP,
		})
		if err != nil {
			return prefix + "诊断结果：连接失败\n原因：" + err.Error(), nil
		}
		return prefix + fmt.Sprintf("诊断结果：连接正常，已完成 TCP/COTP/S7 握手\nEndpoint：%s\nLocalTSAP：%s\nRemoteTSAP：%s", result.Endpoint, result.LocalTSAP, result.RemoteTSAP), nil
	case "iec104":
		result, err := collector.TestIEC104Connection(testCtx, device.Address, device.CommonAddress)
		if err != nil {
			return prefix + "诊断结果：连接失败\n原因：" + err.Error(), nil
		}
		return prefix + fmt.Sprintf("诊断结果：连接正常，已完成 IEC104 STARTDT 握手\n公共地址：%d", result.CommonAddress), nil
	case "opcua":
		result, err := collector.TestOPCUAConnection(testCtx, config.PointConfig{
			Protocol: device.Protocol, Address: device.Address, Username: device.Username, Password: device.Password,
		})
		if err != nil {
			return prefix + "诊断结果：连接失败\n原因：" + err.Error(), nil
		}
		return prefix + "诊断结果：连接正常，已完成 OPC UA Endpoint 握手\nEndpoint：" + result.Endpoint, nil
	case "iec61850":
		result, err := collector.TestIEC61850Connection(testCtx, device.Address)
		if err != nil {
			return prefix + "诊断结果：连接失败\n原因：" + err.Error(), nil
		}
		return prefix + "诊断结果：连接正常，IEC61850 MMS 端口可达\nEndpoint：" + result.Address, nil
	case "modbus-rtu":
		return prefix + "诊断结果：串口设备不单独抢占端口测试，以免中断当前采集。请结合实时点位和报文监控判断；当前设备包含 " + strconv.Itoa(len(device.Points)) + " 个点位。", nil
	default:
		address := normalizeConnectionAddress(device.Protocol, device.Address)
		conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(testCtx, "tcp", address)
		if err != nil {
			return prefix + "诊断结果：TCP 连接失败\n原因：" + err.Error(), nil
		}
		_ = conn.Close()
		return prefix + "诊断结果：TCP 端口可达\nEndpoint：" + address, nil
	}
}

func browseAIPointCandidates(ctx context.Context, device config.DeviceConfig, arguments map[string]interface{}) ([]config.PointConfig, error) {
	search := strings.ToLower(strings.TrimSpace(fmt.Sprint(arguments["search"])))
	parentRef := strings.TrimSpace(fmt.Sprint(arguments["parentRef"]))
	recursive := asBool(arguments["recursive"])
	var points []config.PointConfig
	switch device.Protocol {
	case "opcua":
		if parentRef == "" {
			parentRef = "i=84"
		}
		browseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		nodes, err := collector.BrowseOPCUAVariableNodes(browseCtx, config.PointConfig{
			Protocol: "opcua", Address: device.Address, Username: device.Username, Password: device.Password,
		}, parentRef, 500)
		if !recursive {
			nodes, err = collector.BrowseOPCUANodes(browseCtx, config.PointConfig{
				Protocol: "opcua", Address: device.Address, Username: device.Username, Password: device.Password,
			}, parentRef)
		}
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			if !node.Selectable || search != "" && !strings.Contains(strings.ToLower(node.DisplayName+" "+node.BrowseName+" "+node.NodeID), search) {
				continue
			}
			points = append(points, aiCandidatePoint(device, valueOrDefault(node.DisplayName, node.BrowseName), node.NodeID, "", "", mapOPCUADataType(node.DataType)))
		}
	case "iec61850":
		browseCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		nodes, err := collector.BrowseIEC61850Nodes(browseCtx, device.Address, parentRef, recursive)
		if err != nil {
			return nil, err
		}
		var walk func([]collector.IEC61850BrowseNode)
		walk = func(items []collector.IEC61850BrowseNode) {
			for _, node := range items {
				if node.Leaf && (search == "" || strings.Contains(strings.ToLower(node.Name+" "+node.ObjectRef+" "+node.FC), search)) {
					points = append(points, aiCandidatePoint(device, node.Name, "", node.ObjectRef, node.FC, valueOrDefault(node.DataType, "auto")))
				}
				walk(node.Children)
			}
		}
		walk(nodes)
	case "siemens-s7":
		body := s7ScanRequest{
			DeviceKey: device.DeviceKey, DeviceName: device.Name, PLCModel: device.PLCModel, Address: device.Address,
			Area: strings.TrimSpace(fmt.Sprint(arguments["area"])), DBNumber: uint16(asInt(arguments["dbNumber"])),
			Rack: device.Rack, Slot: device.Slot, LocalTSAP: device.LocalTSAP, RemoteTSAP: device.RemoteTSAP,
			Start: uint16(asInt(arguments["start"])), End: uint16(asInt(arguments["end"])), DataType: strings.TrimSpace(fmt.Sprint(arguments["dataType"])),
		}
		items, _ := scanS7PointRange(ctx, body)
		for _, item := range items {
			if search == "" || strings.Contains(strings.ToLower(item.Name+" "+item.Metric), search) {
				points = append(points, item.PointConfig)
			}
		}
	default:
		return nil, errors.New("当前协议不支持自动浏览点位；Modbus 和 IEC104 请在对话中明确寄存器或 IOA 后生成点位")
	}
	if len(points) > 500 {
		points = points[:500]
	}
	used := map[string]bool{}
	for index := range points {
		base := sanitizeAIMetric(points[index].Metric)
		if base == "" {
			base = fmt.Sprintf("point_%03d", index+1)
		}
		metric := base
		for suffix := 2; used[strings.ToLower(metric)]; suffix++ {
			metric = fmt.Sprintf("%s_%d", base, suffix)
		}
		used[strings.ToLower(metric)] = true
		points[index].Metric = metric
		points[index].Name = device.Name + "@" + metric
		points[index].ApplyDefaults()
	}
	return points, nil
}

func aiCandidatePoint(device config.DeviceConfig, name, nodeID, objectRef, fc, dataType string) config.PointConfig {
	metricSource := name
	if objectRef != "" {
		parts := strings.Split(strings.TrimSpace(strings.SplitN(objectRef, "/", 2)[len(strings.SplitN(objectRef, "/", 2))-1]), ".")
		if len(parts) > 1 {
			metricSource = parts[1]
		} else if len(parts) == 1 {
			metricSource = parts[0]
		}
	}
	return config.PointConfig{
		DeviceKey: device.DeviceKey, ChannelKey: device.ChannelKey, Name: name, Metric: sanitizeAIMetric(metricSource),
		Protocol: device.Protocol, Address: device.Address, SlaveID: device.SlaveID, CommonAddress: device.CommonAddress,
		NodeID: nodeID, ObjectRef: objectRef, FC: fc, Quantity: 1, DataType: valueOrDefault(dataType, "auto"),
		ByteOrder: "big", WordOrder: "big", Scale: 1,
	}
}

func mapOPCUADataType(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "boolean"):
		return "bool"
	case strings.Contains(lower, "float"):
		return "float32"
	case strings.Contains(lower, "double"):
		return "float64"
	case strings.Contains(lower, "uint16"):
		return "uint16"
	case strings.Contains(lower, "int16"):
		return "int16"
	case strings.Contains(lower, "uint32"):
		return "uint32"
	case strings.Contains(lower, "int32"):
		return "int32"
	default:
		return "auto"
	}
}

func asInt(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		result, _ := strconv.Atoi(fmt.Sprint(value))
		return result
	}
}

func asInterfaceSlice(value interface{}) []interface{} {
	items, _ := value.([]interface{})
	return items
}

func (s *Server) applyAIChatAction(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.hasPermission(request, "config.manage") {
		http.Error(writer, "当前账号没有配置管理权限", http.StatusForbidden)
		return
	}
	var applyRequest aiChatActionApplyRequest
	if request.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(writer, request.Body, 256*1024)).Decode(&applyRequest)
	}
	actionID := strings.TrimSpace(strings.TrimPrefix(request.URL.Path, "/api/ai/chat-actions/"))
	s.ai.mu.Lock()
	action := s.ai.actions[actionID]
	s.ai.mu.Unlock()
	if action == nil || time.Now().After(action.ExpiresAt) {
		http.Error(writer, "待确认操作不存在或已过期，请重新发起", http.StatusNotFound)
		return
	}
	user, ok := s.currentUser(request)
	if !ok || user.Username != action.Username {
		http.Error(writer, "不能执行其他会话生成的操作", http.StatusForbidden)
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	revision, err := configFileRevision(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if revision != action.BaseRevision {
		http.Error(writer, "配置已被其他会话修改，请重新生成操作", http.StatusConflict)
		return
	}
	current, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	next := current
	switch action.Tool {
	case "propose_create_collection_device":
		if err := applyAICreateCollectionDevice(&next, action.Arguments); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	case "propose_add_channel":
		if err := applyAIAddChannel(&next, action.Arguments); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	case "propose_add_device":
		if err := applyAIAddDevice(&next, action.Arguments); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	case "propose_update_channel":
		if err := applyAIUpdateChannel(&next, action.Arguments); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	case "propose_set_device_enabled":
		if err := applyAISetDeviceEnabled(&next, action.Arguments); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	case "browse_and_add_points":
		if err := applyAIBrowsedPoints(&next, action.Arguments, applyRequest.SelectedKeys); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(writer, "不受支持的网关操作", http.StatusBadRequest)
		return
	}
	next.ApplyDefaults()
	if err := next.ValidateNewGlobalIdentifierDuplicates(current); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := edgecompute.Validate(next); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.Save(s.configPath, next); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.applyConfig(next); err != nil {
		_ = config.Save(s.configPath, current)
		_ = s.applyConfig(current)
		http.Error(writer, "操作应用失败，已恢复原配置: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.ai.mu.Lock()
	delete(s.ai.actions, action.ID)
	s.ai.mu.Unlock()
	s.appendAudit(auditEvent{
		Username: user.Username, RemoteIP: requestRemoteIP(request), Method: request.Method,
		Path: request.URL.Path, Status: http.StatusOK, Result: "success", Detail: action.Summary,
	})
	writeJSON(writer, map[string]interface{}{"ok": true, "message": action.Title + "成功", "summary": action.Summary})
}

func applyAIAddChannel(cfg *config.Config, arguments map[string]interface{}) error {
	resourceKey := strings.TrimSpace(fmt.Sprint(arguments["resourceKey"]))
	name := strings.TrimSpace(fmt.Sprint(arguments["name"]))
	role := strings.TrimSpace(fmt.Sprint(arguments["role"]))
	protocol := strings.TrimSpace(fmt.Sprint(arguments["protocol"]))
	interval, _ := strconv.Atoi(fmt.Sprint(arguments["collectIntervalMilliseconds"]))
	resource, ok := findAIResource(*cfg, resourceKey)
	if !ok || resource.Type != "network" {
		return errors.New("目标网口资源不存在")
	}
	if name == "" {
		return errors.New("通道名称不能为空")
	}
	if err := validateAIChannelArguments(role, protocol); err != nil {
		return err
	}
	if interval < 10 {
		interval = 1000
	}
	channelKey := nextAIChannelKey(cfg.Channels)
	channel := config.ChannelConfig{
		ChannelKey: channelKey, ResourceKey: resourceKey, Name: name, Type: "network",
		Role: role, CollectIntervalMilliseconds: interval, InterfaceName: resource.Name, Enabled: true,
	}
	if role == "forward" {
		channel.Protocol = "none"
		channel.ForwardProtocol = protocol
	} else {
		channel.Protocol = protocol
		if protocol == "iec104" {
			channel.IEC104 = config.IEC104Config{
				Configured: true, AcquisitionMode: "auto", GeneralInterrogationOnStart: true,
				ClockSyncOnStart: true, ClockSyncIntervalSeconds: 3600,
			}
		}
	}
	cfg.Channels = append(cfg.Channels, channel)
	return nil
}

func applyAICreateCollectionDevice(cfg *config.Config, arguments map[string]interface{}) error {
	channelKey := nextAIChannelKey(cfg.Channels)
	channelArguments := map[string]interface{}{
		"resourceKey": arguments["resourceKey"], "name": arguments["channelName"], "role": "collect",
		"protocol": arguments["protocol"], "collectIntervalMilliseconds": arguments["collectIntervalMilliseconds"],
	}
	if err := applyAIAddChannel(cfg, channelArguments); err != nil {
		return err
	}
	deviceArguments := map[string]interface{}{
		"channelKey": channelKey, "name": arguments["deviceName"], "address": arguments["address"],
		"slaveId": arguments["slaveId"], "commonAddress": arguments["commonAddress"],
		"rack": arguments["rack"], "slot": arguments["slot"],
	}
	if err := applyAIAddDevice(cfg, deviceArguments); err != nil {
		return err
	}
	return nil
}

func applyAIAddDevice(cfg *config.Config, arguments map[string]interface{}) error {
	channelKey := strings.TrimSpace(fmt.Sprint(arguments["channelKey"]))
	channel, ok := findAIChannel(*cfg, channelKey)
	if !ok || channel.Role == "forward" {
		return errors.New("目标采集通道不存在")
	}
	name := strings.TrimSpace(fmt.Sprint(arguments["name"]))
	if name == "" {
		return errors.New("设备名称不能为空")
	}
	deviceKey := nextAIDeviceKey(*cfg)
	device := config.DeviceConfig{
		DeviceKey: deviceKey, ChannelKey: channelKey, Name: name, Protocol: channel.Protocol,
		Address: strings.TrimSpace(fmt.Sprint(arguments["address"])), SlaveID: byte(asInt(arguments["slaveId"])),
		CommonAddress: uint16(asInt(arguments["commonAddress"])), Rack: uint8(asInt(arguments["rack"])),
		Slot: uint8(asInt(arguments["slot"])), Points: []config.PointConfig{},
	}
	if channel.Type == "serial" && channel.Serial != nil {
		device.Address = channel.Serial.Port
	}
	device.ApplyDefaults()
	cfg.Devices = append(cfg.Devices, device)
	return nil
}

func applyAIUpdateChannel(cfg *config.Config, arguments map[string]interface{}) error {
	key := strings.TrimSpace(fmt.Sprint(arguments["channelKey"]))
	for index := range cfg.Channels {
		channel := &cfg.Channels[index]
		if channel.ChannelKey != key {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(arguments["name"]))
		if name != "" {
			channel.Name = name
		}
		interval := asInt(arguments["collectIntervalMilliseconds"])
		if interval >= 10 {
			channel.CollectIntervalMilliseconds = interval
			channel.CollectIntervalSeconds = 0
		}
		channel.Enabled = asBool(arguments["enabled"])
		mode := strings.TrimSpace(fmt.Sprint(arguments["acquisitionMode"]))
		if channel.Protocol == "iec104" && mode != "" {
			if mode != "auto" && mode != "periodic-general-interrogation" {
				return errors.New("IEC104 采集模式无效")
			}
			channel.IEC104.Configured = true
			channel.IEC104.AcquisitionMode = mode
		}
		return nil
	}
	return errors.New("目标通道不存在")
}

func applyAISetDeviceEnabled(cfg *config.Config, arguments map[string]interface{}) error {
	key := strings.TrimSpace(fmt.Sprint(arguments["deviceKey"]))
	enabled := asBool(arguments["enabled"])
	for index := range cfg.Devices {
		if cfg.Devices[index].DeviceKey == key {
			cfg.Devices[index].Enabled = &enabled
			return nil
		}
	}
	for index := range cfg.ForwardDevices {
		if cfg.ForwardDevices[index].DeviceKey == key {
			cfg.ForwardDevices[index].Enabled = &enabled
			return nil
		}
	}
	return errors.New("目标设备不存在")
}

func applyAIBrowsedPoints(cfg *config.Config, arguments map[string]interface{}, selectedKeys []string) error {
	deviceKey := strings.TrimSpace(fmt.Sprint(arguments["deviceKey"]))
	raw, err := json.Marshal(arguments["candidatePoints"])
	if err != nil {
		return errors.New("候选点位内容无效")
	}
	var candidates []config.PointConfig
	if err := json.Unmarshal(raw, &candidates); err != nil {
		return errors.New("候选点位内容无效")
	}
	selected := make(map[string]bool, len(selectedKeys))
	for _, key := range selectedKeys {
		selected[key] = true
	}
	if len(selected) == 0 {
		return errors.New("请至少选择一个候选点位")
	}
	for index := range cfg.Devices {
		device := &cfg.Devices[index]
		if device.DeviceKey != deviceKey {
			continue
		}
		existing := make(map[string]bool, len(device.Points))
		for _, point := range device.Points {
			existing[point.Metric] = true
		}
		added := 0
		for _, point := range candidates {
			if !selected[point.Metric] || existing[point.Metric] {
				continue
			}
			point.DeviceKey, point.ChannelKey, point.Protocol, point.Address = device.DeviceKey, device.ChannelKey, device.Protocol, device.Address
			point.Name = device.Name + "@" + point.Metric
			point.ApplyDefaults()
			device.Points = append(device.Points, point)
			existing[point.Metric] = true
			added++
		}
		if added == 0 {
			return errors.New("所选点位均已存在或选择内容无效")
		}
		return nil
	}
	return errors.New("目标设备不存在")
}

func nextAIChannelKey(channels []config.ChannelConfig) string {
	used := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		used[channel.ChannelKey] = struct{}{}
	}
	for index := 1; ; index++ {
		key := fmt.Sprintf("channel-%03d", index)
		if _, exists := used[key]; !exists {
			return key
		}
	}
}

func (s *Server) aiConfigJobs(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	settings, err := s.ai.loadSettings()
	if err != nil || !settings.Enabled {
		http.Error(writer, "AI 助手未启用", http.StatusServiceUnavailable)
		return
	}
	if strings.TrimSpace(settings.DeepSeek.APIKey) == "" {
		http.Error(writer, "PDF 配置生成需要先配置 DeepSeek API Key", http.StatusServiceUnavailable)
		return
	}
	if strings.TrimSpace(settings.Bailian.APIKey) == "" || strings.TrimSpace(settings.Bailian.WorkspaceID) == "" {
		http.Error(writer, "PDF 配置生成需要配置百炼 API Key 和 Workspace ID", http.StatusServiceUnavailable)
		return
	}
	s.ai.mu.Lock()
	if s.ai.busy {
		s.ai.mu.Unlock()
		http.Error(writer, "当前已有 PDF 配置任务正在运行", http.StatusTooManyRequests)
		return
	}
	s.ai.busy = true
	s.ai.mu.Unlock()
	filePath, fields, err := s.receiveAIPDF(writer, request)
	if err != nil {
		s.ai.mu.Lock()
		s.ai.busy = false
		s.ai.mu.Unlock()
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		_ = os.Remove(filePath)
		s.ai.mu.Lock()
		s.ai.busy = false
		s.ai.mu.Unlock()
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	channelKey := strings.TrimSpace(fields["channelKey"])
	if _, ok := findAIChannel(cfg, channelKey); !ok {
		_ = os.Remove(filePath)
		s.ai.mu.Lock()
		s.ai.busy = false
		s.ai.mu.Unlock()
		http.Error(writer, "请选择有效的采集通道", http.StatusBadRequest)
		return
	}
	targetMode := valueOrDefault(strings.TrimSpace(fields["targetMode"]), "create")
	if targetMode != "create" && targetMode != "update" {
		_ = os.Remove(filePath)
		s.ai.mu.Lock()
		s.ai.busy = false
		s.ai.mu.Unlock()
		http.Error(writer, "目标模式无效", http.StatusBadRequest)
		return
	}
	targetDevice := strings.TrimSpace(fields["targetDeviceKey"])
	if targetMode == "update" {
		device, ok := findAIDevice(cfg, targetDevice)
		if !ok || device.ChannelKey != channelKey {
			_ = os.Remove(filePath)
			s.ai.mu.Lock()
			s.ai.busy = false
			s.ai.mu.Unlock()
			http.Error(writer, "请选择当前通道下的有效设备", http.StatusBadRequest)
			return
		}
	}
	revision, _ := configFileRevision(s.configPath)
	jobID := newAIID("job")
	now := time.Now()
	user, _ := s.currentUser(request)
	job := &aiJob{
		ID: jobID, Status: "queued", Stage: "uploaded", Progress: 5,
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(aiJobTTL),
		ChannelKey: channelKey, TargetMode: targetMode, TargetDevice: targetDevice,
		FilePath: filePath, Username: user.Username, RemoteIP: requestRemoteIP(request),
	}
	ctx, cancel := context.WithTimeout(context.Background(), aiJobTimeout)
	job.cancel = cancel
	s.ai.mu.Lock()
	s.ai.jobs[job.ID] = job
	s.ai.mu.Unlock()
	accepted := *job
	accepted.cancel = nil
	go s.runAIConfigJob(ctx, job, settings, revision)
	writer.WriteHeader(http.StatusAccepted)
	writeJSON(writer, accepted)
}

func (s *Server) receiveAIPDF(writer http.ResponseWriter, request *http.Request) (string, map[string]string, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxAIPDFBytes+1024*1024)
	reader, err := request.MultipartReader()
	if err != nil {
		return "", nil, errors.New("请使用 multipart/form-data 上传 PDF")
	}
	fields := map[string]string{}
	var filePath string
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if filePath != "" {
				_ = os.Remove(filePath)
			}
			return "", nil, err
		}
		if part.FileName() == "" {
			raw, _ := io.ReadAll(io.LimitReader(part, 64*1024))
			fields[part.FormName()] = string(raw)
			_ = part.Close()
			continue
		}
		if part.FormName() != "file" || filePath != "" {
			_ = part.Close()
			continue
		}
		filePath = filepath.Join(s.ai.tempDir, newAIID("upload")+".pdf")
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", nil, err
		}
		written, copyErr := io.Copy(file, io.LimitReader(part, maxAIPDFBytes+1))
		closeErr := file.Close()
		_ = part.Close()
		if copyErr != nil || closeErr != nil || written > maxAIPDFBytes {
			_ = os.Remove(filePath)
			return "", nil, errors.New("PDF 上传失败或超过 25 MB")
		}
	}
	if filePath == "" {
		return "", nil, errors.New("请选择 PDF 文件")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", nil, err
	}
	header := make([]byte, 5)
	_, err = io.ReadFull(file, header)
	_ = file.Close()
	if err != nil || string(header) != "%PDF-" {
		_ = os.Remove(filePath)
		return "", nil, errors.New("文件不是有效的 PDF")
	}
	return filePath, fields, nil
}

func (s *Server) runAIConfigJob(ctx context.Context, job *aiJob, settings aiSettings, revision string) {
	defer func() {
		_ = os.Remove(job.FilePath)
		s.ai.mu.Lock()
		s.ai.busy = false
		s.ai.mu.Unlock()
	}()
	s.updateAIJob(job, "running", "ocr", 15, "正在由百炼解析 PDF", "")
	ocr, err := s.ai.extractPDF(ctx, settings.Bailian, job.FilePath)
	if err != nil {
		message := "PDF 解析失败: " + err.Error()
		s.failAIJob(job, message)
		s.appendAudit(auditEvent{Username: job.Username, RemoteIP: job.RemoteIP, Method: http.MethodPost, Path: "/api/ai/config-jobs", Status: http.StatusBadGateway, Result: "failed", Detail: message})
		return
	}
	s.updateAIJob(job, "running", "generating", 60, "正在由 DeepSeek V4 生成配置草稿", "")
	cfg, err := config.Load(s.configPath)
	if err != nil {
		s.failAIJob(job, err.Error())
		return
	}
	channel, _ := findAIChannel(cfg, job.ChannelKey)
	var existing *config.DeviceConfig
	if job.TargetMode == "update" {
		if found, ok := findAIDevice(cfg, job.TargetDevice); ok {
			existing = &found
		}
	}
	generated, err := s.ai.generateDeviceDraft(ctx, settings.DeepSeek, channel, existing, ocr)
	if err != nil {
		message := "配置生成失败: " + err.Error()
		s.failAIJob(job, message)
		s.appendAudit(auditEvent{Username: job.Username, RemoteIP: job.RemoteIP, Method: http.MethodPost, Path: "/api/ai/config-jobs", Status: http.StatusBadGateway, Result: "failed", Detail: message})
		return
	}
	s.updateAIJob(job, "running", "validating", 85, "正在校验协议字段和全局标识", "")
	draft := normalizeAIDraft(cfg, channel, job.TargetMode, job.TargetDevice, generated)
	draft.ID = newAIID("draft")
	draft.BaseRevision = revision
	draft.CreatedAt = time.Now()
	s.ai.mu.Lock()
	s.ai.drafts[draft.ID] = &draft
	job.Draft = &draft
	s.ai.mu.Unlock()
	s.updateAIJob(job, "completed", "completed", 100, "配置草稿已生成，请检查后应用", "")
	s.appendAudit(auditEvent{Username: job.Username, RemoteIP: job.RemoteIP, Method: http.MethodPost, Path: "/api/ai/config-jobs", Status: http.StatusOK, Result: "success", Detail: fmt.Sprintf("draft=%s channel=%s points=%d", draft.ID, draft.ChannelKey, len(draft.Device.Points))})
}

func (s *Server) updateAIJob(job *aiJob, status, stage string, progress int, message, errorMessage string) {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	job.Status, job.Stage, job.Progress = status, stage, progress
	job.Message, job.Error, job.UpdatedAt = message, errorMessage, time.Now()
	job.ExpiresAt = time.Now().Add(aiJobTTL)
}

func (s *Server) failAIJob(job *aiJob, message string) {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if job.Status == "cancelled" {
		return
	}
	s.ai.lastError = message
	job.Status, job.Stage = "failed", "failed"
	job.Message, job.Error, job.UpdatedAt = "", message, time.Now()
	job.ExpiresAt = time.Now().Add(aiJobTTL)
}

func (manager *aiManager) setLastError(message string) {
	manager.mu.Lock()
	manager.lastError = message
	manager.mu.Unlock()
}

func (manager *aiManager) extractPDF(ctx context.Context, provider aiProviderSettings, filePath string) (string, error) {
	fileURL, err := manager.uploadBailianTemporaryFile(ctx, provider, filePath)
	if err != nil {
		return "", err
	}
	payload := map[string]interface{}{
		"model": provider.Model,
		"input": []interface{}{map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_file", "file_url": fileURL},
				map[string]interface{}{"type": "input_text", "text": "解析这份工业设备 PDF，完整保留通信参数、寄存器/IOA/NodeId、数据类型、字节序、单位、倍率、小数位和表格行。不要总结，输出结构化文档内容。"},
			},
		}},
		"ocr_options": map[string]interface{}{"task": "document_parsing"},
	}
	var response interface{}
	if err := manager.doJSONWithHeaders(ctx, http.MethodPost, apiEndpoint(resolvedAIBaseURL(provider), "responses"), provider.APIKey, payload, &response, map[string]string{
		"X-DashScope-OssResourceResolve": "enable",
	}); err != nil {
		return "", err
	}
	text := findAIText(response)
	if strings.TrimSpace(text) == "" {
		return "", errors.New("百炼未返回可用的文档内容")
	}
	if len(text) > maxOCRBytes {
		return "", errors.New("PDF 解析结果过大，请拆分文档后重试")
	}
	return text, nil
}

type bailianUploadPolicy struct {
	RequestID string `json:"request_id"`
	Data      struct {
		Policy             string `json:"policy"`
		Signature          string `json:"signature"`
		UploadDir          string `json:"upload_dir"`
		UploadHost         string `json:"upload_host"`
		MaxFileSizeMB      string `json:"max_file_size_mb"`
		OSSAccessKeyID     string `json:"oss_access_key_id"`
		OSSObjectACL       string `json:"x_oss_object_acl"`
		OSSForbidOverwrite string `json:"x_oss_forbid_overwrite"`
	} `json:"data"`
}

// uploadBailianTemporaryFile follows Model Studio's documented temporary OSS
// upload flow. The returned oss:// URL is private, tied to the same Alibaba
// Cloud account and expires automatically after 48 hours.
func (manager *aiManager) uploadBailianTemporaryFile(ctx context.Context, provider aiProviderSettings, filePath string) (string, error) {
	base, err := url.Parse(resolvedAIBaseURL(provider))
	if err != nil {
		return "", err
	}
	base.Path = "/api/v1/uploads"
	base.RawQuery = url.Values{"action": {"getPolicy"}, "model": {provider.Model}}.Encode()
	var policy bailianUploadPolicy
	if err := manager.doJSON(ctx, http.MethodGet, base.String(), provider.APIKey, nil, &policy); err != nil {
		return "", fmt.Errorf("获取百炼临时上传凭证失败: %w", err)
	}
	if policy.Data.UploadHost == "" || policy.Data.UploadDir == "" || policy.Data.Policy == "" ||
		policy.Data.Signature == "" || policy.Data.OSSAccessKeyID == "" {
		return "", errors.New("百炼未返回完整的临时上传凭证")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	fileName := newAIID("pdf") + "-" + filepath.Base(filePath)
	objectKey := strings.TrimRight(policy.Data.UploadDir, "/") + "/" + fileName
	go func() {
		defer file.Close()
		var streamErr error
		fields := [][2]string{
			{"OSSAccessKeyId", policy.Data.OSSAccessKeyID},
			{"Signature", policy.Data.Signature},
			{"policy", policy.Data.Policy},
			{"x-oss-object-acl", valueOrDefault(policy.Data.OSSObjectACL, "private")},
			{"x-oss-forbid-overwrite", valueOrDefault(policy.Data.OSSForbidOverwrite, "true")},
			{"key", objectKey},
			{"success_action_status", "200"},
		}
		for _, field := range fields {
			if streamErr = form.WriteField(field[0], field[1]); streamErr != nil {
				break
			}
		}
		if streamErr == nil {
			var part io.Writer
			part, streamErr = form.CreateFormFile("file", fileName)
			if streamErr == nil {
				_, streamErr = io.Copy(part, file)
			}
		}
		if closeErr := form.Close(); streamErr == nil {
			streamErr = closeErr
		}
		_ = writer.CloseWithError(streamErr)
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, policy.Data.UploadHost, reader)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	response, err := manager.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("上传 PDF 到百炼临时存储失败: %s", upstreamError(response.StatusCode, raw))
	}
	return "oss://" + objectKey, nil
}

func (manager *aiManager) generateDeviceDraft(ctx context.Context, provider aiProviderSettings, channel config.ChannelConfig, existing *config.DeviceConfig, document string) (config.DeviceConfig, error) {
	current := interface{}(nil)
	if existing != nil {
		current = config.RedactSecrets(config.Config{Devices: []config.DeviceConfig{*existing}}).Devices[0]
	}
	contextJSON, _ := json.Marshal(map[string]interface{}{
		"channel":        map[string]interface{}{"channelKey": channel.ChannelKey, "name": channel.Name, "protocol": channel.Protocol, "serial": channel.Serial, "network": channel.Network},
		"existingDevice": current,
	})
	system := `你是工业网关配置生成器。请根据文档生成一个 JSON 对象，顶层只能包含 device。
device 字段遵循以下 Go JSON 字段：deviceKey,channelKey,name,protocol,address,slaveId,commonAddress,reconnectIntervalSeconds,rack,slot,localTsap,remoteTsap,iedName,username,password,points。
points 每项可包含：name,metric,address,pointType,slaveId,commonAddress,ioa,area,dbNumber,rack,slot,localTsap,remoteTsap,objectRef,fc,nodeId,function,register,quantity,dataType,byteOrder,wordOrder,bitIndex,scale,offset,unit,decimals。
只生成当前采集通道协议对应的设备与点位。文档没有明确给出的设备地址、从站号、功能码、寄存器、IOA、数据类型或 NodeId 必须留空/为零，绝对不能猜测。不要生成资源、通道、网络、云平台、账号、转发或边缘计算配置。输出必须是 JSON。`
	user := "当前上下文 JSON:\n" + string(contextJSON) + "\n\nPDF 解析内容:\n" + document
	payload := map[string]interface{}{
		"model":           provider.Model,
		"messages":        []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
		"response_format": map[string]string{"type": "json_object"},
		"stream":          false,
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := manager.doJSON(ctx, http.MethodPost, apiEndpoint(provider.BaseURL, "chat/completions"), provider.APIKey, payload, &response); err != nil {
		return config.DeviceConfig{}, err
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return config.DeviceConfig{}, errors.New("DeepSeek 未返回配置")
	}
	var wrapped struct {
		Device config.DeviceConfig `json:"device"`
	}
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &wrapped); err != nil {
		return config.DeviceConfig{}, fmt.Errorf("DeepSeek JSON 无效: %w", err)
	}
	return wrapped.Device, nil
}

func (manager *aiManager) doJSON(ctx context.Context, method, endpoint, apiKey string, payload interface{}, output interface{}) error {
	return manager.doJSONWithHeaders(ctx, method, endpoint, apiKey, payload, output, nil)
}

func (manager *aiManager) doJSONWithHeaders(ctx context.Context, method, endpoint, apiKey string, payload interface{}, output interface{}, headers map[string]string) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := manager.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxOCRBytes+1024*1024))
	if err != nil {
		return err
	}
	if response.StatusCode >= 300 {
		return errors.New(upstreamError(response.StatusCode, raw))
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return fmt.Errorf("上游响应格式无效: %w", err)
	}
	return nil
}

func findAIText(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []interface{}:
		var parts []string
		for _, item := range typed {
			if text := findAIText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]interface{}:
		for _, key := range []string{"ocr_result", "ocrResult", "output_text", "text", "content"} {
			if item, ok := typed[key]; ok {
				if text := findAIText(item); text != "" {
					return text
				}
			}
		}
		for _, key := range []string{"output", "data", "result"} {
			if item, ok := typed[key]; ok {
				if text := findAIText(item); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func normalizeAIDraft(current config.Config, channel config.ChannelConfig, targetMode, targetDevice string, generated config.DeviceConfig) aiDraft {
	draft := aiDraft{ChannelKey: channel.ChannelKey, TargetMode: targetMode, TargetDevice: targetDevice}
	if targetMode == "update" {
		if existing, ok := findAIDevice(current, targetDevice); ok {
			generated = mergeAIDeviceUpdate(existing, generated)
		}
		generated.DeviceKey = targetDevice
	} else {
		// Models must never inject credentials into a newly generated draft.
		generated.Username = ""
		generated.Password = ""
	}
	if generated.DeviceKey == "" {
		generated.DeviceKey = nextAIDeviceKey(current)
		draft.Warnings = append(draft.Warnings, "文档未提供设备标识，已生成 "+generated.DeviceKey)
	}
	if generated.Name == "" {
		generated.Name = generated.DeviceKey
		draft.Warnings = append(draft.Warnings, "文档未提供设备名称，已使用设备标识")
	}
	generated.ChannelKey = channel.ChannelKey
	generated.Protocol = channel.Protocol
	if generated.ReconnectIntervalSeconds <= 0 {
		generated.ReconnectIntervalSeconds = 5
	}
	if generated.Address == "" && channel.Serial != nil {
		generated.Address = channel.Serial.Port
	}
	if generated.Address == "" {
		draft.BlockingIssues = append(draft.BlockingIssues, "设备地址不能为空")
	}
	if (channel.Protocol == "modbus-tcp" || channel.Protocol == "modbus-rtu") && generated.SlaveID == 0 {
		draft.BlockingIssues = append(draft.BlockingIssues, "Modbus 从站 ID 未在文档中确定")
	}
	if channel.Protocol == "iec104" && generated.CommonAddress == 0 {
		draft.BlockingIssues = append(draft.BlockingIssues, "IEC104 公共地址未在文档中确定")
	}
	used := map[string]bool{}
	for _, point := range current.FlattenPoints() {
		if targetMode == "update" && point.DeviceKey == targetDevice {
			continue
		}
		used[strings.ToLower(point.Metric)] = true
	}
	for index := range generated.Points {
		point := &generated.Points[index]
		point.Username = generated.Username
		point.Password = generated.Password
		point.DeviceKey = generated.DeviceKey
		point.ChannelKey = channel.ChannelKey
		point.Protocol = channel.Protocol
		point.Address = generated.Address
		point.SlaveID = generated.SlaveID
		if point.CommonAddress == 0 {
			point.CommonAddress = generated.CommonAddress
		}
		if point.Name == "" {
			point.Name = valueOrDefault(point.Metric, fmt.Sprintf("点位%d", index+1))
		}
		metric := strings.TrimSpace(point.Metric)
		if metric == "" {
			metric = sanitizeAIMetric(point.Name)
			if metric == "" {
				metric = fmt.Sprintf("ai-point-%03d", index+1)
			}
			draft.Warnings = append(draft.Warnings, fmt.Sprintf("点位 %s 未提供标识符，已生成 %s", point.Name, metric))
		}
		base := metric
		for suffix := 2; used[strings.ToLower(metric)]; suffix++ {
			metric = base + "-" + strconv.Itoa(suffix)
		}
		if metric != point.Metric && strings.TrimSpace(point.Metric) != "" {
			draft.Warnings = append(draft.Warnings, fmt.Sprintf("标识符 %s 已存在，调整为 %s", point.Metric, metric))
		}
		point.Metric = metric
		used[strings.ToLower(metric)] = true
		if point.Quantity == 0 {
			point.Quantity = 1
		}
		if point.Scale == 0 {
			point.Scale = 1
		}
		if point.ByteOrder == "" {
			point.ByteOrder = "ABCD"
		}
		if point.WordOrder == "" {
			point.WordOrder = "ABCD"
		}
		switch channel.Protocol {
		case "modbus-tcp", "modbus-rtu":
			if point.Function < 1 || point.Function > 4 {
				draft.BlockingIssues = append(draft.BlockingIssues, fmt.Sprintf("%s 的 Modbus 功能码未确定", point.Name))
			}
			if point.DataType == "" {
				draft.BlockingIssues = append(draft.BlockingIssues, fmt.Sprintf("%s 的数据类型未确定", point.Name))
			}
		case "iec104":
			if point.IOA == 0 {
				draft.BlockingIssues = append(draft.BlockingIssues, fmt.Sprintf("%s 的 IOA 未确定", point.Name))
			}
		case "siemens-s7":
			if strings.TrimSpace(point.Area) == "" {
				draft.BlockingIssues = append(draft.BlockingIssues, fmt.Sprintf("%s 的 S7 存储区未确定", point.Name))
			}
		case "opcua":
			if strings.TrimSpace(point.NodeID) == "" {
				draft.BlockingIssues = append(draft.BlockingIssues, fmt.Sprintf("%s 的 OPC UA NodeId 未确定", point.Name))
			}
		case "iec61850":
			if strings.TrimSpace(point.ObjectRef) == "" {
				draft.BlockingIssues = append(draft.BlockingIssues, fmt.Sprintf("%s 的 IEC61850 ObjectRef 未确定", point.Name))
			}
		}
	}
	if len(generated.Points) == 0 {
		draft.BlockingIssues = append(draft.BlockingIssues, "文档中未识别到任何点位")
	}
	draft.Device = generated
	return draft
}

// mergeAIDeviceUpdate starts from the current device so that model output can
// never erase credentials or protocol-specific connection fields that were not
// present in the PDF. Only explicitly generated non-empty connection values and
// the generated point list are overlaid.
func mergeAIDeviceUpdate(existing, generated config.DeviceConfig) config.DeviceConfig {
	merged := existing
	if generated.Name != "" {
		merged.Name = generated.Name
	}
	if generated.PLCModel != "" {
		merged.PLCModel = generated.PLCModel
	}
	if generated.Address != "" {
		merged.Address = generated.Address
	}
	if generated.SlaveID != 0 {
		merged.SlaveID = generated.SlaveID
	}
	if generated.CommonAddress != 0 {
		merged.CommonAddress = generated.CommonAddress
	}
	if generated.ReconnectIntervalSeconds > 0 {
		merged.ReconnectIntervalSeconds = generated.ReconnectIntervalSeconds
	}
	if generated.Rack != 0 {
		merged.Rack = generated.Rack
	}
	if generated.Slot != 0 {
		merged.Slot = generated.Slot
	}
	if generated.LocalTSAP != "" {
		merged.LocalTSAP = generated.LocalTSAP
	}
	if generated.RemoteTSAP != "" {
		merged.RemoteTSAP = generated.RemoteTSAP
	}
	if generated.IEDName != "" {
		merged.IEDName = generated.IEDName
	}
	merged.Points = generated.Points
	return merged
}

func (s *Server) aiJobItem(writer http.ResponseWriter, request *http.Request) {
	id := strings.TrimPrefix(request.URL.Path, "/api/ai/config-jobs/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(writer, "任务不存在", http.StatusNotFound)
		return
	}
	s.ai.mu.Lock()
	job := s.ai.jobs[id]
	s.ai.mu.Unlock()
	if job == nil {
		http.Error(writer, "任务不存在或已过期", http.StatusNotFound)
		return
	}
	switch request.Method {
	case http.MethodGet:
		s.ai.mu.Lock()
		snapshot := *job
		if job.Draft != nil {
			draftCopy := *job.Draft
			snapshot.Draft = &draftCopy
		}
		s.ai.mu.Unlock()
		writeJSON(writer, snapshot)
	case http.MethodDelete:
		s.ai.mu.Lock()
		if job.cancel != nil {
			job.cancel()
		}
		job.Status, job.Stage, job.Message = "cancelled", "cancelled", "任务已取消"
		job.UpdatedAt, job.ExpiresAt = time.Now(), time.Now().Add(aiJobTTL)
		s.ai.mu.Unlock()
		_ = os.Remove(job.FilePath)
		writeJSON(writer, map[string]bool{"ok": true})
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) applyAIDraft(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.hasPermission(request, "config.manage") {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/api/ai/config-drafts/")
	if !strings.HasSuffix(id, "/apply") {
		http.Error(writer, "not found", http.StatusNotFound)
		return
	}
	id = strings.TrimSuffix(id, "/apply")
	if id == "" {
		http.Error(writer, "草稿不存在", http.StatusNotFound)
		return
	}
	s.ai.mu.Lock()
	stored := s.ai.drafts[id]
	s.ai.mu.Unlock()
	if stored == nil {
		http.Error(writer, "草稿不存在或已过期", http.StatusNotFound)
		return
	}
	var body struct {
		BaseRevision string              `json:"baseRevision"`
		Device       config.DeviceConfig `json:"device"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16*1024*1024)).Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	currentRevision, err := configFileRevision(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	expected := valueOrDefault(strings.TrimSpace(body.BaseRevision), stored.BaseRevision)
	if currentRevision != expected {
		http.Error(writer, "配置已被其他会话修改，请重新生成或刷新草稿", http.StatusConflict)
		return
	}
	current, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	var previous config.Config
	if raw, marshalErr := json.Marshal(current); marshalErr != nil || json.Unmarshal(raw, &previous) != nil {
		http.Error(writer, "复制当前配置失败", http.StatusInternalServerError)
		return
	}
	channel, ok := findAIChannel(current, stored.ChannelKey)
	if !ok {
		http.Error(writer, "目标通道已不存在", http.StatusConflict)
		return
	}
	normalized := normalizeAIDraft(current, channel, stored.TargetMode, stored.TargetDevice, body.Device)
	if len(normalized.BlockingIssues) > 0 {
		writeJSONStatus(writer, http.StatusBadRequest, map[string]interface{}{"error": "草稿仍有待确认字段", "blockingIssues": normalized.BlockingIssues, "warnings": normalized.Warnings})
		return
	}
	found := false
	if stored.TargetMode == "update" {
		for index := range current.Devices {
			if current.Devices[index].DeviceKey == stored.TargetDevice {
				current.Devices[index] = normalized.Device
				found = true
				break
			}
		}
		if !found {
			http.Error(writer, "目标设备已不存在", http.StatusConflict)
			return
		}
	} else {
		for _, device := range current.Devices {
			if strings.EqualFold(device.DeviceKey, normalized.Device.DeviceKey) {
				http.Error(writer, "设备标识已存在", http.StatusBadRequest)
				return
			}
		}
		current.Devices = append(current.Devices, normalized.Device)
	}
	current.ApplyDefaults()
	if err := current.ValidateNewGlobalIdentifierDuplicates(previous); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := current.ValidateGlobalDeviceKeys(); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := current.Validate(); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := edgecompute.Validate(current); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.saveAndApplyConfig(current); err != nil {
		http.Error(writer, "应用 AI 配置失败，原配置已恢复: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.ai.mu.Lock()
	delete(s.ai.drafts, id)
	s.ai.mu.Unlock()
	writeJSON(writer, map[string]interface{}{"ok": true, "deviceKey": normalized.Device.DeviceKey, "points": len(normalized.Device.Points), "restartRequired": false})
}

func findAIChannel(cfg config.Config, key string) (config.ChannelConfig, bool) {
	for _, channel := range cfg.Channels {
		if channel.ChannelKey == key && channel.Role != "forward" && channel.Enabled {
			return channel, true
		}
	}
	return config.ChannelConfig{}, false
}

func findAIAnyChannel(cfg config.Config, key string) (config.ChannelConfig, bool) {
	for _, channel := range cfg.Channels {
		if channel.ChannelKey == key {
			return channel, true
		}
	}
	return config.ChannelConfig{}, false
}

func findAIDevice(cfg config.Config, key string) (config.DeviceConfig, bool) {
	for _, device := range cfg.Devices {
		if device.DeviceKey == key {
			return device, true
		}
	}
	return config.DeviceConfig{}, false
}

func nextAIDeviceKey(cfg config.Config) string {
	used := map[string]bool{}
	for _, device := range cfg.Devices {
		used[strings.ToLower(device.DeviceKey)] = true
	}
	for _, device := range cfg.ForwardDevices {
		used[strings.ToLower(device.DeviceKey)] = true
	}
	for index := 1; ; index++ {
		key := fmt.Sprintf("device-ai-%03d", index)
		if !used[key] {
			return key
		}
	}
}

func sanitizeAIMetric(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			builder.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func apiEndpoint(baseURL, suffix string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/" + strings.TrimLeft(suffix, "/")
}

func resolvedAIBaseURL(provider aiProviderSettings) string {
	baseURL := strings.TrimSpace(provider.BaseURL)
	workspaceID := strings.TrimSpace(provider.WorkspaceID)
	if workspaceID != "" && strings.Contains(baseURL, "dashscope.aliyuncs.com") {
		return "https://" + workspaceID + ".cn-beijing.maas.aliyuncs.com/compatible-mode/v1"
	}
	return baseURL
}

func newAIID(prefix string) string {
	raw := make([]byte, 12)
	_, _ = rand.Read(raw)
	return prefix + "-" + hex.EncodeToString(raw)
}

func upstreamError(status int, body []byte) string {
	var payload struct {
		Error   interface{} `json:"error"`
		Message string      `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	message := strings.TrimSpace(payload.Message)
	if message == "" && payload.Error != nil {
		switch value := payload.Error.(type) {
		case string:
			message = value
		case map[string]interface{}:
			message, _ = value["message"].(string)
		}
	}
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	return fmt.Sprintf("上游服务返回 %d: %s", status, message)
}

func writeJSONStatus(writer http.ResponseWriter, status int, payload interface{}) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

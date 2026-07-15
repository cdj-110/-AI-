package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/datamanager"
	gatewayruntime "weikong-iot-platform/apps/gateway-go/internal/runtime"
)

const (
	webSocketWriteWait  = 8 * time.Second
	webSocketPongWait   = 45 * time.Second
	webSocketPingPeriod = 20 * time.Second
)

var gatewayWebSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:    4096,
	WriteBufferSize:   4096,
	EnableCompression: true,
	CheckOrigin:       sameWebSocketOrigin,
}

type webSocketRequest struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId"`
	Payload   json.RawMessage `json:"payload"`
}

type commandRequest struct {
	Command string          `json:"command"`
	Args    json.RawMessage `json:"args"`
}

type commandResult struct {
	RequestID string      `json:"requestId"`
	Command   string      `json:"command"`
	OK        bool        `json:"ok"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

func (s *Server) webSocket(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	connection, err := gatewayWebSocketUpgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(1024 * 1024)
	_ = connection.SetReadDeadline(time.Now().Add(webSocketPongWait))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(webSocketPongWait))
	})

	events, unsubscribe := s.data.Subscribe(128)
	defer unsubscribe()
	incoming := make(chan webSocketRequest, 16)
	readErrors := make(chan error, 1)
	results := make(chan datamanager.Message, 16)
	commandSlots := make(chan struct{}, 4)
	done := make(chan struct{})
	defer close(done)
	go readWebSocketRequests(connection, incoming, readErrors, done)

	for _, message := range s.data.InitialMessages() {
		if err := writeWebSocketMessage(connection, message); err != nil {
			return
		}
	}

	pingTicker := time.NewTicker(webSocketPingPeriod)
	defer pingTicker.Stop()
	for {
		select {
		case message, open := <-events:
			if !open || writeWebSocketMessage(connection, message) != nil {
				return
			}
		case message := <-results:
			if writeWebSocketMessage(connection, message) != nil {
				return
			}
		case message := <-incoming:
			select {
			case commandSlots <- struct{}{}:
				go func(commandMessage webSocketRequest) {
					defer func() { <-commandSlots }()
					s.executeWebSocketRequest(request.Context(), request, commandMessage, results)
				}(message)
			default:
				result := commandResult{RequestID: message.RequestID, OK: false, Error: "too many commands are running"}
				if writeWebSocketMessage(connection, s.data.Message("command.result", result)) != nil {
					return
				}
			}
		case <-pingTicker.C:
			_ = connection.SetWriteDeadline(time.Now().Add(webSocketWriteWait))
			if err := connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(webSocketWriteWait)); err != nil {
				return
			}
		case <-readErrors:
			return
		case <-request.Context().Done():
			return
		}
	}
}

func readWebSocketRequests(connection *websocket.Conn, incoming chan<- webSocketRequest, failures chan<- error, done <-chan struct{}) {
	for {
		var request webSocketRequest
		if err := connection.ReadJSON(&request); err != nil {
			select {
			case failures <- err:
			case <-done:
			}
			return
		}
		select {
		case incoming <- request:
		case <-done:
			return
		}
	}
}

func writeWebSocketMessage(connection *websocket.Conn, message datamanager.Message) error {
	_ = connection.SetWriteDeadline(time.Now().Add(webSocketWriteWait))
	return connection.WriteJSON(message)
}

func (s *Server) executeWebSocketRequest(ctx context.Context, httpRequest *http.Request, request webSocketRequest, results chan<- datamanager.Message) {
	if request.Type != "command.execute" {
		sendCommandResult(ctx, results, s.data.Message("command.result", commandResult{RequestID: request.RequestID, OK: false, Error: "unsupported websocket request type"}))
		return
	}
	var command commandRequest
	if err := json.Unmarshal(request.Payload, &command); err != nil {
		sendCommandResult(ctx, results, s.data.Message("command.result", commandResult{RequestID: request.RequestID, OK: false, Error: err.Error()}))
		return
	}
	command.Command = strings.TrimSpace(command.Command)
	s.data.PublishLog("INFO", "执行命令："+command.Command)
	data, err := s.runWebSocketCommand(ctx, httpRequest, command)
	result := commandResult{RequestID: request.RequestID, Command: command.Command, OK: err == nil, Data: data}
	if err != nil {
		result.Error = err.Error()
		s.data.PublishLog("ERROR", command.Command+"："+err.Error())
	} else {
		s.data.PublishLog("INFO", command.Command+"：执行成功")
	}
	select {
	case results <- s.data.Message("command.result", result):
	case <-ctx.Done():
	}
}

func sendCommandResult(ctx context.Context, results chan<- datamanager.Message, message datamanager.Message) {
	select {
	case results <- message:
	case <-ctx.Done():
	}
}

func (s *Server) runWebSocketCommand(ctx context.Context, request *http.Request, command commandRequest) (interface{}, error) {
	switch command.Command {
	case "collect.now":
		if !s.hasPermission(request, "config.manage") {
			return nil, errors.New("forbidden")
		}
		if s.runtime == nil {
			return nil, errors.New("gateway runtime is unavailable")
		}
		commandCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		s.runtime.CollectNow(commandCtx)
		return map[string]interface{}{"collected": true}, nil
	case "point.write":
		if !s.hasPermission(request, "config.manage") {
			return nil, errors.New("forbidden")
		}
		return s.runPointWriteCommand(ctx, command.Args)
	case "connection.test":
		if !s.hasPermission(request, "config.manage") {
			return nil, errors.New("forbidden")
		}
		return runConnectionTestCommand(ctx, command.Args)
	case "maintenance.ping":
		if !s.hasPermission(request, "maintenance.run") {
			return nil, errors.New("forbidden")
		}
		return runPingCommand(ctx, command.Args)
	default:
		return nil, fmt.Errorf("unsupported command %q", command.Command)
	}
}

func (s *Server) runPointWriteCommand(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	if s.runtime == nil {
		return nil, errors.New("gateway runtime is unavailable")
	}
	var body struct {
		DeviceKey string      `json:"deviceKey"`
		Metric    string      `json:"metric"`
		Value     interface{} `json:"value"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return nil, err
	}
	body.DeviceKey = strings.TrimSpace(body.DeviceKey)
	body.Metric = strings.TrimSpace(body.Metric)
	if body.DeviceKey == "" || body.Metric == "" || body.Value == nil {
		return nil, errors.New("deviceKey, metric and value are required")
	}
	commandCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	point, err := s.runtime.WritePoint(commandCtx, body.DeviceKey, body.Metric, body.Value)
	if err != nil {
		if errors.Is(err, gatewayruntime.ErrWritablePointNotFound) {
			return nil, fmt.Errorf("invalid writable point: %w", err)
		}
		return nil, err
	}
	return map[string]interface{}{"deviceKey": point.DeviceKey, "metric": point.Metric, "value": body.Value, "readFunction": point.Function}, nil
}

func runConnectionTestCommand(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	var body connectionTestRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	commandCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if body.Protocol == "siemens-s7" {
		result, err := collector.TestS7Connection(commandCtx, config.PointConfig{Protocol: "siemens-s7", Address: body.Address, Rack: body.Rack, Slot: body.Slot, LocalTSAP: body.LocalTSAP, RemoteTSAP: body.RemoteTSAP})
		if err != nil {
			return map[string]interface{}{"address": result.Address}, err
		}
		return map[string]interface{}{"address": result.Address, "message": fmt.Sprintf("连接成功，已完成 S7 握手。参数：%s LocalTSAP=%s RemoteTSAP=%s", result.Endpoint, result.LocalTSAP, result.RemoteTSAP)}, nil
	}
	if body.Protocol == "iec104" {
		commonAddress := body.CommonAddress
		if commonAddress == 0 {
			commonAddress = uint16(body.SlaveID)
		}
		result, err := collector.TestIEC104Connection(commandCtx, body.Address, commonAddress)
		if err != nil {
			return map[string]interface{}{"address": result.Address}, err
		}
		return map[string]interface{}{"address": result.Address, "message": fmt.Sprintf("连接成功，已完成 IEC104 STARTDT 握手。公共地址：%d", result.CommonAddress)}, nil
	}
	if body.Protocol == "opcua" {
		result, err := collector.TestOPCUAConnection(commandCtx, config.PointConfig{Protocol: "opcua", Address: body.Address, Username: body.Username, Password: body.Password})
		if err != nil {
			return map[string]interface{}{"address": result.Endpoint}, err
		}
		return map[string]interface{}{"address": result.Endpoint, "message": "OPC UA 连接成功，已完成端点握手。"}, nil
	}
	address := normalizeConnectionAddress(body.Protocol, body.Address)
	connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(commandCtx, "tcp", address)
	if err != nil {
		return map[string]interface{}{"address": address}, fmt.Errorf("连接失败：%w", err)
	}
	_ = connection.Close()
	return map[string]interface{}{"address": address, "message": "连接成功，TCP 端口可达。"}, nil
}

func runPingCommand(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	var body pingRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(body.Target)
	if target == "" || strings.ContainsAny(target, " \t\r\n") {
		return nil, errors.New("invalid ping target")
	}
	count := body.Count
	if count <= 0 || count > 10 {
		count = 4
	}
	commandCtx, cancel := context.WithTimeout(ctx, time.Duration(count*3+3)*time.Second)
	defer cancel()
	output, err := exec.CommandContext(commandCtx, "ping", pingArgs(target, count)...).CombinedOutput()
	return map[string]interface{}{"target": target, "output": string(output), "ok": err == nil}, nil
}

func sameWebSocketOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, request.Host)
}

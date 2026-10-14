package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type PointCommand struct {
	CommandID string      `json:"commandId"`
	DeviceKey string      `json:"deviceKey"`
	Metric    string      `json:"metric"`
	Value     interface{} `json:"value"`
	IssuedAt  time.Time   `json:"issuedAt"`
	ExpiresAt time.Time   `json:"expiresAt"`
}

type PointCommandResult struct {
	CommandID string `json:"commandId"`
	DeviceKey string `json:"deviceKey"`
	Metric    string `json:"metric"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

func (c *Client) SubscribePointCommands(handler func(PointCommand) PointCommandResult) error {
	c.pointCommandMu.Lock()
	c.pointCommandHandler = handler
	c.pointCommandMu.Unlock()
	return c.subscribePointCommandsNow()
}

func (c *Client) subscribePointCommandsNow() error {
	c.pointCommandMu.RLock()
	handler := c.pointCommandHandler
	c.pointCommandMu.RUnlock()
	if handler == nil {
		return nil
	}
	if !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt is not connected")
	}
	token := c.mqtt.Subscribe(fmt.Sprintf("weikong/gateways/%s/commands/set", c.gatewayKey), 1, func(_ mqtt.Client, message mqtt.Message) {
		// Retained commands must never operate physical equipment.
		if message.Retained() || len(message.Payload()) > 8192 {
			return
		}
		var command PointCommand
		decoder := json.NewDecoder(bytes.NewReader(message.Payload()))
		decoder.UseNumber()
		if err := decoder.Decode(&command); err != nil {
			return
		}
		go func() {
			result := handler(command)
			if err := c.publish(fmt.Sprintf("weikong/gateways/%s/commands/reply", c.gatewayKey), result); err != nil {
				fmt.Printf("publish point command reply failed: %v\n", err)
			}
		}()
	})
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe point commands timeout")
	}
	return token.Error()
}

type commandEntry struct {
	Result    PointCommandResult `json:"result"`
	ExpiresAt time.Time          `json:"expiresAt"`
}

// Reserve on disk before writing. Reconnects and process restarts cannot replay
// an accepted command. A crash between reservation and result remains UNKNOWN.
type PointCommandExecutor struct {
	mu   sync.Mutex
	path string
}

func NewPointCommandExecutor(path string) *PointCommandExecutor {
	return &PointCommandExecutor{path: path}
}

var commandIDPattern = regexp.MustCompile(`^[a-fA-F0-9-]{36}$`)

func (e *PointCommandExecutor) Execute(command PointCommand, write func() error) PointCommandResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := PointCommandResult{CommandID: command.CommandID, DeviceKey: command.DeviceKey, Metric: command.Metric, Status: "FAILED"}
	now := time.Now()
	if !commandIDPattern.MatchString(command.CommandID) || command.DeviceKey == "" || command.Metric == "" || command.Value == nil {
		result.Message = "下发指令缺少有效的编号、设备、点位或值"
		return result
	}
	entries := map[string]commandEntry{}
	if raw, err := os.ReadFile(e.path); err == nil {
		if json.Unmarshal(raw, &entries) != nil {
			result.Message = "指令去重记录损坏，拒绝执行"
			return result
		}
		if entries == nil {
			result.Message = "指令去重记录损坏，拒绝执行"
			return result
		}
	} else if !os.IsNotExist(err) {
		result.Message = "无法读取指令去重记录，拒绝执行"
		return result
	}
	if entry, exists := entries[command.CommandID]; exists {
		if entry.Result.DeviceKey != command.DeviceKey || entry.Result.Metric != command.Metric {
			result.Message = "指令编号与原设备或点位不匹配"
			return result
		}
		return entry.Result
	}
	if !command.ExpiresAt.After(now) || command.IssuedAt.After(now.Add(5*time.Second)) || command.ExpiresAt.Sub(command.IssuedAt) > 30*time.Second || !command.ExpiresAt.After(command.IssuedAt) {
		result.Message = "指令已过期或时间范围不合法，请检查网关时间"
		return result
	}
	for id, entry := range entries {
		if !entry.ExpiresAt.After(now) {
			delete(entries, id)
		}
	}
	if len(entries) >= 1024 {
		result.Message = "指令队列已满，请稍后再试"
		return result
	}
	result.Status = "UNKNOWN"
	result.Message = "指令已接收，执行结果尚未确认；请检查实际值，勿重复下发"
	entries[command.CommandID] = commandEntry{Result: result, ExpiresAt: command.ExpiresAt}
	if err := e.save(entries); err != nil {
		result.Status = "FAILED"
		result.Message = "无法保存指令去重记录，拒绝执行"
		return result
	}
	if err := write(); err != nil {
		result.Status = "FAILED"
		result.Message = err.Error()
	} else {
		result.Status = "SUCCEEDED"
		result.Message = "网关已执行点位写入"
	}
	entries[command.CommandID] = commandEntry{Result: result, ExpiresAt: command.ExpiresAt}
	if err := e.save(entries); err != nil {
		result.Status = "UNKNOWN"
		result.Message = "执行结果无法保存，请检查实际值，勿重复下发"
	}
	return result
}

func (e *PointCommandExecutor) save(entries map[string]commandEntry) error {
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(e.path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(e.path+".tmp", e.path); err != nil {
		return err
	}
	// Sync the parent directory too: reservation must survive a power loss on
	// the EG100 Linux filesystem, not only a normal process restart.
	if runtime.GOOS != "windows" {
		directory, err := os.Open(filepath.Dir(e.path))
		if err != nil {
			return err
		}
		err = directory.Sync()
		_ = directory.Close()
		return err
	}
	return nil
}

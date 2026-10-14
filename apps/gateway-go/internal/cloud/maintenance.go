package cloud

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	MaintenanceServiceRestart = "SERVICE_RESTART"
	MaintenanceGatewayReboot  = "GATEWAY_REBOOT"
)

type MaintenanceCommand struct {
	TaskID     string    `json:"taskId"`
	HardwareID string    `json:"hardwareId"`
	Action     string    `json:"action"`
	IssuedAt   time.Time `json:"issuedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type MaintenanceResult struct {
	TaskID     string `json:"taskId"`
	HardwareID string `json:"hardwareId"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

func (c *Client) SubscribeMaintenance(handler func(MaintenanceCommand) MaintenanceResult, resultPath string) error {
	c.maintenanceMu.Lock()
	c.maintenanceHandler, c.maintenanceResultPath = handler, resultPath
	c.maintenanceMu.Unlock()
	return c.subscribeMaintenanceNow()
}

func (c *Client) subscribeMaintenanceNow() error {
	c.maintenanceMu.RLock()
	handler := c.maintenanceHandler
	c.maintenanceMu.RUnlock()
	if handler == nil || c.hardwareID == "" {
		return nil
	}
	if !c.mqtt.IsConnected() {
		return fmt.Errorf("mqtt is not connected")
	}
	topic := fmt.Sprintf("weikong/factory/%s/maintenance/set", c.hardwareID)
	token := c.mqtt.Subscribe(topic, 1, func(_ mqtt.Client, message mqtt.Message) {
		if message.Retained() || len(message.Payload()) > 4096 {
			return
		}
		var command MaintenanceCommand
		if json.Unmarshal(message.Payload(), &command) != nil {
			return
		}
		go func() {
			result := handler(command)
			if err := c.saveMaintenanceResult(result); err != nil {
				log.Printf("persist maintenance result failed: %v", err)
			}
			if err := c.PublishMaintenanceResult(result); err != nil {
				log.Printf("publish maintenance result failed: %v", err)
				return
			}
			c.removeMaintenanceResult(result.TaskID)
		}()
	})
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe maintenance timeout")
	}
	return token.Error()
}

func (c *Client) PublishMaintenanceResult(result MaintenanceResult) error {
	if c.hardwareID == "" {
		return fmt.Errorf("not an activation channel")
	}
	return c.publish(fmt.Sprintf("weikong/factory/%s/maintenance/reply", c.hardwareID), result)
}

func (c *Client) saveMaintenanceResult(result MaintenanceResult) error {
	c.maintenanceMu.RLock()
	path := c.maintenanceResultPath
	c.maintenanceMu.RUnlock()
	if path == "" {
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func (c *Client) removeMaintenanceResult(taskID string) {
	c.maintenanceMu.RLock()
	path := c.maintenanceResultPath
	c.maintenanceMu.RUnlock()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var result MaintenanceResult
	if json.Unmarshal(raw, &result) == nil && result.TaskID == taskID {
		_ = os.Remove(path)
	}
}

func (c *Client) PublishPendingMaintenanceResult() {
	c.maintenanceMu.RLock()
	path := c.maintenanceResultPath
	c.maintenanceMu.RUnlock()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var result MaintenanceResult
	if json.Unmarshal(raw, &result) != nil || result.HardwareID != c.hardwareID {
		return
	}
	if err := c.PublishMaintenanceResult(result); err == nil {
		c.removeMaintenanceResult(result.TaskID)
	}
}

type maintenanceEntry struct {
	Result    MaintenanceResult `json:"result"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type MaintenanceCommandExecutor struct {
	mu   sync.Mutex
	path string
}

func NewMaintenanceCommandExecutor(path string) *MaintenanceCommandExecutor {
	return &MaintenanceCommandExecutor{path: path}
}

func (e *MaintenanceCommandExecutor) Execute(command MaintenanceCommand, schedule func(string) error) MaintenanceResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := MaintenanceResult{TaskID: command.TaskID, HardwareID: command.HardwareID, Action: command.Action, Status: "FAILED"}
	now := time.Now()
	if !commandIDPattern.MatchString(command.TaskID) || command.HardwareID == "" || (command.Action != MaintenanceServiceRestart && command.Action != MaintenanceGatewayReboot) {
		result.Message = "远程维护指令格式无效"
		return result
	}
	entries := map[string]maintenanceEntry{}
	if raw, err := os.ReadFile(e.path); err == nil {
		if json.Unmarshal(raw, &entries) != nil || entries == nil {
			result.Message = "远程维护去重记录损坏，拒绝执行"
			return result
		}
	} else if !os.IsNotExist(err) {
		result.Message = "无法读取远程维护去重记录，拒绝执行"
		return result
	}
	if entry, exists := entries[command.TaskID]; exists {
		if entry.Result.HardwareID != command.HardwareID || entry.Result.Action != command.Action {
			result.Message = "任务编号与原维护指令不匹配"
			return result
		}
		return entry.Result
	}
	if !command.ExpiresAt.After(now) || command.IssuedAt.After(now.Add(5*time.Second)) || !command.ExpiresAt.After(command.IssuedAt) || command.ExpiresAt.Sub(command.IssuedAt) > 90*time.Second {
		result.Message = "远程维护指令已过期或时间范围无效"
		return result
	}
	for id, entry := range entries {
		if !entry.ExpiresAt.After(now) {
			delete(entries, id)
		}
	}
	if len(entries) >= 128 {
		result.Message = "远程维护去重记录已满"
		return result
	}
	// Reserve the task before scheduling the disruptive action.  The reservation
	// deliberately uses ACCEPTED: if the process loses power between this write
	// and the final write, a redelivered MQTT packet must not invite an operator
	// to repeat a reboot whose outcome is necessarily ambiguous.
	result.Status = "ACCEPTED"
	result.Message = "维护指令已接收，执行状态待确认；请勿重复下发"
	entries[command.TaskID] = maintenanceEntry{Result: result, ExpiresAt: command.ExpiresAt}
	if err := e.save(entries); err != nil {
		result.Message = "无法保存远程维护去重记录，拒绝执行"
		return result
	}
	if err := schedule(command.Action); err != nil {
		result.Status = "FAILED"
		result.Message = err.Error()
	} else {
		result.Status = "ACCEPTED"
		if command.Action == MaintenanceServiceRestart {
			result.Message = "网关已受理服务重启，连接将短暂中断"
		} else {
			result.Message = "网关已受理整机重启，设备重新上线前请勿断电"
		}
	}
	entries[command.TaskID] = maintenanceEntry{Result: result, ExpiresAt: command.ExpiresAt}
	if err := e.save(entries); err != nil {
		result.Status = "ACCEPTED"
		result.Message = "维护操作已安排，但执行结果无法持久化；请勿重复下发"
	}
	return result
}

func (e *MaintenanceCommandExecutor) save(entries map[string]maintenanceEntry) error {
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(e.path), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(e.path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(e.path+".tmp", e.path)
}

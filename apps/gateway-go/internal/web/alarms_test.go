package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/alarm"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestAlarmRulesAPIAndAcknowledge(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.json")
	cfg := alarmWebConfig()
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	store := state.New(cfg)
	manager := alarm.NewManager(cfg, store, configPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	server := New(config.ListenerConfig{}, store, nil, configPath, func(next config.Config) error { manager.UpdateConfig(next, configPath); return nil }, manager)
	rules := []config.AlarmRuleConfig{{RuleID: "high", Name: "温度过高", DeviceKey: "d1", Metric: "temperature", Type: "threshold", Operator: ">", Threshold: floatPointer(80), Level: "CRITICAL"}}
	body, _ := json.Marshal(map[string]interface{}{"rules": rules})
	request := httptest.NewRequest(http.MethodPut, "/api/alarm-rules", bytes.NewReader(body))
	authenticateTestRequest(server, request)
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("save rules status=%d body=%s", response.Code, response.Body.String())
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Alarms.Rules) != 1 || loaded.Alarms.Rules[0].RuleID != "high" {
		t.Fatalf("rules not saved: %#v", loaded.Alarms.Rules)
	}
	store.SetPointValue("d1", "temperature", 90.0)
	deadline := time.Now().Add(2 * time.Second)
	for manager.ActiveSnapshot().ActiveCount == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	historyRequest := httptest.NewRequest(http.MethodGet, "/api/alarms/history?page=1&pageSize=1&level=CRITICAL", nil)
	authenticateTestRequest(server, historyRequest)
	historyResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(historyResponse, historyRequest)
	if historyResponse.Code != http.StatusOK {
		t.Fatalf("history status=%d body=%s", historyResponse.Code, historyResponse.Body.String())
	}
	var page alarm.HistoryPage
	if err := json.Unmarshal(historyResponse.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Level != "CRITICAL" {
		t.Fatalf("unexpected alarm history page: %#v", page)
	}
}

func alarmWebConfig() config.Config {
	cfg := config.Config{GatewayKey: "gw", Points: []config.PointConfig{{DeviceKey: "d1", Name: "温度", Metric: "temperature", Protocol: "modbus-tcp", Address: "127.0.0.1:502", Function: 3, Quantity: 1, DataType: "float64", Scale: 1}}, Web: config.ListenerConfig{Enabled: true}}
	cfg.ApplyDefaults()
	return cfg
}

func floatPointer(value float64) *float64 { return &value }

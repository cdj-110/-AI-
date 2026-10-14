package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// Full-document editors must not become an alternate user, network or cloud
// administration API. Call after masked secrets/defaults have been resolved,
// while holding configMu, and before writing anything to disk.
func (s *Server) authorizeConfigChanges(request *http.Request, previous, next config.Config) error {
	permissions := map[string]string{
		"security": "users.manage", "web": "users.manage",
		"activation": "cloud.manage", "gatewayKey": "cloud.manage",
		"cloudConnectionEnabled": "cloud.manage",
		"mqtt":                   "cloud.manage", "mqttChannels": "cloud.manage", "remoteAccess": "cloud.manage",
		"wifi": "network.manage", "cellular": "network.manage",
		"offlineCache": "offline.manage", "historyStorage": "history.manage",
		"remoteStorage": "history.manage",
		"edgeComputing": "edge.manage", "alarms": "alarms.manage",
		"watchdog": "maintenance.restart",
	}
	beforeRaw, err := json.Marshal(previous)
	if err != nil {
		return err
	}
	afterRaw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	var before, after map[string]json.RawMessage
	if err = json.Unmarshal(beforeRaw, &before); err != nil {
		return err
	}
	if err = json.Unmarshal(afterRaw, &after); err != nil {
		return err
	}
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	for _, key := range ordered {
		if bytes.Equal(before[key], after[key]) {
			continue
		}
		if key == "security" {
			return fmt.Errorf("用户、角色和二次认证只能通过用户管理专用接口修改")
		}
		permission := permissions[key]
		if permission == "" {
			permission = "config.manage"
		}
		if !s.hasPermission(request, permission) {
			return fmt.Errorf("无权修改配置模块 %s（需要 %s）", key, permission)
		}
	}
	return nil
}

package web

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type permissionCatalogItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Description string `json:"description,omitempty"`
}

var permissionCatalog = []permissionCatalogItem{
	{Key: "*", Name: "全部权限", Group: "全部权限", Description: "自动包含当前及后续新增的全部功能权限"},
	{Key: "status.view", Name: "查看运行状态", Group: "运行状态", Description: "查看网关负载、连接和点位实时状态"},
	{Key: "collection_incidents.view", Name: "查看采集异常", Group: "采集异常", Description: "查看各协议采集失败、恢复过程和原因诊断"},
	{Key: "collection_incidents.manage", Name: "管理采集异常", Group: "采集异常", Description: "清理采集异常历史记录"},
	{Key: "config.view", Name: "查看网关配置", Group: "智能网关", Description: "查看资源、通道、设备和点位配置"},
	{Key: "config.manage", Name: "管理网关配置", Group: "智能网关", Description: "新增、修改和删除资源、通道、设备及点位"},
	{Key: "collection.run", Name: "手动采集与连接测试", Group: "智能网关", Description: "立即采集、测试连接和浏览协议点位"},
	{Key: "points.write", Name: "点位写入", Group: "智能网关", Description: "向可写点位下发数值"},
	{Key: "packet.view", Name: "查看实时报文", Group: "智能网关", Description: "查看协议收发报文和通信诊断"},
	{Key: "edge.view", Name: "查看边缘计算", Group: "边缘计算", Description: "查看计算组、表达式和派生点"},
	{Key: "edge.manage", Name: "校验边缘计算", Group: "边缘计算", Description: "执行表达式试算；保存配置还需“管理网关配置”"},
	{Key: "alarms.view", Name: "查看告警", Group: "告警管理", Description: "查看当前告警和告警历史"},
	{Key: "alarms.manage", Name: "管理告警", Group: "告警管理", Description: "确认告警并维护告警规则"},
	{Key: "history.view", Name: "查看历史数据", Group: "历史数据", Description: "查询历史记录、趋势和导出结果"},
	{Key: "history.manage", Name: "管理历史数据", Group: "历史数据", Description: "维护历史任务、存储策略、迁移和导出"},
	{Key: "offline.manage", Name: "管理断线续存", Group: "云平台连接", Description: "配置和清理云平台断线续存数据"},
	{Key: "network.manage", Name: "管理 WiFi/移动网络", Group: "网络管理", Description: "配置有线、WiFi 和移动网络"},
	{Key: "cloud.manage", Name: "管理云平台连接", Group: "云平台连接", Description: "配置 MQTT、微控云、设备授权和连接测试"},
	{Key: "ai.use", Name: "使用 AI 助手", Group: "AI 助手", Description: "对话并从文档生成配置草稿"},
	{Key: "ai.manage", Name: "管理 AI 模型配置", Group: "AI 助手", Description: "配置模型、密钥和连通性测试"},
	{Key: "maintenance.view", Name: "查看系统维护", Group: "系统维护", Description: "进入系统维护并查看基础状态"},
	{Key: "maintenance.diagnose", Name: "执行诊断", Group: "系统维护", Description: "执行网络诊断并下载脱敏诊断包"},
	{Key: "maintenance.time", Name: "管理日期与时间", Group: "系统维护", Description: "设置时区、NTP 服务器并执行校时"},
	{Key: "maintenance.remote", Name: "管理远程访问", Group: "系统维护", Description: "启用或关闭 SSH 远程登录"},
	{Key: "maintenance.restart", Name: "重启服务和网关", Group: "系统维护", Description: "重启网关服务或整台设备"},
	{Key: "maintenance.factory_reset", Name: "恢复出厂设置", Group: "系统维护", Description: "备份后清空工程并恢复出厂配置"},
	{Key: "maintenance.audit", Name: "查看操作审计", Group: "系统维护", Description: "查看维护、登录和配置变更记录"},
	{Key: "users.manage", Name: "管理用户和角色", Group: "用户管理", Description: "维护账号、角色和权限"},
}

type securityPayload struct {
	Users       []securityUserPayload `json:"users"`
	Roles       []config.RoleConfig   `json:"roles"`
	Permissions interface{}           `json:"permissions,omitempty"`
}

type securityUserPayload struct {
	Username           string `json:"username"`
	DisplayName        string `json:"displayName,omitempty"`
	Password           string `json:"password,omitempty"`
	RoleKey            string `json:"roleKey"`
	Enabled            bool   `json:"enabled"`
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
	MFAEnabled         bool   `json:"mfaEnabled,omitempty"`
}

func (s *Server) currentUserFile(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := s.currentUser(request)
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(user)
}

func (s *Server) securityFile(writer http.ResponseWriter, request *http.Request) {
	if !s.hasPermission(request, "users.manage") {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	switch request.Method {
	case http.MethodGet:
		s.getSecurity(writer)
	case http.MethodPut:
		s.saveSecurity(writer, request)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getSecurity(writer http.ResponseWriter) {
	security, _ := s.loadSecurity()
	security.Roles = defaultedRoles(security.Roles)
	if len(security.Users) == 0 {
		security.Users = []config.UserConfig{{
			Username:           s.authState().username,
			DisplayName:        "系统管理员",
			RoleKey:            "admin",
			Enabled:            true,
			MustChangePassword: s.authState().password == "123456",
		}}
	}
	users := make([]securityUserPayload, 0, len(security.Users))
	for _, user := range security.Users {
		users = append(users, securityUserPayload{
			Username:           user.Username,
			DisplayName:        user.DisplayName,
			RoleKey:            valueOrDefault(user.RoleKey, "viewer"),
			Enabled:            user.Enabled,
			MustChangePassword: user.MustChangePassword,
			MFAEnabled:         user.MFAEnabled,
		})
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(securityPayload{Users: users, Roles: security.Roles, Permissions: permissionCatalog})
}

func (s *Server) saveSecurity(writer http.ResponseWriter, request *http.Request) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	var payload securityPayload
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1024*1024)).Decode(&payload); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	previousSecurity := cfg.Security
	existingUsers := map[string]config.UserConfig{}
	for _, user := range cfg.Security.Users {
		existingUsers[user.Username] = user
	}
	if len(cfg.Security.Users) == 0 {
		hash, err := hashPassword(s.authState().password)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		existingUsers[s.authState().username] = config.UserConfig{
			Username: s.authState().username, PasswordHash: hash, RoleKey: "admin", Enabled: true,
			MustChangePassword: s.authState().password == "123456",
		}
	}
	users, err := normalizeSecurityUsers(payload.Users, existingUsers)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if current, ok := s.currentUser(request); ok {
		if existing, exists := existingUsers[current.Username]; exists {
			for _, user := range users {
				if user.Username == current.Username && user.PasswordHash != existing.PasswordHash {
					http.Error(writer, "当前登录账号请通过右上角“修改密码”功能修改密码", http.StatusBadRequest)
					return
				}
			}
		}
	}
	roles, err := normalizeSecurityRoles(payload.Roles)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if !hasEnabledAdmin(users, roles) {
		http.Error(writer, "至少需要保留一个启用的管理员账号", http.StatusBadRequest)
		return
	}
	cfg.Security = config.SecurityConfig{Users: users, Roles: roles}
	if err := config.Save(s.configPath, cfg); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	s.revokeChangedSecuritySessions(previousSecurity, cfg.Security)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]bool{"ok": true})
}

func normalizeSecurityUsers(input []securityUserPayload, existingUsers map[string]config.UserConfig) ([]config.UserConfig, error) {
	seen := map[string]bool{}
	users := make([]config.UserConfig, 0, len(input))
	for _, item := range input {
		username := strings.TrimSpace(item.Username)
		if username == "" {
			return nil, errText("用户账号不能为空")
		}
		if seen[username] {
			return nil, errText("用户账号重复: " + username)
		}
		seen[username] = true
		existing := existingUsers[username]
		passwordHash := existing.PasswordHash
		mustChangePassword := existing.MustChangePassword || item.MustChangePassword
		if strings.TrimSpace(item.Password) != "" {
			if err := validatePassword(username, item.Password); err != nil {
				return nil, errText("账号 " + username + "：" + err.Error())
			}
			hash, err := hashPassword(item.Password)
			if err != nil {
				return nil, err
			}
			passwordHash = hash
			mustChangePassword = true
		}
		if passwordHash == "" {
			return nil, errText("请为账号 " + username + " 设置密码")
		}
		roleKey := valueOrDefault(strings.TrimSpace(item.RoleKey), "viewer")
		enabled := item.Enabled
		if username == "admin" {
			roleKey = "admin"
			enabled = true
		}
		users = append(users, config.UserConfig{
			Username:           username,
			DisplayName:        strings.TrimSpace(item.DisplayName),
			PasswordHash:       passwordHash,
			RoleKey:            roleKey,
			Enabled:            enabled,
			MustChangePassword: mustChangePassword,
			MFAEnabled:         existing.MFAEnabled,
			TOTPSecret:         existing.TOTPSecret,
			RecoveryCodeHashes: append([]string(nil), existing.RecoveryCodeHashes...),
			TOTPLastCounter:    existing.TOTPLastCounter,
		})
	}
	if !seen["admin"] {
		return nil, errText("超级管理员 admin 不可删除")
	}
	sort.SliceStable(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users, nil
}

func (s *Server) revokeChangedSecuritySessions(previous, next config.SecurityConfig) {
	oldUsers := map[string]config.UserConfig{}
	newUsers := map[string]config.UserConfig{}
	for _, user := range previous.Users {
		oldUsers[user.Username] = user
	}
	for _, user := range next.Users {
		newUsers[user.Username] = user
	}
	oldRoles := rolePermissionFingerprints(previous.Roles)
	newRoles := rolePermissionFingerprints(next.Roles)
	auth := s.authState()
	auth.mu.Lock()
	defer auth.mu.Unlock()
	for token, session := range auth.sessions {
		oldUser, existed := oldUsers[session.Username]
		newUser, exists := newUsers[session.Username]
		changed := !exists || !newUser.Enabled
		if existed && exists {
			changed = changed || oldUser.PasswordHash != newUser.PasswordHash || oldUser.RoleKey != newUser.RoleKey || oldUser.MustChangePassword != newUser.MustChangePassword || oldUser.MFAEnabled != newUser.MFAEnabled || oldUser.TOTPSecret != newUser.TOTPSecret
		}
		if !existed && exists {
			// The first persisted security save replaces the environment fallback account.
			changed = true
		}
		if oldRoles[session.RoleKey] != newRoles[newUser.RoleKey] {
			changed = true
		}
		if changed {
			delete(auth.sessions, token)
		}
	}
}

func rolePermissionFingerprints(roles []config.RoleConfig) map[string]string {
	result := map[string]string{}
	for _, role := range defaultedRoles(roles) {
		permissions := uniqueStrings(role.Permissions)
		sort.Strings(permissions)
		result[role.RoleKey] = strings.Join(permissions, "\x00")
	}
	return result
}

func normalizeSecurityRoles(input []config.RoleConfig) ([]config.RoleConfig, error) {
	base := defaultedRoles(input)
	seen := map[string]bool{}
	for i := range base {
		base[i].RoleKey = strings.TrimSpace(base[i].RoleKey)
		base[i].Name = strings.TrimSpace(base[i].Name)
		base[i].Description = strings.TrimSpace(base[i].Description)
		base[i].Permissions = uniqueStrings(base[i].Permissions)
		if base[i].RoleKey == "" || base[i].Name == "" {
			return nil, errText("角色标识和名称不能为空")
		}
		if seen[base[i].RoleKey] {
			return nil, errText("角色标识重复: " + base[i].RoleKey)
		}
		seen[base[i].RoleKey] = true
	}
	sort.SliceStable(base, func(i, j int) bool {
		if base[i].BuiltIn != base[j].BuiltIn {
			return base[i].BuiltIn
		}
		return base[i].RoleKey < base[j].RoleKey
	})
	return base, nil
}

func defaultedRoles(roles []config.RoleConfig) []config.RoleConfig {
	result := append([]config.RoleConfig{}, roles...)
	ensureRole := func(role config.RoleConfig) {
		for i, item := range result {
			if item.RoleKey != role.RoleKey {
				continue
			}
			if role.BuiltIn {
				result[i].Name = role.Name
				result[i].Description = role.Description
				result[i].Permissions = role.Permissions
				result[i].BuiltIn = true
				return
			}
			return
		}
		result = append(result, role)
	}
	ensureRole(config.RoleConfig{RoleKey: "admin", Name: "管理员", Description: "拥有网关全部配置和维护权限", Permissions: []string{"*"}, BuiltIn: true})
	ensureRole(config.RoleConfig{RoleKey: "operator", Name: "运维人员", Description: "可维护网关采集、边缘计算、告警、历史数据、网络和云平台", Permissions: []string{"status.view", "collection_incidents.view", "collection_incidents.manage", "config.view", "config.manage", "collection.run", "points.write", "packet.view", "edge.view", "edge.manage", "alarms.view", "alarms.manage", "history.view", "history.manage", "offline.manage", "network.manage", "cloud.manage", "ai.use", "maintenance.view", "maintenance.diagnose", "maintenance.time", "maintenance.remote", "maintenance.restart", "maintenance.audit"}, BuiltIn: true})
	ensureRole(config.RoleConfig{RoleKey: "viewer", Name: "只读用户", Description: "可查看网关运行态、配置、采集异常、边缘计算和告警", Permissions: []string{"status.view", "collection_incidents.view", "config.view", "edge.view", "alarms.view"}, BuiltIn: true})
	return result
}

func hasEnabledAdmin(users []config.UserConfig, roles []config.RoleConfig) bool {
	adminRoles := map[string]bool{}
	for _, role := range roles {
		for _, permission := range role.Permissions {
			if permission == "*" || permission == "users.manage" {
				adminRoles[role.RoleKey] = true
				break
			}
		}
	}
	for _, user := range users {
		if user.Enabled && adminRoles[user.RoleKey] {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

type errText string

func (e errText) Error() string { return string(e) }

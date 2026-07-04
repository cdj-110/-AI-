package web

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

var permissionCatalog = []struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Group string `json:"group"`
}{
	{Key: "*", Name: "全部权限", Group: "系统"},
	{Key: "status.view", Name: "查看运行状态", Group: "运行状态"},
	{Key: "config.view", Name: "查看网关配置", Group: "智能网关"},
	{Key: "config.manage", Name: "管理网关配置", Group: "智能网关"},
	{Key: "network.manage", Name: "管理 WiFi/移动网络", Group: "网络"},
	{Key: "cloud.manage", Name: "管理云平台连接", Group: "云平台"},
	{Key: "maintenance.run", Name: "执行系统维护", Group: "系统维护"},
	{Key: "users.manage", Name: "管理用户和角色", Group: "用户管理"},
}

type securityPayload struct {
	Users       []securityUserPayload `json:"users"`
	Roles       []config.RoleConfig   `json:"roles"`
	Permissions interface{}           `json:"permissions,omitempty"`
}

type securityUserPayload struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Password    string `json:"password,omitempty"`
	RoleKey     string `json:"roleKey"`
	Enabled     bool   `json:"enabled"`
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
			Username:    s.authState().username,
			DisplayName: "系统管理员",
			RoleKey:     "admin",
			Enabled:     true,
		}}
	}
	users := make([]securityUserPayload, 0, len(security.Users))
	for _, user := range security.Users {
		users = append(users, securityUserPayload{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			RoleKey:     valueOrDefault(user.RoleKey, "viewer"),
			Enabled:     user.Enabled,
		})
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(securityPayload{Users: users, Roles: security.Roles, Permissions: permissionCatalog})
}

func (s *Server) saveSecurity(writer http.ResponseWriter, request *http.Request) {
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
	existingHashes := map[string]string{}
	for _, user := range cfg.Security.Users {
		existingHashes[user.Username] = user.PasswordHash
	}
	if len(cfg.Security.Users) == 0 {
		hash, err := hashPassword(s.authState().password)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		existingHashes[s.authState().username] = hash
	}
	users, err := normalizeSecurityUsers(payload.Users, existingHashes)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
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
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]bool{"ok": true})
}

func normalizeSecurityUsers(input []securityUserPayload, existingHashes map[string]string) ([]config.UserConfig, error) {
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
		passwordHash := existingHashes[username]
		if strings.TrimSpace(item.Password) != "" {
			hash, err := hashPassword(item.Password)
			if err != nil {
				return nil, err
			}
			passwordHash = hash
		}
		if passwordHash == "" {
			return nil, errText("请为账号 " + username + " 设置密码")
		}
		users = append(users, config.UserConfig{
			Username:     username,
			DisplayName:  strings.TrimSpace(item.DisplayName),
			PasswordHash: passwordHash,
			RoleKey:      valueOrDefault(strings.TrimSpace(item.RoleKey), "viewer"),
			Enabled:      item.Enabled,
		})
	}
	sort.SliceStable(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users, nil
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
	ensureRole(config.RoleConfig{RoleKey: "operator", Name: "运维人员", Description: "可查看状态并维护网关、网络和云平台配置", Permissions: []string{"status.view", "config.view", "config.manage", "network.manage", "cloud.manage", "maintenance.run"}, BuiltIn: true})
	ensureRole(config.RoleConfig{RoleKey: "viewer", Name: "只读用户", Description: "可查看智能网关运行态", Permissions: []string{"config.view"}, BuiltIn: true})
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

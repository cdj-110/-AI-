# 远程维护在线状态修复与本地部署（2026-10-13）

## 原因与改动

原弹框使用列表缓存的lastSeenAt与浏览器时钟判断，超过120秒误报离线；2秒轮询仅更新任务记录。网关实时心跳及授权正常，但页面旧记录未刷新。

- 新增超级管理员专用GET `/api/gateway-maintenance/gateways/:gatewayId/status`，只读返回最新心跳、服务器判定在线、授权激活/确认、维护通道和云连接策略；不返回设备密钥等敏感字段。
- 服务端统一维护状态函数，120秒心跳窗口同时用于状态接口与指令提交校验。浏览器时间偏差不再决定在线状态；异常超前心跳超过30秒拒绝判在线。
- 弹框每5秒同步状态与维护记录，同时更新对应列表心跳；连接在线与授权激活分别显示，附最新心跳、禁止操作原因。
- 初次请求显示正在检查；请求失败显示状态未知，不伪装离线，维护按钮禁用；离线、未激活、云连接被禁用、通道未就绪均保留后端拦截。
- 防轮询重叠、关闭/切换后旧响应覆盖，并清理定时器。
- 两个任务GET/POST旧URL保持兼容，角色仍仅SUPER_ADMIN；二次确认、先审计后发送、非retain MQTT、任务去重等安全规则不变。

## 验证

- API `npm run test:gateway-maintenance` 通过，MQTT发送全部为桩，不访问实机。
- `node scripts/verify-maintenance-status.cjs`：心跳新鲜度/120秒边界、在线与授权分离、通道/云策略、只读返回及密钥不外泄通过。
- API `node scripts/verify-tenant-user-action-permissions.cjs` 通过。
- Web `npm run build:all`：两端类型检查、构建通过；最终管理端Docker构建也通过。
- Web `node scripts/verify-maintenance-poll.cjs`：执行源码轮询逻辑，最新状态、列表心跳更新、失败处理、防重叠和关闭后拒绝旧响应通过。
- `git diff --check` 通过。未跑真实浏览器完整交互、真实重启/重启回执、网络长时间故障演练（not_run）；不得以状态检查代替实机重启验收。

## 部署

用户本轮明确请求“修改并部署”。目标现有本地Compose项目，已执行 `docker compose build api admin-web` 和 `docker compose up -d --no-deps api admin-web`。无数据库迁移、不部署网关、不重启ingest/数据库/EMQX。

管理端首页200；新增状态接口未认证401。部署容器代码读取截图网关WK20265AF2F5C6，服务器判定online=true、activated=true、authorizationState=ACTIVE，检查时心跳距服务器约12秒。只做读取，没有创建维护任务或向网关发送重启指令。

历史清理开关仍false。候选改动未提交，保留现有工作树。

回滚镜像已在构建前保存：`weikong-maintenance-rollback-api:20261013-113725`、`weikong-maintenance-rollback-admin-web:20261013-113725`；回滚只替换本次两服务制品，无须回滚数据库。回滚会恢复原缓存状态问题，需向用户说明。

# 租户停用与可选设备云连接限制

## 操作规则

- 管理平台 → 租户管理 → 编辑 → 停用，可勾选“同时停用全部设备云连接”。新操作默认不勾选；编辑已停用租户时保留已保存选项。
- 不勾选：只停用全部成员的平台访问；设备仍按其独立权限接入、上报。
- 勾选：额外禁止租户所有设备 MQTT 接入/发布/订阅、数据接收、远程配置、点位下发、远程维护及 OTA 下发/下载。断开已有 MQTT 和远程访问连接。网关本地采集与计算不受影响。
- 重新启用租户，或在停用状态下取消勾选，会解除租户级限制；设备等待自身自动重连，不伪造在线状态。设备单独禁用及子设备持久禁用记录不变。
- 已开始在网关本地执行的指令或安装不能撤回；应等待升级完成后再停用云连接。
- MQTT 主动断开失败时，接口保留已保存策略并返回失败明细；页面提示管理员检查连接服务后重新保存重试，不宣称所有连接已断开。

## 发布要求

先执行 API 的数据库迁移 `20260923090000_tenant_device_cloud_policy`，再同步发布 API、ingest、管理前端和租户前端。旧租户默认不禁用设备云连接。本次无需更新 EG100 固件。

## 连接管理认证维护（2026-09-23）

- 连接管理已改为只使用独立 API Key/Secret，不再回退到 Dashboard 管理账号登录。
- 线上凭据位于 API 持久化机密卷 `/var/lib/weikong/secrets/emqx-connection-api.json`，权限 600；EMQX 数据卷保留 `/opt/emqx/data/weikong-connection-api.json` 的受限恢复副本。文件内容不得写入日志或提交仓库。
- 专用密钥名：`weikong-connection-admin-20260923`。经明确批准，EMQX 6.0.0 使用 administrator 角色；这不是仅有断开连接权限的细粒度密钥。
- 到期时间：2026-10-23 09:15:53（Asia/Shanghai）。到期前必须轮换，不能自动延长获批有效期。
- 轮换时先创建新密钥、只读验证，再更新受限凭据文件，确认管理接口可用后撤销旧密钥。不要更改 Dashboard 管理员密码作为替代方案。
- `EMQX_API_CREDENTIALS_FILE` 指定文件时优先读取文件；缺失或格式无效时拒绝操作。文件变量设为空时才读取 `EMQX_API_KEY` 与 `EMQX_API_SECRET`。

## 回归验证

- API：构建后执行 `node scripts/verify-tenant-cloud-policy.cjs`。
- ingest：构建后执行 `node scripts/verify-tenant-cloud-policy.cjs`（隔离模拟，不连接真实设备或数据库）。
- 回归：tenant-accounts、device-cloud-access、gateway-child-cloud-restriction、device-commands、gateway-maintenance、ota-retention。
- 人工验收：用测试租户分别验证不勾选/勾选/恢复；检查旧登录会话被拦截、设备重连权限、设备单独禁用不被恢复、其他租户不受影响。

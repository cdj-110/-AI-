# 租户平台设备分组

## 范围与规则

- 入口在租户端设备管理页的筛选栏中，以紧凑下拉浮层展示全部设备、未分组和自定义分组；支持分组名搜索及设备名称/编号、类型联合筛选。新建、重命名和删除入口集中在浮层内，批量“调整分组”仅在勾选设备后显示。
- 首版是单层分组，一台设备最多归属一个组。可点击列表分组列单独调整，或勾选当前页设备批量调整（接口每次最多 100 台）。切换筛选/分页时清空选择。
- 租户管理员可以新建、重命名、删除分组和调整归属；租户普通成员仅查看/筛选。分组不是权限边界，不扩大或缩减现有点位权限。
- 删除分组会在事务中将组内设备置为未分组，再删除组。网关、子设备、采集、禁用状态和云连接均不改变。网关分组不自动继承给子设备。
- 新设备默认未分组，已有设备迁移后也全部保持未分组，不擅自按网关关系分类。

## 数据与接口

- `DeviceGroup` 按租户隔离；同一租户的去首尾空格名称唯一，长度 1–50 字符。
- `Device.groupId` 可空；复合外键 `(groupId, tenantId)` 保证数据库层面无法跨租户挂组。
- `GET /api/device-groups` 返回租户分组和设备数量。
- `POST /api/device-groups`、`PATCH/DELETE /api/device-groups/:id` 管理分组。
- `PATCH /api/device-groups/membership` 请求 `{ deviceIds, groupId }`，明确传 `null` 表示取消分组；缺失 groupId、重复 ID、超量设备均拒绝。
- `GET /api/devices?groupId=<uuid>` 筛选分组；`groupId=ungrouped` 查看未分组，不传表示全部。列表携带 group 名称。
- 分组写操作使用可串行化事务、有限并发冲突重试及操作日志；含外租户/已删除设备的批量操作整批拒绝。

## 发布顺序

1. 备份线上业务数据库。
2. 用 API 的 Prisma migrations 执行 `20260923110000_device_groups`（新增表/可空字段/索引，不删除现有数据）。
3. 更新 API、租户 Web；同步 ingest Prisma schema/client。此功能不需要 EG100 更新。
4. 验证租户管理员分组操作、普通成员只读和跨租户访问拒绝。

2026-09-23 已部署到云平台。首次发布因 Docker Desktop 的 AI/Model Runner 和 Secrets Engine 遗留运行时套接字导致引擎异常，迁移尚未启动时即停止；随后保留故障目录、关闭未使用的 Docker AI、恢复引擎，并清理未使用的构建缓存释放空间。恢复后重新完成加密备份、迁移和发布。

- 发布脚本：`tools/deploy-device-groups-20260923.cjs`。
- 已执行迁移：`20260923110000_device_groups`；初始 `DeviceGroup=0`、已分组设备为 0，原设备全部保持“未分组”。
- 已更新 API、租户 Web、ingest；admin-web、PostgreSQL、TimescaleDB、Redis、EMQX、OTA HTTPS 容器未替换，EG100 未更新。
- 已验证分组只读服务、未登录访问拒绝、租户与管理端入口、MQTT 订阅恢复、账号/租户/设备数量及受保护配置不变。
- 最终加密数据库备份：`secrets/production-batch-deploy-20260923032452`。
- 最终发布与回退记录：`artifacts/device-groups-deploy-20260923032445/deployment.json`、`compose.rollback.json`。
- Docker 恢复副本：`artifacts/docker-recovery-20260923/settings-store.json`；原 Docker 设置和两个故障运行目录另存于用户配置目录，未执行恢复出厂设置。

### 紧凑分组入口更新

2026-09-23 11:58（北京时间）已将固定左侧分组栏改为筛选栏下拉浮层并部署到租户平台。

- 仅重新构建并替换租户前端 `web`；API、管理平台、PostgreSQL、TimescaleDB、Redis、EMQX、ingest、OTA HTTPS 容器保持原实例，EG100 未更新。
- 已验证新静态资源包含分组搜索、设备分组和批量调整功能；匿名设备分组接口仍返回 401，HTTPS 代理配置检查及重载成功。
- 发布与回滚记录：`artifacts/device-groups-compact-ui-deploy-20260923035816/deployment.json`、`compose.rollback.json`。
- 原前端静态文件备份：`artifacts/device-groups-compact-ui-deploy-20260923035816/web.original.tar.gz`；原镜像回滚标签为 `wk-device-groups-compact-ui-rollback:20260923035816`。

## 验证

- API build、API/ingest Prisma schema 校验、ingest TypeScript 检查通过。
- `apps/api/scripts/verify-device-groups.cjs` 创建独立临时 PostgreSQL 16 容器，运行全部迁移并验证 DTO、角色、跨租户访问、数据库复合外键、同名冲突、原子批量调整、联合筛选与数量、删除组保留设备、删除租户级联。测试后仅移除该测试容器。
- `apps/web/scripts/verify-device-groups.mjs` 使用模拟接口验证新建/重命名、重复名及失败重试、批量调整、删除取消/确认、筛选、只读角色与小屏布局。
- 租户端 Vite 生产打包通过。对 `tsconfig.app.json` 执行完整类型检查仍存在原有错误：DeviceEventsPanel.vue、DeviceDetail.vue 的隐式 any，DeviceMap.vue 的可空 amap，Events.vue 的 EventItem.deviceKey，Register.vue 的旧 LoginResponse 类型。本功能新增文件和 Devices.vue 未报类型错误；没有把这些无关问题混入本次修改。

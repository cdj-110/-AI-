# 平台端数据中心（2026-10-13，已本地部署）

用户要求平台端增加数据中心，可选择设备/点位导出。开发完成后用户本轮明确请求“部署”，已部署到现有本地Compose项目，未发布远端。

## 功能与边界

- 租户侧独立“数据中心”菜单 `/data-center`，页面按需加载。
- 搜索并多选设备，逐设备选择点位或全选；只显示启用且未忽略的云点位；设备候选每次最多100个，可通过关键词继续搜索。
- 时间范围、最近1小时/24小时/7天、套餐期限和宽限时间展示；日期超出保存范围禁选，查询/导出时重新读取保存期限。
- 原始历史记录服务端分页，缺失值展示“—”，不补0；不同设备记录不强制拼接。
- CSV UTF-8 BOM下载，可用Excel打开，复用现有防公式注入处理；每次最多2000条。超过上限在查询后导出前二次提醒，导出响应仍明确提醒截断，不能把部分数据冒充完整导出。
- 单次最多20点位、跨度31天、分页最多100行；保存半年/一年并非单次全量导出，需要分段。大批量异步导出/XLSX任务尚未实现。
- 新端点GET `/api/devices/data/retention`、POST `/api/devices/data/history`、POST `/api/devices/data/export`；DTO验证嵌套UUID/点位标识、ISO时间、数量、分页。
- 沿用DEVICE_VIEW与现有设备范围，不要求SCADA_VIEW，不新增独立权限或改变权限存储。限TENANT_ADMIN/TENANT_USER，服务端每个查询/导出页仍校验租户、设备可访问范围、点位启用状态及保存期限；前端隐藏不是安全边界。
- 查询筛选发生变化时清空旧结果；异步响应有代次检查，离开页面后不应用旧设备/点位/查询响应。
- 本次只更新本地API与tenant web，不执行迁移、费用变更、设备控制、数据清理；不改变网关本地历史配置。

## 候选改动

`apps/web/src/views/DataCenter.vue`、tenant router/MainLayout菜单；API devices controller/HistoryQueryDto，复用已部署的历史保存、查询、导出服务。未创建提交，保留已有工作树。

## 验证

- API `npm run build`、`node scripts/verify-data-center.cjs`：通过，含DTO拒绝空/超额/无效点位及时间、租户角色、权限与actor传递。
- API `node scripts/verify-history-retention.cjs`、`node scripts/verify-tenant-user-action-permissions.cjs`：通过，保留既有越权/保存范围/查询参数保护测试。
- Web `npm run build:all`：租户与管理端类型检查和构建通过；数据中心独立懒加载块。
- `node scripts/verify-data-center-ui.cjs <bundled-playwright-path>`：独立Chrome浏览器、全部API模拟。TENANT_USER只有DEVICE_VIEW而无SCADA_VIEW，选择两设备、筛选启用点位、正确查询payload、实际CSV下载事件、无页面异常通过；桌面1440及手机390像素截图，手机页面无整体横向溢出，表格横向滚动。
- 截图：`outputs/data-center-20261013/desktop.png`、`mobile.png`（模拟数据，不是线上用户数据）。
- `git diff --check`：通过。
- 新端点真实登录HTTP闭环、大量设备/千万级数据性能、异步大文件导出：not_run；没有用浏览器模拟测试冒充上线验收。

## 部署和回滚

下一次用户明确请求部署时更新API和tenant web即可，无数据库迁移。验证新端点角色/设备范围与实际导出，并保留旧制品。回滚恢复上一版API及tenant web，菜单页面一并恢复，不改动现有历史数据。

## 本地部署验证（2026-10-13 13:22 +08:00）

- 构建 `docker compose build api web` 成功，容器内Prisma生成/API构建与tenant类型检查/构建通过。
- 更新 `docker compose up -d --no-deps api web` 成功；未迁移数据库，未重启管理端、GoView、采集服务、数据库、EMQX或网关。
- 回滚镜像：`weikong-data-center-rollback-api:20261013-131448`、`weikong-data-center-rollback-web:20261013-131448`。
- 新API镜像sha256前缀 `b41560574e75`，tenant web `60436c55ff92`。
- 实际 `/data-center` 页面HTTP200，三个新接口未认证均401。
- 隔离Chrome读取5174实际部署页面资源、所有业务API模拟：两设备点位选择、查询、CSV下载、无页面错误及手机无整页横向溢出通过，截图 `outputs/data-center-20261013-deployed/`。不冒充真实登录HTTP闭环。
- 部署容器实际编译的controller/DTO角色权限转发检查通过。
- 容器内只读抽查实际历史服务通过（分页10条、图表100条、CSV内容在内存中300条），超保存范围拒绝；未创建登录会话，未保存或输出真实数据内容。
- `HISTORY_CLEANUP_ENABLED=false`；近期API日志未匹配错误或清理日志，现有历史数据未删除。
- 实际用户登录HTTP全流程和大批量性能仍not_run，按需验收；不因此扩大网关控制权限。

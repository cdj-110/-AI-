# 云历史数据保存与组态查询（2026-10-13 候选）

## 行为

- 套餐名称不改：TRIAL 体验版7天、BASIC 基础版6个自然月、ENTERPRISE 企业版9个自然月、PROFESSIONAL 专业版12个自然月。
- Asia/Shanghai 自然月，月底截到目标月最后一天；不是月份乘30天。
- 管理端可配置套餐保存天数/自然月（二选一，不能无限）；缩短期限通知用户并保留7天查询/导出保护。
- HISTORY 扩展每份3个月，可叠加，开通时绑定当前订阅到期日。普通套餐续费不自动延长已有扩展包。订单保存月份快照，避免商品数量编辑影响已提交订单。累计扩展上限1200个月。
- 默认扩展商品为未销售、价格待定；后端拒绝下单，不能以0元商品绕过。待财务定价后再由管理端启售。
- 订阅到期恢复体验版7天历史期限；历史扩展到期恢复套餐期限。缩短后的超期数据临时可查询/导出7天；升级不恢复已清理数据。
- 到期前30/7/3/1天通知，到期及清理宽限结束通知；去重键绑定实际到期日期，续费不会吞掉下一周期提醒。
- API、组态历史表格、图表、设备趋势、首页趋势使用相同保存期限；服务端设备授权不依赖前端隐藏日期。最多20个点位，单次跨度31天，分页100条；保存12个月不代表一次查询12个月。
- 图表按时间范围自动生成时间桶并取平均值（约500个设备时间桶）；表格显示原始记录，缺失值为null，不补0。图表非数字点位不进入平均值序列。
- 编辑绑定、预览图表和高级表格禁用超期日期/未来日期；失效的旧时间范围调整到有效范围内最近1小时并提示。基础表格临时调整查询，不自动覆盖用户保存配置。
- 高级表格CSV导出同样限制时间和设备权限，单次最多2000条，超限明确提示分批导出；文本防CSV公式注入。
- 当前云平台没有独立的“点位历史记录开关”，因此不伪造“未开启历史”的判断；无数据提示引导检查云端上报/保存。网关本地历史开关与云端历史不是同一功能，本次未改网关。

## 清理安全

- `HISTORY_CLEANUP_ENABLED` 默认false；本次没有启用或删除任何现有业务数据。
- 超出套餐期限的数据额外保留7天再进入删除范围。权益缩短另有固定7天保护边界，期间不删原可查询范围。
- 启用还需填写带时区的 `HISTORY_CLEANUP_ACTIVATED_AT`，首次清理必须在此时间7天以后。
- 管理端客户订阅“清理影响”只读取影响行数/时间边界；API `GET /api/subscriptions/customers/:tenantId/history-cleanup-impact` 限超级管理员。
- 开启后每小时运行，单批1000条、单租户每轮最多5批；按租户设备UUID删除，绝不全库drop_chunks。权益变更和清理使用业务行锁互斥，批次重新读取权益。
- 无归属设备/仅有旧deviceKey的历史记录不自动删除，须另行归属审计；备份保留规则本次未改。
- 首次启用需先备份并复核影响报告，再明确批准环境、制品、启用日期。时序库删除与业务库行锁不是分布式事务，数据库故障须监控维护日志。

## 变更及验证

变更范围：API Prisma追加迁移、subscriptions保存/扩展/提醒/清理、devices与dashboard查询；web套餐/扩容/订阅/清理影响；GoView时间边界/历史图表/高级表格导出；compose清理环境开关。保留既有工作树，未提交；已按用户请求部署到本地Docker，未部署远端生产环境。

- `apps/api: npm run prisma:generate; npm run build`：通过。
- `apps/api: node scripts/verify-history-retention.cjs`：自然月/套餐/扩展到期/7天保护/授权/查询参数/禁售/清理关闭开关，通过。
- `apps/api: npm run test:tenant-user-action-permissions`：通过。
- `apps/api: node scripts/verify-history-retention-db.cjs`：一次性独立TimescaleDB容器，实际追加迁移、套餐名称、待定价商品、分页、AVG、CSV、租户隔离删除SQL通过；测试容器已移除。没有连接现有数据库。
- `apps/web: npm run build:all`：最终两套类型检查和构建通过。
- `apps/goview-designer: npm run build-cli`：最终构建通过；此命令不包含完整类型检查。
- `apps/goview-designer: node tools/test-history-retention-window.cjs; node tools/test-history-chart.cjs`：时间边界/快捷范围/会话隔离/旧范围调整/四个Vue组件编译与图表逻辑通过。
- 浏览器真实登录/支付回调/到期通知长时间运行/大数据性能/生产迁移：not_run。没有伪造完整端到端验收。

## 部署与回滚

部署前需批准 `20261013090000_history_retention` 追加迁移；先备份，再迁移业务库，再更新API和两端web及GoView。本次用户请求重新部署后已执行本地追加迁移，见下方记录。

回滚先关闭清理开关、恢复上一版API/web/GoView制品；追加数据库列保留，不做破坏性删列。已物理删除数据只能从经确认的备份恢复，不能通过升级套餐恢复。

## 本地部署结果（2026-10-13 11:01 +08:00）

- 用户授权：本轮“重新部署”；目标为现有本地Compose项目，不涉及远端/网关发布。
- 构建：`docker compose build api web admin-web goview-designer` 成功。
- 迁移检查：仅本次历史期限迁移待执行，无失败迁移。`docker compose run --rm --no-deps api npx prisma migrate deploy` 仅应用 `20261013090000_history_retention`，成功。
- 更新：`docker compose up -d --no-deps api web admin-web goview-designer` 成功；数据库、ingest、EMQX未重启。
- 部署镜像：API `9f5c946dc15f`，tenant `a8915753b4db`，admin `83d0ec2df35f`，GoView `e6d054aa47e7`（均为sha256前缀）。
- 备份：项目Git目录外 `C:\Users\18507\Documents\物联网平台\deployment-backups\history-retention-20261013-104949`；业务库custom dump 234235字节，`pg_restore -l`检查成功；四个旧容器文件系统tar已导出。旧Docker镜像底层digest缺失，tag/commit失败，故回滚备份采用rootfs导出，不虚报旧镜像tag成功。恢复rootfs时需导入镜像并设置原启动命令（API `/app`下`node dist/main.js`；前端 `/docker-entrypoint.sh`启动`nginx -g 'daemon off;'`），仍复用现有Compose卷与环境；不要直接恢复业务库覆盖新数据。
- HTTP：5174租户端、5176管理端、8080编辑器及5174 `/api/changelog` 返回200。
- 实际配置：体验版7天、基础版6月、企业版9月、专业版12月；扩展每份3月，pricePending=true、enabled=false。
- 容器内只读验证脚本 `apps/api/scripts/verify-history-retention-deployed.cjs` 通过：实际套餐/权益、清理影响统计、历史分页、AVG图表、CSV导出和超期范围拒绝。样本返回10条分页数据、100条图表桶、451条导出记录；未输出内容、未创建登录会话。
- 新历史边界GET与导出POST接口未认证均返回401；API近期日志未匹配错误或清理日志。
- `HISTORY_CLEANUP_ENABLED=false`、`HISTORY_CLEANUP_ACTIVATED_AT`为空。未启用清理，未删除现有遥测数据。后续开启清理仍须先审核影响报告并明确启用日期。
- 未测：真实浏览器登录支付闭环、长期提醒、大数据性能及远端生产环境；这些不因页面200被视为验收。

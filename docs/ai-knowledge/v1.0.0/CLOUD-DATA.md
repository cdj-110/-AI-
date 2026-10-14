# 云平台与数据基线
版本：1.0.0｜状态：事实与冲突并存

## 已确认：云平台
S-06 的 apps/web/package.json 声明 Vue3、TypeScript、Vite、Element Plus、Pinia、Vue Router、Axios、ECharts，管理端和租户端分别构建。S-05 README 描述 ADMIN 与 TENANT 两套入口，共用 API。端口是文档/开发配置，不是生产地址证明。
apps/api 声明 NestJS 10、Prisma 6、JWT/Passport、bcryptjs、ioredis、mqtt、pg、ws。apps/ingest 是独立 TypeScript 消费服务。本次只查清单和若干源码标记，未核实全部依赖锁定版本。
本次目录检查包含 auth、devices、factory-gateways、products、subscriptions、alarms、remote-config、remote-access、gateway-maintenance、ota、scada-projects 等；目录存在不等于各模块验收。

S-17：GoView 设计器源码于 2026-10-08 替换，保留平台鉴权/API 和数据对接。编辑态静态展示，预览/发布态保留数据更新。不能将上游新增组件或历史查询接口自动记为平台支持。

## 数据库冲突：必须同时保留
| 证据对象 | 业务数据 | 时序数据 | 状态 |
|---|---|---|---|
| S-02 用户近期说明 | MySQL | InfluxDB | 已确认用户口径；部署待确认 |
| S-07 当前本地 Prisma | PostgreSQL | 不由此推断 | 已确认文件事实 |
| S-08 ingest/Compose | PostgreSQL 连接配置 | TimescaleDB pg Pool、hypertable | 已确认文件事实 |
| S-18 2026-09-19 记录 | PostgreSQL | TimescaleDB | 已确认存在历史部署记录；未复验 |
| 当前生产/GitHub | 未读取 | 未读取 | 待确认 Q-01/Q-02 |

网关 go.mod 存在 MySQL 驱动，只能说明网关依赖；不能用它证明云业务数据库采用 MySQL。未读到 InfluxDB 实现不等于整个产品没有 InfluxDB，亦可能属于不同项目或历史版本。

本地 Compose 声明 postgres:16、timescale/timescaledb:latest-pg16、redis:7、emqx/emqx:latest；这些是配置标签，不是实际镜像摘要。EMQX 6.0.0 仅见 S-16 历史记录。禁止把 latest 推断为具体版本。

## 数据流及边界
按当前本地文件整理的链路（生产映射待核对）：
现场设备 → Go 网关采集 → MQTT/EMQX → ingest → TimescaleDB；
浏览器 → NestJS API → Prisma/PostgreSQL；
API/实时流 → 前端/组态；下行命令另经权限、目标设备和任务状态检查。
Redis 用途、队列语义、所有实时链路及直接 MQTT 设备路径尚未完整审计。

S-09 的主题约定：
- 网关心跳：weikong/devices/{gatewayKey}/heartbeat
- 子设备心跳：weikong/gateways/{gatewayKey}/children/{childKey}/heartbeat
- 子设备遥测：weikong/gateways/{gatewayKey}/children/{childKey}/telemetry
文档要求网关凭证只代发绑定子设备。主题里的设备标识不能替代后端归属校验。

S-18 记录启动恢复逻辑：等待 TimescaleDB 初始化，失败持续重试；MQTT 新连接重新订阅，旧连接重试任务终止。不能仅凭 MQTT 在线推断消费正常。

## 待确认的契约
需用实际代码和脱敏载荷补齐：设备/点位标识唯一范围、源时间和接收时间、时区、数值类型、质量码、重复消息、乱序、空值、删除及设备转移后的历史归属。检查源码中的 device_id+time 唯一索引与真实业务是否匹配，不能假定它覆盖所有幂等需求。
MySQL/InfluxDB 若仍为目标方案，需单独 ADR 说明版本、迁移边界、兼容性、回滚和数据验收；本包不推荐或执行迁移。

## 建议的研发回归范围（草案）
云前端验证管理/租户两套构建、登录、权限菜单、异常提示和组态保存/预览；API 验证跨租户访问、设备范围、MQTT ACL、幂等与事务；ingest 验证数据库未就绪、重连、权限变更和非法归属消息。运行命令以各目录现有 scripts 为准，不能编造仓库根 npm test。


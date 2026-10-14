# 云平台与数据基线 V1.0.1

## 已确认
用户于本聊天明确确认：Go 网关可向 MySQL 写入采集数据；云平台使用 PostgreSQL 存储业务数据、TimescaleDB 存储时序数据。Q-01 已关闭，不需要数据库迁移。
本地 apps/api 与 apps/ingest 的 Prisma provider=postgresql，ingest 使用 pg Pool 和 TimescaleDB，和用户确认一致。EMQX 承担 MQTT 接入；具体运行版本未核实。

## 模块与边界
- apps/web：Vue3/Vite/TypeScript，管理端 build:admin、租户端 build:tenant。
- apps/api：NestJS/Prisma 云 API；apps/ingest：独立 TypeScript 采集消费。
- apps/gateway-go：Go 网关，含独立 webui；MySQL 驱动用于网关输出，不代表云业务库选型。
- apps/goview-designer：独立 GoView 设计器，Linux 构建入口 build-cli；它与主前端版本不同。
- apps/gateway-python：并行网关，是否长期维护仍待确认，不能自动删除。

当前文件描述的数据主链：设备 → Go 网关 → MQTT/EMQX → ingest → TimescaleDB；浏览器 → NestJS → Prisma/PostgreSQL。网关直接向 MySQL 输出是独立路径。每条生产链路的实际版本和部署拓扑仍需 Q-02 对齐。

## 待确认
生产提交、镜像摘要、数据库精确版本、保留周期、备份恢复和性能目标。数据契约仍需核对标识范围、时间戳、质量码、乱序/重复、设备转移后的历史归属。不得把模型或脚本存在视为完整业务验收。

## 历史口径
旧聊天的“云平台 MySQL/InfluxDB”不再适用于当前架构；没有证据支持推断曾经执行数据库迁移。网关支持 MySQL 与其他数据库的范围需分别验证。

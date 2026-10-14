# 证据索引与访问记录
版本：1.0.0｜读取日期：2026-10-09（Asia/Shanghai）

## 访问范围
已读取相关聊天、本机项目选定文档/清单/源码标记及本地 Git 状态。未连接 GitHub 远端，未 fetch/pull/push；未查询服务器、数据库、容器运行状态或实机；没有执行原文中的部署指令。
本机项目定位：C:/Users/18507/Documents/物联网平台/weikong-iot-platform。
本地 HEAD：02e1ff59f86bc424855dcf99f4a6c066534f0cfd。已跟踪变更条目 458，未跟踪文件未纳入该统计。指纹记录另附 source-fingerprints.json，不将当前工作树内容归属于 HEAD。

路径均相对上述项目根，导入其他仓库后需核实映射。聊天链接只有有权限用户可访问。

| 来源 ID | 资料 | 本次读到的范围/限制 |
|---|---|---|
| S-01 | [方案区别比较](chatgpt-conversation://6ac860c7-8668-83e8-b506-9256c8d2a550) | 用户明确 GitHub 和最终人工审批；助手正文仅引用占位，无附件 |
| S-02 | 本地聊天 01a11ebd-a783-7120-bc1f-980501faf4ec，用户消息 01a11ebd-f65a-7e60-b8a7-b67ac36e8e04 | 用户列 Vue3/Vite/TS、NestJS、Go、MySQL、InfluxDB、EMQX、2核2GB；助手方案仅为建议 |
| S-03 | [系统设计说明书](chatgpt-conversation://6a842434-5560-83ee-ad9b-06592c8e76c1) | 用户曾指定 Vue3/TS5/Vite5 和 Python FastAPI 云后端；附件正文未取得 |
| S-04 | [物联网平台收费建议](chatgpt-conversation://6ab47f7e-77a0-83e8-8e5f-7f31f144bed4)，用户消息 8fa44eff-c9a2-46b8-818a-934e51a1152f | 明确选择买网关赠送可配置额度；助手价格不是已批准价格 |
| S-05 | README.md | 骨架说明、双前端、旧 ingest 占位描述；部分已过时 |
| S-06 | apps/web/package.json；apps/api/package.json；apps/ingest/package.json | 声明依赖及 scripts；未安装、构建或测试 |
| S-07 | apps/api/prisma/schema.prisma；apps/ingest/prisma/schema.prisma | provider=postgresql；API 商业模型名称；未完整审计模型逻辑 |
| S-08 | docker-compose.yml；apps/ingest/src/main.ts | 镜像标签、TimescaleDB 变量/Pool/初始化/索引标记；不收录凭据值 |
| S-09 | apps/gateway-go/README.md；apps/gateway-go/go.mod | 网关主题、协议文档、缓存规则、Go/依赖声明；README 有过时段落 |
| S-10 | apps/gateway-python/README.md | 明确 Python 并行网关及未迁移清单 |
| S-11 | apps/gateway-go/internal/collector、internal/web、internal/forward、internal/runtime 的选定协议文件清单 | 只用于“文件存在”的事实；没有逐个审计实现或执行测试 |
| S-12 | apps/gateway-go/docs/iec61850_mms_build.md；apps/gateway-go/docs/opcda_t02_data_integrity_report.md | 构建标签、原生依赖及历史 T02 测试边界 |
| S-13 | docs/gateway-authorization-security.md | 授权设计、v2、严格构建、备份与历史测试限制；省略身份/路径细节 |
| S-14 | docs/license-file-format-20260917.md | .lic 下载名和 .json 兼容、历史验证 |
| S-15 | docs/tenant-member-permissions-and-device-scope-20260923.md | 功能权限、动态设备范围与测试入口 |
| S-16 | docs/tenant-cloud-policy.md | 租户/设备停用、EMQX 管理权限和历史有效期；未读凭据文件 |
| S-17 | docs/goview-replacement-20261008.md | 替换、后续静态编辑及历史部署记录、业务验收限制 |
| S-18 | docs/ingest-startup-recovery-20260919.md | 启动恢复及历史部署记录；未读取其引用的原始部署报告 |
| S-19 | docs/product-catalog-20260917.md | 产品库与后续 EG100 规格纠正；未核实 BOM |
| S-20 | docs/gateway-remote-maintenance-20260921.md | 维护任务、身份/时限/去重及部署前置说明；不推断后续完成 |

## 证据抽取规则
聊天仅取可见用户表述作为需求确认；源文档中“已部署”“测试通过”均保留为历史报告级别。本包不验证原报告真实性，也不复制其中秘密、设备身份或账户数据。
不同时间记录出现差异时，后续明确更正仅替代同一范围的旧口径；跨环境差异保留待确认。

本轮未读取整个 ChatGPT 项目的全部历史与附件，也未访问商业建议中的竞品网页。竞品价格未被引入基线，避免将旧建议当作当前市场事实。


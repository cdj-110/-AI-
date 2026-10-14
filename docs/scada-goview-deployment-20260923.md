# 云组态与原生 GoView 设计器

2026-09-23 已部署到云平台。

- 租户平台“云组态”保留项目卡片；“设计”和“预览”直接打开原生 GoView 路由，不再使用替代设计页面。
- 新增独立 `goview-designer` 容器，局域网入口为 `http://192.168.1.141:8080/`。
- API 新增租户隔离的组态项目、GoView 配置兼容、设备与点位选择接口；设备范围继续受当前账号权限约束。
- 已执行数据库迁移 `20260923190000_scada_projects`。迁移只新增 `ScadaProject` 表和索引，发布后项目数为 0。
- 生产数据库加密备份位于 `secrets/production-batch-deploy-20260923084718`，加解密校验通过。
- 发布记录与旧程序文件备份位于 `artifacts/scada-goview-deploy-20260923-164737/`。
- API、租户平台、GoView 首页及局域网入口检查通过；原生 GoView 工作空间已在浏览器中实际渲染验证。
- PostgreSQL、TimescaleDB、Redis、EMQX、ingest、管理平台和 OTA HTTPS 容器未替换；原账号、租户、设备和出厂网关数量校验不变。


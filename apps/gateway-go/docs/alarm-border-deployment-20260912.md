# 严重告警黑边修正部署

- 2026-09-12 11:11 部署 EG100（192.168.1.214）。左侧强调边始终使用用户设置的严重告警原色，其他边框在浅色与原色间闪烁；不再使用文字深色。静音、拖动和卡片内容不变。
- `verify-alarm-card-border.mjs`、前端类型检查及构建通过。
- 正式 ARMv7 / CGO SQLite、opcda、iec61850_mms 程序 SHA-256：`31ea8f570d2169c2e0ad1ffa1cb6eb6895251e3b1e2979cfac66acab7a230626`。
- 原程序、配置、2986 条告警历史已备份至本机 `build/alarm-border-backup-20260912/` 和设备 `/mnt/udisk/.weikong-deploy/alarm-border-backup-20260912/`；备份校验通过，配置哈希未变化。
- 11:11:11 服务启动，健康及就绪检查通过，52 个点位调度启动（非所有点位质量 GOOD 的断言）。
- Web 登录通过，线上 `index-zxGguzRh.js`、`index-DNGLvQj7.css` 与本地构建逐字节一致，四个 WAV 校验通过。3 条规则策略保留，历史 2990 条；12 次活动告警快照排序稳定。
- 用户需刷新告警面板标签页以加载新版样式。

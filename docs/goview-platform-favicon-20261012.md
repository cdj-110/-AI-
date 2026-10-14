# GoView 标签页图标统一

候选：backup-20260624 现有工作树，未提交、未部署。

GoView 入口的 favicon 和 apple-touch-icon 改为用户端平台相同的蓝色云形 PNG；原样复用 apps/web/src/assets/tenant-platform-logo.png，在设计器 public 内保存独立副本以兼容独立 Docker 构建。文件字节及 SHA256 相同。编辑与预览为同一 SPA 入口，共享此引用；新的图标文件名避免沿用旧 logo.png 缓存。

Windows 验证：npm run build-cli 通过（不包含类型检查，保留既有打包警告）；node tools/test-platform-favicon.cjs --dist 通过，确认源码/构建 HTML 引用、平台/设计器/构建 PNG 字节一致。

未测：部署后浏览器标签页视觉与各浏览器 favicon 缓存刷新。风险：已打开的旧标签页可能需重新打开或刷新才能显示新图标。不改运行组件、平台管理端图标或真实业务数据。

回滚：恢复 index.html 的原图标引用，仅移除此次新增的 tenant-platform-favicon.png，不覆盖此前功能。

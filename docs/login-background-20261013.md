# 两平台登录背景统一

候选：在已授权 backup-20260624 分支继续工作，保留 Login.vue 中原有多方式登录、验证码、记忆用户名、品牌区分和会话恢复修改。未正式提交、未部署。

仅追加 Login.vue 的 scoped .auth-page:not(.embedded-login) 背景样式，复用网关 internal/web/auth.go 现有登录背景：左上20%/10%的浅蓝光晕、32%渐隐、135度蓝灰渐变（#f8fbff到#eef4fb）。登录卡片与其阴影不变；内嵌重新登录保持白底；注册、找回密码及网关不变。不生成新背景资产、不改变认证或业务数据。

验证：npm run build:all 两端类型检查/构建均通过；verify-login-background.cjs 浏览器验证背景一致、客户端必填校验、登录方式切换、375×812无横向溢出和内嵌白底；所有API模拟，无真实账号操作/页面错误。截图对照已检查，apps/web/design-qa.md final result passed。临时预览5434用户端/5436管理端保留供用户查看，不修改原部署服务。

未测真实认证与完整跨浏览器/可访问性；既有大包警告保留。回滚只移除本批新增5行 scoped 背景规则，不能整体恢复 Login.vue，否则会覆盖已有功能。

## 用户授权部署

2026-10-13 按“重新部署”执行 docker compose up -d --build --no-deps web admin-web，两前端镜像构建/类型检查成功，5174用户端和5176管理端正常运行；API未重启，未进行迁移或业务数据写入。两端nginx -t通过。verify-login-background.cjs --deployed 通过：部署页面背景一致、空表单校验/模式切换、手机无横向溢出、内嵌登录仍为白底，无pageerror；截图保存在 outputs/login-background-20261013-deployed。真实认证未测试。

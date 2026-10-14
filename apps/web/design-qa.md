# 登录背景对照检查（2026-10-13）

## 范围与来源

仅将管理端/用户端独立登录背景与用户提供的网关登录截图统一，保留两端已有卡片、图标、文案、间距和认证交互；不是将网关整个登录表单移植到平台。复用现有网关 auth.go 的原生背景声明，不新建背景图、字体或图标。

Source visual truth: ../../outputs/login-background-20261013/source-gateway.png。
Implementation screenshots: ../../outputs/login-background-20261013/{tenant,admin}-desktop.png 与 {tenant,admin}-mobile.png。
Full-view comparison evidence: {tenant,admin}-comparison.png，左网关参考，右对应实现。
Focused region comparison evidence: {tenant,admin}-background-detail.png，左参考与右实现各取原图左上600×360背景，验证渐变颜色和位置，无缩放。

桌面源图1920×911像素；实现1920×911 CSS与像素，deviceScaleFactor=1；并列比较3840×911像素。移动实现375×812 CSS与像素。状态为未登录、空表单、密码登录。参考账号输入有焦点，平台不自动加焦点；比较目标为背景，该控件差异不作为问题。

## 必需检查

- 字体/文案：沿用现有平台字体、标题和表单文案；管理端与租户端品牌及登录说明不同属于既有产品差异，不复制网关文案。
- 布局/间距：沿用平台居中卡片与既有高度；平台功能更多，卡片比网关高，属于本次保留范围。手机卡片完整可用，无水平溢出。
- 颜色/令牌：复用 radial-gradient(circle at 20% 10%, #dbeafe 0, transparent 32%) 与 linear-gradient(135deg, #f8fbff, #eef4fb)。实际管理/用户端 computed backgroundImage 一致；并列全图和局部背景检查未见可执行漂移。
- 图片资产：平台已有云形/W品牌素材不变，不重画或替换参考网关图标。背景来自同项目网关现有代码，不以截图作为页面背景。
- 交互/内容：空表单必填校验、密码/验证码方式切换通过。/session-login 背景为纯白且无渐变。未更改账号、验证码和认证业务逻辑。

## 验证与限制

npm run build:all 完成 tenant/admin 两次 vue-tsc --noEmit 和Vite构建，无新增构建错误（既有依赖注释、大包警告保留）。独立Chrome无头浏览器、临时5434/5436预览，所有API拦截为模拟响应，未提交真实账号或登录操作，pageerror为空。

node scripts/verify-login-background.cjs <bundled-playwright-path> 通过。已实际打开上述6张比较/手机图后评审，背景范围无P0/P1/P2问题。初次测试未等待异步表单错误出现而断言失败，补充等待后通过；未因此变更产品视觉，无视觉修复循环。

未测试真实密码/验证码认证、完整可访问性或其他浏览器。未部署。注册/找回密码页与网关代码不变。

final result: passed

# 待激活页顶部品牌区局部检查

实机部署完成（2026-09-16 17:25）：本轮最终样式已部署到 EG100，顶部白栏贯穿视口，桌面和手机实测均为 56px。发布程序 SHA256 为 `34dd9008c33d3011be0778baab7bd97f534be53cbd2b2aa01dbaa5992a60626a`；更新前备份目录 `/mnt/udisk/.weikong-deploy/remote-access-auto-backup-20260916-172329`。原程序、配置及 6 个数据文件备份校验通过；配置哈希未变、点位数仍为 52。健康和就绪检查通过，保持未激活、采集停止，无效签名授权仍被拒绝。独立 Chrome 实机检查通过：白栏宽度/高度、匿名待激活入口、复制身份、导入前登录、退出返回公开入口、手机无溢出、无页面错误。实机截图：`../artifacts/license-unactivated-20260916/eg100-desktop.png` 和 `../artifacts/license-unactivated-20260916/eg100-mobile.png`。以下“尚未部署”仅为此前迭代的历史记录。

高度统一更新：用户要求与网关内部一致，桌面及手机顶部栏均调整为 56px（与内部 `.brand` 高度相同）。构建及独立浏览器测试通过，新增桌面/手机 56px 高度断言；最新截图沿用 `desktop-full-width-header.png` 和 `mobile-full-width-header.png`。尚未部署到 EG100。

最新迭代：用户进一步明确要求“通长的白条”。已将顶部改为贯穿整个视口的白色栏，桌面高度 64px、手机高度 60px，无圆角、无外侧留白；Logo 和中英文品牌名称保持左侧紧凑布局，中央激活卡片及权限流程不变。构建及独立 Chrome 回归通过，检查白底、左边界 0 和完整视口宽度，桌面/手机截图已人工检查。最新截图为 `../apps/gateway-go/webui/artifacts/activation-page/desktop-full-width-header.png` 和 `../apps/gateway-go/webui/artifacts/activation-page/mobile-full-width-header.png`。尚未部署到 EG100。以下记录保留之前的设计迭代。

范围仅为用户标注的顶部区域，不涉及重做激活页或调整权限。参考证据是用户本轮提供的截图；浏览器工具不可用后，经用户允许改用独立 Chrome 验证本地构建页面。

1. 原顶部区域：标注区域为宽约 1240px 的白色横幅，信息只有左侧品牌，视觉重量和信息量不匹配，与渐变背景及 650px 激活卡片脱节。源码确认继承了全局 `header` 的白底、底边、吸顶和背景模糊。
2. 按用户最新要求调整后的桌面状态：页头容器透明、无边框、无吸顶、无模糊；左上角使用 218×56px 的白色横向品牌条，沿用网关内部的 34px 图标及紧凑中英文文字比例，距页面左侧 24px，不再与中央激活卡片对齐。截图：`../apps/gateway-go/webui/artifacts/activation-page/desktop-white-brand.png`。
3. 修改后手机状态：品牌条宽度调整为 198px、距左侧 14px；设备身份可折行，操作按钮没有横向溢出。截图：`../apps/gateway-go/webui/artifacts/activation-page/mobile-white-brand.png`。

验证：前端构建和独立浏览器回归通过；计算样式明确检查页头容器透明背景、0px 底边、static 定位、无模糊，以及品牌条白底、尺寸和桌面/手机左侧位置。桌面、手机最终截图均已检查。匿名入口、复制身份、导入前登录、导入权限与失败状态保持正常。

限制：本轮验证对象是使用模拟授权身份的本地实际构建页面，不是已部署实机新版；不宣称完整无障碍合规。截图可确认视觉层级和手机重排，完整屏幕阅读器、对比度和所有键盘流程不在此次局部检查范围。尚未部署本轮样式更新到 EG100。

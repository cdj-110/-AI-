# 物联云跨端客户端

基于 Vue 3 + uni-app，与现有物联云平台共用账号、租户、角色和设备 API。

## 已实现

- 物联云账号登录和 JWT 会话。
- 设备总览、在线/离线状态和搜索。
- 单设备实时数据查看（跨端 WebSocket 推送，断线时 1 秒 HTTP 降级刷新）。
- 管理员新增直连设备、网关、网关子设备。
- 租户管理员扫描交付二维码绑定出厂网关；支持手动输入 SN 和 6 位绑定码兜底。
- 选择所属网关和物模型模板。
- 创建后一次性展示 MQTT 连接凭证。
- Android、iOS、H5、微信小程序统一代码。

## 本地运行

```powershell
npm.cmd install
npm.cmd run dev:h5
```

默认地址为 `http://127.0.0.1:3100`。手机调试时，在登录页展开“服务器设置”，填写电脑局域网地址，例如 `http://192.168.1.141:3100`。

## 发布前配置

1. 将 API 部署到 HTTPS 域名。
2. 在 `src/manifest.json` 填写微信小程序 AppID。
3. 在微信公众平台把 API 域名加入 request 合法域名。
4. 在 HBuilderX 中配置 Android 包名、iOS Bundle ID、签名证书和隐私权限说明。
5. 正式环境通过 `VITE_API_BASE_URL` 写入云平台域名。

## 构建

```powershell
npm.cmd run build:h5
npm.cmd run build:mp-weixin
npm.cmd run build:app
```

- H5：`dist/build/h5`
- 微信小程序：`dist/build/mp-weixin`，使用微信开发者工具导入。
- Android/iOS：`dist/build/app`，使用 HBuilderX 配置签名后生成 APK/AAB/IPA。

设备新增权限与物联云 Web 端保持一致：超级管理员和租户管理员可新增，普通租户用户只读。移动端不会建立独立账号体系。

## 网关扫码绑定

- 公司管理平台登记网关或重置绑定码后，会生成一次性绑定二维码。
- App 在设备页点击“＋”，选择“扫码绑定网关”，校验设备信息后由用户确认绑定。
- 二维码内容为 `weikong-gateway-bind` JSON，仅包含网关 SN 和 6 位绑定码；不包含 WiFi、平台密码或设备密钥。
- Android 仅在主动扫码时请求相机权限；不需要文件、照片、音频或位置权限。

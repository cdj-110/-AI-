# Android 测试壳

该工程把 uni-app 的 H5 构建结果打进原生 WebView，用于生成可直接安装的局域网测试 APK。

- 默认 API 地址由打包时的 `VITE_API_BASE_URL` 写入，登录页仍可修改服务器地址。
- 为方便连接局域网内的 HTTP 平台，测试包启用了明文 HTTP；正式发布时应改为 HTTPS。
- APK 使用 Android 默认 debug 签名，只用于内部安装测试。

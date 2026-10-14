# 微控 Windows 标签打印服务

面向 Godex G500 等已安装 Windows 驱动的标签打印机。服务只监听 `127.0.0.1:17896`，管理平台可直接提交二维码标签并静默进入 Windows 打印队列。

## 安装

以当前 Windows 用户运行：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\windows-print-agent\install.ps1 -PrinterName "Godex G500 GZPL"
```

如果管理平台不是本机 `5176`，安装时传入它的完整来源地址：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\windows-print-agent\install.ps1 -PrinterName "Godex G500 GZPL" -AllowedOrigin "http://192.168.1.10:5176"
```

安装完成后，将终端显示的配对密钥填入管理平台“打印服务设置”。服务随当前用户登录自动启动。

## 接收管理 App 的云端任务

先在 PC 管理平台创建打印终端并复制终端令牌，再重新安装：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\windows-print-agent\install.ps1 `
  -PrinterName "Godex G500" `
  -CloudApiUrl "http://云平台地址:3100" `
  -CloudAgentToken "wkpa_xxx"
```

打印电脑只需主动访问云平台，不需要开放本机端口给手机或公网。

如管理平台部署在其他域名，把该地址加入：

`%LOCALAPPDATA%\Weikong\PrintAgent\config.json` 的 `allowedOrigins`，然后结束打印代理进程并重新登录 Windows。

## 卸载

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\windows-print-agent\uninstall.ps1
```

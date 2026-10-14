# 云平台远程访问（第一阶段）

第一阶段提供“云平台登录后直接打开 EG100 管理页面”的最小闭环。EG100 主动连接云端，不需要公网 IP，也不开放现场 8088 入站端口。

## 云端配置

API 服务新增两个环境变量：

```env
REMOTE_ACCESS_PORT=3201
REMOTE_ACCESS_PUBLIC_URL=https://remote.example.com
REMOTE_ACCESS_SESSION_IDLE_SECONDS=900
REMOTE_ACCESS_LINK_TTL_SECONDS=900
REMOTE_ACCESS_COOKIE_MAX_AGE_SECONDS=604800
```

`REMOTE_ACCESS_PUBLIC_URL` 应由 HTTPS 反向代理指向 API 容器的 `3201` 端口，并启用 WebSocket 转发。开发环境留空时，平台根据当前访问主机和 `REMOTE_ACCESS_PORT` 生成链接。

远程访问会话保存在 Redis 中。浏览器有 HTTP 或 WebSocket 活动时会自动刷新空闲过期时间，因此 API 重启后 Cookie 会话仍可继续使用；持续无操作达到 `REMOTE_ACCESS_SESSION_IDLE_SECONDS` 后自动失效。`/connect` 链接仍为一次性链接，并按 `REMOTE_ACCESS_LINK_TTL_SECONDS` 过期。`REMOTE_ACCESS_COOKIE_MAX_AGE_SECONDS` 只控制浏览器会话标识的保留时间（默认 7 天），访问权限始终以 Redis 空闲超时为准，有效 HTTP 活动也会同步刷新 Cookie。

云端 API 的 `/api/remote-access/tunnel` 接受网关主动 WebSocket 连接。远程代理端口只接受云平台签发的一次性链接，并换取 HttpOnly 会话 Cookie；会话在持续活动时自动续期，无操作超过默认 15 分钟后失效。

## EG100 配置

在“云平台连接 → 云端远程访问”中启用并配置：

```json
{
  "remoteAccess": {
    "enabled": true,
    "endpoint": "wss://cloud.example.com/api/remote-access/tunnel",
    "localUrl": "https://127.0.0.1:8088",
    "insecureSkipVerify": false,
    "localInsecureSkipVerify": true
  }
}
```

用户名和密码留空时复用微控云激活凭证；没有激活文件时复用已启用的 MQTT 设备凭证。保存配置后隧道会热重连，不需要重启网关。

## 使用方式

1. 用户登录租户云平台并打开网关设备详情。
2. 远程通道在线时，“远程访问”按钮可用。
3. 点击后云平台生成一次性短期链接，并在新标签页打开 EG100 管理页面。
4. 当前阶段仍使用 EG100 自己的本地账号登录，避免云端身份被错误提升成本地管理员。

## 安全边界

- 只允许代理 `127.0.0.1` 或 `localhost`，禁止把隧道作为现场内网跳板。
- 网关必须同时匹配 `gatewayKey`、设备用户名和设备密码。
- 只有 `SUPER_ADMIN`、`TENANT_ADMIN` 可以创建远程会话；租户隔离在服务端校验。
- 一次性链接使用后立即失效；远程会话默认空闲 15 分钟失效，活动期间自动续期。
- 创建远程会话写入平台操作日志。
- 生产环境必须使用 `wss://` 和可信证书，不得启用 `insecureSkipVerify`。
- `localInsecureSkipVerify` 只影响回环地址上的本机 HTTPS；它与公网 WSS
  校验隔离，用于 EG100 管理页采用自签名证书的现场环境。

## 后续阶段

第二阶段应增加云平台 TOTP 二次确认、远程会话列表/强制下线、按权限映射的免密 SSO、完整访问审计以及浏览器 SSH。访问现场网段的能力应单独使用 WireGuard，并保持默认关闭。

# EG100 网关应用 OTA（首版）

范围：只替换 `/userdata/weikong/bin/weikong-gateway`（含内置网页）；不更新内核、驱动或分区。配置、授权和历史数据不包含在升级包中，也不在升级时清空。管理平台由超级管理员上传 `.wku`，在“出厂网关 → 应用升级”选择在线且已激活的 EG100 下发，并查看结果。设备安装时会短暂离线。

## 上线前准备

1. 为 OTA **单独**生成 Ed25519 密钥对。私钥只放离线构建环境，不能进入管理平台、网关、仓库或升级包。管理平台配置 `OTA_TRUSTED_PUBLIC_KEYS`（JSON，键为 keyId，值为 32 字节原始公钥的 Base64）；构建 EG100 应用时把同一公钥环通过 `WK_OTA_TRUSTED_PUBLIC_KEYS` 注入。未注入公钥的网关会拒绝升级。
2. 管理平台配置独立随机 `OTA_DOWNLOAD_SECRET`（至少 32 字符）、`OTA_PUBLIC_BASE_URL=https://...`。设备授权文件中已签名的 `cloudApi` 必须指向同一 HTTPS 域名；HTTP 授权地址不会用于 OTA。
3. 管理平台执行数据库迁移 `20260921090000_gateway_app_ota`，并保留 `ota_packages` 持久卷。反向代理需允许最大约 70 MiB 的上传。EG100 需要已挂载 `/mnt/udisk` 或 `/media/udisk1` 暂存空间，以及 `/userdata/weikong/bin` 的可用空间。
4. 首次启用需要先通过现有受控部署方式把带 OTA 公钥和接收器的网关版本装到 EG100。不能用 OTA 更新尚不支持 OTA 的旧版本。

## 制作包与下发

离线构建的 ARMv7 网关可执行文件先用 gzip 压缩，再在离线环境设置 `OTA_SIGNING_PRIVATE_KEY`（PKCS#8 DER 的 Base64）和 `OTA_SIGNING_KEY_ID`，运行 `node tools/create-eg100-ota-package.mjs <gateway.gz> <output.wku> <递增发布序号> <版本号>`。发布序号需严格递增，例如 `2026092101`。上传后管理平台和网关会分别验签、核对型号、文件大小和 SHA-256。

平台下发任务使用设备专属 MQTT Topic；设备从平台 HTTPS 地址持短期令牌下载。设备验签并在 U 盘暂存新程序，校验 ARMv7 ELF 和摘要；安装脚本放在内置受保护存储。EG100 内置分区无法同时容纳两个程序，因此先在 U 盘备份旧程序并校验，再停服务、替换程序、执行 `-check-config` 和启动检查。失败时自动恢复旧程序。成功或回退回执保存在本地文件，并在云连接恢复后重试上报。管理端超时显示“待人工确认”，不能把超时当作失败并盲目重发。

## 首版边界

- 只接受 `EG100`、最大 64 MiB 压缩包，未配置可信公钥/HTTPS/下载密钥时拒绝任务。
- 升级包签名密钥与授权文件签名密钥分开管理；升级包版本回退由网关的本地发布序号限制。首次 OTA 的基线序号为 0。
- 安装前配置检查和安装后服务存活检查已实现；正式批量升级前应在实体 EG100 上验证断电、U 盘拔出、服务启动失败、网络中断的恢复流程，并灰度放量。
- 这个版本不包含差分更新、整机系统固件更新、定时升级或批量波次调度。

## 2026-09-21 部署状态

- 已生成独立 OTA Ed25519 密钥对；私钥仅保存在本机受保护的 `secrets/ota-20260921/ota-private-key.env`，公钥指纹为 `08a25f829350cab3`。管理平台仅配置公钥和独立下载令牌。
- 管理平台已完成 `20260921090000_gateway_app_ota` 迁移，部署前数据库备份保存在受保护的 `secrets/production-batch-deploy-20260921023031`；旧 API 和管理端镜像分别保留为 `wk-ota-api-rollback:20260921`、`wk-ota-admin-rollback:20260921`。
- EG100 已通过受控部署安装 OTA 接收和验签程序，保留原配置与授权；安装前的原始程序备份位于 `/mnt/udisk/weikong-backups/ota-bootstrap-20260921/`，最终版安装前的程序备份位于 `/mnt/udisk/weikong-backups/ota-bootstrap-final-20260921/`。
- 2026-09-21 已为同一局域网测试配置 `https://192.168.1.141:3443`，使用受保护的专用 CA 签发带 IP SAN 的服务器证书。EG100 系统信任库已加入该 CA，原始信任库备份在 `/mnt/udisk/weikong-backups/https-lan-20260921/`；网关上的 Go 客户端严格验签通过。`OTA_PUBLIC_BASE_URL` 与新授权 `cloudApi` 已指向同一 HTTPS 地址，WSS 隧道地址也同步更新。
- EG100 第 9 版授权已签发、预检、导入并向管理平台回报有效；导入前的配置和第 8 版授权备份位于 `/mnt/udisk/weikong-backups/https-license-20260921/`。签名授权文件保存在受保护的 `secrets/https-lan-20260921/activation-WK20265AF2F5C6-seq9.lic`，其中包含设备密钥，不要公开传输。
- 这个 HTTPS 入口只面向当前局域网和已安装专用 CA 的设备，不是公网受信任域名。服务器证书约一年后需要轮换；若平台 IP 变化，须重新签发带新地址的证书和设备授权。尚未执行真实 OTA 包下发、断电及回滚实机验证，批量下发前必须完成这些测试。

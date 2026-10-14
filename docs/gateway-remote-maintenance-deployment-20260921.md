# EG100 远程维护上线记录

上线时间：2026-09-21 17:35:32 +08:00

## 云平台

- 迁移前 PostgreSQL 自定义格式备份已通过 `pg_restore --list` 校验。
- 已执行迁移 `20260921170000_gateway_remote_maintenance`，生产库新增 `GatewayMaintenanceTask` 表、两个索引及出厂网关外键。
- 已重新构建并更新 `api` 与 `admin-web`，租户端、遥测接收、PostgreSQL、TimescaleDB、Redis、EMQX 和 OTA HTTPS 服务未重建。
- API 已映射远程维护 GET/POST 路由，管理端首页返回 HTTP 200，API 鉴权探测按预期返回 HTTP 401。
- 实际出厂网关 `WK20265AF2F5C6` 的远程维护历史接口验证成功；未发起重启操作。
- 回滚镜像：`wk-gateway-maintenance-api-rollback:20260921172228`、`wk-gateway-maintenance-admin-rollback:20260921172228`。

迁移前备份保存在受保护目录 `secrets/gateway-maintenance-deploy-20260921172228/database.dump`，SHA-256：

`1db76b0604982a72b670e1811fd82a6ee141cc6cf03052cdfb3478131fdf0b6e`

## EG100 V0.7 OTA

- 版本：`V0.7`
- 型号：`EG100`
- 发布序号：`2026092108`
- 升级包：`artifacts/ota-v0.7-20260921/eg100-v0.7-release-2026092108.wku`
- 升级包 SHA-256：`8ceae79ad5f164f29f9bb411cb07c4d108c7f8facc6a724f3c37860605f14b5a`
- gzip 载荷 SHA-256：`2df99742a821a97a72bb8c2076e1aa4e652dfd9cdf97d5edf9eaf2678fb0f631`
- 解压后 ARMv7 程序 SHA-256：`02a04f6cdaf28d66c49cdb00c293ba1804fdba71ecfebf4033e6cb269371f3ce`
- 签名密钥编号：`eg100-ota-20260921`
- 管理平台升级包编号：`2436746e-e7dc-4087-8339-734cf7750d7d`

升级包已通过平台同源验签、清单校验、载荷摘要与长度校验；解压结果与生产 ARM ELF 二进制逐字节一致，且包含 `V0.7` 版本标记。升级包已经上传到“升级包管理”，但没有下发到任何网关。

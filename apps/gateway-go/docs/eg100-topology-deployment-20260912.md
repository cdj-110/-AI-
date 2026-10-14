# EG100 首页及设备拓扑部署

- 目标：192.168.1.214；2026-09-12 16:45:53 启动新版。
- 更新：首页聚合异常摘要、设备拓扑页、云平台三标签页、远程数据库存储布局。
- 新程序 SHA256：`b8aff00ec48700b4387f2a18132f01baaecce15b7ebe04be90cc05b5382cca9d`。
- 配置 SHA256 部署前后相同：`01a75e87a5b3e2c9a348a893131e1e0cd5633361973385160fd5d38f78c618ff`。
- ARMv7 CGO 构建保留 opcda、iec61850_mms；相关汇总测试通过。

## 备份

设备目录：`/mnt/udisk/.weikong-deploy/topology-backup-20260912-1642`。
旧程序同时备份至本机 `build/topology-deployment-20260912-1642/gateway-original.gz`，解压哈希与部署前相同。
配置保存在设备 root-only 备份目录，未输出凭据。
停止服务后备份告警库、历史库及 PostgreSQL/MySQL/SQL Server 三个缓存队列；全部 SQLite quick_check 通过。
因系统分区空间不足以容纳两份展开程序，验证回退副本后替换；部署脚本附故障回退。

## 实机验收

- healthz / readyz 正常，52 个点位，采集时间持续更新。
- `/api/status/overview` 正常返回设备统计及最多 5 组异常。
- 实机 JS/CSS 与本地构建逐字节 SHA256 相同。
- 三个数据库任务均连接正常，写入计数持续增长；各队列均观察到排空，丢弃数为 0。16:47:47 各发送 184 条，最终待补传均为 0。
- USB 原分区保持 rw 挂载；厂商 pilot 未运行。
- 验收快照有 11 个异常点位（IEC104、DL/T 645、OPC UA、一个 OPC DA 点及关联计算点），服务健康不代表所有现场设备通信正常。本次未修改这些设备参数。
- 系统分区仍使用 97%，余量约 5 MB；本次没有删除无关数据。

详细记录：`build/topology-deployment-20260912-1642/verification.json`。

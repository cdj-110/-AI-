# EG100 系统空间清理与拓扑更新

## 系统分区占用原因

EG100 根分区 `ubi0:rootfs` 总计 161820 KiB（约 158 MiB）。首页旧“硬盘”统计来自 `ReadSystemMetrics()` 对 `/` 的统计，不是外接 U 盘总容量。

主要占用包括厂商系统与运行库、当前约 27 MiB 网关程序、活动告警/采集异常数据库，以及两份遗留旧程序备份（共 19482695 字节）。UBIFS 压缩影响物理空间，不能把 du 文件逻辑大小简单等同于 df 使用量。

U 盘 `/dev/sda4` 挂载 `/mnt/udisk`，使用约 20%；历史库及远程数据库队列仍保存在原分区，不是本次系统盘接近满的主要原因。

## 已执行的可恢复清理

只迁移两个明确检查过的文件：

- `/userdata/weikong/backups/dlt-select-20260910/weikong-gateway.original.gz`
- `/userdata/weikong/backups/usb-guard-20260910/weikong-gateway.original.gz`

归档目的地：`/mnt/udisk/.weikong-deploy/flash-backup-archive-20260912/`，目录700，文件600。复制、sync、SHA256一致及gzip校验通过后，删除系统盘两份对应副本。旧配置、厂商程序停用回退脚本仍留原位。

- dlt-select 哈希：`8cd1623d0747012af892afdcdbbda112715285751ae84a1cdd6be539221dfe9d`
- usb-guard 哈希：`27d83106858aa7d053091b78f870dc085e1e7c9ecbca6c923f8bce633b450ebc`

清理即时：97% → 86%，可用4236 KiB → 23080 KiB；释放18844 KiB。17:04复核可用22552 KiB（运行数据库持续写入有波动）。未停止服务，未清除历史、异常、告警或队列数据。

验收：程序与配置哈希未变，健康正常，52个点位持续采集；PG/MySQL/SQL Server各发送1792条，均连接正常、pending=0、dropped=0。

## 本地前端修改（尚未部署）

- 拓扑改为云平台 → 网关 → 串口/网口 → 通道及设备的上下分支结构。
- 增加统一设备图标、采集状态、空闲端口，保留搜索/筛选/折叠/设备定位。
- 首页“硬盘”改为“系统分区”，悬浮说明统计 `/`，不包含外接 U 盘。
- 新前端正式构建及拓扑测试通过；示例页经内置浏览器桌面/窄屏检查，QA见项目根 `design-qa.md`。

后续部署继续使用USB回退目录，不将大程序备份长期留在系统分区。

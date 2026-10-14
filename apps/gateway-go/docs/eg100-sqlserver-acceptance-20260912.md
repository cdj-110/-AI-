# EG100 SQL Server 实库联调验收

2026-09-12，最后状态核对约 15:46（北京时间）。

## 结论

通过 SQL Server 2022 实库及 EG100 写入/服务中断补传测试。本次没有修改网关程序，也没有重启 EG100。

- 首批 8 点位、2 轮共 16 条入库，质量均 GOOD；只读连接与表结构检查约 133 ms。
- 单独停止 SQL Server 测试容器后，网关持久缓存 24 条记录。恢复数据库后，逐条 ID 核对 24 条全部入库，无重复。
- 断库期间网关健康、采集时间继续更新，MySQL/PostgreSQL 保持连接并继续写入。
- 最终快照 SQL Server 280 条、280 个唯一 ID、8 个点位，280 条 GOOD；三个存储任务连接正常、待补传 0、未入队 0。条数会持续增长。
- 配置保存前后比较，仅 remoteStorage 新增一个目标；原 MySQL/PostgreSQL 配置（含密码）和其他顶层配置不变。

## 本机驱动实库测试

使用独立 eg100_tester 账号创建随机 remote_it_* 临时表，真实跨批写入 205 条并重复提交两次，最终只有 205 条。uint64 最大值 18446744073709551615、负数、布尔、NULL、中文及四字节 emoji 回读通过。测试还校验只读表结构。临时表已由测试清理，测试账号完成后禁用。

运行的是 `TestRemoteDatabaseIntegration`，没有因为 SQL Server 联调改动生产代码。此前 MySQL 修复版本仍为 EG100 当前程序：`057b748881ac1115536eec35c393c6a4deb1b6345659b5829103ced1e6f367f6`。

## 环境与权限

| 项目 | 内容 |
| --- | --- |
| SQL Server | 2022 Developer 64-bit，16.0.4275.2，仅用于开发测试 |
| 官方镜像 | mcr.microsoft.com/mssql/server，digest sha256:97b448857967be55e005424a660056fe6d51814435804dc07e8f79f028bab5fb |
| 容器 / 数据卷 | eg100-sqlserver-integration / eg100-sqlserver-integration-data |
| 内网绑定 | 192.168.1.141:1433，已获用户明确同意 |
| 本机绑定 | 127.0.0.1:1433，没有绑定 0.0.0.0 |
| 资源上限 | 容器 3 GB / 2 CPU，SQL Server 内存设置 2048 MB |
| 库 / 表 | eg100_integration / dbo.eg100_samples |
| 网关账号 | eg100_writer，只对该表授予 SELECT、INSERT |
| 权限实测 | 非 sysadmin、非 db_owner；SELECT=1、INSERT=1、DELETE=0、ALTER=0 |
| 任务 | eg100-sqlserver-integration / EG100 SQL Server 实库联调 |
| 采样 / 批量 / 超时 | 5 秒 / 最多 100 条 / 10 秒 |
| 缓存 | /mnt/udisk/.remote-storage/eg100-sqlserver-integration.db，16 MB |

8 点位与另外两库一致：noise-meter-1/NOISE_DB；s7-01/s7-P01、s7-P02；opcda-matrikon-test/boolean、real4、string、uint8、opcda-P15。

正式采集表由初始化程序创建，EG100 的 createTable=false，避免给网关账号建表或修改表的权限。自动建表能力由隔离临时表测试验证。

sa、eg100_writer、eg100_tester 使用独立随机密码，Windows DPAPI 加密凭据位于 `build/sqlserver-integration-20260912/`。sa 仅在初始化及账号管理时使用，未配置到网关或 DBeaver；它仍为启用状态，并不声明为仅本机登录。eg100_tester 已禁用，网关接口密码返回掩码。当前仅为可信内网测试环境，不是生产安全配置。

## 查看数据

已向 DBeaver 添加 **EG100 SQL Server 联调库** 无密码连接模板，并打开工作区 `tools/EG100-SQLServer最近采集数据.sql`。查询只读返回最近 100 条并按北京时间显示。

自动导入数据库密码被安全检查拦截，因此没有向 DBeaver 传入或保存密码，也没有自动连接；首次使用仍需配置认证及证书相关设置。不能将此状态视作 DBeaver 连接验收通过。

## 范围与后续

- SQL Server 使用官方 Developer 测试版，依据 [微软容器快速入门](https://learn.microsoft.com/en-us/sql/linux/install-upgrade/quickstart-install-docker?view=sql-server-ver17)。不可当作生产许可证使用。
- EG100 此次 TLS disabled；没有验收可信 CA/主机名证书验证。生产环境需要单独配置 TLS 和访问控制。
- 本次验证数据库服务停止/恢复；没有做带 SQL Server 待补传队列重启 EG100、物理断网/掉电、满缓存、长时间压力测试。
- MySQL、PostgreSQL、SQL Server 现在均已通过基本 EG100 实库及断库补传；不等于全部可靠性/安全性场景验证完成。
- 本机/Docker 必须保持运行。测试容器未设置自动重启；数据在独立 Docker 卷中持久化。
- 原始验收 JSON 位于 `build/sqlserver-integration-20260912/`。脚本为 `deploy/eg100/provision_sqlserver_test_20260912.ps1` 和 `deploy/eg100/sqlserver_live_20260912.py`，后者复用已验证的网关连接及断库测试代码。

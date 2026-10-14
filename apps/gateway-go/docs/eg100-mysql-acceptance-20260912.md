# EG100 MySQL 实库联调验收

2026-09-12，最后核对 15:34（北京时间）。

## 结果

通过：MySQL 8.4.11 连接、建表、多批写入、重复重放、中文/emoji、uint64 最大值、EG100 实际采样与数据库中断补传。

- EG100 192.168.1.214，8 个测试点位每 5 秒写入，任务保持启用。
- 验收快照：MySQL 共 288 条、288 个唯一记录号、8 个点位，288 条 GOOD。
- MySQL 停止期间，网关缓存 24 条；恢复后逐个记录号核对，24 条全部入库，无重复，待补传和未入队均为 0。
- 此次数据库中断测试不重启网关。期间原 PostgreSQL 保持连接、继续发送、待补传为 0。
- 更新程序后网关健康，52 个配置点位，采集时间继续更新；不代表所有外部设备都在线。

## 修复的真实问题

原程序通过 `mysql.NewConnector(Config)` 直接连接，却将 `charset=utf8mb4` 放入 `Config.Params`。驱动把它当作 MySQL 系统变量，触发 `Error 1193: Unknown system variable 'charset'`。

改用当前依赖 v1.9.3 提供的 `Config.Apply(mysql.Charset("utf8mb4", ""))`，使其发出正确的 `SET NAMES`。实库测试增加 client/connection/results 三项字符集断言，并回读中文及四字节 emoji。

验证：

- 独立库实测 205 条跨批次记录，重复写入两次后仍为 205 条。
- uint64 最大值 18446744073709551615 精确回读，中文文本及 emoji 一致。
- 三驱动事务、回滚、冲突及 SQL Server 参数分批测试通过。
- Linux CGO 全量 `go test ./... -count=1` 通过。
- remotestore/config 竞态测试通过。
- ARMv7 正式包包含 SQLite CGO、opcda、iec61850_mms。前端未修改。

## 连接配置

| 项目 | 值 |
| --- | --- |
| 本机服务 | Docker `eg100-mysql-integration`，MySQL 8.4.11 |
| EG100 访问地址 | 192.168.1.141:3306（用户明确授权开放） |
| DBeaver 本机地址 | 127.0.0.1:3306 |
| 库 / 表 | eg100_integration / eg100_samples |
| 专用账号 | eg100_writer，仅此库 SELECT、INSERT、UPDATE、CREATE，连接数上限 8 |
| 数据卷 | eg100-mysql-integration-data，未覆盖任何原业务库 |
| 任务 | eg100-mysql-integration / EG100 MySQL 实库联调 |
| 网关缓存 | /mnt/udisk/.remote-storage/eg100-mysql-integration.db，16 MB |
| 批次 / 超时 | 最多 100 条 / 10 秒 |

点位与 PostgreSQL 一致：noise-meter-1/NOISE_DB；s7-01/s7-P01、s7-P02；opcda-matrikon-test/boolean、real4、string、uint8、opcda-P15。

密码通过 Windows DPAPI 加密保存在 `build/mysql-integration-20260912/*.credential.clixml`，未写入此文档；网关接口返回掩码。自动测试账号 eg100_tester 在测试后已锁定。镜像默认建立的 root@% 在最终验收发现后已锁定；已断言不存在可登录的非 localhost root 账号。未来创建脚本明确设置 MYSQL_ROOT_HOST=localhost。

MySQL 只绑定 localhost 及指定内网 IP，没有 0.0.0.0 绑定。旧 localhost 容器 `eg100-mysql-integration-local` 保留为停止状态，不得与新实例同时启动（共享数据卷）。当前不配置容器自动重启，电脑/Docker 重启后需启动联调实例。

## 部署与备份

经用户明确同意后更新程序，约 15:27 停服备份、15:30:39 重新启动、15:30:44 就绪健康检查通过。历史库约 256 MB，全量完整性检查导致此次维护耗时约 3 分钟。

- 新程序 28,310,940 字节，SHA-256：`057b748881ac1115536eec35c393c6a4deb1b6345659b5829103ced1e6f367f6`。
- 旧程序：`493ebd9f346532f00981a1f602358886cc3c3293bdca9725def2e0d8c4ecb061`。
- USB 备份：`/mnt/udisk/.weikong-deploy/mysql-fix-backup-20260912`，含原程序压缩包、原配置、告警库、历史库、PostgreSQL 缓存库；数据库完整性检查均为 ok，目录 700、文件 600。
- 本机另存原程序压缩包 `build/mysql-integration-20260912/gateway-before-mysql.gz`，校验一致。
- 更新程序前后配置 hash 一致；之后只新增 remoteStorage 内的 MySQL 任务。最终逐项比较，原 PostgreSQL 配置（包括密码）及其他顶层配置不变。
- 就绪检测失败时部署脚本可恢复旧程序。本次未触发回滚。

## 从本机查看

DBeaver 已保存连接 **EG100 MySQL 联调库**，仍保留原 PostgreSQL 连接。查询文件在工作区根 `tools/EG100-MySQL最近采集数据.sql`，只读查询最近 100 条，并按北京时间显示。

也可展开连接 → eg100_integration → 表 → eg100_samples → 查看数据。

## 验证边界

- EG100 的测试连接使用 TLS disabled，仅用于此次内网联调；正式部署需要可信证书与证书验证。DBeaver 本机连接使用 sslMode=REQUIRED，不等于服务器身份验证。
- 本次验证 MySQL 服务停止/恢复，不等同物理拔线、掉电、满缓存或长期压力测试；本次没有专门做 MySQL 待补传状态下网关重启测试。
- 原始结果在 `build/mysql-integration-20260912/`。表持续写入，验收条数会继续增长。
- SQL Server、国产数据库仍不在此次验收范围。

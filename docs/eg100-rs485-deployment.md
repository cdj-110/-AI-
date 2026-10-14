# EG100 双 RS485 部署与联调

## 1. 接口对应

| 接口 | 设备节点 | 终端电阻 |
| --- | --- | --- |
| RS485-1 | `/dev/ttyS1` | J10 |
| RS485-2 | `/dev/ttyS2` | J11 |

网关在总线末端时接入对应的 120 Ohm 终端电阻。接线时连接 A、B 和信号地，避免与 RS232 的 `/dev/ttyS3`、`/dev/ttyS5` 混用。

## 2. 部署目录

```text
/userdata/weikong/bin/weikong-gateway
/userdata/weikong/config/config.local.json
/userdata/weikong/data/gateway-spool.jsonl
```

在板端创建目录：

```sh
mkdir -p /userdata/weikong/bin /userdata/weikong/config /userdata/weikong/data
```

从 `gateway-artifacts/go/EG100/current` 选择当前 EG100 程序或发布包，将其中的网关程序上传为 `/userdata/weikong/bin/weikong-gateway`；将 `apps/gateway-go/config.eg100.example.json` 上传为 `/userdata/weikong/config/config.local.json`，然后执行：

```sh
chmod 755 /userdata/weikong/bin/weikong-gateway
chmod 600 /userdata/weikong/config/config.local.json
```

## 3. 上电前检查

确认两个串口没有被系统控制台占用：

```sh
cat /proc/cmdline
ls -l /dev/ttyS1 /dev/ttyS2
```

`console=` 参数不应指向 `ttyS1` 或 `ttyS2`。

根据真实仪表修改配置中的以下字段：

- `baudRate`、`dataBits`、`stopBits`、`parity`
- `slaveId`
- `function`、`register`、`quantity`、`dataType`

寄存器手册使用 40001/30001 表示法时，配置中的 `register` 通常填写从零开始的偏移量，例如 40001 对应 0。应以设备说明书为准。

## 4. 前台联调

首次测试先关闭 MQTT，在前台启动：

```sh
/userdata/weikong/bin/weikong-gateway -config /userdata/weikong/config/config.local.json
```

另一个终端检查状态：

```sh
wget -qO- http://127.0.0.1:8088/api/status
```

也可以从同一局域网访问：

```text
http://<EG100-IP>:8088
```

重点检查每个点位的当前值、更新时间和错误信息。常见问题：

| 现象 | 检查项 |
| --- | --- |
| 超时 | A/B 接线、从站地址、波特率、校验位 |
| Illegal data address | 寄存器地址是否需要减 1 |
| CRC 错误 | 干扰、接地、终端电阻、波特率 |
| 一个串口正常另一个异常 | `/dev/ttyS1`、`/dev/ttyS2` 与物理端子是否对应 |

## 5. 开机启动

确认前台联调正常后，将 `deploy/eg100/S99weikong-gateway` 放到板端：

```sh
cp S99weikong-gateway /etc/init.d/S99weikong-gateway
chmod 755 /etc/init.d/S99weikong-gateway
/etc/init.d/S99weikong-gateway start
```

如果根文件系统只读，应在 Buildroot overlay 中加入该脚本，再重新生成 `rootfs.img`。

## 6. 双总线验收

1. RS485-1 和 RS485-2 各连接一台设备，确认两边同时更新。
2. 每条总线逐步增加到 4、8、16 台设备。
3. 记录完整轮询耗时、超时率和 CRC 错误数。
4. 断开一个从站，确认另一条总线继续采集。
5. 恢复从站，确认 15 到 30 秒退避后自动恢复。
6. 连续运行至少 72 小时，再确定正式容量指标。

## 7. 双网口管理

本地页面的“智能网关 -> 网口配置”会读取 Linux 中 `eth0`/`eth1` 的真实状态，包括 MAC、链路、速率、IP、网关和 DNS。

修改后点击“应用网口配置”：

1. 配置保存到网关配置文件。
2. 系统网卡在 1 秒后重新配置。
3. 持久配置写入 `/etc/network/interfaces.d/<interface>`。

修改当前管理网口的 IP 会中断当前浏览器和 SSH 连接，需使用新 IP 重新连接。建议先在 `eth1` 上验证配置，再调整 `eth0`。

## 8. 看门狗与进程监督

EG100 提供 `/dev/watchdog`。生产运行采用两层恢复机制：

1. 网关可执行文件以 `-supervise` 模式启动父监督进程。
2. 父进程检查子进程和 `http://127.0.0.1:8088/api/healthz`，异常时优先重启子进程。
3. 连续恢复失败达到阈值后写入安全模式文件；启用硬件看门狗时停止喂狗，由硬件复位整机。
4. 下次启动检测到安全模式文件后，不再启用硬件看门狗和自动重启，保留 SSH 排障能力。

推荐配置：

```json
"watchdog": {
  "enabled": true,
  "hardwareEnabled": false,
  "device": "/dev/watchdog",
  "hardwareTimeoutSeconds": 90,
  "feedIntervalSeconds": 10,
  "healthCheckSeconds": 5,
  "healthTimeoutSeconds": 3,
  "startupGraceSeconds": 45,
  "failureThreshold": 4,
  "restartLimit": 3,
  "restartWindowSeconds": 300,
  "safeModeFile": "/userdata/weikong/data/watchdog-safe-mode"
}
```

首次部署必须保持 `hardwareEnabled: false`，先验证软件监督：

```sh
/etc/init.d/S99weikong-gateway status
cat /var/run/weikong-gateway.pid
wget -qO- http://127.0.0.1:8088/api/healthz
```

确认软件监督、维护重启和升级流程稳定后，才能在现场测试机开启硬件看门狗。开启前至少验证：正常停机不会复位、升级期间不会误复位、杀死子进程可自动恢复、连续失败会进入安全模式。

排除故障并确认新版本可用后，清除安全模式标记再重启监督器：

```sh
rm -f /userdata/weikong/data/watchdog-safe-mode
/etc/init.d/S99weikong-gateway restart
```

MQTT 离线、单个采集点错误或从站超时属于业务故障，不应触发整机复位。

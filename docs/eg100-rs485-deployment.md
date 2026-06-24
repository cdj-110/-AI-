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

将 `weikong-gateway-linux-armv7` 上传为 `/userdata/weikong/bin/weikong-gateway`，将 `config.eg100.example.json` 上传为 `/userdata/weikong/config/config.local.json`，然后执行：

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

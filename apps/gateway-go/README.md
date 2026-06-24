# 微控 IoT Go 网关服务

这个目录是网关端第一版骨架，用来把现场 Modbus 设备采集到的数据上报到云平台。

## 当前对接云平台主题

网关本体心跳：

```text
weikong/devices/{gatewayKey}/heartbeat
```

网关代发子设备心跳：

```text
weikong/gateways/{gatewayKey}/children/{childKey}/heartbeat
```

网关代发子设备遥测：

```text
weikong/gateways/{gatewayKey}/children/{childKey}/telemetry
```

云平台会校验：只有设备类型为“网关”的 MQTT 凭证，才能代发绑定在该网关下面的子设备数据。

## 本地启动

```bash
cd apps/gateway-go
cp config.example.json config.local.json
go mod tidy
go run ./cmd/gateway -config config.local.json
```

## 出厂激活文件

量产时推荐由云平台生成 SN、DeviceSecret 和 BindCode。工厂工具读取网关 HardwareId 后调用云平台登记接口，再把激活文件写入网关：

```json
{
  "hardwareId": "硬件唯一 ID",
  "sn": "WK2026A1B2C3D4",
  "deviceSecret": "云平台生成的 DeviceSecret",
  "broker": "tcp://mqtt.example.com:1883"
}
```

`config.local.json` 中配置：

```json
{
  "activation": {
    "file": "activation.local.json"
  }
}
```

启动时网关会读取硬件 ID 和激活文件，使用 `factory:{HardwareId}` 作为 MQTT 用户名连接云平台，并持续发布 `weikong/factory/{HardwareId}/heartbeat`。客户用 SN + BindCode 绑定后，同一套 DeviceSecret 会被平台授权发布正式设备主题。

启动后本地状态页默认地址：

```text
http://127.0.0.1:8088
```

状态接口：

```text
GET /api/status
```

页面会展示 MQTT 连接状态、采集周期、每个点位的当前值、更新时间和最近错误。

页面里的“配置文件”页签支持两种方式：

- 表单配置：维护网关基础信息、MQTT 信息、点表新增/复制/删除。
- JSON 高级编辑：直接编辑完整配置文件。

保存时会校验配置格式和必要字段，保存成功后需要重启网关服务才会生效。

## 配置重点

### 离线应急缓存

- 离线缓存默认关闭，只允许使用系统已挂载的 TF 卡或 U 盘，不会写入板载根分区。
- Web 页面会显示介质类型、设备路径、挂载点、总容量、剩余空间和缓存占用。
- `offlineCache.maxSizeMB` 只能设置为 `12-16`，默认 `16` MB。
- 可移动存储拔出或挂载失效后缓存自动停止；重新检测到介质后才可再次开启。
- MQTT 恢复后按顺序补传；容量达到上限时自动淘汰最旧数据，缓存文件不会突破配置上限。

```json
{
  "offlineCache": {
    "enabled": true,
    "maxSizeMB": 16,
    "storagePath": "/media/sdcard"
  }
}
```

- `gatewayKey`：云平台中网关设备的设备编号。
- `mqtt.clientId` / `mqtt.username` / `mqtt.password`：从云平台网关设备详情里的 MQTT 信息复制。
- `points[].deviceKey`：子设备设备编号，必须已经在云平台创建，并归属到该网关。
- `points[].metric`：上报到云平台的物模型标识符。
- `points[].protocol`：当前支持 `modbus-tcp`、`modbus-rtu`、`siemens-s7`、`iec104`、`iec61850` 和 `opcua`。
- `devices[].address`：OPC UA 设备填写 Endpoint URL，例如 `opc.tcp://192.168.1.10:4840`。
- `devices[].username` / `devices[].password`：OPC UA 用户名认证；用户名留空时使用匿名认证。
- `points[].nodeId`：OPC UA 点位 NodeId，例如 `ns=2;s=Temperature` 或 `ns=3;i=1001`。
- 当前 OPC UA 客户端支持 `SecurityPolicy=None` 下的匿名或用户名/密码认证。
- `points[].objectRef`：IEC 61850 点位对象引用，例如 `LD0/XCBR1.Pos.stVal`。
- `points[].fc`：IEC 61850 功能约束，常见值为 `ST`、`MX`、`SP`、`CF`。
- `points[].function`：支持 `1` 线圈、`2` 离散输入、`3` 保持寄存器、`4` 输入寄存器。
- `points[].dataType`：支持 `bool`、`uint16`、`int16`、`uint32`、`int32`、`float32`。
- `points[].byteOrder`：寄存器内字节序，支持 `big` 和 `little`，默认 `big`。
- `points[].wordOrder`：32 位数据的寄存器顺序，支持 `normal` 和 `swap`，默认 `normal`。
- `points[].bitIndex`：从 16 位寄存器中取某一位时使用，范围 `0-15`。
- `points[].scale` / `points[].offset`：数值换算公式为 `原始值 * scale + offset`。

## 点表配置例子

读取保持寄存器中的 32 位浮点温度：

```json
{
  "deviceKey": "child-001",
  "metric": "temperature",
  "protocol": "modbus-tcp",
  "address": "192.168.1.50:502",
  "slaveId": 1,
  "function": 3,
  "register": 0,
  "quantity": 2,
  "dataType": "float32",
  "byteOrder": "big",
  "wordOrder": "normal"
}
```

读取线圈状态：

```json
{
  "deviceKey": "child-001",
  "metric": "running",
  "protocol": "modbus-tcp",
  "address": "192.168.1.50:502",
  "slaveId": 1,
  "function": 1,
  "register": 10,
  "quantity": 1,
  "dataType": "bool"
}
```

IEC 61850 对象引用点位：

```json
{
  "deviceKey": "ied-001",
  "metric": "breaker_status",
  "protocol": "iec61850",
  "address": "192.168.1.60:102",
  "objectRef": "LD0/XCBR1.Pos.stVal",
  "fc": "ST",
  "quantity": 1,
  "dataType": "int32"
}
```

说明：当前网关已经支持 IEC 61850 配置入口和 TCP 102 端口连接测试；真实 MMS 读值需要继续接入 IEC 61850 MMS 协议栈。

## 后续模块

- Web 配置页：当前已支持表单化点表维护和 JSON 高级编辑，后续可继续扩展为热加载。
- Modbus TCP 从站/转发：把采集到的点位映射成本地从站寄存器。
- 远程升级：建议先由云端下发升级任务，网关下载包后校验签名再切换版本。

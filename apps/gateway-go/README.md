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

- `gatewayKey`：云平台中网关设备的设备编号。
- `mqtt.clientId` / `mqtt.username` / `mqtt.password`：从云平台网关设备详情里的 MQTT 信息复制。
- `points[].deviceKey`：子设备设备编号，必须已经在云平台创建，并归属到该网关。
- `points[].metric`：上报到云平台的物模型标识符。
- `points[].protocol`：当前支持 `modbus-tcp` 和 `modbus-rtu`。
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

## 后续模块

- Web 配置页：当前已支持表单化点表维护和 JSON 高级编辑，后续可继续扩展为热加载。
- Modbus TCP 从站/转发：把采集到的点位映射成本地从站寄存器。
- 远程升级：建议先由云端下发升级任务，网关下载包后校验签名再切换版本。

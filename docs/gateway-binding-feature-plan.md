# 网关出厂激活与客户绑定功能说明

## 1. 功能目标

本功能用于把低成本工业网关从“工厂生产”到“客户绑定使用”的流程标准化，方便公司开发团队复用到正式产品中。

目标流程：

1. 硬件出厂自带唯一 `HardwareId`。
2. 工厂扫码或工具读取 `HardwareId`。
3. 云平台生成 `SN`、`DeviceSecret`、`BindCode` 和激活文件。
4. 工厂只烧录 `DeviceSecret` 或 `activation.local.json`。
5. Go 网关开机读取本机 `HardwareId` 和本地凭证。
6. Go 网关自动连接云平台 MQTT。
7. 云平台收到心跳后标记为“在线未绑定”。
8. 客户登录云平台，输入 `SN + BindCode` 绑定网关。
9. 平台把网关归属到客户租户。
10. 客户在设备列表中看到网关在线。
11. 网关采集和上报的数据进入该客户账号。

## 2. 角色与边界

| 角色 | 主要操作 |
| --- | --- |
| 工厂管理员 | 读取 HardwareId，在云平台登记出厂网关，下载激活文件，烧录到网关 |
| 平台管理员 | 管理出厂设备库、重置未绑定设备的绑定码、查看在线未绑定状态 |
| 客户管理员 | 使用 SN + BindCode 绑定网关到自己的租户 |
| Go 网关 | 读取 HardwareId，加载激活文件，连接 MQTT，上报心跳和采集数据 |
| MQTT/数据接入服务 | 校验连接权限，接收心跳和遥测，更新在线状态与数据归属 |

## 3. 核心概念

| 名称 | 说明 |
| --- | --- |
| `HardwareId` | 硬件唯一 ID，可来自芯片 ID、CPU Serial、eMMC CID 或网卡 MAC 组合摘要 |
| `SN` | 平台生成的业务设备序列号，用于客户绑定和售后识别 |
| `DeviceSecret` | 平台生成的 MQTT 连接密钥，只保存哈希，不应明文长期展示 |
| `BindCode` | 客户绑定码，建议 6 位或更强的一次性绑定码，只允许未绑定设备使用 |
| `activation.local.json` | 工厂烧录到网关的本地激活文件 |
| 未绑定在线 | 网关已连接云平台，但还没有归属客户租户 |
| 已绑定在线 | 网关已归属客户租户，并持续上报心跳或遥测 |

## 4. 当前代码可复用模块

| 模块 | 位置 | 当前状态 |
| --- | --- | --- |
| 云平台出厂网关接口 | `apps/api/src/factory-gateways` | 已有原型 |
| 出厂网关数据库模型 | `apps/api/prisma/schema.prisma` | 已有原型 |
| MQTT 鉴权 | `apps/api/src/mqtt/mqtt-auth.controller.ts` | 已支持工厂通道和正式设备主题 |
| MQTT 数据接入 | `apps/ingest/src/main.ts` | 已支持工厂心跳、设备心跳、子设备遥测 |
| 云平台出厂网关页面 | `apps/web/src/views/FactoryGateways.vue` | 已有原型 |
| Go 网关硬件 ID | `apps/gateway-go/internal/hardware` | 已有原型 |
| Go 网关设备激活页 | `apps/gateway-go/internal/web/page.go` | 已有原型 |
| Go 网关 MQTT 客户端 | `apps/gateway-go/internal/cloud/mqtt.go` | 已有原型 |
| Go 网关配置结构 | `apps/gateway-go/internal/config/config.go` | 已有原型 |

## 5. 业务流程

### 5.1 工厂登记

1. 工厂工具读取网关 `HardwareId`。
2. 平台管理员进入“出厂网关”页面。
3. 输入 `HardwareId` 和批次号。
4. 平台生成：
   - `SN`
   - `DeviceSecret`
   - `BindCode`
   - `activation.local.json`
5. 工厂下载激活文件，并烧录到网关。

### 5.2 网关首次上线

1. Go 网关启动。
2. 读取本机 `HardwareId`。
3. 读取 `activation.local.json`。
4. 使用以下 MQTT 身份连接：
   - `clientId`: `factory_{HardwareId}`
   - `username`: `factory:{HardwareId}`
   - `password`: `DeviceSecret`
5. 周期发布工厂心跳：
   - topic: `weikong/factory/{HardwareId}/heartbeat`
6. 云平台校验通过后更新：
   - `firstSeenAt`
   - `lastSeenAt`
   - 状态显示为“在线未绑定”

### 5.3 客户绑定

1. 客户登录云平台。
2. 输入 `SN + BindCode`。
3. 平台校验：
   - `SN` 存在
   - `BindCode` 正确
   - 网关未绑定
   - 网关未禁用
4. 平台创建或关联正式网关设备：
   - `deviceType`: `GATEWAY`
   - `deviceKey`: `SN`
   - `tenantId`: 当前客户租户
5. 更新出厂网关状态为 `BOUND`。
6. 后续心跳和遥测归属到该客户租户。

## 6. MQTT 主题约定

| 场景 | Topic |
| --- | --- |
| 出厂未绑定心跳 | `weikong/factory/{hardwareId}/heartbeat` |
| 直连设备心跳 | `weikong/devices/{deviceKey}/heartbeat` |
| 直连设备遥测 | `weikong/devices/{deviceKey}/telemetry` |
| 网关子设备心跳 | `weikong/gateways/{gatewayKey}/children/{childDeviceKey}/heartbeat` |
| 网关子设备遥测 | `weikong/gateways/{gatewayKey}/children/{childDeviceKey}/telemetry` |

建议生产规则：

- 未绑定阶段只允许发布 `weikong/factory/{hardwareId}/heartbeat`。
- 绑定完成后允许该网关发布自己的正式设备主题。
- 网关代发子设备数据时，必须校验 `gatewayKey` 是否归属当前网关。
- 客户端不允许跨租户发布或订阅数据。

## 7. 激活文件格式

建议统一为：

```json
{
  "hardwareId": "1F4D815969EBDA2C9395C2A4D59CC40F",
  "sn": "WK2026A1B2C3D4",
  "deviceSecret": "cloud-generated-secret",
  "broker": "tcp://cloud.example.com:1883",
  "enabled": true
}
```

字段说明：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `hardwareId` | 否 | 可省略，网关启动时自动读取本机 HardwareId |
| `sn` | 是 | 云平台生成的 SN |
| `deviceSecret` | 是 | 云平台生成的连接密钥 |
| `broker` | 是 | MQTT Broker 地址 |
| `enabled` | 否 | 是否启用激活通道，默认启用 |

注意：

- 不建议让网关自己生成激活文件，因为云平台无法确认密钥是否可信。
- `DeviceSecret` 应由云平台生成并保存哈希。
- 激活文件在 Linux 网关上建议权限为 `0600`。
- 生产环境可以增加签名字段，防止激活文件被篡改。

## 8. 云平台功能需求

### 8.1 出厂网关管理

功能：

- 新增出厂网关。
- 按 `HardwareId`、`SN` 搜索。
- 查看状态：未上线、在线未绑定、已绑定、禁用。
- 下载激活文件。
- 重置未绑定设备的绑定码。
- 禁用或启用出厂网关。

验收：

- 重复 `HardwareId` 不允许登记。
- `DeviceSecret` 和 `BindCode` 只在生成或重置时展示。
- 已绑定设备不允许重置绑定码。
- 禁用设备不能通过 MQTT 鉴权。

### 8.2 客户绑定

功能：

- 客户输入 `SN + BindCode`。
- 可填写设备名称和位置。
- 绑定成功后设备进入当前租户。
- 绑定成功后跳转到设备详情或设备列表。

验收：

- 错误绑定码提示明确。
- 已绑定 SN 不能重复绑定。
- 超级管理员绑定时需要明确目标租户，不能默认误绑定。
- 绑定后遥测数据只能在所属租户下查看。

### 8.3 在线状态

功能：

- 工厂心跳更新出厂网关在线状态。
- 正式设备心跳更新设备在线状态。
- 连接断开不应立即离线，应该经过短暂宽限期。

验收：

- 网关断网后在设定超时时间内变为离线。
- 页面状态、弹窗提醒、设备列表状态使用同一份状态来源。
- 不出现弹窗已经离线但设备列表仍在线的明显不一致。

## 9. Go 网关功能需求

### 9.1 硬件 ID

功能：

- 启动时读取硬件唯一 ID。
- 在本地“运行状态”或“设备激活”页面展示。
- 读取失败时给出可诊断原因。

建议读取优先级：

1. 芯片唯一 ID。
2. eMMC CID。
3. CPU Serial。
4. 主网卡 MAC。
5. 多字段组合后做 SHA-256 摘要。

### 9.2 设备激活

功能：

- 本地页面展示 `HardwareId`。
- 支持上传或粘贴 `activation.local.json`。
- 保存后自动应用 MQTT 连接。
- 激活通道和手动 MQTT 通道可以分别启用或停用。

验收：

- 保存激活文件后不需要重启即可连接。
- 激活通道关闭后不再连接工厂主题。
- 手动 MQTT 通道关闭后不影响本地采集。
- 两个通道同时开启时页面要明确显示各自状态。

### 9.3 上报数据

功能：

- 支持网关自身心跳。
- 支持子设备心跳。
- 支持子设备遥测。
- 支持断线重连。
- 后续支持本地缓存和断点续传。

验收：

- MQTT 断开后自动重连。
- 网络恢复后状态能恢复在线。
- 遥测 payload 能被云平台实时指标和最近上报正确展示。

## 10. 数据模型建议

### 10.1 FactoryGateway

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `hardwareId` | 硬件唯一 ID，唯一索引 |
| `sn` | 平台 SN，唯一索引 |
| `deviceSecretHash` | DeviceSecret 哈希 |
| `bindCodeHash` | BindCode 哈希 |
| `batchNo` | 生产批次 |
| `status` | `UNBOUND`、`BOUND`、`DISABLED` |
| `firstSeenAt` | 首次上线时间 |
| `lastSeenAt` | 最近心跳时间 |
| `boundAt` | 绑定时间 |
| `deviceId` | 绑定后的云平台设备 ID |
| `createdAt` | 创建时间 |
| `updatedAt` | 更新时间 |

### 10.2 Device

| 字段 | 说明 |
| --- | --- |
| `deviceKey` | 设备标识，网关建议使用 SN |
| `deviceType` | `GATEWAY`、`GATEWAY_CHILD`、`DIRECT` |
| `tenantId` | 所属租户 |
| `status` | `ONLINE`、`OFFLINE` |
| `lastSeenAt` | 最近在线或遥测时间 |
| `mqttClientId` | MQTT ClientId |
| `mqttUsername` | MQTT Username |
| `mqttPasswordHash` | MQTT 密码哈希 |

## 11. 接口建议

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| `GET` | `/api/factory-gateways` | 出厂网关列表 | 超级管理员 |
| `POST` | `/api/factory-gateways` | 登记出厂网关并生成激活信息 | 超级管理员 |
| `POST` | `/api/factory-gateways/bind` | 客户绑定网关 | 超级管理员、租户管理员 |
| `POST` | `/api/factory-gateways/:id/bind-code/reset` | 重置绑定码 | 超级管理员 |
| `POST` | `/api/mqtt/auth` | MQTT 鉴权 | Broker 回调 |
| `POST` | `/api/mqtt/acl` | MQTT ACL | Broker 回调 |

后续建议补充：

- `POST /api/factory-gateways/:id/disable`
- `POST /api/factory-gateways/:id/enable`
- `GET /api/factory-gateways/:id/activation-file`
- `GET /api/factory-gateways/:id/logs`

## 12. 安全要求

1. `DeviceSecret` 不允许明文落库，只保存哈希。
2. `BindCode` 不允许明文落库，只保存哈希。
3. 激活文件只在生成时下载，后续不能再次查看明文密钥。
4. 已绑定设备不能再次绑定到其他租户。
5. MQTT ACL 必须限制 topic，不能只校验用户名密码。
6. 操作日志记录登记、绑定、重置绑定码、禁用、启用。
7. 登录日志记录 IP、账号、时间和结果。
8. 生产环境建议 activation 文件增加签名和过期策略。

## 13. 页面建议

### 云平台

- 左侧菜单增加“出厂网关”或放入“设备管理”下。
- 页面分为：
  - 出厂网关列表
  - 登记网关
  - 绑定网关
  - 最近上线状态
  - 操作记录

### Go 网关本地页面

- 左侧菜单：
  - 运行状态
  - 智能网关
  - 云平台连接
  - 设备激活
  - 接口配置
  - 采集点表
  - 系统维护
- “云平台连接”展示手动 MQTT 通道。
- “设备激活”展示激活通道。
- 运行状态只展示运行参数和连接状态，不展示编辑表单。

## 14. 关键验收场景

| 场景 | 预期结果 |
| --- | --- |
| 新 HardwareId 登记 | 平台生成 SN、DeviceSecret、BindCode、激活文件 |
| 重复 HardwareId 登记 | 返回错误 |
| 网关导入激活文件后启动 | 平台显示在线未绑定 |
| 错误 BindCode 绑定 | 返回错误，不创建设备 |
| 正确 SN + BindCode 绑定 | 创建网关设备并归属当前租户 |
| 已绑定网关再次绑定 | 返回错误 |
| 网关断网 | 超时后设备离线 |
| 网关恢复网络 | 自动上线 |
| 子设备遥测上报 | 数据进入绑定租户，设备详情实时更新 |
| 禁用出厂网关 | MQTT 鉴权拒绝 |

## 15. 当前风险与待确认

| 风险 | 说明 | 建议 |
| --- | --- | --- |
| HardwareId 来源不统一 | 不同硬件平台可读取字段不同 | 明确目标硬件后固化读取优先级 |
| 激活文件泄露 | 文件内含 DeviceSecret | 文件权限 0600，后续增加签名和密钥轮换 |
| 超级管理员绑定默认租户 | 容易绑定到错误租户 | 产品化时要求显式选择租户 |
| 工厂通道和手动 MQTT 通道并存 | 使用者可能混淆 | 页面明确“生产推荐激活通道，手动通道仅调试” |
| 状态来源不一致 | 弹窗、列表、详情可能使用不同事件源 | 统一由 ingest 写状态，前端只消费同一事件 |

## 16. 推荐落地顺序

1. 固化云平台出厂网关数据模型和接口。
2. 完成 Go 网关硬件 ID 读取和激活文件导入。
3. 完成 MQTT 鉴权和 ACL。
4. 完成工厂心跳和在线未绑定状态。
5. 完成客户绑定页面和租户归属。
6. 完成正式设备数据路由和权限隔离。
7. 补齐操作日志、设备日志和异常告警。
8. 做端到端联调和部署文档。


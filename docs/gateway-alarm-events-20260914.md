# 网关告警事件上报

## 行为

网关告警规则产生以下生命周期变化时，各生成一条事件：

- `ACTIVE`：告警触发
- `ACKNOWLEDGED`：操作员确认知晓
- `RESOLVED`：条件恢复后自动恢复
- `MANUAL_RESOLVED`：操作员手动恢复

活动告警的实时值刷新不会生成事件，避免按采集周期重复上报。

## MQTT 协议

- Topic：`weikong/gateways/{gatewayKey}/events`
- QoS：1
- Payload：版本化 JSON，顶层包含 `schemaVersion`、`eventType`、`gatewayKey`、`reportedAt` 和 `event`
- 去重：云端按 `gatewayKey + eventId + state + updatedAt` 生成唯一键

## 可靠性

事件先写入网关本地 `.runtime/cloud-alarm-events.jsonl`，再发布到云端。连接中断时保留未发送事件，恢复后按顺序重传。队列采用至少一次语义；如果在收到 MQTT 确认后立即断电，云端唯一键会消除可能的重复消息。

## 云端展示

云平台新增“事件中心”，租户用户只能查看本租户事件，公司管理员可查看全平台事件。页面支持网关、等级、事件动作、时间范围和关键字筛选，并可查看完整事件详情。

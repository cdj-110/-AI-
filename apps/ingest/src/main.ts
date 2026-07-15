import { Prisma, PrismaClient } from '@prisma/client';
import mqtt from 'mqtt';
import { Pool } from 'pg';

process.env.DATABASE_URL ??= 'postgresql://weikong:weikong123@localhost:5432/weikong_iot?schema=public';

const prisma = new PrismaClient();
const mqttUrl = process.env.MQTT_URL ?? 'mqtt://localhost:1883';
// 直连设备上报主题：设备编号来自 Topic 中间段。
const heartbeatTopic = 'weikong/devices/+/heartbeat';
const telemetryTopic = 'weikong/devices/+/telemetry';
// 网关代发主题：网关设备用自己的凭证替子设备发布心跳/遥测。
const gatewayChildHeartbeatTopic = 'weikong/gateways/+/children/+/heartbeat';
const gatewayChildTelemetryTopic = 'weikong/gateways/+/children/+/telemetry';
const gatewayTopologyTopic = 'weikong/gateways/+/topology/reported';
const factoryGatewayHeartbeatTopic = 'weikong/factory/+/heartbeat';
// EMQX 系统事件用于快速感知 MQTT 客户端连接/断开。
const connectedTopic = '$SYS/brokers/+/clients/+/connected';
const disconnectedTopic = '$SYS/brokers/+/clients/+/disconnected';
const timescale = new Pool({
  connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5433/weikong_ts',
});
const offlineTimeoutSeconds = Number(process.env.DEVICE_OFFLINE_TIMEOUT_SECONDS ?? 90);
const scanIntervalSeconds = Number(process.env.DEVICE_OFFLINE_SCAN_INTERVAL_SECONDS ?? 30);
// ingest 重启后会丢失内存中的“已连接客户端集合”，保护期内不做离线扫描，避免误判。
const offlineScanStartupGraceSeconds = Number(process.env.DEVICE_OFFLINE_SCAN_STARTUP_GRACE_SECONDS ?? 300);
const temperatureHighThreshold = Number(process.env.TEMPERATURE_HIGH_THRESHOLD ?? 30);
const humidityHighThreshold = Number(process.env.HUMIDITY_HIGH_THRESHOLD ?? 80);
const batteryLowThreshold = Number(process.env.BATTERY_LOW_THRESHOLD ?? 20);
const connectedDeviceKeys = new Set<string>();
const disconnectGraceSeconds = Number(process.env.DEVICE_DISCONNECT_GRACE_SECONDS ?? 3);
const pendingOfflineTimers = new Map<string, ReturnType<typeof setTimeout>>();
const statusNotifyIntervalMs = Number(process.env.DEVICE_STATUS_NOTIFY_INTERVAL_MS ?? 5000);
const telemetryRealtimeWindowMs = Number(process.env.TELEMETRY_REALTIME_WINDOW_SECONDS ?? 30) * 1000;
const lastStatusNotifyMap = new Map<string, number>();
const startedAt = Date.now();
const resolvedAlarmSuppression = new Set<string>();
const client = mqtt.connect(mqttUrl, {
  clientId: `weikong-ingest-${Date.now()}`,
  username: process.env.MQTT_USERNAME ?? 'platform-ingest',
  password: process.env.MQTT_PASSWORD ?? process.env.MQTT_INGEST_PASSWORD ?? 'platform-ingest-secret',
});

interface AlarmRule {
  metric: string;
  type: string;
  level: string;
  threshold: number;
  isAbnormal: (value: number) => boolean;
  message: (deviceName: string, value: number, threshold: number) => string;
}

interface DeviceLogTarget {
  id: string;
  tenantId?: string | null;
  deviceKey: string;
  name?: string | null;
  status?: string;
}

const alarmRules: AlarmRule[] = [
  {
    metric: 'temperature',
    type: 'TEMPERATURE_HIGH',
    level: 'WARNING',
    threshold: temperatureHighThreshold,
    isAbnormal: (value) => value > temperatureHighThreshold,
    message: (name, value, threshold) => `${name} 温度过高：${value}°C，阈值 ${threshold}°C`,
  },
  {
    metric: 'humidity',
    type: 'HUMIDITY_HIGH',
    level: 'WARNING',
    threshold: humidityHighThreshold,
    isAbnormal: (value) => value > humidityHighThreshold,
    message: (name, value, threshold) => `${name} 湿度过高：${value}%，阈值 ${threshold}%`,
  },
  {
    metric: 'battery',
    type: 'BATTERY_LOW',
    level: 'WARNING',
    threshold: batteryLowThreshold,
    isAbnormal: (value) => value < batteryLowThreshold,
    message: (name, value, threshold) => `${name} 电量过低：${value}%，阈值 ${threshold}%`,
  },
];

client.on('connect', () => {
  console.log(`[ingest] connected to ${mqttUrl}`);
  client.subscribe(heartbeatTopic, (error) => {
    if (error) console.error('[ingest] subscribe failed', error);
    else console.log(`[ingest] subscribed to ${heartbeatTopic}`);
  });
  client.subscribe(telemetryTopic, (error) => {
    if (error) console.error('[ingest] telemetry subscribe failed', error);
    else console.log(`[ingest] subscribed to ${telemetryTopic}`);
  });
  client.subscribe([gatewayChildHeartbeatTopic, gatewayChildTelemetryTopic], (error) => {
    if (error) console.error('[ingest] gateway child subscribe failed', error);
    else console.log('[ingest] subscribed to gateway child topics');
  });
  client.subscribe(gatewayTopologyTopic, (error) => {
    if (error) console.error('[ingest] gateway topology subscribe failed', error);
    else console.log(`[ingest] subscribed to ${gatewayTopologyTopic}`);
  });
  client.subscribe(factoryGatewayHeartbeatTopic, (error) => {
    if (error) console.error('[ingest] factory gateway subscribe failed', error);
    else console.log(`[ingest] subscribed to ${factoryGatewayHeartbeatTopic}`);
  });
  client.subscribe([connectedTopic, disconnectedTopic], (error) => {
    if (error) console.error('[ingest] broker status subscribe failed', error);
    else console.log('[ingest] subscribed to MQTT client status events');
  });
});

client.on('message', async (topic, payload) => {
  if (topic.startsWith('$SYS/brokers/')) {
    await processConnectionStatus(topic);
    return;
  }
  const factoryHeartbeat = topic.match(/^weikong\/factory\/([^/]+)\/heartbeat$/);
  if (factoryHeartbeat) {
    await processFactoryGatewayHeartbeat(factoryHeartbeat[1]);
    return;
  }
  const topologyReport = topic.match(/^weikong\/gateways\/([^/]+)\/topology\/reported$/);
  if (topologyReport) {
    await processGatewayTopology(topologyReport[1], payload);
    return;
  }
  // 同一入口同时处理直连设备和网关子设备，先把 Topic 解析成统一路由结构。
  const routedTopic = parseDeviceTopic(topic);
  if (!routedTopic) return;
  const { deviceKey, gatewayKey, messageType } = routedTopic;
  try {
    const device = gatewayKey
      ? await findGatewayChild(gatewayKey, deviceKey)
      : await prisma.device.findUnique({ where: { deviceKey } });
    if (!device) {
      console.warn(gatewayKey
        ? `[ingest] ignored unbound child ${gatewayKey}/${deviceKey}`
        : `[ingest] ignored unknown device ${deviceKey}`);
      return;
    }
    await updateDeviceStatus(device, 'ONLINE', messageType === 'telemetry' ? 'TELEMETRY' : 'HEARTBEAT');
    if (messageType === 'telemetry') {
      // 支持扁平 JSON，也支持设备原始的 d.*.Val 嵌套格式。
      const telemetry = normalizeTelemetryPayload(JSON.parse(payload.toString()));
      const receivedAt = new Date();
      const eventTime = telemetry.time ?? receivedAt;
      const stored = await timescale.query<{ time: Date }>(
        `INSERT INTO telemetry_events (time, device_key, metrics)
         SELECT $1, $2, $3
         WHERE NOT EXISTS (
           SELECT 1 FROM telemetry_events WHERE device_key = $2 AND time = $1
         )
         RETURNING time`,
        [eventTime, device.deviceKey, telemetry.metrics],
      );
      if (!stored.rowCount) return;
      const isRealtime = Math.abs(receivedAt.getTime() - eventTime.getTime()) <= telemetryRealtimeWindowMs;
      if (isRealtime) {
        await timescale.query("SELECT pg_notify('telemetry_updates', $1)", [
          JSON.stringify({ deviceKey: device.deviceKey, time: stored.rows[0].time, metrics: telemetry.metrics }),
        ]);
      }
      await discoverMetrics(device.id, telemetry.metrics);
      if (isRealtime) await processAlarms(device, telemetry.metrics);
      console.log(gatewayKey ? `[ingest] gateway telemetry ${gatewayKey}/${device.deviceKey}` : `[ingest] telemetry ${device.deviceKey}`);
    } else {
      console.log(gatewayKey ? `[ingest] gateway heartbeat ${gatewayKey}/${device.deviceKey}` : `[ingest] heartbeat ${device.deviceKey}`);
    }
  } catch (error) {
    console.error(`[ingest] failed heartbeat ${deviceKey}`, error);
  }
});

function parseDeviceTopic(topic: string) {
  const direct = topic.match(/^weikong\/devices\/([^/]+)\/(heartbeat|telemetry)$/);
  if (direct) return { deviceKey: direct[1], messageType: direct[2] as 'heartbeat' | 'telemetry' };
  const gatewayChild = topic.match(/^weikong\/gateways\/([^/]+)\/children\/([^/]+)\/(heartbeat|telemetry)$/);
  if (gatewayChild) {
    return {
      gatewayKey: gatewayChild[1],
      deviceKey: gatewayChild[2],
      messageType: gatewayChild[3] as 'heartbeat' | 'telemetry',
    };
  }
  return null;
}

function normalizeTelemetryPayload(payload: unknown) {
  if (!isPlainObject(payload)) throw new Error('telemetry payload must be a JSON object');
  // 如果是 { d: { YH01: { Val: {...} } }, ts }，只取 Val 里的真实指标。
  const metrics = extractNestedValMetrics(payload) ?? payload;
  if (!isPlainObject(metrics)) throw new Error('telemetry metrics must be a JSON object');
  const sanitizedMetrics = sanitizeMetrics(metrics);
  if (!Object.keys(sanitizedMetrics).length) throw new Error('telemetry metrics must be a non-empty JSON object');
  return {
    metrics: sanitizedMetrics,
    time: parseTelemetryTime(payload.ts),
  };
}

function sanitizeMetrics(metrics: Record<string, unknown>) {
  return Object.fromEntries(Object.entries(metrics).filter(([key]) => key.trim().length > 0));
}

function extractNestedValMetrics(payload: Record<string, unknown>) {
  if (!isPlainObject(payload.d)) return null;
  const groups = Object.entries(payload.d)
    .filter((entry): entry is [string, Record<string, unknown>] => isPlainObject(entry[1]) && isPlainObject(entry[1].Val))
    .map(([groupKey, group]) => ({ groupKey, metrics: group.Val as Record<string, unknown> }));
  if (!groups.length) return null;
  if (groups.length === 1) return groups[0].metrics;

  // 多个分组时，如果字段重名，加上分组前缀避免覆盖。
  const merged: Record<string, unknown> = {};
  const duplicateKeys = new Set<string>();
  const seenKeys = new Set<string>();
  for (const group of groups) {
    for (const key of Object.keys(group.metrics)) {
      if (seenKeys.has(key)) duplicateKeys.add(key);
      seenKeys.add(key);
    }
  }
  for (const group of groups) {
    for (const [key, value] of Object.entries(group.metrics)) {
      merged[duplicateKeys.has(key) ? `${group.groupKey}.${key}` : key] = value;
    }
  }
  return merged;
}

function parseTelemetryTime(value: unknown) {
  if (typeof value !== 'string' && typeof value !== 'number') return undefined;
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? undefined : time;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

async function findGatewayChild(gatewayKey: string, childKey: string) {
  return prisma.device.findFirst({
    where: {
      deviceKey: childKey,
      deviceType: 'GATEWAY_CHILD',
      gateway: {
        deviceKey: gatewayKey,
        deviceType: 'GATEWAY',
      },
    },
  });
}

async function processFactoryGatewayHeartbeat(hardwareId: string) {
  const normalizedHardwareId = hardwareId.trim().toUpperCase();
  const now = new Date();
  const factoryGateway = await prisma.factoryGateway.findUnique({
    where: { hardwareId: normalizedHardwareId },
    include: { device: { select: { id: true, tenantId: true, deviceKey: true, name: true, status: true } } },
  });
  if (!factoryGateway) {
    console.warn(`[ingest] ignored unknown factory gateway ${normalizedHardwareId}`);
    return;
  }
  await prisma.factoryGateway.update({
    where: { id: factoryGateway.id },
    data: {
      firstSeenAt: factoryGateway.firstSeenAt ?? now,
      lastSeenAt: now,
    },
  });
  if (factoryGateway.device) {
    await updateDeviceStatus(factoryGateway.device, 'ONLINE', 'FACTORY_HEARTBEAT');
  }
  console.log(`[ingest] factory heartbeat ${normalizedHardwareId}${factoryGateway.device ? ` -> ${factoryGateway.device.deviceKey}` : ' (unbound)'}`);
}

interface ReportedTopologyDevice {
  deviceKey: string;
  name?: string;
  protocol?: string;
  address?: string;
  slaveId?: number;
  pointCount?: number;
  metrics?: ReportedTopologyMetric[];
}

interface ReportedTopologyMetric {
  identifier: string;
  name?: string;
  dataType?: string;
  unit?: string;
}

async function processGatewayTopology(gatewayKey: string, payload: Buffer) {
  let parsed: Record<string, unknown>;
  try {
    parsed = JSON.parse(payload.toString());
  } catch (error) {
    console.warn(`[ingest] ignored invalid topology ${gatewayKey}`, error);
    return;
  }
  const payloadGatewayKey = stringValue(parsed.gatewayKey) || gatewayKey;
  if (payloadGatewayKey !== gatewayKey) {
    console.warn(`[ingest] ignored topology gateway mismatch topic=${gatewayKey} payload=${payloadGatewayKey}`);
    return;
  }
  const gateway = await prisma.device.findFirst({
    where: { deviceKey: gatewayKey, deviceType: 'GATEWAY' },
    include: { children: true },
  });
  if (!gateway) {
    console.warn(`[ingest] ignored topology from unknown gateway ${gatewayKey}`);
    return;
  }
  const reportedDevices = normalizeTopologyDevices(parsed.devices);
  const deviceKeys = [...new Set(reportedDevices.map((device) => device.deviceKey))];
  const existingDevices = deviceKeys.length
    ? await prisma.device.findMany({
        where: { deviceKey: { in: deviceKeys } },
        include: { gateway: { select: { id: true, deviceKey: true } } },
      })
    : [];
  const existingByKey = new Map(existingDevices.map((device) => [device.deviceKey, device]));
  const reportedKeySet = new Set(deviceKeys);
  const items: Array<Record<string, unknown>> = [];
  const conflicts: Array<{ deviceKey: string; reason: string }> = [];

  for (const reported of reportedDevices) {
    const existing = existingByKey.get(reported.deviceKey);
    if (!existing) {
      items.push({ ...reported, status: 'NEW' });
      continue;
    }
    const conflictReason =
      existing.id === gateway.id ? 'deviceKey matches gateway itself'
      : existing.tenantId !== gateway.tenantId ? 'deviceKey belongs to another tenant'
      : existing.gatewayId && existing.gatewayId !== gateway.id ? 'deviceKey belongs to another gateway'
      : existing.deviceType !== 'GATEWAY_CHILD' && existing.deviceType !== 'DIRECT' ? `device type ${existing.deviceType} cannot become gateway child`
      : '';
    if (conflictReason) {
      items.push({
        ...reported,
        status: 'CONFLICT',
        existingDeviceId: existing.id,
        currentGatewayId: existing.gatewayId,
        currentGatewayKey: existing.gateway?.deviceKey,
        conflictReason,
      });
      conflicts.push({ deviceKey: reported.deviceKey, reason: conflictReason });
      continue;
    }
    items.push({
      ...reported,
      status: hasTopologyChange(existing, reported, gateway.id) ? 'UPDATE' : 'UNCHANGED',
      existingDeviceId: existing.id,
      currentGatewayId: existing.gatewayId,
      currentGatewayKey: existing.gateway?.deviceKey,
    });
  }

  for (const child of gateway.children) {
    if (reportedKeySet.has(child.deviceKey)) continue;
    items.push({
      deviceKey: child.deviceKey,
      name: child.name,
      protocol: child.protocol,
      status: 'MISSING_UNREPORTED',
      existingDeviceId: child.id,
      currentGatewayId: gateway.id,
      currentGatewayKey: gateway.deviceKey,
    });
  }

  const hasPendingChange = items.some((item) => ['NEW', 'UPDATE', 'MISSING_UNREPORTED'].includes(String(item.status)));
  const status = conflicts.length ? 'CONFLICT' : hasPendingChange ? 'PENDING' : 'CONFIRMED';
  await prisma.gatewayTopologyDraft.upsert({
    where: { gatewayDeviceId: gateway.id },
    create: {
      gatewayDeviceId: gateway.id,
      gatewayKey,
      reportedConfigVersion: stringValue(parsed.configVersion) || undefined,
      items: items as unknown as Prisma.InputJsonValue,
      conflicts: conflicts.length ? conflicts as unknown as Prisma.InputJsonValue : undefined,
      status,
      reportedAt: parseDate(parsed.updatedAt) ?? new Date(),
    },
    update: {
      gatewayKey,
      reportedConfigVersion: stringValue(parsed.configVersion) || null,
      items: items as unknown as Prisma.InputJsonValue,
      conflicts: conflicts.length ? conflicts as unknown as Prisma.InputJsonValue : Prisma.DbNull,
      status,
      reportedAt: parseDate(parsed.updatedAt) ?? new Date(),
      confirmedAt: null,
      confirmedBy: null,
      ignoredAt: null,
      ignoredBy: null,
    },
  });
  console.log(`[ingest] topology ${gatewayKey} devices=${reportedDevices.length} status=${status}`);
}

function normalizeTopologyDevices(value: unknown): ReportedTopologyDevice[] {
  if (!Array.isArray(value)) return [];
  const seen = new Set<string>();
  const devices: ReportedTopologyDevice[] = [];
  for (const item of value) {
    if (!isPlainObject(item)) continue;
    const deviceKey = stringValue(item.deviceKey).trim();
    if (!deviceKey || seen.has(deviceKey)) continue;
    seen.add(deviceKey);
    const metrics = Array.isArray(item.metrics)
      ? item.metrics.map(normalizeTopologyMetric).filter((metric): metric is ReportedTopologyMetric => Boolean(metric))
      : [];
    devices.push({
      deviceKey,
      name: stringValue(item.name) || undefined,
      protocol: stringValue(item.protocol) || undefined,
      address: stringValue(item.address) || undefined,
      slaveId: typeof item.slaveId === 'number' ? item.slaveId : undefined,
      pointCount: typeof item.pointCount === 'number' ? item.pointCount : metrics.length,
      metrics,
    });
  }
  return devices;
}

function normalizeTopologyMetric(value: unknown): ReportedTopologyMetric | null {
  if (!isPlainObject(value)) return null;
  const identifier = stringValue(value.identifier).trim();
  if (!identifier) return null;
  return {
    identifier,
    name: stringValue(value.name) || undefined,
    dataType: stringValue(value.dataType) || undefined,
    unit: stringValue(value.unit) || undefined,
  };
}

function hasTopologyChange(
  existing: { name: string; protocol: string; gatewayId: string | null },
  reported: ReportedTopologyDevice,
  gatewayId: string,
) {
  return existing.gatewayId !== gatewayId
    || Boolean(reported.name && reported.name !== existing.name)
    || Boolean(reported.protocol && reported.protocol !== existing.protocol);
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function parseDate(value: unknown) {
  if (typeof value !== 'string' && typeof value !== 'number') return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

async function processConnectionStatus(topic: string) {
  const match = topic.match(/^\$SYS\/brokers\/[^/]+\/clients\/([^/]+)\/(connected|disconnected)$/);
  if (!match) return;
  const [, mqttClientId, event] = match;
  if (mqttClientId.startsWith('weikong-ingest-')) return;
  const device = await findDeviceByMqttClientId(mqttClientId);
  if (!device) return;
  if (event === 'connected') {
    // 连接事件可以立即把设备置为在线，断开事件会先等待 grace 时间防抖。
    connectedDeviceKeys.add(device.deviceKey);
    cancelPendingOffline(device.deviceKey);
    await recordDeviceLog(device, 'MQTT_CONNECTED', 'MQTT_BROKER', 'MQTT 客户端已连接', { mqttClientId });
    await updateDeviceStatus(device, 'ONLINE', 'MQTT_BROKER');
    console.log(`[ingest] client connected ${device.deviceKey}`);
    return;
  }

  connectedDeviceKeys.delete(device.deviceKey);
  await recordDeviceLog(device, 'MQTT_DISCONNECTED', 'MQTT_BROKER', 'MQTT 客户端已断开，等待离线判定', { mqttClientId, disconnectGraceSeconds });
  schedulePendingOffline(device.deviceKey);
  console.log(`[ingest] client disconnected ${device.deviceKey}, waiting ${disconnectGraceSeconds}s before marking offline`);
}

async function findDeviceByMqttClientId(mqttClientId: string) {
  if (mqttClientId.startsWith('factory_')) {
    const hardwareId = mqttClientId.slice('factory_'.length).trim().toUpperCase();
    if (hardwareId) {
      const factoryGateway = await prisma.factoryGateway.findUnique({
        where: { hardwareId },
        include: { device: { select: { id: true, tenantId: true, deviceKey: true, name: true, status: true } } },
      });
      if (factoryGateway?.device) return factoryGateway.device;
    }
  }
  return prisma.device.findFirst({
    where: { OR: [{ mqttClientId }, { deviceKey: mqttClientId }] },
    select: { id: true, tenantId: true, deviceKey: true, name: true, status: true },
  });
}

async function updateDeviceStatus(device: DeviceLogTarget, status: string, source: string) {
  if (status === 'ONLINE') cancelPendingOffline(device.deviceKey);
  const lastSeenAt = new Date();
  await prisma.device.update({
    where: { id: device.id },
    data: {
      status,
      ...(status === 'ONLINE' ? { lastSeenAt } : {}),
    },
  });
  if (device.status !== status) {
    await recordDeviceLog(
      device,
      status,
      source,
      status === 'ONLINE' ? '设备已上线' : '设备已离线',
      status === 'ONLINE' ? { lastSeenAt } : undefined,
    );
  }
  // 状态变化必推送；持续在线时按节流频率推送 lastSeenAt，保证页面时间不会太滞后。
  if (device.status !== status || shouldNotifyStatusRefresh(device.deviceKey, status)) {
    await notifyDeviceStatus({ id: device.id, deviceKey: device.deviceKey, status, lastSeenAt: status === 'ONLINE' ? lastSeenAt : undefined });
  }
}

function shouldNotifyStatusRefresh(deviceKey: string, status: string) {
  if (status !== 'ONLINE') return false;
  const now = Date.now();
  const previous = lastStatusNotifyMap.get(deviceKey) ?? 0;
  if (now - previous < statusNotifyIntervalMs) return false;
  lastStatusNotifyMap.set(deviceKey, now);
  return true;
}

function cancelPendingOffline(deviceKey: string) {
  const timer = pendingOfflineTimers.get(deviceKey);
  if (!timer) return;
  clearTimeout(timer);
  pendingOfflineTimers.delete(deviceKey);
}

function schedulePendingOffline(deviceKey: string) {
  cancelPendingOffline(deviceKey);
  pendingOfflineTimers.set(deviceKey, setTimeout(() => {
    pendingOfflineTimers.delete(deviceKey);
    void markDeviceOffline(deviceKey);
  }, disconnectGraceSeconds * 1000));
}

async function markDeviceOffline(deviceKey: string) {
  const device = await prisma.device.findUnique({ where: { deviceKey }, select: { id: true, tenantId: true, deviceKey: true, name: true, status: true } });
  if (!device || device.status === 'OFFLINE') return;
  await prisma.device.update({ where: { id: device.id }, data: { status: 'OFFLINE' } });
  lastStatusNotifyMap.delete(deviceKey);
  await recordDeviceLog(device, 'OFFLINE', 'MQTT_BROKER', '设备已离线');
  await notifyDeviceStatus({ id: device.id, deviceKey, status: 'OFFLINE' });
  console.log(`[ingest] client offline ${deviceKey}`);
}

async function recordDeviceLog(device: DeviceLogTarget, type: string, source: string, message: string, detail?: Record<string, unknown>, level = 'INFO') {
  try {
    await prisma.deviceLog.create({
      data: {
        tenantId: device.tenantId ?? undefined,
        deviceId: device.id,
        deviceKey: device.deviceKey,
        deviceName: device.name ?? undefined,
        type,
        level,
        source,
        message,
        detail: detail ? JSON.parse(JSON.stringify(detail)) : undefined,
      },
    });
  } catch (error) {
    console.warn('[ingest] failed to write device log', error);
  }
}

async function notifyDeviceStatus(device: { id: string; deviceKey: string; status: string; lastSeenAt?: Date }) {
  await prisma.$executeRawUnsafe(
    'SELECT pg_notify($1, $2)',
    'device_status_updates',
    JSON.stringify(device),
  );
}

async function processAlarms(device: { id: string; tenantId: string; name: string }, metrics: Record<string, unknown>) {
  // 已忽略字段不参与告警，避免页面看不到的字段仍在后台触发告警。
  const activeMetrics = await prisma.deviceMetric.findMany({ where: { deviceId: device.id, ignored: false }, select: { identifier: true } });
  const activeIdentifiers = new Set(activeMetrics.map((metric) => metric.identifier));
  const configuredRules = await prisma.deviceAlarmRule.findMany({ where: { deviceId: device.id, enabled: true } });
  const configuredIdentifiers = new Set(configuredRules.map((rule) => rule.identifier));
  for (const rule of configuredRules) {
    if (!activeIdentifiers.has(rule.identifier)) continue;
    const value = metrics[rule.identifier];
    if (typeof value !== 'number' || !Number.isFinite(value)) continue;
    await syncAlarm(
      device,
      `RULE_${rule.id}`,
      rule.level,
      `${device.name} ${rule.identifier} ${rule.operator} ${rule.threshold}，当前值 ${value}`,
      value,
      rule.threshold,
      compare(value, rule.operator, rule.threshold),
      isRecovered(value, rule.operator, rule.threshold, rule.hysteresis),
    );
  }
  for (const rule of alarmRules.filter((rule) => !configuredIdentifiers.has(rule.metric))) {
    if (!activeIdentifiers.has(rule.metric)) continue;
    const value = metrics[rule.metric];
    if (typeof value !== 'number' || !Number.isFinite(value)) continue;
    const abnormal = rule.isAbnormal(value);
    await syncAlarm(device, rule.type, rule.level, rule.message(device.name, value, rule.threshold), value, rule.threshold, abnormal, !abnormal);
  }
}

async function discoverMetrics(deviceId: string, metrics: Record<string, unknown>) {
  // 新字段自动注册到设备物模型；用户后续可在设备配置中改名、排序、忽略。
  for (const [identifier, value] of Object.entries(metrics)) {
    if (!identifier.trim()) continue;
    await prisma.deviceMetric.upsert({
      where: { deviceId_identifier: { deviceId, identifier } },
      create: { deviceId, identifier, ...defaultMetricMetadata(identifier), dataType: inferDataType(value) },
      update: {},
    });
  }
}

function defaultMetricMetadata(identifier: string) {
  const metadata: Record<string, { name: string; unit?: string; sortOrder?: number }> = {
    temperature: { name: '温度', unit: '°C', sortOrder: 10 },
    humidity: { name: '湿度', unit: '%', sortOrder: 20 },
    pressure: { name: '压力', unit: 'kPa', sortOrder: 30 },
    fenbei: { name: '分贝', unit: 'dB', sortOrder: 40 },
    voltage: { name: '电压', unit: 'V', sortOrder: 50 },
    current: { name: '电流', unit: 'A', sortOrder: 60 },
    power: { name: '功率', unit: 'W', sortOrder: 70 },
    battery: { name: '电量', unit: '%', sortOrder: 80 },
  };
  return metadata[identifier] ?? { name: identifier };
}

function inferDataType(value: unknown) {
  if (typeof value === 'number') return 'NUMBER';
  if (typeof value === 'boolean') return 'BOOLEAN';
  if (typeof value === 'string') return 'STRING';
  return 'OBJECT';
}

function compare(value: number, operator: string, threshold: number) {
  if (operator === '>') return value > threshold;
  if (operator === '>=') return value >= threshold;
  if (operator === '<') return value < threshold;
  return value <= threshold;
}

function isRecovered(value: number, operator: string, threshold: number, hysteresis: number) {
  if (operator === '>' || operator === '>=') return value <= threshold - hysteresis;
  return value >= threshold + hysteresis;
}

async function syncAlarm(
  device: { id: string; tenantId: string; name: string },
  type: string,
  level: string,
  message: string,
  value: number,
  threshold: number,
  abnormal: boolean,
  recovered: boolean,
) {
  const alarmKey = `${device.id}:${type}`;
  const openAlarm = await prisma.alarm.findFirst({ where: { deviceId: device.id, type, status: 'OPEN' } });
  if (recovered) {
    resolvedAlarmSuppression.delete(alarmKey);
  }
  if (abnormal && !openAlarm) {
    if (resolvedAlarmSuppression.has(alarmKey)) return;
    const latestAlarm = await prisma.alarm.findFirst({ where: { deviceId: device.id, type }, orderBy: { createdAt: 'desc' } });
    if (latestAlarm?.status === 'RESOLVED') {
      resolvedAlarmSuppression.add(alarmKey);
      return;
    }
    const alarm = await prisma.alarm.create({
      data: { tenantId: device.tenantId, deviceId: device.id, type, level, message, value, threshold },
    });
    await notifyAlarmEvent({
      id: alarm.id,
      tenantId: alarm.tenantId,
      deviceId: alarm.deviceId,
      deviceName: device.name,
      type: alarm.type,
      level: alarm.level,
      message: alarm.message,
      status: alarm.status,
      createdAt: alarm.createdAt,
    });
    console.log(`[ingest] alarm opened ${type} for ${device.name}`);
  } else if (recovered && openAlarm) {
    const alarm = await prisma.alarm.update({ where: { id: openAlarm.id }, data: { status: 'RESOLVED', resolvedAt: new Date() } });
    resolvedAlarmSuppression.delete(alarmKey);
    await notifyAlarmEvent({
      id: alarm.id,
      tenantId: alarm.tenantId,
      deviceId: alarm.deviceId,
      deviceName: device.name,
      type: alarm.type,
      level: alarm.level,
      message: alarm.message,
      status: alarm.status,
      createdAt: alarm.createdAt,
      resolvedAt: alarm.resolvedAt,
    });
    console.log(`[ingest] alarm resolved ${type} for ${device.name}`);
  }
}

async function notifyAlarmEvent(alarm: Record<string, unknown>) {
  await prisma.$executeRawUnsafe(
    'SELECT pg_notify($1, $2)',
    'alarm_events',
    JSON.stringify(alarm),
  );
}

client.on('error', (error) => console.error('[ingest] mqtt error', error));

setInterval(async () => {
  if (Date.now() - startedAt < offlineScanStartupGraceSeconds * 1000) return;
  // 兜底离线扫描：如果设备长时间没有 lastSeenAt 更新，且当前未记录为已连接，则标记离线。
  const threshold = new Date(Date.now() - offlineTimeoutSeconds * 1000);
  try {
    const staleDevices = await prisma.device.findMany({
      where: { status: 'ONLINE', lastSeenAt: { lt: threshold } },
      select: { id: true, tenantId: true, deviceKey: true, name: true },
    });
    const staleIds = staleDevices.filter((device) => !connectedDeviceKeys.has(device.deviceKey)).map((device) => device.id);
    if (!staleIds.length) return;
    const result = await prisma.device.updateMany({
      where: { id: { in: staleIds } },
      data: { status: 'OFFLINE' },
    });
    if (result.count) {
      await Promise.all(staleDevices
        .filter((device) => staleIds.includes(device.id))
        .map(async (device) => {
          await recordDeviceLog(device, 'OFFLINE', 'OFFLINE_SCAN', '设备超过离线阈值，已判定离线', { offlineTimeoutSeconds });
          await notifyDeviceStatus({ id: device.id, deviceKey: device.deviceKey, status: 'OFFLINE' });
        }));
      console.log(`[ingest] marked ${result.count} device(s) offline`);
    }
  } catch (error) {
    console.error('[ingest] offline scan failed', error);
  }
}, scanIntervalSeconds * 1000);

async function shutdown() {
  client.end();
  await prisma.$disconnect();
  await timescale.end();
}

async function initTimescale() {
  await timescale.query('CREATE EXTENSION IF NOT EXISTS timescaledb');
  await timescale.query(`
    CREATE TABLE IF NOT EXISTS telemetry_events (
      time TIMESTAMPTZ NOT NULL,
      device_key TEXT NOT NULL,
      metrics JSONB NOT NULL
    )
  `);
  await timescale.query("SELECT create_hypertable('telemetry_events', 'time', if_not_exists => TRUE)");
  const deduplicated = await timescale.query(`
    DELETE FROM telemetry_events target
    USING (
      SELECT device_key, time, min(ctid) AS keep_ctid
      FROM telemetry_events
      GROUP BY device_key, time
      HAVING count(*) > 1
    ) duplicates
    WHERE target.device_key = duplicates.device_key
      AND target.time = duplicates.time
      AND target.ctid <> duplicates.keep_ctid
  `);
  if (deduplicated.rowCount) console.log(`[ingest] removed ${deduplicated.rowCount} duplicate telemetry row(s)`);
  await timescale.query('CREATE UNIQUE INDEX IF NOT EXISTS telemetry_events_device_time_uidx ON telemetry_events (device_key, time)');
}

initTimescale().catch((error) => {
  console.error('[ingest] timescale init failed', error);
  process.exitCode = 1;
});

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

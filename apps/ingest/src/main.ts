import { Prisma, PrismaClient } from '@prisma/client';
import mqtt from 'mqtt';
import { Pool } from 'pg';
import { parseLicenseReport, persistLicenseReport } from './license-report';
import { reportedPointAccess, syncGatewayPointAccess } from './gateway-point-access';
import { isAbortError, retryUntilReady } from './startup-readiness';

process.env.DATABASE_URL ??= 'postgresql://weikong@localhost:5432/weikong_iot?schema=public';

const prisma = new PrismaClient();
const mqttUrl = process.env.MQTT_URL ?? 'mqtt://localhost:1883';
// 直连设备上报主题：设备编号来自 Topic 中间段。
const heartbeatTopic = 'weikong/devices/+/heartbeat';
const telemetryTopic = 'weikong/devices/+/telemetry';
// 网关代发主题：网关设备用自己的凭证替子设备发布心跳/遥测。
const gatewayChildHeartbeatTopic = 'weikong/gateways/+/children/+/heartbeat';
const gatewayChildTelemetryTopic = 'weikong/gateways/+/children/+/telemetry';
const gatewayTopologyTopic = 'weikong/gateways/+/topology/reported';
const gatewayEventTopic = 'weikong/gateways/+/events';
const factoryGatewayHeartbeatTopic = 'weikong/factory/+/heartbeat';
// EMQX 系统事件用于快速感知 MQTT 客户端连接/断开。
const connectedTopic = '$SYS/brokers/+/clients/+/connected';
const disconnectedTopic = '$SYS/brokers/+/clients/+/disconnected';
const timescale = new Pool({
  connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong@localhost:5433/weikong_ts',
});
const offlineTimeoutSeconds = Number(process.env.DEVICE_OFFLINE_TIMEOUT_SECONDS ?? 90);
const scanIntervalSeconds = Number(process.env.DEVICE_OFFLINE_SCAN_INTERVAL_SECONDS ?? 30);
// ingest 重启后会丢失内存中的“已连接客户端集合”，保护期内不做离线扫描，避免误判。
const offlineScanStartupGraceSeconds = Number(process.env.DEVICE_OFFLINE_SCAN_STARTUP_GRACE_SECONDS ?? 300);
const temperatureHighThreshold = Number(process.env.TEMPERATURE_HIGH_THRESHOLD ?? 30);
const humidityHighThreshold = Number(process.env.HUMIDITY_HIGH_THRESHOLD ?? 80);
const batteryLowThreshold = Number(process.env.BATTERY_LOW_THRESHOLD ?? 20);
const connectedDeviceIds = new Set<string>();
const disconnectGraceSeconds = Number(process.env.DEVICE_DISCONNECT_GRACE_SECONDS ?? 3);
const pendingOfflineTimers = new Map<string, ReturnType<typeof setTimeout>>();
const statusNotifyIntervalMs = Number(process.env.DEVICE_STATUS_NOTIFY_INTERVAL_MS ?? 5000);
const telemetryRealtimeWindowMs = Number(process.env.TELEMETRY_REALTIME_WINDOW_SECONDS ?? 30) * 1000;
const lastStatusNotifyMap = new Map<string, number>();
const startedAt = Date.now();
const resolvedAlarmSuppression = new Set<string>();
let timescaleReady: Promise<void>;
const startupAbort = new AbortController();
const dependencyRetryMs = Math.max(1000, Number(process.env.DEPENDENCY_RETRY_SECONDS ?? 5) * 1000);
let mqttConnectionGeneration = 0;
// A resolved alarm may only fire again after this worker has observed the
// metric inside its recovery range. Keep that re-arm state separate from the
// suppression set so a historical RESOLVED row does not suppress it forever.
const rearmedAlarmKeys = new Set<string>();
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

client.on('connect', async () => {
  const generation = ++mqttConnectionGeneration;
  console.log(`[ingest] connected to ${mqttUrl}`);
  try {
    // Do not consume telemetry while a startup migration is rebuilding the
    // identity index. Otherwise ON CONFLICT can briefly have no matching key.
    await timescaleReady;
  } catch (error) {
    if (!isAbortError(error)) console.error('[ingest] dependency readiness failed', error);
    return;
  }
  await retryUntilReady(
    async () => {
      if (!client.connected || generation !== mqttConnectionGeneration) throw Object.assign(new Error('MQTT connection changed'), { name: 'AbortError' });
      await subscribeAllTopics();
    },
    {
      retryMs: dependencyRetryMs,
      signal: startupAbort.signal,
      shouldRetry: () => client.connected && generation === mqttConnectionGeneration,
      onFailure: (error, attempt) => {
        if (!isAbortError(error) && (attempt === 1 || attempt % 12 === 0)) console.error(`[ingest] MQTT subscribe attempt ${attempt} failed; retrying`, error);
      },
    },
  ).catch((error) => { if (!isAbortError(error)) console.error('[ingest] MQTT subscription recovery failed', error); });
});

client.on('close', () => { mqttConnectionGeneration += 1; });

function subscribe(topics: string | string[]) {
  return new Promise<void>((resolve, reject) => {
    client.subscribe(topics, (error, grants) => {
      if (error) return reject(error);
      if (grants?.some(grant => grant.qos === 128)) return reject(new Error('MQTT broker rejected one or more subscriptions'));
      resolve();
    });
  });
}

async function subscribeAllTopics() {
  await Promise.all([
    subscribe(heartbeatTopic),
    subscribe(telemetryTopic),
    subscribe([gatewayChildHeartbeatTopic, gatewayChildTelemetryTopic]),
    subscribe(gatewayTopologyTopic),
    subscribe(gatewayEventTopic),
    subscribe(factoryGatewayHeartbeatTopic),
    subscribe([connectedTopic, disconnectedTopic]),
  ]);
  console.log('[ingest] subscribed to all MQTT ingestion topics');
}

client.on('message', async (topic, payload) => {
  if (topic.startsWith('$SYS/brokers/')) {
    await processConnectionStatus(topic);
    return;
  }
  const factoryHeartbeat = topic.match(/^weikong\/factory\/([^/]+)\/heartbeat$/);
  if (factoryHeartbeat) {
    await processFactoryGatewayHeartbeat(factoryHeartbeat[1], payload);
    return;
  }
  const topologyReport = topic.match(/^weikong\/gateways\/([^/]+)\/topology\/reported$/);
  if (topologyReport) {
    await processGatewayTopology(topologyReport[1], payload);
    return;
  }
  const gatewayEvent = topic.match(/^weikong\/gateways\/([^/]+)\/events$/);
  if (gatewayEvent) {
    await processGatewayEvent(gatewayEvent[1], payload);
    return;
  }
  // 同一入口同时处理直连设备和网关子设备，先把 Topic 解析成统一路由结构。
  const routedTopic = parseDeviceTopic(topic);
  if (!routedTopic) return;
  const { deviceKey, gatewayKey, messageType } = routedTopic;
  try {
    const device = gatewayKey
      ? await findGatewayChild(gatewayKey, deviceKey)
      : await prisma.device.findFirst({
          where: { deviceKey, gatewayId: null, deviceType: { not: 'GATEWAY_CHILD' }, cloudAccessEnabled: true, tenant: { deviceCloudAccessDisabled: false } },
        });
    if (!device) {
      console.warn(gatewayKey
        ? `[ingest] ignored unbound child ${gatewayKey}/${deviceKey}`
        : `[ingest] ignored unknown device ${deviceKey}`);
      return;
    }
    if (!await updateDeviceStatus(device, 'ONLINE', messageType === 'telemetry' ? 'TELEMETRY' : 'HEARTBEAT')) return;
    if (messageType === 'telemetry') {
      // 支持扁平 JSON，也支持设备原始的 d.*.Val 嵌套格式。
      const telemetry = normalizeTelemetryPayload(JSON.parse(payload.toString()));
      const receivedAt = new Date();
      const eventTime = telemetry.time ?? receivedAt;
      const stored = await timescale.query<{ time: Date }>(
        `INSERT INTO telemetry_events (time, device_id, device_key, metrics)
         SELECT $1, $2, $3, $4
         WHERE NOT EXISTS (
           SELECT 1 FROM telemetry_events WHERE device_id = $2 AND time = $1
         )
         ON CONFLICT (device_id, time) DO NOTHING
         RETURNING time`,
        [eventTime, device.id, device.deviceKey, telemetry.metrics],
      );
      if (!stored.rowCount) return;
      const isRealtime = Math.abs(receivedAt.getTime() - eventTime.getTime()) <= telemetryRealtimeWindowMs;
      if (isRealtime) {
        await timescale.query("SELECT pg_notify('telemetry_updates', $1)", [
          JSON.stringify({ deviceId: device.id, deviceKey: device.deviceKey, time: stored.rows[0].time, metrics: telemetry.metrics }),
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
      cloudAccessEnabled: true,
      tenant: { deviceCloudAccessDisabled: false },
        gateway: {
          deviceKey: gatewayKey,
          deviceType: 'GATEWAY',
          gatewayId: null,
          cloudAccessEnabled: true,
          tenant: { deviceCloudAccessDisabled: false },
      },
    },
  });
}

const gatewayEventStates = new Set(['ACTIVE', 'ACKNOWLEDGED', 'RESOLVED', 'MANUAL_RESOLVED']);
const gatewayEventLevels = new Set(['INFO', 'WARNING', 'CRITICAL']);

async function processGatewayEvent(topicGatewayKey: string, payload: Buffer) {
  try {
    const parsed = JSON.parse(payload.toString()) as Record<string, unknown>;
    const payloadGatewayKey = stringValue(parsed.gatewayKey) || topicGatewayKey;
    if (payloadGatewayKey !== topicGatewayKey) {
      console.warn(`[ingest] ignored event gateway mismatch topic=${topicGatewayKey} payload=${payloadGatewayKey}`);
      return;
    }
    const event = isPlainObject(parsed.event) ? parsed.event : parsed;
    const sourceEventId = stringValue(event.eventId);
    const state = stringValue(event.state).toUpperCase();
    if (!sourceEventId || !gatewayEventStates.has(state)) {
      console.warn(`[ingest] ignored invalid gateway event ${topicGatewayKey}`);
      return;
    }
    const gateway = await prisma.device.findFirst({
      where: { deviceKey: topicGatewayKey, deviceType: 'GATEWAY', gatewayId: null, cloudAccessEnabled: true, tenant: { deviceCloudAccessDisabled: false } },
      select: { id: true, tenantId: true, deviceKey: true, name: true, status: true },
    });
    if (!gateway) {
      console.warn(`[ingest] ignored event from unknown gateway ${topicGatewayKey}`);
      return;
    }
    const sourceDeviceKey = stringValue(event.deviceKey);
    const sourceDevice = sourceDeviceKey ? await findGatewayChild(topicGatewayKey, sourceDeviceKey) : null;
    const occurredAt = validDate(event.occurredAt) ?? new Date();
    const sourceUpdatedAt = validDate(event.updatedAt) ?? occurredAt;
    const reportedAt = validDate(parsed.reportedAt) ?? new Date();
    const levelValue = stringValue(event.level).toUpperCase();
    const level = gatewayEventLevels.has(levelValue) ? levelValue : 'WARNING';
    const dedupeKey = [topicGatewayKey, sourceEventId, state, sourceUpdatedAt.toISOString()].join('|');
    await prisma.gatewayEvent.create({
      data: {
        dedupeKey,
        tenantId: gateway.tenantId,
        gatewayId: gateway.id,
        deviceId: sourceDevice?.id,
        sourceEventId,
        eventType: stringValue(parsed.eventType).toUpperCase() || 'ALARM',
        state,
        level,
        ruleId: optionalString(event.ruleId),
        ruleName: optionalString(event.ruleName),
        parentRuleId: optionalString(event.parentRuleId),
        parentRuleName: optionalString(event.parentRuleName),
        limitKey: optionalString(event.limitKey),
        limitName: optionalString(event.limitName),
        deviceKey: sourceDeviceKey || undefined,
        pointName: optionalString(event.pointName),
        metric: optionalString(event.metric),
        valueText: event.valueText === undefined ? optionalString(event.value) : optionalString(event.valueText),
        unit: optionalString(event.unit),
        threshold: finiteNumber(event.threshold),
        quality: optionalString(event.quality),
        message: stringValue(event.message) || `${stringValue(event.ruleName) || sourceEventId} ${state}`,
        acknowledged: Boolean(event.acknowledged || state === 'ACKNOWLEDGED'),
        occurredAt,
        sourceUpdatedAt,
        acknowledgedAt: validDate(event.acknowledgedAt),
        recoveredAt: validDate(event.recoveredAt),
        reportedAt,
        detail: parsed as Prisma.InputJsonObject,
      },
    });
    await updateDeviceStatus(gateway, 'ONLINE', 'GATEWAY_EVENT');
    await prisma.$executeRawUnsafe(
      'SELECT pg_notify($1, $2)',
      'gateway_events',
      JSON.stringify({ tenantId: gateway.tenantId, gatewayId: gateway.id, sourceEventId, state, level, occurredAt }),
    );
    console.log(`[ingest] gateway event ${topicGatewayKey}/${sourceEventId} ${state}`);
  } catch (error) {
    if (error instanceof Prisma.PrismaClientKnownRequestError && error.code === 'P2002') {
      return;
    }
    console.error(`[ingest] failed gateway event ${topicGatewayKey}`, error);
  }
}

function optionalString(value: unknown) {
  const normalized = stringValue(value).trim();
  return normalized || undefined;
}

function finiteNumber(value: unknown) {
  if (value === null || value === undefined || value === '') return undefined;
  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
}

function validDate(value: unknown) {
  if (typeof value !== 'string' && typeof value !== 'number' && !(value instanceof Date)) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

async function processFactoryGatewayHeartbeat(hardwareId: string, payload: Buffer) {
  const normalizedHardwareId = hardwareId.trim().toUpperCase();
  const now = new Date();
  const factoryGateway = await prisma.factoryGateway.findUnique({
    where: { hardwareId: normalizedHardwareId },
    include: { device: { select: { id: true, tenantId: true, deviceKey: true, name: true, status: true, cloudAccessEnabled: true, tenant: { select: { deviceCloudAccessDisabled: true } } } } },
  });
  if (!factoryGateway) {
    console.warn(`[ingest] ignored unknown factory gateway ${normalizedHardwareId}`);
    return;
  }
  if(['DISABLED','REMOVED'].includes(factoryGateway.status))return;
  if (factoryGateway.device?.tenant.deviceCloudAccessDisabled) return;
  if (factoryGateway.status === 'BOUND' && factoryGateway.device?.cloudAccessEnabled === false) return;
  const report=parseLicenseReport(payload,factoryGateway);
  if(report){
    await prisma.$transaction(tx=>persistLicenseReport(tx,factoryGateway,report,now));
  }
    const heartbeat = await prisma.factoryGateway.updateMany({
    where: { id: factoryGateway.id, licenseSequence:factoryGateway.licenseSequence, status:factoryGateway.status },
    data: {
      firstSeenAt: factoryGateway.firstSeenAt ?? now,
      lastSeenAt: now,
    },
  });
    if (heartbeat.count && factoryGateway.device) {
    await updateDeviceStatus(factoryGateway.device, 'ONLINE', 'FACTORY_HEARTBEAT');
  }
  console.log(`[ingest] factory heartbeat ${normalizedHardwareId}${factoryGateway.device ? ` -> ${factoryGateway.device.deviceKey}` : ' (unbound)'}`);
}

interface ReportedTopologyDevice {
	pointAccessVersion?: number;
  deviceKey: string;
  name?: string;
  protocol?: string;
  address?: string;
  slaveId?: number;
  pointCount?: number;
  metrics?: ReportedTopologyMetric[];
}

interface ReportedTopologyMetric {
	canRead?: boolean;
	canWrite?: boolean;
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
    where: { deviceKey: gatewayKey, deviceType: 'GATEWAY', gatewayId: null, cloudAccessEnabled: true, tenant: { deviceCloudAccessDisabled: false } },
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
        where: { gatewayId: gateway.id, deviceKey: { in: deviceKeys } },
        include: { gateway: { select: { id: true, deviceKey: true } } },
      })
    : [];
  const existingByKey = new Map(existingDevices.map((device) => [device.deviceKey, device]));
  const restrictions = deviceKeys.length
    ? await prisma.gatewayChildCloudRestriction.findMany({
        where: { gatewayDeviceId: gateway.id, childDeviceKey: { in: deviceKeys } },
      })
    : [];
  const restrictionByKey = new Map(restrictions.map((restriction) => [restriction.childDeviceKey, restriction]));
  const reportedKeySet = new Set(deviceKeys);
  const items: Array<Record<string, unknown>> = [];
  const conflicts: Array<{ deviceKey: string; reason: string }> = [];

  for (const reported of reportedDevices) {
    const existing = existingByKey.get(reported.deviceKey);
    const restriction = restrictionByKey.get(reported.deviceKey);
    if (restriction) {
      items.push({
        ...reported,
        status: 'PLATFORM_DISABLED',
        existingDeviceId: existing?.id,
        currentGatewayId: existing?.gatewayId ?? gateway.id,
        currentGatewayKey: gateway.deviceKey,
        conflictReason: restriction.reason || '该子设备已被管理平台禁用，只能由平台管理员解禁',
      });
      continue;
    }
    if (!existing) {
      items.push({ ...reported, status: 'NEW' });
      continue;
    }
    const conflictReason = existing.deviceType !== 'GATEWAY_CHILD'
      ? `device type ${existing.deviceType} cannot become gateway child`
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
    // Only actual gateway capabilities are synchronized; names/precision/visibility stay user-controlled.
    await syncGatewayPointAccess(prisma, existing.id, gateway.id, gateway.tenantId, reported.metrics ?? []);
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
      ...(parsed.pointAccessVersion === 1 ? { pointAccessVersion: 1 } : {}),
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
      ...(item.pointAccessVersion === 1 ? { pointAccessVersion: 1 } : {}),
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
    ...reportedPointAccess(value),
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
    connectedDeviceIds.add(device.id);
    cancelPendingOffline(device.id);
    await recordDeviceLog(device, 'MQTT_CONNECTED', 'MQTT_BROKER', 'MQTT 客户端已连接', { mqttClientId });
    await updateDeviceStatus(device, 'ONLINE', 'MQTT_BROKER');
    console.log(`[ingest] client connected ${device.deviceKey}`);
    return;
  }

  connectedDeviceIds.delete(device.id);
  await recordDeviceLog(device, 'MQTT_DISCONNECTED', 'MQTT_BROKER', 'MQTT 客户端已断开，等待离线判定', { mqttClientId, disconnectGraceSeconds });
  schedulePendingOffline(device.id);
  console.log(`[ingest] client disconnected ${device.deviceKey}, waiting ${disconnectGraceSeconds}s before marking offline`);
}

async function findDeviceByMqttClientId(mqttClientId: string) {
  if (mqttClientId.startsWith('factory_')) {
    const hardwareId = mqttClientId.slice('factory_'.length).trim().toUpperCase();
    if (hardwareId) {
      const factoryGateway = await prisma.factoryGateway.findUnique({
        where: { hardwareId },
        include: { device: { select: { id: true, tenantId: true, deviceKey: true, name: true, status: true, cloudAccessEnabled: true, tenant: { select: { deviceCloudAccessDisabled: true } } } } },
      });
      if (factoryGateway?.device?.cloudAccessEnabled && !factoryGateway.device.tenant.deviceCloudAccessDisabled) return factoryGateway.device;
    }
  }
  return prisma.device.findFirst({
    where: {
      cloudAccessEnabled: true,
      tenant: { deviceCloudAccessDisabled: false },
      NOT: { deviceType: 'GATEWAY_CHILD', gatewayId: null },
      OR: [{ mqttClientId }, { deviceKey: mqttClientId, gatewayId: null, deviceType: { not: 'GATEWAY_CHILD' } }],
    },
    select: { id: true, tenantId: true, deviceKey: true, name: true, status: true },
  });
}

async function updateDeviceStatus(device: DeviceLogTarget, status: string, source: string) {
  if (status === 'ONLINE') cancelPendingOffline(device.id);
  const lastSeenAt = new Date();
  const updated = await prisma.device.updateMany({
    where: { id: device.id, cloudAccessEnabled: true, tenant: { deviceCloudAccessDisabled: false } },
    data: {
      status,
      ...(status === 'ONLINE' ? { lastSeenAt } : {}),
    },
  });
  if (!updated.count) return false;
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
  if (device.status !== status || shouldNotifyStatusRefresh(device.id, status)) {
    await notifyDeviceStatus({ id: device.id, deviceKey: device.deviceKey, status, lastSeenAt: status === 'ONLINE' ? lastSeenAt : undefined });
  }
  return true;
}

function shouldNotifyStatusRefresh(deviceId: string, status: string) {
  if (status !== 'ONLINE') return false;
  const now = Date.now();
  const previous = lastStatusNotifyMap.get(deviceId) ?? 0;
  if (now - previous < statusNotifyIntervalMs) return false;
  lastStatusNotifyMap.set(deviceId, now);
  return true;
}

function cancelPendingOffline(deviceId: string) {
  const timer = pendingOfflineTimers.get(deviceId);
  if (!timer) return;
  clearTimeout(timer);
  pendingOfflineTimers.delete(deviceId);
}

function schedulePendingOffline(deviceId: string) {
  cancelPendingOffline(deviceId);
  pendingOfflineTimers.set(deviceId, setTimeout(() => {
    pendingOfflineTimers.delete(deviceId);
    void markDeviceOffline(deviceId);
  }, disconnectGraceSeconds * 1000));
}

async function markDeviceOffline(deviceId: string) {
  const device = await prisma.device.findUnique({ where: { id: deviceId }, select: { id: true, tenantId: true, deviceKey: true, name: true, status: true, cloudAccessEnabled: true } });
  if (!device || !device.cloudAccessEnabled || device.status === 'OFFLINE') return;
  await prisma.device.updateMany({ where: { id: device.id, cloudAccessEnabled: true }, data: { status: 'OFFLINE' } });
  lastStatusNotifyMap.delete(deviceId);
  await recordDeviceLog(device, 'OFFLINE', 'MQTT_BROKER', '设备已离线');
  await notifyDeviceStatus({ id: device.id, deviceKey: device.deviceKey, status: 'OFFLINE' });
  console.log(`[ingest] client offline ${device.deviceKey}`);
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
  const entries = Object.entries(metrics).filter(([identifier]) => identifier.trim());
  const existing = await prisma.deviceMetric.findMany({
    where: { deviceId, identifier: { in: entries.map(([identifier]) => identifier) } },
    select: { identifier: true },
  });
  const existingIdentifiers = new Set(existing.map(item => item.identifier));
  const quota = await prisma.$queryRaw<Array<{ tenantId: string; pointLimit: number | null; subscriptionStatus: string | null }>>`
    SELECT d."tenantId",
           CASE WHEN s."basePointLimit" = 0 THEN 0 ELSE s."basePointLimit" + s."addonPointLimit" END AS "pointLimit",
           s."status" AS "subscriptionStatus"
    FROM "Device" d
    LEFT JOIN "Tenant" t ON t."id" = d."tenantId"
    LEFT JOIN "TenantSubscription" s ON s."tenantId" = t."id"
    WHERE d."id" = ${deviceId}
    LIMIT 1
  `;
  let allowedNew = Number.POSITIVE_INFINITY;
  if (quota[0]?.subscriptionStatus === 'EXPIRED' || quota[0]?.subscriptionStatus === 'SUSPENDED') allowedNew = 0;
  else if (quota[0]?.pointLimit && quota[0].pointLimit > 0) {
    const used = await prisma.deviceMetric.count({ where: { device: { tenantId: quota[0].tenantId }, enabled: true, ignored: false } });
    allowedNew = Math.max(0, quota[0].pointLimit - used);
  }
  for (const [identifier, value] of entries) {
    if (!existingIdentifiers.has(identifier)) {
      if (allowedNew <= 0) continue;
      allowedNew -= 1;
    }
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
    rearmedAlarmKeys.add(alarmKey);
  }
  if (abnormal && !openAlarm) {
    if (resolvedAlarmSuppression.has(alarmKey)) return;
    if (!rearmedAlarmKeys.has(alarmKey)) {
      const latestAlarm = await prisma.alarm.findFirst({ where: { deviceId: device.id, type }, orderBy: { createdAt: 'desc' } });
      if (latestAlarm?.status === 'RESOLVED') {
        resolvedAlarmSuppression.add(alarmKey);
        return;
      }
    }
    const alarm = await prisma.alarm.create({
      data: { tenantId: device.tenantId, deviceId: device.id, type, level, message, value, threshold },
    });
    rearmedAlarmKeys.delete(alarmKey);
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
      where: { status: 'ONLINE', cloudAccessEnabled: true, lastSeenAt: { lt: threshold } },
      select: { id: true, tenantId: true, deviceKey: true, name: true },
    });
    const staleIds = staleDevices.filter((device) => !connectedDeviceIds.has(device.id)).map((device) => device.id);
    if (!staleIds.length) return;
    const result = await prisma.device.updateMany({
      where: { id: { in: staleIds }, cloudAccessEnabled: true },
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
  startupAbort.abort();
  client.end();
  await prisma.$disconnect();
  await timescale.end();
}

async function initTimescale() {
  await timescale.query('CREATE EXTENSION IF NOT EXISTS timescaledb');
  await timescale.query(`
    CREATE TABLE IF NOT EXISTS telemetry_events (
      time TIMESTAMPTZ NOT NULL,
      device_id TEXT NOT NULL,
      device_key TEXT NOT NULL,
      metrics JSONB NOT NULL
    )
  `);
  await timescale.query("SELECT create_hypertable('telemetry_events', 'time', if_not_exists => TRUE)");
  await timescale.query('ALTER TABLE telemetry_events ADD COLUMN IF NOT EXISTS device_id TEXT');
  const columnState = await timescale.query<{ is_nullable: string }>(`
    SELECT is_nullable
    FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'telemetry_events' AND column_name = 'device_id'
  `);
  if (columnState.rows[0]?.is_nullable === 'YES') {
    // Earlier releases used the local device key as the time-series identity.
    // Backfill while keys are still unambiguous, then isolate rows whose source
    // device was deleted instead of attaching them to a future same-key device.
    const devices = await prisma.device.findMany({ select: { id: true, deviceKey: true } });
    const idsByKey = new Map<string, string[]>();
    for (const device of devices) idsByKey.set(device.deviceKey, [...(idsByKey.get(device.deviceKey) ?? []), device.id]);
    for (const [deviceKey, ids] of idsByKey) {
      if (ids.length !== 1) continue;
      await timescale.query('UPDATE telemetry_events SET device_id = $1 WHERE device_id IS NULL AND device_key = $2', [ids[0], deviceKey]);
    }
    await timescale.query("UPDATE telemetry_events SET device_id = 'legacy:' || md5(device_key) WHERE device_id IS NULL");
    const deduplicated = await timescale.query(`
      DELETE FROM telemetry_events target
      USING (
        SELECT device_id, time, min(ctid) AS keep_ctid
        FROM telemetry_events
        GROUP BY device_id, time
        HAVING count(*) > 1
      ) duplicates
      WHERE target.device_id = duplicates.device_id
        AND target.time = duplicates.time
        AND target.ctid <> duplicates.keep_ctid
    `);
    if (deduplicated.rowCount) console.log(`[ingest] removed ${deduplicated.rowCount} duplicate telemetry row(s)`);
    await timescale.query('ALTER TABLE telemetry_events ALTER COLUMN device_id SET NOT NULL');
  }

  const identityIndex = await timescale.query<{ indexdef: string }>(`
    SELECT indexdef FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'telemetry_events'
      AND indexname = 'telemetry_events_device_time_uidx'
  `);
  if (identityIndex.rows[0] && !identityIndex.rows[0].indexdef.includes('(device_id,')) {
    await timescale.query('DROP INDEX telemetry_events_device_time_uidx');
  }
  await timescale.query('CREATE UNIQUE INDEX IF NOT EXISTS telemetry_events_device_time_uidx ON telemetry_events (device_id, time)');
}

timescaleReady = retryUntilReady(initTimescale, {
  retryMs: dependencyRetryMs,
  signal: startupAbort.signal,
  onFailure: (error, attempt) => { if (attempt === 1 || attempt % 12 === 0) console.error(`[ingest] timescale init attempt ${attempt} failed; retrying`, error); },
});
timescaleReady.then(() => console.log('[ingest] timescale ready')).catch((error) => {
  if (!isAbortError(error)) console.error('[ingest] timescale recovery stopped', error);
});

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

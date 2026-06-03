import { PrismaClient } from '@prisma/client';
import mqtt from 'mqtt';
import { Pool } from 'pg';

process.env.DATABASE_URL ??= 'postgresql://weikong:weikong123@localhost:5432/weikong_iot?schema=public';

const prisma = new PrismaClient();
const mqttUrl = process.env.MQTT_URL ?? 'mqtt://localhost:1883';
const heartbeatTopic = 'weikong/devices/+/heartbeat';
const telemetryTopic = 'weikong/devices/+/telemetry';
const connectedTopic = '$SYS/brokers/+/clients/+/connected';
const disconnectedTopic = '$SYS/brokers/+/clients/+/disconnected';
const timescale = new Pool({
  connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong:weikong123@localhost:5433/weikong_ts',
});
const offlineTimeoutSeconds = Number(process.env.DEVICE_OFFLINE_TIMEOUT_SECONDS ?? 90);
const scanIntervalSeconds = Number(process.env.DEVICE_OFFLINE_SCAN_INTERVAL_SECONDS ?? 30);
const temperatureHighThreshold = Number(process.env.TEMPERATURE_HIGH_THRESHOLD ?? 30);
const humidityHighThreshold = Number(process.env.HUMIDITY_HIGH_THRESHOLD ?? 80);
const batteryLowThreshold = Number(process.env.BATTERY_LOW_THRESHOLD ?? 20);
const connectedDeviceKeys = new Set<string>();
const disconnectGraceSeconds = Number(process.env.DEVICE_DISCONNECT_GRACE_SECONDS ?? 8);
const pendingOfflineTimers = new Map<string, ReturnType<typeof setTimeout>>();
const client = mqtt.connect(mqttUrl, {
  clientId: `weikong-ingest-${Date.now()}`,
  username: process.env.MQTT_USERNAME ?? 'platform-ingest',
});

interface AlarmRule {
  metric: string;
  type: string;
  level: string;
  threshold: number;
  isAbnormal: (value: number) => boolean;
  message: (deviceName: string, value: number, threshold: number) => string;
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
  const deviceKey = topic.split('/')[2];
  if (!deviceKey) return;
  try {
    const device = await prisma.device.findUnique({ where: { deviceKey } });
    if (!device) {
      console.warn(`[ingest] ignored unknown device ${deviceKey}`);
      return;
    }
    await updateDeviceStatus(device.id, device.deviceKey, 'ONLINE', device.status);
    if (topic.endsWith('/telemetry')) {
      const metrics = JSON.parse(payload.toString()) as Record<string, unknown>;
      if (!metrics || typeof metrics !== 'object' || Array.isArray(metrics)) throw new Error('telemetry payload must be a JSON object');
      const stored = await timescale.query<{ time: Date }>(
        'INSERT INTO telemetry_events (time, device_key, metrics) VALUES (NOW(), $1, $2) RETURNING time',
        [deviceKey, metrics],
      );
      await timescale.query("SELECT pg_notify('telemetry_updates', $1)", [
        JSON.stringify({ deviceKey, time: stored.rows[0].time, metrics }),
      ]);
      await discoverMetrics(device.id, metrics);
      await processAlarms(device, metrics);
      console.log(`[ingest] telemetry ${deviceKey}`);
    } else {
      console.log(`[ingest] heartbeat ${deviceKey}`);
    }
  } catch (error) {
    console.error(`[ingest] failed heartbeat ${deviceKey}`, error);
  }
});

async function processConnectionStatus(topic: string) {
  const match = topic.match(/^\$SYS\/brokers\/[^/]+\/clients\/([^/]+)\/(connected|disconnected)$/);
  if (!match) return;
  const [, deviceKey, event] = match;
  if (event === 'connected') {
    connectedDeviceKeys.add(deviceKey);
    cancelPendingOffline(deviceKey);
    const device = await prisma.device.findUnique({ where: { deviceKey }, select: { id: true, status: true } });
    if (!device) return;
    await updateDeviceStatus(device.id, deviceKey, 'ONLINE', device.status);
    console.log(`[ingest] client connected ${deviceKey}`);
    return;
  }

  connectedDeviceKeys.delete(deviceKey);
  schedulePendingOffline(deviceKey);
  console.log(`[ingest] client disconnected ${deviceKey}, waiting ${disconnectGraceSeconds}s before marking offline`);
}

async function updateDeviceStatus(deviceId: string, deviceKey: string, status: string, previousStatus?: string) {
  if (status === 'ONLINE') cancelPendingOffline(deviceKey);
  await prisma.device.update({
    where: { id: deviceId },
    data: {
      status,
      ...(status === 'ONLINE' ? { lastSeenAt: new Date() } : {}),
    },
  });
  if (previousStatus !== status) await notifyDeviceStatus({ id: deviceId, deviceKey, status });
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
  const device = await prisma.device.findUnique({ where: { deviceKey }, select: { id: true, status: true } });
  if (!device || device.status === 'OFFLINE') return;
  await prisma.device.update({ where: { id: device.id }, data: { status: 'OFFLINE' } });
  await notifyDeviceStatus({ id: device.id, deviceKey, status: 'OFFLINE' });
  console.log(`[ingest] client offline ${deviceKey}`);
}

async function notifyDeviceStatus(device: { id: string; deviceKey: string; status: string }) {
  await prisma.$executeRawUnsafe(
    'SELECT pg_notify($1, $2)',
    'device_status_updates',
    JSON.stringify(device),
  );
}

async function processAlarms(device: { id: string; tenantId: string; name: string }, metrics: Record<string, unknown>) {
  const configuredRules = await prisma.deviceAlarmRule.findMany({ where: { deviceId: device.id, enabled: true } });
  const configuredIdentifiers = new Set(configuredRules.map((rule) => rule.identifier));
  for (const rule of configuredRules) {
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
    const value = metrics[rule.metric];
    if (typeof value !== 'number' || !Number.isFinite(value)) continue;
    const abnormal = rule.isAbnormal(value);
    await syncAlarm(device, rule.type, rule.level, rule.message(device.name, value, rule.threshold), value, rule.threshold, abnormal, !abnormal);
  }
}

async function discoverMetrics(deviceId: string, metrics: Record<string, unknown>) {
  for (const [identifier, value] of Object.entries(metrics)) {
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
  const openAlarm = await prisma.alarm.findFirst({ where: { deviceId: device.id, type, status: 'OPEN' } });
  if (abnormal && !openAlarm) {
    await prisma.alarm.create({
      data: { tenantId: device.tenantId, deviceId: device.id, type, level, message, value, threshold },
    });
    console.log(`[ingest] alarm opened ${type} for ${device.name}`);
  } else if (recovered && openAlarm) {
    await prisma.alarm.update({ where: { id: openAlarm.id }, data: { status: 'RESOLVED', resolvedAt: new Date() } });
    console.log(`[ingest] alarm resolved ${type} for ${device.name}`);
  }
}

client.on('error', (error) => console.error('[ingest] mqtt error', error));

setInterval(async () => {
  const threshold = new Date(Date.now() - offlineTimeoutSeconds * 1000);
  try {
    const staleDevices = await prisma.device.findMany({
      where: { status: 'ONLINE', lastSeenAt: { lt: threshold } },
      select: { id: true, deviceKey: true },
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
        .map((device) => notifyDeviceStatus({ id: device.id, deviceKey: device.deviceKey, status: 'OFFLINE' })));
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
}

initTimescale().catch((error) => {
  console.error('[ingest] timescale init failed', error);
  process.exitCode = 1;
});

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

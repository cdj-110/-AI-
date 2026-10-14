require('reflect-metadata');
const assert = require('node:assert/strict');
const { randomUUID } = require('node:crypto');
const { validate } = require('class-validator');
const { CreateMaintenanceTaskDto } = require('../dist/gateway-maintenance/create-maintenance-task.dto');
const { GatewayMaintenanceService } = require('../dist/gateway-maintenance/gateway-maintenance.service');
const { MqttAuthController } = require('../dist/mqtt/mqtt-auth.controller');

async function main() {
  const gatewayId = randomUUID();
  const gateway = {
    id: gatewayId,
    sn: 'WK-REMOTE-TEST',
    hardwareId: 'HWREMOTE001',
    status: 'BOUND',
    licenseActivatedAt: new Date(),
    lastSeenAt: new Date(),
  };
  const rows = [];
  const events = [];
  const matches = (row, where) => Object.entries(where).every(([key, value]) => {
    if (key === 'status' && value && typeof value === 'object') return value.in.includes(row.status);
    if (key === 'expiresAt' && value?.gt) return row.expiresAt > value.gt;
    return row[key] === value;
  });
  const prisma = {
    factoryGateway: { findUnique: async ({ where }) => where.id === gatewayId || where.hardwareId === gateway.hardwareId ? gateway : null },
    gatewayMaintenanceTask: {
      count: async ({ where }) => rows.filter(row => matches(row, where)).length,
      create: async ({ data }) => {
        const row = { id: randomUUID(), status: 'PENDING', completedAt: null, ...data };
        rows.push(row);
        return row;
      },
      update: async ({ where, data }) => {
        const row = rows.find(item => item.id === where.id);
        Object.assign(row, data);
        return row;
      },
      updateMany: async ({ where, data }) => {
        let count = 0;
        for (const row of rows) if (matches(row, where)) { Object.assign(row, data); count++; }
        return { count };
      },
      findUniqueOrThrow: async ({ where }) => rows.find(row => row.id === where.id),
      findUnique: async ({ where }) => {
        const row = rows.find(item => item.id === where.id);
        return row ? { ...row, gateway } : null;
      },
      findMany: async () => rows,
    },
  };
  const logs = { record: async () => events.push('audit') };
  const service = new GatewayMaintenanceService(prisma, logs);
  const published = [];
  service.client = {
    connected: true,
    publish(topic, payload, options, callback) {
      events.push('publish');
      published.push({ topic, body: JSON.parse(payload), options });
      callback();
    },
  };
  service.ready = true;
  const actor = { sub: 'root-user', username: 'root', role: 'SUPER_ADMIN', tenantId: null };

  assert.deepEqual(await validate(Object.assign(new CreateMaintenanceTaskDto(), { action: 'SERVICE_RESTART' })), []);
  assert.ok((await validate(Object.assign(new CreateMaintenanceTaskDto(), { action: 'SHELL' }))).length);
  const task = await service.create(gatewayId, 'SERVICE_RESTART', actor, { ip: '127.0.0.1' });
  assert.equal(task.status, 'SENT');
  assert.deepEqual(events.slice(0, 2), ['audit', 'publish'], 'audit must be durable before a destructive command is sent');
  assert.equal(published[0].topic, `weikong/factory/${gateway.hardwareId}/maintenance/set`);
  assert.equal(published[0].options.retain, false);
  assert.equal(published[0].body.action, 'SERVICE_RESTART');
  assert.equal(new Date(published[0].body.expiresAt) - new Date(published[0].body.issuedAt), 60_000);
  await assert.rejects(() => service.create(gatewayId, 'GATEWAY_REBOOT', actor, {}), /进行中的/);

  await service.handleReply(`weikong/factory/OTHER/maintenance/reply`, Buffer.from(JSON.stringify({ ...published[0].body, status: 'ACCEPTED' })));
  assert.equal(task.status, 'SENT');
  await service.handleReply(`weikong/factory/${gateway.hardwareId}/maintenance/reply`, Buffer.from(JSON.stringify({ ...published[0].body, hardwareId: gateway.hardwareId, status: 'ACCEPTED', message: 'accepted' })));
  assert.equal(task.status, 'ACCEPTED');
  assert.equal(task.message, 'accepted');

  const acl = new MqttAuthController({
    factoryGateway: { findUnique: async ({ where }) => where.hardwareId === gateway.hardwareId ? gateway : null },
    device: { findUnique: async () => null },
  });
  for (const [username, action, topic, expected] of [
    [`factory:${gateway.hardwareId}`, 'subscribe', `weikong/factory/${gateway.hardwareId}/maintenance/set`, 'allow'],
    [`factory:${gateway.hardwareId}`, 'publish', `weikong/factory/${gateway.hardwareId}/maintenance/reply`, 'allow'],
    [`factory:${gateway.hardwareId}`, 'subscribe', 'weikong/factory/OTHER/maintenance/set', 'deny'],
    ['platform-ingest', 'publish', `weikong/factory/${gateway.hardwareId}/maintenance/set`, 'allow'],
    ['platform-ingest', 'subscribe', 'weikong/factory/+/maintenance/reply', 'allow'],
  ]) assert.equal((await acl.acl({ username, action, topic })).result, expected);

  console.log('Gateway maintenance checks passed: DTO validation, online gate, single active task, audit-before-send, non-retained MQTT, reply identity matching, task completion and topic ACL isolation. No real gateway, broker or database was changed.');
}

main().catch(error => { console.error(error); process.exitCode = 1; });

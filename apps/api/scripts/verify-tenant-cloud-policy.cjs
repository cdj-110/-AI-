require('reflect-metadata');
const assert = require('node:assert/strict');
const bcrypt = require('bcryptjs');
const { ValidationPipe } = require('@nestjs/common');
const { TenantsService } = require('../dist/tenants/tenants.service');
const { UpdateTenantDto } = require('../dist/tenants/dto/update-tenant.dto');
const { MqttAuthController } = require('../dist/mqtt/mqtt-auth.controller');
const { DeviceCommandsService } = require('../dist/device-commands/device-commands.service');
const { RemoteConfigService } = require('../dist/remote-config/remote-config.service');
const { GatewayMaintenanceService } = require('../dist/gateway-maintenance/gateway-maintenance.service');
const { OtaService } = require('../dist/ota/ota.service');
const { JwtStrategy } = require('../dist/auth/jwt.strategy');
const { AuthService } = require('../dist/auth/auth.service');

async function main() {
  const pipe = new ValidationPipe({ transform: true, whitelist: true });
  for (const value of ['false', 'true', 0, 1]) {
    await assert.rejects(pipe.transform({ deviceCloudAccessDisabled: value }, { type: 'body', metatype: UpdateTenantDto }));
  }
  for (const value of [false, true]) {
    const dto = await pipe.transform({ deviceCloudAccessDisabled: value }, { type: 'body', metatype: UpdateTenantDto });
    assert.equal(dto.deviceCloudAccessDisabled, value);
  }
  let tenant = { id: 'tenant-a', status: 'ACTIVE', deviceCloudAccessDisabled: false };
  const devices = [
    { id: 'normal', tenantId: 'tenant-a', cloudAccessEnabled: true, status: 'ONLINE', mqttClientId: 'normal', factoryGateway: { hardwareId: 'ABC' } },
    { id: 'banned', tenantId: 'tenant-a', cloudAccessEnabled: false, status: 'DISABLED', mqttClientId: 'banned' },
    { id: 'other', tenantId: 'tenant-b', cloudAccessEnabled: true, status: 'ONLINE', mqttClientId: 'other' },
  ];
  const disconnected = [], remoteClosed = [];
  const prisma = {
    $queryRaw: async () => [],
    tenant: {
      findUnique: async () => ({ ...tenant }),
      update: async ({ data }) => (tenant = { ...tenant, ...data }),
      create: async ({ data }) => data,
    },
    device: {
      updateMany: async ({ where, data }) => {
        assert.deepEqual(data, { status: 'OFFLINE' }); // Never changes individual permissions.
        for (const d of devices) if (d.tenantId === where.tenantId && d.status === where.status) Object.assign(d, data);
      },
      findMany: async ({ where }) => devices.filter(d => d.tenantId === where.tenantId),
    },
    $transaction: async fn => fn(prisma),
  };
  const service = new TenantsService(prisma, { disconnect: async id => { disconnected.push(id); return id === 'banned' ? 'unavailable' : 'disconnected'; } }, { disconnectDevice: id => remoteClosed.push(id) });
  assert.equal((await service.create({ name: 'new', status: 'ACTIVE', deviceCloudAccessDisabled: true })).deviceCloudAccessDisabled, false);
  await service.update('tenant-a', { status: 'DISABLED' });
  assert.equal(tenant.deviceCloudAccessDisabled, false);
  assert.equal(devices[0].status, 'ONLINE');
  assert.equal(disconnected.length, 0);
  const blocked = await service.update('tenant-a', { deviceCloudAccessDisabled: true });
  assert.equal(tenant.deviceCloudAccessDisabled, true);
  assert.equal(devices[0].status, 'OFFLINE');
  assert.equal(devices[1].status, 'DISABLED');
  assert.equal(devices[1].cloudAccessEnabled, false);
  assert.equal(devices[2].status, 'ONLINE');
  assert.deepEqual(disconnected.sort(), ['banned', 'factory_ABC', 'normal']);
  assert.deepEqual(remoteClosed.sort(), ['banned', 'normal']);
  assert(blocked.disconnectResults.some(r => r.result === 'unavailable'));
  await service.update('tenant-a', { name: 'rename' });
  assert.equal(tenant.deviceCloudAccessDisabled, true, 'unrelated edit preserves policy');
  await service.update('tenant-a', { deviceCloudAccessDisabled: false });
  assert.equal(tenant.status, 'DISABLED', 'can restore device connections without restoring account access');
  await service.update('tenant-a', { deviceCloudAccessDisabled: true });
  await service.update('tenant-a', { status: 'ACTIVE' });
  assert.equal(tenant.deviceCloudAccessDisabled, false);
  assert.equal(devices[1].cloudAccessEnabled, false, 'reenabling tenant never lifts individual bans');
  await service.update('tenant-a', { status: 'DISABLED' });
  assert.equal(tenant.deviceCloudAccessDisabled, false, 'a later suspension defaults to accounts only');

  const hash = await bcrypt.hash('test-only-password', 4);
  let blockedPolicy = false;
  const target = () => ({ id: 'device-a', deviceKey: 'a', deviceType: 'GATEWAY', tenantId: 'tenant-a',
    cloudAccessEnabled: true, mqttClientId: 'client-a', mqttPasswordHash: hash, tenant: { status: 'DISABLED', deviceCloudAccessDisabled: blockedPolicy } });
  const cloudDb = {
    device: { findUnique: async () => target(), findFirst: async () => target() },
    factoryGateway: { findUnique: async () => ({ status: 'BOUND', model: 'EG100', deviceSecretHash: hash, device: target() }) },
    otaPackage: { findUnique: async () => ({ model: 'EG100', fileAvailable: true }) },
  };
  const mqtt = new MqttAuthController(cloudDb);
  for (const disabled of [false, true]) {
    blockedPolicy = disabled;
    const result = disabled ? 'deny' : 'allow';
    assert.equal((await mqtt.auth({ username: 'a', clientid: 'client-a', password: 'test-only-password' })).result, result);
    assert.equal((await mqtt.auth({ username: 'factory:ABC', clientid: 'factory_ABC', password: 'test-only-password' })).result, result);
    assert.equal((await mqtt.acl({ username: 'a', clientid: 'client-a', action: 'publish', topic: 'weikong/devices/a/telemetry' })).result, result);
    assert.equal((await mqtt.acl({ username: 'factory:ABC', clientid: 'factory_ABC', action: 'publish', topic: 'weikong/factory/ABC/heartbeat' })).result, result);
    assert.equal((await mqtt.acl({ username: 'factory:ABC', clientid: 'factory_ABC', action: 'subscribe', topic: 'weikong/factory/ABC/ota/set' })).result, result);
  }
  const actor = { sub: 'admin', role: 'SUPER_ADMIN' };
  const caps = await new DeviceCommandsService(cloudDb, {}, {}).capabilities(actor, 'device-a');
  assert.equal(caps.available, false);
  assert.match(caps.reason, /租户/);
  await assert.rejects(new RemoteConfigService(cloudDb, {}).create(actor, 'device-a', { config: {} }), /租户/);
  const uuid = '00000000-0000-0000-0000-000000000001';
  await assert.rejects(new GatewayMaintenanceService(cloudDb, {}).create(uuid, 'SERVICE_RESTART', actor, {}), /租户/);
  await assert.rejects(new OtaService(cloudDb).createTask(uuid, uuid, actor), /租户/);

  const loginUser = { id: 'user-a', password: hash, status: 'ACTIVE', role: 'TENANT_ADMIN', tenantId: 'tenant-a', updatedAt: new Date() };
  const loginLogs = [];
  const authDb = {
    user: { findFirst: async () => loginUser, findUnique: async () => ({ ...loginUser, tenant: { status: 'DISABLED' } }) },
    tenant: { findUnique: async () => ({ status: 'DISABLED' }) },
    loginLog: { create: async data => loginLogs.push(data) },
  };
  const sessions = { assertLoginAllowed: async () => {}, requiresCaptcha: async () => false, clearLoginFailures: async () => {}, validate: async () => {} };
  await assert.rejects(new AuthService(authDb, {}, sessions, {}, {}).login({ username: 'a@example.test', password: 'test-only-password' }, { ip: '127.0.0.1' }, 'tenant'), /租户已被停用/);
  await assert.rejects(new JwtStrategy({ get: () => 'test-secret' }, authDb, sessions).validate({ sub: 'user-a', sid: 'session', uv: loginUser.updatedAt.getTime() }), /租户已被停用/);
  console.log('Tenant cloud policy passed: optional scope, persistence, restoration, tenant isolation, MQTT auth/ACL, remote controls and account access.');
}
main().catch(error => { console.error(error); process.exitCode = 1; });

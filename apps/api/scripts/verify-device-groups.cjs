/* Uses a disposable local PostgreSQL container only; never reads the project's DB URL. */
require('reflect-metadata');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const { randomUUID, randomBytes } = require('node:crypto');
const { resolve } = require('node:path');
const { ValidationPipe } = require('@nestjs/common');
const { Reflector } = require('@nestjs/core');
const { PrismaClient } = require('@prisma/client');
const { DeviceGroupsService } = require('../dist/devices/device-groups.service');
const { DeviceGroupsController } = require('../dist/devices/device-groups.controller');
const { DevicesService } = require('../dist/devices/devices.service');
const { RolesGuard } = require('../dist/common/guards/roles.guard');
const { PermissionsGuard } = require('../dist/common/guards/permissions.guard');
const { SaveDeviceGroupDto, AssignDeviceGroupDto } = require('../dist/devices/dto/device-group.dto');

const name = `wk-device-groups-test-${randomUUID()}`;
let created = false, db;
function docker(...args) {
  const r = spawnSync('docker', args, { encoding: 'utf8' });
  if (r.status !== 0) throw new Error(r.stderr || 'Docker command failed');
  return r.stdout.trim();
}
async function main() {
  const pipe = new ValidationPipe({ transform: true, whitelist: true });
  const save = value => pipe.transform(value, { type: 'body', metatype: SaveDeviceGroupDto });
  const assign = value => pipe.transform(value, { type: 'body', metatype: AssignDeviceGroupDto });
  assert.equal((await save({ name: ' 一号车间 ', tenantId: 'foreign' })).name, '一号车间');
  assert.equal((await save({ name: '一号车间', tenantId: 'foreign' })).tenantId, undefined);
  for (const value of ['', ' ', 'x'.repeat(51), null]) await assert.rejects(() => save({ name: value }));
  const id = randomUUID();
  for (const value of [
    { deviceIds: [], groupIds: [], mode: 'REPLACE' },
    { deviceIds: [id], groupIds: [] },
    { deviceIds: [id, id], groupIds: [], mode: 'REPLACE' },
    { deviceIds: ['bad'], groupIds: [], mode: 'REPLACE' },
    { deviceIds: [id], groupIds: ['bad'], mode: 'ADD' },
    { deviceIds: [id], groupIds: [id, id], mode: 'ADD' },
    { deviceIds: [id], groupIds: [], mode: 'MOVE' },
    { deviceIds: Array.from({ length: 101 }, randomUUID), groupIds: [], mode: 'REPLACE' },
  ]) await assert.rejects(() => assign(value));
  assert.equal((await assign({ deviceIds: [id], groupIds: [], mode: 'REPLACE' })).mode, 'REPLACE');
  const guard = new RolesGuard(new Reflector());
  for (const role of ['TENANT_ADMIN', 'TENANT_USER', 'SUPER_ADMIN', undefined]) {
    for (const method of ['list', 'create', 'rename', 'remove', 'assign']) {
      assert.equal(guard.canActivate({ getHandler: () => DeviceGroupsController.prototype[method], getClass: () => DeviceGroupsController, switchToHttp: () => ({ getRequest: () => ({ user: { role } }) }) }), ['TENANT_ADMIN', 'TENANT_USER'].includes(role));
    }
  }
  const permissionGuard = new PermissionsGuard(new Reflector());
  const permissionContext = (method, permissions) => ({ getHandler: () => DeviceGroupsController.prototype[method], getClass: () => DeviceGroupsController, switchToHttp: () => ({ getRequest: () => ({ user: { role: 'TENANT_USER', permissions } }) }) });
  assert.equal(permissionGuard.canActivate(permissionContext('list', ['DEVICE_VIEW'])), true);
  assert.equal(permissionGuard.canActivate(permissionContext('create', ['DEVICE_GROUP_MANAGE'])), true);
  assert.throws(() => permissionGuard.canActivate(permissionContext('create', ['DEVICE_VIEW'])), /没有访问/);
  const password = randomBytes(18).toString('hex');
  docker('run', '--detach', '--rm', '--name', name, '-e', `POSTGRES_PASSWORD=${password}`, '-e', 'POSTGRES_DB=groups_test', '-p', '127.0.0.1::5432', 'postgres:16');
  created = true;
  for (let i = 0; i < 30; i++) {
    const r = spawnSync('docker', ['exec', name, 'pg_isready', '-U', 'postgres'], { encoding: 'utf8' });
    if (r.status === 0) break;
    await new Promise(r => setTimeout(r, 500));
  }
  const port = docker('port', name, '5432/tcp').split(':').at(-1);
  assert.match(port, /^\d+$/);
  const url = `postgresql://postgres:${password}@127.0.0.1:${port}/groups_test?schema=public`;
  const migration = spawnSync(process.execPath, ['node_modules/prisma/build/index.js', 'migrate', 'deploy'], { cwd: resolve(__dirname, '..'), env: { ...process.env, DATABASE_URL: url }, encoding: 'utf8' });
  assert.equal(migration.status, 0, migration.stderr);
  db = new PrismaClient({ datasources: { db: { url } } });
  const a = await db.tenant.create({ data: { name: 'Tenant A' } }), b = await db.tenant.create({ data: { name: 'Tenant B' } });
  const actor = { sub: randomUUID(), role: 'TENANT_ADMIN', tenantId: a.id };
  const other = { ...actor, tenantId: b.id };
  const service = new DeviceGroupsService(db);
  const makeDevice = (tenantId, key, more = {}) => db.device.create({ data: { tenantId, deviceKey: key, name: key, mqttClientId: key, mqttUsername: key, mqttPasswordHash: 'test-only', ...more } });
  const gateway = await makeDevice(a.id, 'gateway', { deviceType: 'GATEWAY', status: 'ONLINE' });
  const child = await makeDevice(a.id, 'child', { deviceType: 'GATEWAY_CHILD', gatewayId: gateway.id, cloudAccessEnabled: false, status: 'DISABLED' });
  const foreign = await makeDevice(b.id, 'foreign');
  const group = await service.create(actor, { name: '一号车间' });
  const foreignGroup = await service.create(other, { name: '一号车间' });
  await assert.rejects(() => service.create(actor, { name: '一号车间' }), /同名分组/);
  await service.rename(actor, group.id, { name: '生产区' });
  const second = await service.create(actor, { name: '二号车间' });
  await assert.rejects(() => service.rename(actor, second.id, { name: '生产区' }), /同名分组/);
  for (const invalid of [{ ...actor, tenantId: null }, { ...actor, role: 'SUPER_ADMIN' }, { ...actor, role: 'TENANT_USER' }]) {
    await assert.rejects(async () => service.create(invalid, { name: '无权限' }), /无权/);
    await assert.rejects(async () => service.assign(invalid, { deviceIds: [gateway.id], groupIds: [group.id], mode: 'ADD' }), /无权/);
  }
  const delegated = { ...actor, sub: randomUUID(), role: 'TENANT_USER', permissions: ['DEVICE_VIEW', 'DEVICE_GROUP_MANAGE'], deviceScopeMode: 'ALL' };
  const delegatedGroup = await service.create(delegated, { name: '成员管理分组' });
  await service.remove(delegated, delegatedGroup.id);
  const customUser = await db.user.create({ data: { tenantId: a.id, username: 'scoped-group-manager', password: 'test-only', role: 'TENANT_USER', deviceScopeMode: 'CUSTOM', permissions: ['DEVICE_VIEW', 'DEVICE_GROUP_MANAGE'] } });
  await db.userDeviceAccess.create({ data: { userId: customUser.id, deviceId: gateway.id } });
  const scopedActor = { sub: customUser.id, role: 'TENANT_USER', tenantId: a.id, permissions: customUser.permissions, deviceScopeMode: 'CUSTOM' };
  const scopedGroup = await service.create(scopedActor, { name: '受限成员分组' });
  assert.equal(await db.userDeviceGroupAccess.count({ where: { userId: customUser.id, groupId: scopedGroup.id } }), 1);
  assert.equal((await service.assign(scopedActor, { deviceIds: [gateway.id], groupIds: [scopedGroup.id], mode: 'ADD' })).count, 1);
  await assert.rejects(() => service.assign(scopedActor, { deviceIds: [child.id], groupIds: [scopedGroup.id], mode: 'ADD' }), /未调整任何设备/);
  await assert.rejects(() => service.rename(scopedActor, group.id, { name: '越权修改' }), /无权访问/);
  await service.rename(scopedActor, scopedGroup.id, { name: '受限成员分组-已改名' });
  await service.remove(scopedActor, scopedGroup.id);
  for (const action of [() => service.rename(actor, foreignGroup.id, { name: 'bad' }), () => service.remove(actor, foreignGroup.id), () => service.assign(actor, { deviceIds: [gateway.id], groupIds: [foreignGroup.id], mode: 'ADD' })]) await assert.rejects(action, /无权访问|不存在/);
  await assert.rejects(() => service.assign(actor, { deviceIds: [gateway.id, foreign.id], groupIds: [group.id], mode: 'ADD' }), /未调整任何设备/);
  assert.equal(await db.deviceGroupMembership.count({ where: { deviceId: gateway.id } }), 0);
  await assert.rejects(() => db.deviceGroupMembership.create({ data: { deviceId: gateway.id, tenantId: a.id, groupId: foreignGroup.id } }), e => e.code === 'P2003');
  const assigned = await service.assign(actor, { deviceIds: [gateway.id, child.id], groupIds: [group.id], mode: 'ADD' });
  assert.equal(assigned.count, 2);
  await service.assign(actor, { deviceIds: [gateway.id], groupIds: [second.id], mode: 'ADD' });
  assert.equal(await db.deviceGroupMembership.count({ where: { deviceId: gateway.id } }), 2);
  const snapshot = await db.device.findUnique({ where: { id: child.id } });
  assert.equal(snapshot.gatewayId, gateway.id);
  assert.equal(snapshot.cloudAccessEnabled, false);
  assert.equal(snapshot.status, 'DISABLED');
  const list = await service.list({ ...actor, role: 'TENANT_USER' });
  assert.equal(list.totalDevices, 2); assert.equal(list.ungroupedDevices, 0);
  assert.equal(list.items.find(g => g.id === group.id)._count.devices, 2);
  assert.ok(list.items.every(g => g.tenantId === a.id));
  // Run the actual device list query without starting telemetry listener connections.
  const devices = Object.create(DevicesService.prototype); devices.prisma = db;
  const result = await devices.findAll(actor, 1, 10, '', '', '', '', '', group.id);
  assert.equal(result.total, 2); assert.ok(result.items.every(item => item.groups.some(groupItem => groupItem.name === '生产区')));
  assert.equal(result.stats.total, 2);
  assert.equal((await devices.findAll(actor, 1, 10, '', '', '', '', '', foreignGroup.id)).total, 0);
  await assert.rejects(() => devices.findAll({ ...actor, tenantId: null }, 1, 10, ''), /未关联租户/);
  await service.assign(actor, { deviceIds: [child.id], groupIds: [group.id], mode: 'REMOVE' });
  assert.equal((await devices.findAll(actor, 1, 10, 'child', 'GATEWAY_CHILD', '', '', '', 'ungrouped')).total, 1);
  assert.equal((await devices.findAll(actor, 1, 10, 'child', 'GATEWAY_CHILD', '', '', '', group.id)).total, 0);
  const removed = await service.remove(actor, group.id);
  assert.equal(removed.removedMemberships, 1);
  assert.equal(await db.device.count({ where: { tenantId: a.id } }), 2);
  assert.equal(await db.deviceGroupMembership.count({ where: { deviceId: gateway.id } }), 1);
  assert.deepEqual(await db.device.findUnique({ where: { id: child.id } }), snapshot);
  // Tenant cascade must still work with the tenant-scoped group FK.
  await service.assign(other, { deviceIds: [foreign.id], groupIds: [foreignGroup.id], mode: 'ADD' });
  await db.tenant.delete({ where: { id: b.id } });
  assert.equal(await db.deviceGroup.count({ where: { tenantId: b.id } }), 0);
  assert.equal(await db.device.count({ where: { tenantId: b.id } }), 0);
  console.log('PASS migrations, DTOs, role and button permissions, delegated and scoped group management, tenant isolation, composite FK, unique names, multi-group membership, conflict-free add/remove/replace, filters/counts, safe removal and tenant cascade.');
}
main().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => {
  await db?.$disconnect();
  if (created) docker('rm', '--force', name);
});

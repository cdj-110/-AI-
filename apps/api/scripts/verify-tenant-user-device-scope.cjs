/* Uses a disposable local PostgreSQL container only; never reads the project's DB URL. */
require('reflect-metadata');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const { randomBytes, randomUUID } = require('node:crypto');
const { resolve } = require('node:path');
const { PrismaClient } = require('@prisma/client');
const { Reflector } = require('@nestjs/core');
const { UsersService } = require('../dist/users/users.service');
const { DevicesService } = require('../dist/devices/devices.service');
const { DeviceGroupsService } = require('../dist/devices/device-groups.service');
const { DevicesController } = require('../dist/devices/devices.controller');
const { AlarmsController } = require('../dist/alarms/alarms.controller');
const { PermissionsGuard } = require('../dist/common/guards/permissions.guard');

const containerName = `wk-user-scope-test-${randomUUID()}`;
let containerCreated = false;
let db;

function docker(...args) {
  const result = spawnSync('docker', args, { encoding: 'utf8' });
  if (result.status !== 0) throw new Error(result.stderr || 'Docker command failed');
  return result.stdout.trim();
}

async function main() {
  const permissionGuard = new PermissionsGuard(new Reflector());
  const permissionContext = (controller, method, permissions) => ({
    getHandler: () => controller.prototype[method],
    getClass: () => controller,
    switchToHttp: () => ({ getRequest: () => ({ user: { role: 'TENANT_USER', permissions } }) }),
  });
  assert.equal(permissionGuard.canActivate(permissionContext(DevicesController, 'findAll', ['MAP_VIEW'])), true);
  assert.throws(() => permissionGuard.canActivate(permissionContext(DevicesController, 'findAll', ['ALARM_VIEW'])), /没有访问该功能/);
  assert.equal(permissionGuard.canActivate(permissionContext(AlarmsController, 'findAll', ['ALARM_VIEW'])), true);
  assert.throws(() => permissionGuard.canActivate(permissionContext(AlarmsController, 'findAll', ['DEVICE_VIEW'])), /没有访问该功能/);

  const password = randomBytes(18).toString('hex');
  docker('run', '--detach', '--rm', '--name', containerName, '-e', `POSTGRES_PASSWORD=${password}`, '-e', 'POSTGRES_DB=user_scope_test', '-p', '127.0.0.1::5432', 'postgres:16');
  containerCreated = true;
  for (let attempt = 0; attempt < 30; attempt++) {
    const ready = spawnSync('docker', ['exec', containerName, 'pg_isready', '-U', 'postgres'], { encoding: 'utf8' });
    if (ready.status === 0) break;
    await new Promise(resolveWait => setTimeout(resolveWait, 500));
  }
  const port = docker('port', containerName, '5432/tcp').split(':').at(-1);
  assert.match(port, /^\d+$/);
  const databaseUrl = `postgresql://postgres:${password}@127.0.0.1:${port}/user_scope_test?schema=public`;
  const migration = spawnSync(process.execPath, ['node_modules/prisma/build/index.js', 'migrate', 'deploy'], {
    cwd: resolve(__dirname, '..'),
    env: { ...process.env, DATABASE_URL: databaseUrl },
    encoding: 'utf8',
  });
  assert.equal(migration.status, 0, migration.stderr);
  db = new PrismaClient({ datasources: { db: { url: databaseUrl } } });

  const tenantA = await db.tenant.create({ data: { name: 'Tenant A' } });
  const tenantB = await db.tenant.create({ data: { name: 'Tenant B' } });
  const admin = await db.user.create({
    data: { tenantId: tenantA.id, username: 'admin-a', email: 'admin-a@example.test', password: 'test-only', role: 'TENANT_ADMIN' },
  });
  const actorAdmin = { sub: admin.id, role: 'TENANT_ADMIN', tenantId: tenantA.id };
  const groupService = new DeviceGroupsService(db);
  const lineOne = await groupService.create(actorAdmin, { name: '一号产线' });
  const lineTwo = await groupService.create(actorAdmin, { name: '二号产线' });
  const foreignGroup = await db.deviceGroup.create({ data: { tenantId: tenantB.id, name: '外部产线' } });
  const createDevice = (tenantId, deviceKey) => db.device.create({
    data: { tenantId, deviceKey, name: deviceKey, mqttClientId: deviceKey, mqttUsername: deviceKey, mqttPasswordHash: 'test-only' },
  });
  const groupedDevice = await createDevice(tenantA.id, 'grouped-device');
  const directDevice = await createDevice(tenantA.id, 'direct-device');
  const hiddenDevice = await createDevice(tenantA.id, 'hidden-device');
  const foreignDevice = await createDevice(tenantB.id, 'foreign-device');
  await groupService.assign(actorAdmin, { deviceIds: [groupedDevice.id], groupIds: [lineOne.id], mode: 'ADD' });
  await groupService.assign(actorAdmin, { deviceIds: [hiddenDevice.id], groupIds: [lineTwo.id], mode: 'ADD' });
  await db.deviceGroupMembership.create({ data: { tenantId: tenantB.id, deviceId: foreignDevice.id, groupId: foreignGroup.id } });

  const users = new UsersService(db);
  const child = await users.create(actorAdmin, {
    username: 'operator-a',
    password: 'password-123',
    email: 'operator-a@example.test',
    role: 'TENANT_USER',
    permissions: ['DEVICE_VIEW', 'ALARM_VIEW'],
    deviceScopeMode: 'CUSTOM',
    groupIds: [lineOne.id],
    deviceIds: [directDevice.id],
  });
  assert.equal(child.password, undefined);
  assert.deepEqual(child.permissions, ['DEVICE_VIEW', 'ALARM_VIEW']);
  assert.deepEqual(child.groupIds, [lineOne.id]);
  assert.deepEqual(child.deviceIds, [directDevice.id]);

  const actorChild = {
    sub: child.id,
    role: 'TENANT_USER',
    tenantId: tenantA.id,
    permissions: child.permissions,
    deviceScopeMode: 'CUSTOM',
  };
  const devices = Object.create(DevicesService.prototype);
  devices.prisma = db;
  let visible = await devices.findAll(actorChild, 1, 100, '');
  assert.deepEqual(new Set(visible.items.map(item => item.id)), new Set([groupedDevice.id, directDevice.id]));
  assert.equal(visible.total, 2);
  await assert.rejects(() => devices.findOne(actorChild, hiddenDevice.id), /无权访问/);
  await assert.rejects(() => devices.findOne(actorChild, foreignDevice.id), /无权访问/);

  // Whole-group authorization is dynamic: later devices inherit access automatically.
  const laterDevice = await createDevice(tenantA.id, 'later-device');
  await groupService.assign(actorAdmin, { deviceIds: [laterDevice.id], groupIds: [lineOne.id], mode: 'ADD' });
  visible = await devices.findAll(actorChild, 1, 100, '');
  assert.deepEqual(new Set(visible.items.map(item => item.id)), new Set([groupedDevice.id, directDevice.id, laterDevice.id]));
  // Moving a device out of an authorized group removes inherited access.
  await groupService.assign(actorAdmin, { deviceIds: [groupedDevice.id], groupIds: [lineTwo.id], mode: 'REPLACE' });
  visible = await devices.findAll(actorChild, 1, 100, '');
  assert.deepEqual(new Set(visible.items.map(item => item.id)), new Set([directDevice.id, laterDevice.id]));

  const visibleGroups = await groupService.list(actorChild);
  assert.equal(visibleGroups.totalDevices, 2);
  assert.equal(visibleGroups.items.length, 1);
  assert.equal(visibleGroups.items[0].id, lineOne.id);
  assert.equal(visibleGroups.items[0]._count.devices, 1);

  const options = await users.deviceScopeOptions(actorAdmin);
  assert.ok(options.groups.every(group => group.tenantId === undefined));
  assert.equal(options.groups.length, 2);
  assert.equal(options.devices.length, 4);
  assert.ok(options.devices.every(device => device.id !== foreignDevice.id));
  assert.deepEqual(options.devices.find(device => device.id === hiddenDevice.id).groupIds, [lineTwo.id]);

  await assert.rejects(() => users.create(actorAdmin, {
    username: 'cross-tenant', password: 'password-123', email: 'cross@example.test', role: 'TENANT_USER',
    deviceScopeMode: 'CUSTOM', deviceIds: [foreignDevice.id], groupIds: [],
  }), /不属于当前租户/);
  await assert.rejects(() => users.update(actorAdmin, child.id, { deviceScopeMode: 'CUSTOM', deviceIds: [], groupIds: [] }), /至少选择/);

  await users.update(actorAdmin, child.id, { deviceScopeMode: 'ALL', permissions: ['DASHBOARD_VIEW'] });
  const allScope = { ...actorChild, deviceScopeMode: 'ALL', permissions: ['DASHBOARD_VIEW'] };
  assert.equal((await devices.findAll(allScope, 1, 100, '')).total, 4);

  const defaultChild = await users.create(actorAdmin, {
    username: 'default-operator', password: 'password-123', email: 'default@example.test', role: 'TENANT_USER',
  });
  assert.equal(defaultChild.deviceScopeMode, 'ALL');
  assert.equal(defaultChild.permissions.length, 7);
  console.log('PASS permissions, direct-device scope, dynamic group scope, tenant isolation, scope updates, safe defaults and public responses.');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
}).finally(async () => {
  await db?.$disconnect();
  if (containerCreated) docker('rm', '--force', containerName);
});

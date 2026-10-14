const assert = require('node:assert/strict');
const { GatewayTopologyService } = require('../dist/gateway-topology/gateway-topology.service');
const { DevicesService } = require('../dist/devices/devices.service');

async function main() {
  const gateway = {
    id: 'gateway-id', tenantId: 'tenant-id', deviceKey: 'gateway-1',
    deviceType: 'GATEWAY', gatewayId: null, children: [],
  };
  const restriction = {
    id: 'restriction-id', gatewayDeviceId: gateway.id, childDeviceKey: 'child-1',
    disabledAt: new Date('2026-09-22T00:00:00.000Z'), disabledBy: 'admin-id', reason: '管理平台禁用',
  };
  let savedDraft;
  let createdDevices = 0;
  const prisma = {
    device: {
      findFirst: async () => gateway,
      findMany: async () => [],
      findUnique: async () => gateway,
    },
    gatewayChildCloudRestriction: {
      findMany: async () => [restriction],
    },
    gatewayTopologyDraft: {
      upsert: async (args) => {
        savedDraft = args.create;
        return args.create;
      },
      findUnique: async () => ({
        id: 'draft-id', gatewayDeviceId: gateway.id,
        items: [{ deviceKey: 'child-1', name: '子设备 1', status: 'NEW' }],
      }),
    },
    $transaction: async (callback) => callback({
      gatewayChildCloudRestriction: { findMany: async () => [restriction] },
      device: {
        findFirst: async () => null,
        create: async () => { createdDevices += 1; },
        update: async () => { throw new Error('a deleted restricted child must not be recreated'); },
      },
      gatewayTopologyDraft: { update: async () => undefined },
    }),
  };

  const service = new GatewayTopologyService(prisma);
  await service.report({
    gatewayKey: gateway.deviceKey,
    devices: [{ deviceKey: 'child-1', name: '子设备 1', protocol: 'MODBUS' }],
  });
  assert.equal(savedDraft.items[0].status, 'PLATFORM_DISABLED');
  assert.equal(savedDraft.status, 'CONFIRMED');
  assert.match(savedDraft.items[0].conflictReason, /管理平台禁用/);

  const result = await service.confirm(
    { sub: 'tenant-admin-id', username: 'tenant-admin', role: 'TENANT_ADMIN', tenantId: gateway.tenantId },
    gateway.id,
  );
  assert.equal(result.created, 0);
  assert.equal(result.blocked, 1);
  assert.equal(createdDevices, 0);

  const disabledChild = {
    id: 'child-id', tenantId: gateway.tenantId, deviceKey: 'child-1', name: '子设备 1',
    deviceType: 'GATEWAY_CHILD', gatewayId: gateway.id, cloudAccessEnabled: false,
    cloudDisabledAt: restriction.disabledAt, cloudDisabledBy: restriction.disabledBy,
    cloudDisabledReason: restriction.reason, children: [], gateway: null, factoryGateway: null,
  };
  let restrictionUpserts = 0;
  let deletedDevices = 0;
  const deviceServiceContext = Object.assign(Object.create(DevicesService.prototype), {
    prisma: {
      device: { findUnique: async () => disabledChild },
      $transaction: async (callback) => callback({
        gatewayChildCloudRestriction: { upsert: async () => { restrictionUpserts += 1; } },
        factoryGateway: { findUnique: async () => null },
        device: { delete: async () => { deletedDevices += 1; } },
      }),
    },
  });
  await assert.rejects(
    () => DevicesService.prototype.remove.call(
      deviceServiceContext,
      { sub: 'tenant-admin-id', username: 'tenant-admin', role: 'TENANT_ADMIN', tenantId: gateway.tenantId },
      disabledChild.id,
    ),
    /已被管理平台禁用/,
  );
  assert.equal(deletedDevices, 0, 'tenant must not delete a platform-disabled device');

  await DevicesService.prototype.remove.call(
    deviceServiceContext,
    { sub: 'super-admin-id', username: 'admin', role: 'SUPER_ADMIN', tenantId: null },
    disabledChild.id,
  );
  assert.equal(restrictionUpserts, 1, 'super-admin deletion must preserve the child restriction');
  assert.equal(deletedDevices, 1);
  console.log('gateway child cloud restriction verification passed');
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

require('reflect-metadata');
const assert = require('node:assert/strict');
const { FactoryGatewaysService } = require('../dist/factory-gateways/factory-gateways.service');
const { FactoryGatewaysController } = require('../dist/factory-gateways/factory-gateways.controller');

async function check(count, failDelete = false) {
  const original = {
    factory: { id: 'factory', sn: 'WK-TEST', hardwareId: 'A'.repeat(32), status: 'BOUND', deviceId: 'gateway', boundAt: new Date(), device: { id: 'gateway', name: 'gateway', tenantId: 'old-tenant' } },
    devices: [{ id: 'gateway' }, ...Array.from({ length: count }, (_, i) => ({ id: `child-${i}`, gatewayId: 'gateway', tenantId: 'old-tenant', status: 'ONLINE', deviceType: 'GATEWAY_CHILD', deviceKey: `key-${i}`, data: ['keep'] })), { id: 'other-child', gatewayId: 'other-gateway', tenantId: 'other-tenant', status: 'ONLINE', deviceType: 'GATEWAY_CHILD', deviceKey: 'key-0' }, { id: 'previously-detached', gatewayId: null, tenantId: 'old-tenant', status: 'OFFLINE', deviceType: 'GATEWAY_CHILD', deviceKey: 'key-0' }],
  };
  let state = structuredClone(original);
  const prisma = { $transaction: async callback => {
    const draft = structuredClone(state);
    const tx = {
      factoryGateway: {
        findUnique: async ({ where }) => { assert.equal(where.id, 'factory'); return draft.factory; },
        update: async ({ where, data }) => { assert.equal(where.id, 'factory'); Object.assign(draft.factory, data); },
      },
      device: {
        updateMany: async ({ where, data }) => {
          assert.deepEqual(where, { gatewayId: 'gateway' });
          assert.deepEqual(data, { gatewayId: null, status: 'OFFLINE' });
          let changed = 0;
          for (const device of draft.devices) if (device.gatewayId === where.gatewayId) { Object.assign(device, data); changed++; }
          return { count: changed };
        },
        delete: async ({ where }) => {
          assert.equal(where.id, 'gateway');
          assert.ok(draft.devices.every(device => device.gatewayId !== 'gateway'));
          if (failDelete) throw new Error('simulated failure');
          draft.devices = draft.devices.filter(device => device.id !== where.id);
        },
      },
    };
    const result = await callback(tx);
    state = draft;
    return result;
  } };
  const service = new FactoryGatewaysService(prisma, {});
  if (failDelete) {
    await assert.rejects(() => service.reclaim('factory'), /simulated failure/);
    assert.deepEqual(state, original, 'failed reclamation partially detached children');
    return;
  }
  const result = await service.reclaim('factory');
  assert.equal(result.releasedChildrenCount, count);
  assert.equal(state.factory.status, 'UNBOUND');
  assert.equal(state.factory.deviceId, null);
  assert.equal(state.factory.boundAt, null);
  assert.equal(state.devices.some(device => device.id === 'gateway'), false);
  for (const child of state.devices.filter(device => device.id.startsWith('child-'))) {
    assert.equal(child.gatewayId, null);
    assert.equal(child.status, 'OFFLINE');
    assert.equal(child.tenantId, 'old-tenant');
    assert.equal(child.deviceType, 'GATEWAY_CHILD');
    assert.deepEqual(child.data, ['keep']);
  }
  assert.deepEqual(state.devices.find(device => device.id === 'other-child'), original.devices.at(-2));
  assert.deepEqual(state.devices.find(device => device.id === 'previously-detached'), original.devices.at(-1));
}

(async () => {
  assert.deepEqual(Reflect.getMetadata('roles', FactoryGatewaysController.prototype.reclaim), ['SUPER_ADMIN']);
  await check(0);
  await check(12);
  await check(12, true);
  for (const factory of [null, { status: 'REMOVED' }, { status: 'UNBOUND', deviceId: null }]) {
    const service = new FactoryGatewaysService({ $transaction: callback => callback({ factoryGateway: { findUnique: async () => factory } }) }, {});
    await assert.rejects(() => service.reclaim('missing'));
  }
  console.log('Gateway reclamation checks passed: super-admin only, children preserved and detached/offline, tenant isolation, transaction rollback, missing/unbound rejection.');
})().catch(error => { console.error(error); process.exitCode = 1; });

const assert = require('node:assert/strict');
const { reportedPointAccess, syncGatewayPointAccess } = require('../dist/gateway-point-access');
async function main() {
  assert.deepEqual(reportedPointAccess({ canRead: true, canWrite: false }), { canRead: true, canWrite: false });
  assert.deepEqual(reportedPointAccess({ canRead: 'true', canWrite: 'false' }), {});
  assert.deepEqual(reportedPointAccess({}), {}, 'old reports must not change model permissions');
  const calls = [], prisma = { deviceMetric: { updateMany: async args => calls.push(args) } };
  await syncGatewayPointAccess(prisma, 'child-a', 'gateway-a', 'tenant-a', [
    { identifier: 'output', canWrite: true }, { identifier: 'input', canWrite: false }, { identifier: 'old-report' },
  ]);
  assert.equal(calls.length, 2);
  for (const args of calls) {
    assert.deepEqual(args.where.device, { gatewayId: 'gateway-a', tenantId: 'tenant-a' });
    assert.equal(args.where.deviceId, 'child-a');
    assert.deepEqual(Object.keys(args.data), ['accessMode'], 'custom names/units/precision/visibility must remain unchanged');
    assert.equal(args.where.accessMode.not, args.data.accessMode, 'already synchronized rows should not be rewritten');
  }
  assert.deepEqual(calls[0].where.identifier.in, ['output']);
  assert.deepEqual(calls[1].where.identifier.in, ['input']);
  calls.length = 0;
  await syncGatewayPointAccess(prisma, 'child-a', 'gateway-a', 'tenant-a', [{ identifier: 'legacy' }]);
  assert.equal(calls.length, 0);
  await syncGatewayPointAccess(prisma, 'child-a', 'gateway-a', 'tenant-a', Array.from({ length: 5000 }, (_, i) => ({ identifier: 'p' + i, canWrite: i % 2 === 0 })));
  assert.equal(calls.length, 2, 'large reports must use bounded batches');
  console.log('Gateway access ingestion passed: explicit booleans, read-only/read-write sync, gateway and tenant scope, legacy preservation, display metadata preservation and bounded bulk updates. No live connections or database writes.');
}
main().catch(error => { console.error(error); process.exitCode = 1; });

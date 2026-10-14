require('reflect-metadata');
const assert = require('node:assert/strict');
const { PrismaClient } = require('@prisma/client');
const { FactoryGatewaysService } = require('../dist/factory-gateways/factory-gateways.service');
const { normalizeGatewayModel, productionBatchPrefix, highestBatchSequence } = require('../dist/factory-gateways/production-batch');

async function main() {
  assert.equal(normalizeGatewayModel(), 'EG100');
  assert.equal(normalizeGatewayModel(' eg200 '), 'EG200');
  assert.throws(() => normalizeGatewayModel('bad/model'));
  assert.equal(productionBatchPrefix('EG100', new Date('2026-09-17T15:59:59Z')), '20260917-EG100-');
  assert.equal(productionBatchPrefix('EG100', new Date('2026-09-17T16:00:00Z')), '20260918-EG100-');
  assert.equal(highestBatchSequence('20260917-EG100-', [
    { batchNo: '20260917-EG100-008' }, { batchNo: '20260917-EG100-003' },
    { batchNo: '20260917-EG100-custom' }, { batchNo: '20260916-EG100-999' },
  ]), 8);
  console.log('Production batch format, validation and Beijing midnight checks passed.');
  if (!process.env.BATCH_TEST_DATABASE_URL) return;
  const url = new URL(process.env.BATCH_TEST_DATABASE_URL);
  assert.equal(url.hostname, '127.0.0.1');
  assert.equal(url.port, '5547');
  assert.equal(url.pathname, '/batch_test', 'Only the isolated batch_test database may be used');
  const db = new PrismaClient({ datasources: { db: { url: url.toString() } } });
  try {
    assert.equal(await db.factoryGateway.count(), 0, 'Test database must be empty');
    for (const code of ['EG100', 'EG200', 'EG300']) {
      await db.productModel.upsert({ where: { code }, create: { code, name: code }, update: { enabled: true } });
    }
    const service = new FactoryGatewaysService(db, { get: () => undefined });
    // Test-only credentials/signing are stubbed; production keys are never read.
    service.createActivationFile = input => ({ ...input, signature: 'test-only' });
    service.encryptCredential = () => 'test-only-encrypted';
    const dto = (number, overrides = {}) => ({
      hardwareId: number.toString(16).toUpperCase().padStart(32, '0'),
      chipUsid: number.toString(16).toUpperCase().padStart(16, '0'), ...overrides,
    });
    const registrations = await Promise.all(Array.from({ length: 16 }, (_, index) => service.register(dto(index + 1))));
    const prefix = productionBatchPrefix('EG100');
    assert.deepEqual(registrations.map(item => item.batchNo).sort(), Array.from({ length: 16 }, (_, index) => `${prefix}${String(index + 1).padStart(3, '0')}`));
    assert.ok(registrations.every(item => item.model === 'EG100'));
    const differentModel = await service.register(dto(20, { model: 'eg200' }));
    assert.equal(differentModel.batchNo, `${productionBatchPrefix('EG200')}001`);
    await service.register(dto(21, { batchNo: 'MANUAL-BATCH' }));
    assert.equal((await service.register(dto(22))).batchNo, `${prefix}017`);
    await service.register(dto(23, { batchNo: `${prefix}050` }));
    assert.equal((await service.register(dto(24))).batchNo, `${prefix}051`);
    await db.factoryGateway.update({ where: { hardwareId: dto(24).hardwareId }, data: { status: 'REMOVED' } });
    assert.equal((await service.register(dto(24))).batchNo, `${prefix}052`);
    await assert.rejects(() => service.register(dto(24)), /已经登记/);
    assert.equal((await service.register(dto(25))).batchNo, `${prefix}053`);
    // Fail the gateway insert after the counter allocation: the transaction must
    // roll back the counter too, so a failed registration consumes no sequence.
    const failDb = {
      factoryGateway: db.factoryGateway,
      $transaction: callback => db.$transaction(tx => callback({
        $queryRaw: tx.$queryRaw.bind(tx),
        productModel: tx.productModel,
        factoryGateway: { findMany: args => tx.factoryGateway.findMany(args), create: async () => { throw new Error('test insert failed'); } },
      })),
    };
    const failing = new FactoryGatewaysService(failDb, { get: () => undefined });
    failing.createActivationFile = service.createActivationFile;
    failing.encryptCredential = service.encryptCredential;
    await assert.rejects(() => failing.register(dto(26)), /test insert failed/);
    assert.equal((await service.register(dto(27))).batchNo, `${prefix}054`);
    // Counter persists even after removal of its old gateway record.
    await db.factoryGateway.delete({ where: { hardwareId: dto(27).hardwareId } });
    assert.equal((await service.register(dto(28))).batchNo, `${prefix}055`);
    // On initial upgrade, existing matching batches seed a new model's counter.
    const eg300 = productionBatchPrefix('EG300');
    await service.register(dto(29, { model: 'EG300', batchNo: `${eg300}008` }));
    assert.equal((await service.register(dto(30, { model: 'EG300' }))).batchNo, `${eg300}009`);
    console.log('Isolated PostgreSQL checks passed: 16 concurrent registrations, model isolation, manual override, legacy seed, removal, duplicate rejection, transaction rollback.');
  } finally {
    await db.$disconnect();
  }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });

// Run inside the deployed API container via stdin. All test writes roll back.
require('reflect-metadata');
const assert = require('node:assert/strict');
const { randomBytes } = require('node:crypto');
const { PrismaClient } = require('@prisma/client');
const { FactoryGatewaysService } = require('./dist/factory-gateways/factory-gateways.service');
const { productionBatchPrefix } = require('./dist/factory-gateways/production-batch');
const db = new PrismaClient();
const rollback = new Error('EXPECTED_BATCH_VERIFICATION_ROLLBACK');
(async () => {
  const prefix = productionBatchPrefix('EG100');
  const before = await db.factoryProductionCounter.findUnique({ where: { prefix } });
  const count = await db.factoryGateway.count();
  let verified = false;
  try {
    await db.$transaction(async tx => {
      const service = new FactoryGatewaysService({
        factoryGateway: tx.factoryGateway,
        $transaction: callback => callback(tx),
      }, { get: () => undefined });
      // Only test batch allocation; never issue a real deliverable license.
      service.createActivationFile = () => ({ signature: 'test-only' });
      service.encryptCredential = () => 'test-only';
      const identity = () => ({ hardwareId: randomBytes(16).toString('hex').toUpperCase(), chipUsid: randomBytes(8).toString('hex').toUpperCase() });
      const first = await service.register(identity());
      const second = await service.register(identity());
      assert.ok(first.batchNo.startsWith(prefix));
      assert.ok(second.batchNo.startsWith(prefix));
      assert.equal(Number(second.batchNo.slice(prefix.length)), Number(first.batchNo.slice(prefix.length)) + 1);
      assert.equal(first.model, 'EG100');
      verified = true;
      throw rollback;
    }, { timeout: 15000 });
  } catch (error) {
    if (error !== rollback && error.message !== rollback.message) throw error;
  }
  assert.ok(verified);
  assert.equal(await db.factoryGateway.count(), count, 'Test gateway records persisted');
  assert.deepEqual(await db.factoryProductionCounter.findUnique({ where: { prefix } }), before, 'Test consumed production sequence');
  console.log('Deployed API batch allocation verified; all test records and sequence allocations rolled back.');
})().catch(error => { console.error(error.message); process.exitCode = 1; }).finally(() => db.$disconnect());

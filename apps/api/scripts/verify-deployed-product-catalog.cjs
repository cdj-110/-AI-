// Execute through stdin in /app of the deployed API. No test writes persist.
require('reflect-metadata');
const assert = require('node:assert/strict');
const { randomBytes } = require('node:crypto');
const { PrismaClient } = require('@prisma/client');
const { JwtService } = require('@nestjs/jwt');
const { ProductsService } = require('./dist/products/products.service');
const { FactoryGatewaysService } = require('./dist/factory-gateways/factory-gateways.service');
const { productionBatchPrefix } = require('./dist/factory-gateways/production-batch');
const db = new PrismaClient();
const rollback = new Error('EXPECTED_PRODUCT_CATALOG_TEST_ROLLBACK');
(async () => {
  const eg100 = await db.productModel.findUnique({ where: { code: 'EG100' } });
  assert.ok(eg100 && eg100.enabled);
  assert.equal(eg100.ethernetPortCount, 2);
  assert.equal(eg100.rs232PortCount, 2);
  assert.equal(eg100.rs485PortCount, 2);
  assert.equal(eg100.canPortCount, null);
  assert.equal(eg100.supports4G, true);
  assert.equal(eg100.supportsWifi, true);
  const before = {
    products: await db.productModel.findMany({ orderBy: { code: 'asc' } }),
    factories: await db.factoryGateway.count(),
    counters: await db.factoryProductionCounter.findMany({ orderBy: { prefix: 'asc' } }),
  };
  let verified = false;
  try {
    await db.$transaction(async tx => {
      const code = `VERIFY-${randomBytes(6).toString('hex').toUpperCase()}`;
      const products = new ProductsService(tx);
      await products.create({ code, name: 'Rollback-only product test', ethernetPortCount: 4, rs232PortCount: 2, rs485PortCount: 2, canPortCount: 1, supports4G: true, supportsWifi: false });
      const gateways = new FactoryGatewaysService({ factoryGateway: tx.factoryGateway, $transaction: callback => callback(tx) }, { get: () => undefined });
      gateways.createActivationFile = () => ({ signature: 'test-only' });
      gateways.encryptCredential = () => 'test-only';
      const identity = () => ({ hardwareId: randomBytes(16).toString('hex').toUpperCase(), chipUsid: randomBytes(8).toString('hex').toUpperCase(), model: code });
      await products.update(code, { enabled: false });
      await assert.rejects(() => gateways.register(identity()), /停用/);
      assert.equal(await tx.factoryProductionCounter.findUnique({ where: { prefix: productionBatchPrefix(code) } }), null);
      await products.update(code, { enabled: true });
      const first = await gateways.register(identity());
      const second = await gateways.register(identity());
      assert.equal(first.batchNo, `${productionBatchPrefix(code)}001`);
      assert.equal(second.batchNo, `${productionBatchPrefix(code)}002`);
      assert.equal((await products.findAll()).find(item => item.code === code)._count.gateways, 2);
      verified = true;
      throw rollback;
    }, { timeout: 15000 });
  } catch (error) {
    if (error !== rollback && error.message !== rollback.message) throw error;
  }
  assert.ok(verified);
  assert.deepEqual(await db.productModel.findMany({ orderBy: { code: 'asc' } }), before.products);
  assert.equal(await db.factoryGateway.count(), before.factories);
  assert.deepEqual(await db.factoryProductionCounter.findMany({ orderBy: { prefix: 'asc' } }), before.counters);
  // Short-lived, in-memory diagnostic tokens; no passwords or tokens are printed.
  const jwt = new JwtService({ secret: process.env.JWT_SECRET });
  for (const role of ['SUPER_ADMIN', 'TENANT_ADMIN']) {
    const user = await db.user.findFirst({ where: { role }, select: { id: true, username: true, role: true, tenantId: true } });
    assert.ok(user, `Missing account role ${role}`);
    const token = jwt.sign({ sub: user.id, username: user.username, role: user.role, tenantId: user.tenantId }, { expiresIn: '60s' });
    const response = await fetch('http://127.0.0.1:3100/api/products', { headers: { Authorization: `Bearer ${token}` } });
    assert.equal(response.status, role === 'SUPER_ADMIN' ? 200 : 403);
    if (role === 'SUPER_ADMIN') assert.ok((await response.json()).data.some(product => product.code === 'EG100'));
  }
  assert.equal((await fetch('http://127.0.0.1:3100/api/products')).status, 401);
  console.log('Deployed product catalog verified: EG100 seed, CRUD, disabled-model rejection, batch sequence, admin-only HTTP access; all test writes rolled back.');
})().catch(error => { console.error(error.message); process.exitCode = 1; }).finally(() => db.$disconnect());

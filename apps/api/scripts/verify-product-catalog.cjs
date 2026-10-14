require('reflect-metadata');
const assert = require('node:assert/strict');
const { validate } = require('class-validator');
const { Reflector } = require('@nestjs/core');
const { PrismaClient } = require('@prisma/client');
const { Client } = require('pg');
const fs = require('node:fs');
const path = require('node:path');
const { ProductsService } = require('../dist/products/products.service');
const { ProductsController } = require('../dist/products/products.controller');
const { CreateProductDto } = require('../dist/products/dto/create-product.dto');
const { UpdateProductDto } = require('../dist/products/dto/update-product.dto');
const { RolesGuard } = require('../dist/common/guards/roles.guard');
const { FactoryGatewaysService } = require('../dist/factory-gateways/factory-gateways.service');

async function main() {
  const guard = new RolesGuard(new Reflector());
  for (const role of ['SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER']) {
    for (const method of ['findAll', 'create', 'update', 'remove']) {
      assert.equal(guard.canActivate({ getHandler: () => ProductsController.prototype[method], getClass: () => ProductsController, switchToHttp: () => ({ getRequest: () => ({ user: { role } }) }) }), role === 'SUPER_ADMIN');
    }
  }
  assert.deepEqual(await validate(Object.assign(new CreateProductDto(), { code: 'EG100', name: 'Test', ethernetPortCount: null, supports4G: null })), []);
  for (const data of [{ code: '', name: 'Test' }, { code: 'EG100', name: '' }, { code: 'EG100', name: 'Test', serialPortCount: -1 }, { code: 'EG100', name: 'Test', serialPortCount: 1.5 }, { code: 'EG100', name: 'Test', ethernetPortCount: 65 }, { code: 'EG100', name: 'Test', supportsWifi: 'yes' }]) {
    assert.ok((await validate(Object.assign(new CreateProductDto(), data))).length > 0);
  }
  assert.deepEqual(await validate(Object.assign(new UpdateProductDto(), { supports4G: null, rs232PortCount: null, rs485PortCount: null, canPortCount: null })), []);
  for (const field of ['rs232PortCount', 'rs485PortCount', 'canPortCount']) {
    for (const count of [-1, 1.5, 65]) {
      assert.ok((await validate(Object.assign(new CreateProductDto(), { code: 'TEST', name: 'Test', [field]: count }))).length > 0);
    }
  }
  console.log('Product catalog DTO and super-admin authorization checks passed.');
  if (!process.env.BATCH_TEST_DATABASE_URL) return;
  const url = new URL(process.env.BATCH_TEST_DATABASE_URL);
  assert.equal(url.hostname, '127.0.0.1'); assert.equal(url.port, '5547'); assert.equal(url.pathname, '/batch_test');
  // Replay the migration against legacy references in an isolated, rolled-back
  // schema so that upgrading cannot orphan an existing non-EG100 model.
  const client = new Client({ connectionString: url.toString() });
  await client.connect();
  try {
    await client.query('BEGIN');
    await client.query('CREATE SCHEMA catalog_upgrade_test');
    await client.query('SET LOCAL search_path TO catalog_upgrade_test');
    await client.query('CREATE TABLE "FactoryGateway" (id SERIAL PRIMARY KEY, model TEXT NOT NULL)');
    await client.query('INSERT INTO "FactoryGateway" (model) VALUES (\'EG100\'),(\'LEGACY-MODEL\')');
    await client.query(fs.readFileSync(path.join(__dirname, '../prisma/migrations/20260917033000_product_catalog/migration.sql'), 'utf8'));
    await client.query('UPDATE "ProductModel" SET "serialPortCount"=8 WHERE code=\'LEGACY-MODEL\'');
    await client.query(fs.readFileSync(path.join(__dirname, '../prisma/migrations/20260917040000_typed_product_ports/migration.sql'), 'utf8'));
    assert.equal((await client.query('SELECT count(*)::int AS count FROM "FactoryGateway"')).rows[0].count, 2);
    assert.equal((await client.query('SELECT count(*)::int AS count FROM "ProductModel"')).rows[0].count, 2);
    assert.equal((await client.query('SELECT "supports4G" FROM "ProductModel" WHERE code=\'LEGACY-MODEL\'')).rows[0].supports4G, null);
    const legacy = (await client.query('SELECT "serialPortCount","rs232PortCount","rs485PortCount","canPortCount" FROM "ProductModel" WHERE code=\'LEGACY-MODEL\'')).rows[0];
    assert.deepEqual(legacy, { serialPortCount: 8, rs232PortCount: null, rs485PortCount: null, canPortCount: null });
    await client.query('ROLLBACK');
    console.log('Legacy-model migration preservation check passed; test schema rolled back.');
  } finally { await client.end(); }
  const db = new PrismaClient({ datasources: { db: { url: url.toString() } } });
  try {
    const products = new ProductsService(db);
    const gateways = new FactoryGatewaysService(db, { get: () => undefined });
    gateways.createActivationFile = input => ({ ...input, signature: 'test-only' });
    gateways.encryptCredential = () => 'test-only';
    const eg100 = await db.productModel.findUnique({ where: { code: 'EG100' } });
    assert.ok(eg100, 'Migration must seed EG100');
    assert.equal(eg100.ethernetPortCount, 2); assert.equal(eg100.serialPortCount, 4); assert.equal(eg100.rs232PortCount, 2); assert.equal(eg100.rs485PortCount, 2); assert.equal(eg100.canPortCount, 0); assert.equal(eg100.supports4G, true); assert.equal(eg100.supportsWifi, true);
    const product = await products.create({ code: ' test-4g ', name: ' Test model ', ethernetPortCount: 4, rs232PortCount: 4, rs485PortCount: 4, canPortCount: 1, supports4G: true, supportsWifi: false });
    assert.equal(product.code, 'TEST-4G'); assert.equal(product.name, 'Test model');
    assert.equal(product.rs232PortCount, 4); assert.equal(product.rs485PortCount, 4); assert.equal(product.canPortCount, 1);
    await assert.rejects(() => products.create({ code: 'test-4g', name: 'Duplicate' }), /已存在/);
    await assert.rejects(() => products.create({ code: ' ', name: 'Empty' }), /请填写产品型号/);
    const identity = { hardwareId: 'F'.repeat(32), chipUsid: 'F'.repeat(16), model: 'TEST-4G' };
    await products.update('TEST-4G', { enabled: false });
    const before = await db.factoryProductionCounter.count();
    await assert.rejects(() => gateways.register(identity), /停用/);
    assert.equal(await db.factoryProductionCounter.count(), before, 'Disabled product consumed a sequence');
    await assert.rejects(() => gateways.register({ ...identity, model: 'UNKNOWN' }), /不存在/);
    await products.update('TEST-4G', { enabled: true, supportsWifi: true });
    const registered = await gateways.register(identity);
    assert.equal(registered.model, 'TEST-4G'); assert.match(registered.batchNo, /^\d{8}-TEST-4G-001$/);
    const listed = await products.findAll();
    assert.equal(listed.find(item => item.code === 'TEST-4G')._count.gateways, 1);
    await assert.rejects(() => products.remove('TEST-4G'), /已有出厂网关引用/);
    await products.update('TEST-4G', { enabled: false, supportsWifi: null, rs232PortCount: null, canPortCount: null });
    assert.ok(await db.factoryGateway.findUnique({ where: { id: registered.id } }), 'Disabling a model removed its gateway');
    await products.create({ code: 'UNUSED', name: 'Unused' });
    await products.remove('UNUSED');
    assert.equal(await db.productModel.findUnique({ where: { code: 'UNUSED' } }), null);
    await assert.rejects(() => products.update('MISSING', { name: 'X' }), /不存在/);
    await assert.rejects(() => products.update('EG100', { enabled: null }), /不能为空/);
    console.log('Isolated PostgreSQL product catalog checks passed: migration seed, CRUD, duplicate protection, referenced-model deletion, disabled/unknown registration, sequence consistency, nullable capabilities.');
  } finally { await db.$disconnect(); }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });

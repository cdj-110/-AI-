const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { OtaService } = require('../dist/ota/ota.service');

const packageId = '11111111-1111-4111-8111-111111111111';
const fileName = '22222222-2222-4222-8222-222222222222.wku';
const pack = {
  id: packageId, model: 'EG100', release: 2026092102n, version: 'V0.1', keyId: 'fixture',
  artifactSha256: 'a'.repeat(64), packageSha256: 'b'.repeat(64), fileName, size: 4,
  fileAvailable: true, purgedAt: null, createdBy: 'tester', createdAt: new Date(),
};

function serviceFor({ current = [], active = [] } = {}) {
  const updates = [];
  const prisma = {
    otaTask: {
      updateMany: async () => ({ count: 0 }),
      findMany: async () => current,
      groupBy: async () => active,
    },
    otaPackage: {
      findUnique: async () => ({ ...pack }),
      findMany: async () => [{ ...pack, _count: { tasks: 3 } }],
      update: async ({ data }) => { updates.push(data); return { ...pack, ...data }; },
    },
  };
  return { service: new OtaService(prisma), updates };
}

(async () => {
  const current = serviceFor({ current: [{ gatewayId: 'gateway-1', packageId }] });
  await assert.rejects(() => current.service.purgePackage(packageId), error => error.getStatus?.() === 409 && /正在使用/.test(error.message));

  const active = serviceFor({ active: [{ packageId, _count: { _all: 1 } }] });
  await assert.rejects(() => active.service.purgePackage(packageId), error => error.getStatus?.() === 409 && /进行中/.test(error.message));

  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'wk-ota-retention-'));
  process.env.OTA_PACKAGE_DIR = directory;
  await fs.writeFile(path.join(directory, fileName), 'test');
  const safe = serviceFor();
  const result = await safe.service.purgePackage(packageId);
  assert.equal(result.fileAvailable, false);
  assert.ok(result.purgedAt instanceof Date);
  assert.equal(safe.updates.length, 1);
  await assert.rejects(() => fs.stat(path.join(directory, fileName)), error => error.code === 'ENOENT');

  const list = serviceFor({ current: [{ gatewayId: 'gateway-1', packageId }] });
  const [view] = await list.service.listPackages();
  assert.equal(view.currentDeviceCount, 1);
  assert.equal(view.activeTaskCount, 0);
  assert.equal(view.canPurge, false);

  await fs.rm(directory, { recursive: true, force: true });
  delete process.env.OTA_PACKAGE_DIR;
  console.log('OTA package retention checks passed');
})().catch(error => { console.error(error); process.exit(1); });

// Read-only check in the deployed API container. Never initialize the worker or issue commands.
const assert = require('node:assert/strict');
const { PrismaClient } = require('@prisma/client');
const { DeviceLinkageService } = require('./dist/device-linkage/device-linkage.service');
async function main() {
  const prisma = new PrismaClient();
  try {
    const migrations = await prisma.$queryRaw`SELECT finished_at FROM _prisma_migrations WHERE migration_name='20261014120000_device_linkage' AND rolled_back_at IS NULL`;
    assert.ok(migrations[0]?.finished_at);
    const defaults = await prisma.$queryRaw`SELECT column_default FROM information_schema.columns WHERE table_schema='public' AND table_name='DeviceLinkageRule' AND column_name='enabled'`;
    assert.equal(defaults[0].column_default, 'false');
    const [rules, enabled, runs] = await Promise.all([prisma.deviceLinkageRule.count(), prisma.deviceLinkageRule.count({ where: { enabled: true } }), prisma.deviceLinkageRun.count()]);
    assert.equal(enabled, 0, 'Deployment must not enable device controls');
    assert.equal(runs, 0, 'Deployment must not dispatch device controls');
    const user = await prisma.user.findFirst({ where: { role: 'TENANT_ADMIN', status: 'ACTIVE', tenantId: { not: null } } });
    if (user) {
      const service = new DeviceLinkageService(prisma, {}, { create: () => { throw Error('Device writes prohibited'); } }, {});
      const actor = { sub: user.id, username: user.username, role: user.role, tenantId: user.tenantId };
      const list = await service.list(actor); assert.ok(Array.isArray(list.items));
      await assert.rejects(() => service.list({ ...actor, role: 'TENANT_USER' }), /租户管理员/);
    }
    const response = await fetch('http://127.0.0.1:3100/api/device-linkage');
    assert.equal(response.status, 401);
    console.log(JSON.stringify({ result: 'PASS deployed linkage migration/default-disabled/read-only list/role guard/anonymous HTTP401', rules, enabled, runs, realDeviceWrites: 0 }));
  } finally { await prisma.$disconnect(); }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });

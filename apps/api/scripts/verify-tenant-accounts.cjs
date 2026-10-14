require('reflect-metadata');
const assert = require('node:assert/strict');
const { ValidationPipe } = require('@nestjs/common');
const { Reflector } = require('@nestjs/core');
const { TenantsController } = require('../dist/tenants/tenants.controller');
const { TenantsService } = require('../dist/tenants/tenants.service');
const { TenantAccountsQueryDto } = require('../dist/tenants/dto/tenant-accounts-query.dto');
const { RolesGuard } = require('../dist/common/guards/roles.guard');

async function main() {
  const guard = new RolesGuard(new Reflector());
  for (const role of ['SUPER_ADMIN', 'TENANT_ADMIN', 'TENANT_USER', undefined]) {
    assert.equal(guard.canActivate({
      getHandler: () => TenantsController.prototype.accounts,
      getClass: () => TenantsController,
      switchToHttp: () => ({ getRequest: () => ({ user: { role } }) }),
    }), role === 'SUPER_ADMIN');
  }
  const pipe = new ValidationPipe({ transform: true, whitelist: true });
  const metadata = { type: 'query', metatype: TenantAccountsQueryDto };
  for (const input of [{ page: '0' }, { page: '1.5' }, { pageSize: '51' }, { role: 'SUPER_ADMIN' }, { keyword: 'x'.repeat(101) }]) {
    await assert.rejects(() => pipe.transform(input, metadata));
  }
  const query = await pipe.transform({ page: '2', pageSize: '10', keyword: ' alice ', role: 'TENANT_USER', tenantId: 'other' }, metadata);
  assert.equal(query.tenantId, undefined);
  const fakeDb = {
    tenant: { findUnique: async ({ where }) => where.id === 'tenant-a' ? { id: where.id } : null },
    user: {
      findMany: async (args) => {
        assert.equal(args.where.tenantId, 'tenant-a');
        assert.equal(args.where.role, 'TENANT_USER');
        assert.equal(args.where.OR[0].username.contains, 'alice');
        assert.equal(args.skip, 10);
        assert.equal(args.take, 10);
        assert.equal(args.select.password, undefined);
        assert.equal(args.select.loginLogs.where.success, true);
        assert.equal(args.select.loginLogs.take, 1);
        assert.deepEqual(Object.keys(args.select).sort(), ['id', 'username', 'nickname', 'email', 'phone', 'role', 'status', 'createdAt', 'loginLogs'].sort());
        return [{ id: 'user-a', username: 'alice', loginLogs: [{ createdAt: '2026-09-23T00:00:00Z' }] }, { id: 'user-b', username: 'new-user', loginLogs: [] }];
      },
      count: async ({ where }) => { assert.equal(where.tenantId, 'tenant-a'); assert.equal(where.role, 'TENANT_USER'); return 12; },
    },
    $transaction: (operations) => Promise.all(operations),
  };
  const service = new TenantsService(fakeDb);
  const result = await service.accounts('tenant-a', query);
  assert.equal(result.total, 12);
  assert.equal(result.items[0].lastLoginAt, '2026-09-23T00:00:00Z');
  assert.equal(result.items[1].lastLoginAt, null);
  assert.equal(result.items[0].loginLogs, undefined);
  await assert.rejects(() => service.accounts('missing', query), /租户不存在/);
  console.log('Tenant account role guard, query validation, tenant scope, pagination and safe-field checks passed.');
}
main().catch((error) => { console.error(error); process.exitCode = 1; });

require('reflect-metadata');
const assert = require('node:assert/strict');
const { UsersService } = require('../dist/users/users.service');

async function main() {
  const actor = { sub: 'operator', role: 'TENANT_ADMIN', tenantId: 'tenant-a' };
  let stored = { id: 'member', username: 'legacy-name', nickname: 'Previous name', email: 'member@example.test', phone: null, tenantId: 'tenant-a', role: 'TENANT_USER', status: 'ACTIVE', password: 'never-return' };
  const service = new UsersService({ user: {
    findUnique: async () => ({ ...stored }),
    findFirst: async () => null,
    update: async ({ where, data }) => {
      assert.equal(where.id, stored.id);
      stored = { ...stored, ...data };
      return stored;
    },
    create: async ({ data }) => ({ id: 'new-member', ...data }),
  } });
  const updated = await service.update(actor, 'member', { username: ' Updated name ' });
  assert.equal(updated.username, 'Updated name');
  assert.equal(updated.nickname, 'Updated name');
  assert.equal(updated.email, 'member@example.test');
  assert.equal(updated.tenantId, 'tenant-a');
  assert.equal(updated.password, undefined);
  const statusOnly = await service.update(actor, 'member', { status: 'DISABLED' });
  assert.equal(statusOnly.nickname, 'Updated name');
  assert.equal(statusOnly.username, 'Updated name');
  const created = await service.create(actor, { username: ' New name ', nickname: 'Stale hidden name', email: 'new@example.test', password: 'test-password', role: 'TENANT_USER' });
  assert.equal(created.username, 'New name');
  assert.equal(created.nickname, 'New name');
  assert.equal(created.password, undefined);
  console.log('Unified user name create/update, unrelated-edit preservation and login-contact preservation checks passed.');
}
main().catch((error) => { console.error(error); process.exitCode = 1; });

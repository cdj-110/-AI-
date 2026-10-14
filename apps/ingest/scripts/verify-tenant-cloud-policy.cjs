const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

async function main() {
  let blocked = true;
  let statusWrites = 0, telemetryWrites = 0, factoryWrites = 0;
  const queries = [];
  const handlers = {};
  const device = () => ({ id: 'd', tenantId: 't', deviceKey: 'd', name: 'test', status: 'ONLINE', cloudAccessEnabled: true, tenant: { deviceCloudAccessDisabled: blocked } });
  const prisma = {
    $executeRawUnsafe: async () => 0,
    device: {
      findFirst: async ({ where }) => {
        queries.push(where);
        assert.equal(where.tenant?.deviceCloudAccessDisabled, false, 'each ingestion lookup must check tenant policy');
        if (where.gateway) assert.equal(where.gateway.tenant?.deviceCloudAccessDisabled, false);
        return blocked ? null : device();
      },
      updateMany: async ({ where }) => {
        assert.equal(where.tenant?.deviceCloudAccessDisabled, false, 'status writes recheck current policy');
        if (!blocked) statusWrites++;
        return { count: blocked ? 0 : 1 };
      },
    },
    factoryGateway: {
      findUnique: async () => ({ id: 'factory', status: 'BOUND', device: device() }),
      updateMany: async () => { factoryWrites++; return { count: 1 }; },
    },
  };
  const pool = { query: async sql => { if (sql.includes('INSERT INTO telemetry_events')) telemetryWrites++; return { rows: [], rowCount: 0 }; } };
  const context = {
    exports: {}, process: { env: {}, on() {} }, Buffer, AbortController,
    console: { log() {}, warn() {}, error(...args) { throw new Error(args.join(' ')); } },
    setInterval() {}, setTimeout() {}, clearInterval() {}, clearTimeout() {},
    require(name) {
      if (name === '@prisma/client') return { PrismaClient: function () { return prisma; } };
      if (name === 'mqtt') return { connect: () => ({ on: (event, fn) => { handlers[event] = fn; } }) };
      if (name === 'pg') return { Pool: function () { return pool; } };
      if (name === './startup-readiness') return { retryUntilReady: async () => {}, isAbortError: () => false };
      if (name === './license-report') return { parseLicenseReport: () => null };
      return {};
    },
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../dist/main.js'), 'utf8'), context);
  await handlers.message('weikong/devices/d/telemetry', Buffer.from('{"value":1}'));
  await handlers.message('weikong/gateways/g/children/d/telemetry', Buffer.from('{"value":1}'));
  await handlers.message('weikong/factory/ABC/heartbeat', Buffer.from('{}'));
  await handlers.message('weikong/gateways/g/topology/reported', Buffer.from('{"gatewayKey":"g","devices":[]}'));
  await handlers.message('weikong/gateways/g/events', Buffer.from('{"eventId":"e","state":"ACTIVE"}'));
  assert.equal(statusWrites + telemetryWrites + factoryWrites, 0, 'blocked traffic must not mutate online status, telemetry or heartbeat');
  assert(queries.length >= 3);
  blocked = false;
  await handlers.message('weikong/devices/d/telemetry', Buffer.from('{"value":1}'));
  await handlers.message('weikong/gateways/g/children/d/telemetry', Buffer.from('{"value":1}'));
  await handlers.message('weikong/factory/ABC/heartbeat', Buffer.from('{}'));
  assert.equal(statusWrites, 3); // Direct device, child and bound factory heartbeat.
  assert.equal(telemetryWrites, 2);
  assert.equal(factoryWrites, 1);
  console.log('Ingest tenant policy passed: blocked direct/child telemetry, factory heartbeat, topology/events; unblocked traffic resumes. No database or broker was accessed.');
}
main().catch(error => { console.error(error); process.exitCode = 1; });

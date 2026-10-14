const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `device-groups-deploy-${stamp}`);
const migration = '20260923110000_device_groups';
const updated = ['api', 'web', 'ingest'];
const preserved = ['postgres', 'timescaledb', 'redis', 'emqx', 'ota-https', 'admin-web'];
const docker = args => cp.execFileSync('docker', args, { cwd: root, maxBuffer: 192 * 1024 * 1024 });
const run = args => {
  const result = cp.spawnSync('docker', args, { cwd: root, stdio: 'inherit' });
  assert.ok(result.status === 0, `Deployment step failed: ${args.slice(0, 4).join(' ')}`);
};
const inspect = service => {
  const id = docker(['compose', 'ps', '-q', service]).toString().trim();
  assert.ok(id, `Missing service: ${service}`);
  return JSON.parse(docker(['inspect', id]))[0];
};
const env = item => Object.fromEntries(item.Config.Env.map(line => {
  const i = line.indexOf('='); return [line.slice(0, i), line.slice(i + 1)];
}));
const query = sql => docker(['compose', 'exec', '-T', 'postgres', 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'weikong', '-d', 'weikong_iot', '-At', '-c', sql]).toString().trim();
const secretHashes = () => docker(['compose', 'exec', '-T', 'api', 'node', '-e', `const fs=require('fs'),c=require('crypto'); console.log(JSON.stringify(['jwt-secret','mail-password','emqx-connection-api.json'].map(n=>[n,c.createHash('sha256').update(fs.readFileSync('/var/lib/weikong/secrets/'+n)).digest('hex')])));`]).toString().trim();
async function http(url, status) {
  for (let i = 0; i < 60; i++) {
    try { if ((await fetch(url, {signal: AbortSignal.timeout(3000)})).status === status) return; } catch {}
    await new Promise(r => setTimeout(r, 500));
  }
  throw new Error(`HTTP verification failed: ${url}`);
}
async function main() {
  const config = JSON.parse(docker(['compose', 'config', '--format', 'json']));
  const before = Object.fromEntries([...updated, ...preserved].map(s => [s, inspect(s)]));
  for (const s of ['api', 'ingest']) {
    const live = env(before[s]);
    for (const [key, value] of Object.entries(config.services[s].environment)) {
      assert.ok(String(value ?? '') === (live[key] ?? ''), `Configuration mismatch (value redacted): ${s}.${key}`);
    }
  }
  const secretsBefore = secretHashes();
  const applied = JSON.parse(query(`SELECT COALESCE(json_agg(json_build_object('name',migration_name,'checksum',checksum)),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
  assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE finished_at IS NULL AND rolled_back_at IS NULL;`), '0', 'Unresolved migration exists');
  const migrationRoot = path.join(root, 'apps/api/prisma/migrations');
  const local = fs.readdirSync(migrationRoot).filter(n => fs.existsSync(path.join(migrationRoot, n, 'migration.sql')));
  for (const item of applied) {
    assert.ok(local.includes(item.name), `Missing migration: ${item.name}`);
    const hash = crypto.createHash('sha256').update(fs.readFileSync(path.join(migrationRoot, item.name, 'migration.sql'))).digest('hex');
    assert.equal(hash, item.checksum, `Historical migration modified: ${item.name}`);
  }
  const pending = local.filter(n => !applied.some(a => a.name === n));
  assert.deepEqual(pending, [migration]);
  fs.mkdirSync(out, {recursive: true});
  const backup = JSON.parse(cp.execFileSync(process.execPath, ['tools/backup-production-batch-deploy.cjs'], {cwd: root, encoding: 'utf8', maxBuffer: 4 * 1024 * 1024}).trim());
  assert.ok(backup.encryptedBackupVerified);
  const rollbackServices = {};
  for (const s of updated) {
    const tag = `wk-device-groups-rollback-${s}:${stamp}`;
    docker(['tag', before[s].Image, tag]); rollbackServices[s] = {image: tag};
  }
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({services: rollbackServices}, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({databaseBackupDirectory: backup.backupDirectory, pending, rollbackServices}, null, 2));
  console.log(JSON.stringify({stage: 'backup-verified', out, pending, configurationUnchanged: true}));
  let replacing = false;
  try {
    run(['compose', 'build', ...updated]);
    run(['compose', 'run', '--rm', '--no-deps', 'api', 'npx', 'prisma', 'migrate', 'deploy']);
    assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='${migration}' AND finished_at IS NOT NULL AND rolled_back_at IS NULL;`), '1');
    assert.equal(query(`SELECT count(*) FROM pg_constraint WHERE conname='Device_groupId_tenantId_fkey';`), '1');
    assert.equal(query(`SELECT count(*) FROM "Device" WHERE "groupId" IS NOT NULL;`), '0');
    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', 'api', 'web']);
    await http('http://127.0.0.1:3100/api/device-groups', 401);
    await http('http://127.0.0.1:5174/', 200);
    // Refresh upstream DNS without changing unrelated service images or containers.
    for (const s of ['admin-web', 'ota-https']) {
      docker(['compose', 'exec', '-T', s, 'nginx', '-t']);
      docker(['compose', 'exec', '-T', s, 'nginx', '-s', 'reload']);
    }
    await http('http://127.0.0.1:5174/api/device-groups', 401);
    await http('http://127.0.0.1:5176/api/devices', 401);
    run(['compose', 'up', '-d', '--no-deps', 'ingest']);
    let ready = false;
    for (let i = 0; i < 45; i++) {
      const logs = docker(['compose', 'logs', '--since', '2m', '--tail', '100', 'ingest']).toString();
      if (inspect('ingest').State.Running && logs.includes('[ingest] subscribed to all MQTT ingestion topics')) {ready = true; break;}
      await new Promise(r => setTimeout(r, 1000));
    }
    assert.ok(ready, 'MQTT ingestion did not recover');
    const smoke = docker(['compose', 'exec', '-T', 'api', 'node', '-e', `const {PrismaClient}=require('@prisma/client'); const {DeviceGroupsService}=require('./dist/devices/device-groups.service'); const p=new PrismaClient(); (async()=>{let checked=0; for(const t of await p.tenant.findMany({select:{id:true}})){const result=await new DeviceGroupsService(p).list({role:'TENANT_ADMIN',tenantId:t.id}); const count=await p.device.count({where:{tenantId:t.id}}); if(result.totalDevices!==count||result.ungroupedDevices!==count) throw Error('Group count mismatch'); checked++;} console.log(JSON.stringify({readOnlyGroupServiceVerified:true,tenantsChecked:checked}));})().catch(()=>{console.error('Readonly group check failed');process.exitCode=1}).finally(()=>p.$disconnect());`]).toString().trim();
    console.log(smoke);
    const html = await (await fetch('http://127.0.0.1:5174/')).text();
    const asset = html.match(/src="([^"]+\.js)"/)[1];
    const js = await (await fetch(new URL(asset, 'http://127.0.0.1:5174'))).text();
    assert.ok(js.includes('device-groups/membership') && js.includes('设备分组'), 'Tenant assets missing grouping feature');
    assert.ok(secretHashes() === secretsBefore, 'Protected secret files changed (values redacted)');
    for (const s of ['api', 'ingest']) {
      const oldEnv = env(before[s]), current = env(inspect(s));
      for (const key of Object.keys(config.services[s].environment)) assert.ok(current[key] === oldEnv[key], `Setting changed (value redacted): ${s}.${key}`);
    }
    for (const s of preserved) assert.equal(inspect(s).Id, before[s].Id, `Unrelated container replaced: ${s}`);
    const after = JSON.parse(query(`SELECT json_build_object('factoryCount',(SELECT count(*) FROM "FactoryGateway"),'deviceCount',(SELECT count(*) FROM "Device"),'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t));`));
    for (const k of ['factoryCount', 'deviceCount', 'usersHash', 'tenantsHash']) assert.equal(after[k], backup.before[k], `Preservation check failed: ${k}`);
    docker(['compose', 'exec', '-T', 'web', 'nginx', '-t']);
    const result = {deployedAt: new Date().toISOString(), migrationsApplied: pending, updatedServices: updated, frontendAsset: asset, tenantGroupReadVerified: true, unauthenticatedRequestsRejected: true, mqttSubscriptionsRestored: true, credentialsUnchanged: true, accountsAndTenantsUnchanged: true, deviceCountUnchanged: true, eg100Updated: false, databaseBackupDirectory: backup.backupDirectory, rollbackFile};
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', ...updated]);
      for (const s of ['admin-web', 'ota-https']) docker(['compose', 'exec', '-T', s, 'nginx', '-s', 'reload']);
      console.error('Application rollback completed; additive migration retained.');
    }
    throw error;
  }
}
main().catch(error => { console.error(error.stack || error.message); process.exitCode = 1; });

const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `device-group-many-to-many-deploy-${stamp}`);
const migration = '20260923170000_device_group_many_to_many';
const updatedServices = ['api', 'web', 'admin-web'];
const preservedServices = ['postgres', 'timescaledb', 'redis', 'emqx', 'ingest', 'ota-https'];
const docker = args => cp.execFileSync('docker', args, { cwd: root, maxBuffer: 192 * 1024 * 1024 });
const run = args => {
  const result = cp.spawnSync('docker', args, { cwd: root, stdio: 'inherit' });
  assert.equal(result.status, 0, `Deployment step failed: docker ${args.join(' ')}`);
};
const inspect = service => {
  const id = docker(['compose', 'ps', '-q', service]).toString().trim();
  assert.ok(id && !id.includes('\n'), `Missing service: ${service}`);
  return JSON.parse(docker(['inspect', id]))[0];
};
const environment = container => Object.fromEntries(container.Config.Env.map(line => {
  const separator = line.indexOf('=');
  return [line.slice(0, separator), line.slice(separator + 1)];
}));
const query = sql => docker(['compose', 'exec', '-T', 'postgres', 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'weikong', '-d', 'weikong_iot', '-At', '-c', sql]).toString().trim();
const secretHashes = () => docker(['compose', 'exec', '-T', 'api', 'node', '-e', `const fs=require('fs'),c=require('crypto'); console.log(JSON.stringify(['jwt-secret','mail-password','emqx-connection-api.json'].map(n=>[n,c.createHash('sha256').update(fs.readFileSync('/var/lib/weikong/secrets/'+n)).digest('hex')])));`]).toString().trim();
const stableSnapshot = () => JSON.parse(query(`SELECT json_build_object(
  'deviceCount',(SELECT count(*) FROM "Device"),
  'tenantCount',(SELECT count(*) FROM "Tenant"),
  'userCount',(SELECT count(*) FROM "User"),
  'groupCount',(SELECT count(*) FROM "DeviceGroup"),
  'legacyAssignmentCount',(SELECT count(*) FROM "Device" WHERE "groupId" IS NOT NULL),
  'devicesHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'tenantId',"tenantId",'deviceKey',"deviceKey",'name',name,'status',status,'groupId',"groupId")::text,'' ORDER BY id),'')) FROM "Device"),
  'groupsHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'tenantId',"tenantId",'name',name)::text,'' ORDER BY id),'')) FROM "DeviceGroup"),
  'usersHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'tenantId',"tenantId",'username',username,'role',role,'status',status)::text,'' ORDER BY id),'')) FROM "User")
);`));

async function waitFor(url, expectedStatus) {
  for (let attempt = 0; attempt < 60; attempt++) {
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(3000) });
      if (response.status === expectedStatus) return response;
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(`HTTP verification failed: ${url}`);
}

async function frontendAsset(baseUrl, markers = []) {
  const html = await (await waitFor(`${baseUrl}/`, 200)).text();
  const assetPath = html.match(/src="([^"]+\.js)"/)?.[1];
  assert.ok(assetPath, `Missing JavaScript asset: ${baseUrl}`);
  const javascript = await (await waitFor(new URL(assetPath, baseUrl).toString(), 200)).text();
  for (const marker of markers) assert.ok(javascript.includes(marker), `Frontend asset is missing marker: ${marker}`);
  return assetPath;
}

(async () => {
  const composeConfig = JSON.parse(docker(['compose', 'config', '--format', 'json']));
  const before = Object.fromEntries([...updatedServices, ...preservedServices].map(service => [service, inspect(service)]));
  const apiEnvironmentBefore = environment(before.api);
  for (const [key, value] of Object.entries(composeConfig.services.api.environment)) {
    assert.equal(apiEnvironmentBefore[key] ?? '', String(value ?? ''), `API configuration mismatch (value redacted): ${key}`);
  }
  const secretsBefore = secretHashes();
  const dataBefore = stableSnapshot();
  assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE finished_at IS NULL AND rolled_back_at IS NULL;`), '0', 'Unresolved migration exists');
  const applied = JSON.parse(query(`SELECT COALESCE(json_agg(json_build_object('name',migration_name,'checksum',checksum)),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
  const migrationRoot = path.join(root, 'apps', 'api', 'prisma', 'migrations');
  const localMigrations = fs.readdirSync(migrationRoot).filter(name => fs.existsSync(path.join(migrationRoot, name, 'migration.sql')));
  for (const item of applied) {
    assert.ok(localMigrations.includes(item.name), `Missing historical migration: ${item.name}`);
    const checksum = crypto.createHash('sha256').update(fs.readFileSync(path.join(migrationRoot, item.name, 'migration.sql'))).digest('hex');
    assert.equal(checksum, item.checksum, `Historical migration modified: ${item.name}`);
  }
  const pending = localMigrations.filter(name => !applied.some(item => item.name === name));
  assert.deepEqual(pending, [migration]);

  fs.mkdirSync(out, { recursive: true });
  const backup = JSON.parse(cp.execFileSync(process.execPath, ['tools/backup-production-batch-deploy.cjs'], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 4 * 1024 * 1024,
  }).trim());
  assert.equal(backup.encryptedBackupVerified, true);
  const rollbackServices = {};
  for (const service of updatedServices) {
    const tag = `wk-device-group-m2m-rollback-${service}:${stamp}`;
    docker(['tag', before[service].Image, tag]);
    rollbackServices[service] = { image: tag };
  }
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: rollbackServices }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ pending, dataBefore, databaseBackupDirectory: backup.backupDirectory, rollbackServices }, null, 2));
  console.log(JSON.stringify({ stage: 'backup-verified', out, pending, legacyAssignments: dataBefore.legacyAssignmentCount }));

  let replacing = false;
  try {
    run(['compose', 'build', ...updatedServices]);
    run(['compose', 'run', '--rm', '--no-deps', 'api', 'npx', 'prisma', 'migrate', 'deploy']);
    assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='${migration}' AND finished_at IS NOT NULL AND rolled_back_at IS NULL;`), '1');
    assert.equal(query(`SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='DeviceGroupMembership';`), '1');
    assert.equal(query(`SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='Device' AND column_name='groupId';`), '1', 'Rollback compatibility column was not retained');
    assert.equal(Number(query(`SELECT count(*) FROM "DeviceGroupMembership";`)), Number(dataBefore.legacyAssignmentCount), 'Existing assignments were not migrated exactly once');
    assert.equal(query(`SELECT count(*) FROM "Device" d WHERE d."groupId" IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "DeviceGroupMembership" m WHERE m."deviceId"=d.id AND m."groupId"=d."groupId" AND m."tenantId"=d."tenantId");`), '0');

    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', ...updatedServices]);
    await waitFor('http://127.0.0.1:3100/api/device-groups', 401);
    const tenantAsset = await frontendAsset('http://127.0.0.1:5174', ['加入分组', '移出分组', '所属分组（可多选）', '分组 + 单独']);
    const adminAsset = await frontendAsset('http://127.0.0.1:5176');
    for (const service of ['web', 'admin-web', 'ota-https']) {
      docker(['compose', 'exec', '-T', service, 'nginx', '-t']);
      docker(['compose', 'exec', '-T', service, 'nginx', '-s', 'reload']);
    }
    await waitFor('http://127.0.0.1:5174/api/device-groups', 401);
    await waitFor('http://127.0.0.1:5176/api/devices', 401);

    const smoke = docker(['compose', 'exec', '-T', 'api', 'node', '-e', `const {PrismaClient}=require('@prisma/client'); const {DeviceGroupsService}=require('./dist/devices/device-groups.service'); const {UsersService}=require('./dist/users/users.service'); const p=new PrismaClient(); (async()=>{let tenantsChecked=0,optionsChecked=0; for(const t of await p.tenant.findMany({select:{id:true}})){const r=await new DeviceGroupsService(p).list({role:'TENANT_ADMIN',tenantId:t.id}); const total=await p.device.count({where:{tenantId:t.id}}); const ungrouped=await p.device.count({where:{tenantId:t.id,groupMemberships:{none:{}}}}); if(r.totalDevices!==total||r.ungroupedDevices!==ungrouped) throw Error('Group summary mismatch'); tenantsChecked++; const admin=await p.user.findFirst({where:{tenantId:t.id,role:'TENANT_ADMIN'},select:{id:true}}); if(admin){await new UsersService(p).deviceScopeOptions({sub:admin.id,role:'TENANT_ADMIN',tenantId:t.id}); optionsChecked++;}} console.log(JSON.stringify({multiGroupReadVerified:true,tenantsChecked,optionsChecked}));})().catch(e=>{console.error(e.message);process.exitCode=1}).finally(()=>p.$disconnect());`]).toString().trim();
    console.log(smoke);

    assert.deepEqual(stableSnapshot(), dataBefore, 'Existing devices, groups, tenants or users changed');
    assert.equal(secretHashes(), secretsBefore, 'Protected secret files changed (values redacted)');
    const currentApiEnvironment = environment(inspect('api'));
    for (const key of Object.keys(composeConfig.services.api.environment)) {
      assert.equal(currentApiEnvironment[key], apiEnvironmentBefore[key], `API setting changed (value redacted): ${key}`);
    }
    for (const service of preservedServices) assert.equal(inspect(service).Id, before[service].Id, `Unrelated service was recreated: ${service}`);

    const result = {
      deployedAt: new Date().toISOString(),
      migrationsApplied: pending,
      updatedServices,
      tenantAsset,
      adminAsset,
      multiGroupReadVerified: true,
      existingAssignmentsMigrated: Number(dataBefore.legacyAssignmentCount),
      existingDataUnchanged: true,
      encryptedBackupVerified: true,
      credentialsUnchanged: true,
      unrelatedServicesUnchanged: true,
      eg100Updated: false,
      databaseBackupDirectory: backup.backupDirectory,
      rollbackFile,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', ...updatedServices]);
      docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-s', 'reload']);
      console.error('Application rollback completed; backward-compatible migration retained.');
    }
    throw error;
  }
})().catch(error => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

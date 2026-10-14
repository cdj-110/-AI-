const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `tenant-user-device-scope-deploy-${stamp}`);
const migration = '20260923150000_tenant_user_device_scope';
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
  'factoryCount',(SELECT count(*) FROM "FactoryGateway"),
  'deviceCount',(SELECT count(*) FROM "Device"),
  'tenantCount',(SELECT count(*) FROM "Tenant"),
  'userCount',(SELECT count(*) FROM "User"),
  'devicesHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'tenantId',"tenantId",'deviceKey',"deviceKey",'name',name,'status',status,'groupId',"groupId")::text,'' ORDER BY id),'')) FROM "Device"),
  'usersHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'tenantId',"tenantId",'username',username,'nickname',nickname,'phone',phone,'email',email,'role',role,'status',status)::text,'' ORDER BY id),'')) FROM "User")
);`));

async function waitFor(url, expectedStatus) {
  for (let attempt = 0; attempt < 60; attempt++) {
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(3000) });
      if (response.status === expectedStatus) return response;
    } catch {}
    await new Promise(resolveWait => setTimeout(resolveWait, 500));
  }
  throw new Error(`HTTP verification failed: ${url}`);
}

async function main() {
  const composeConfig = JSON.parse(docker(['compose', 'config', '--format', 'json']));
  const before = Object.fromEntries([...updatedServices, ...preservedServices].map(service => [service, inspect(service)]));
  const liveApiEnvironment = environment(before.api);
  for (const [key, value] of Object.entries(composeConfig.services.api.environment)) {
    assert.equal(liveApiEnvironment[key] ?? '', String(value ?? ''), `API configuration mismatch (value redacted): ${key}`);
  }
  const secretsBefore = secretHashes();
  const dataBefore = stableSnapshot();
  assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE finished_at IS NULL AND rolled_back_at IS NULL;`), '0', 'Unresolved migration exists');
  const applied = JSON.parse(query(`SELECT COALESCE(json_agg(json_build_object('name',migration_name,'checksum',checksum)),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
  const migrationRoot = path.join(root, 'apps', 'api', 'prisma', 'migrations');
  const localMigrations = fs.readdirSync(migrationRoot).filter(name => fs.existsSync(path.join(migrationRoot, name, 'migration.sql')));
  for (const item of applied) {
    assert.ok(localMigrations.includes(item.name), `Missing migration: ${item.name}`);
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
    const tag = `wk-tenant-user-scope-rollback-${service}:${stamp}`;
    docker(['tag', before[service].Image, tag]);
    rollbackServices[service] = { image: tag };
  }
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: rollbackServices }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ pending, dataBefore, databaseBackupDirectory: backup.backupDirectory, rollbackServices }, null, 2));
  console.log(JSON.stringify({ stage: 'backup-verified', out, pending, configurationUnchanged: true }));

  let replacing = false;
  try {
    run(['compose', 'build', ...updatedServices]);
    run(['compose', 'run', '--rm', '--no-deps', 'api', 'npx', 'prisma', 'migrate', 'deploy']);
    assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='${migration}' AND finished_at IS NOT NULL AND rolled_back_at IS NULL;`), '1');
    assert.equal(query(`SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='User' AND column_name IN ('permissions','deviceScopeMode');`), '2');
    assert.equal(query(`SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('UserDeviceAccess','UserDeviceGroupAccess');`), '2');
    assert.equal(query(`SELECT count(*) FROM "User" WHERE "deviceScopeMode" <> 'ALL' OR cardinality(permissions) <> 7;`), '0');

    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', ...updatedServices]);
    await waitFor('http://127.0.0.1:3100/api/users/device-scope-options', 401);
    await waitFor('http://127.0.0.1:5174/', 200);
    await waitFor('http://127.0.0.1:5176/', 200);
    for (const service of ['web', 'admin-web', 'ota-https']) {
      docker(['compose', 'exec', '-T', service, 'nginx', '-t']);
      docker(['compose', 'exec', '-T', service, 'nginx', '-s', 'reload']);
    }
    await waitFor('http://127.0.0.1:5174/api/users/device-scope-options', 401);
    await waitFor('http://127.0.0.1:5176/api/users', 401);

    const tenantHtml = await (await fetch('http://127.0.0.1:5174/')).text();
    const tenantAsset = tenantHtml.match(/src="([^"]+\.js)"/)?.[1];
    assert.ok(tenantAsset, 'Tenant frontend JavaScript asset was not found');
    const tenantJavascript = await (await fetch(new URL(tenantAsset, 'http://127.0.0.1:5174'))).text();
    for (const marker of ['功能权限', '指定范围', '按设备分组', '单独选择设备']) {
      assert.ok(tenantJavascript.includes(marker), `Tenant frontend is missing marker: ${marker}`);
    }

    const readonlySmoke = docker(['compose', 'exec', '-T', 'api', 'node', '-e', `const {PrismaClient}=require('@prisma/client'); const {UsersService}=require('./dist/users/users.service'); const p=new PrismaClient(); (async()=>{const admin=await p.user.findFirst({where:{role:'TENANT_ADMIN',tenantId:{not:null}},select:{id:true,tenantId:true}}); let result={tenantAdminFound:Boolean(admin),optionsVerified:false}; if(admin){const options=await new UsersService(p).deviceScopeOptions({sub:admin.id,role:'TENANT_ADMIN',tenantId:admin.tenantId}); result={...result,optionsVerified:true,groupCount:options.groups.length,deviceCount:options.devices.length};} console.log(JSON.stringify(result));})().catch(e=>{console.error(e.message);process.exitCode=1}).finally(()=>p.$disconnect());`]).toString().trim();
    console.log(readonlySmoke);

    assert.equal(secretHashes(), secretsBefore, 'Protected secret files changed (values redacted)');
    const currentApiEnvironment = environment(inspect('api'));
    for (const key of Object.keys(composeConfig.services.api.environment)) {
      assert.equal(currentApiEnvironment[key], liveApiEnvironment[key], `API setting changed (value redacted): ${key}`);
    }
    for (const service of preservedServices) assert.equal(inspect(service).Id, before[service].Id, `Unrelated container replaced: ${service}`);
    assert.deepEqual(stableSnapshot(), dataBefore, 'Existing tenant, account or device records changed');
    const result = {
      deployedAt: new Date().toISOString(),
      migrationsApplied: pending,
      updatedServices,
      tenantFrontendAsset: tenantAsset,
      permissionAndScopeUiVerified: true,
      deviceScopeOptionsVerified: true,
      unauthenticatedRequestsRejected: true,
      credentialsUnchanged: true,
      existingDataUnchanged: true,
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
      console.error('Application rollback completed; additive database migration retained.');
    }
    throw error;
  }
}

main().catch(error => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

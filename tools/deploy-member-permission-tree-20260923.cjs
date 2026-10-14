const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `member-permission-tree-deploy-${stamp}`);
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
const query = sql => docker(['compose', 'exec', '-T', 'postgres', 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'weikong', '-d', 'weikong_iot', '-At', '-c', sql]).toString().trim();
const environment = container => Object.fromEntries(container.Config.Env.map(line => {
  const separator = line.indexOf('=');
  return [line.slice(0, separator), line.slice(separator + 1)];
}));
const secretHashes = () => docker(['compose', 'exec', '-T', 'api', 'node', '-e', `const fs=require('fs'),c=require('crypto'); console.log(JSON.stringify(['jwt-secret','mail-password','emqx-connection-api.json'].map(n=>[n,c.createHash('sha256').update(fs.readFileSync('/var/lib/weikong/secrets/'+n)).digest('hex')])));`]).toString().trim();
const dataSnapshot = () => JSON.parse(query(`SELECT json_build_object(
  'factoryCount',(SELECT count(*) FROM "FactoryGateway"),
  'deviceCount',(SELECT count(*) FROM "Device"),
  'tenantCount',(SELECT count(*) FROM "Tenant"),
  'userCount',(SELECT count(*) FROM "User"),
  'groupsCount',(SELECT count(*) FROM "DeviceGroup"),
  'usersHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'status',status,'permissions',permissions,'deviceScopeMode',"deviceScopeMode")::text,'' ORDER BY id),'')) FROM "User")
);`));

async function waitFor(url, expectedStatus) {
  for (let attempt = 0; attempt < 80; attempt++) {
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
  assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE finished_at IS NULL AND rolled_back_at IS NULL;`), '0', 'Unresolved migration exists');
  const applied = JSON.parse(query(`SELECT COALESCE(json_agg(migration_name),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
  const migrationRoot = path.join(root, 'apps', 'api', 'prisma', 'migrations');
  const localMigrations = fs.readdirSync(migrationRoot).filter(name => fs.existsSync(path.join(migrationRoot, name, 'migration.sql')));
  assert.deepEqual(localMigrations.filter(name => !applied.includes(name)), [], 'Pending database migration exists');

  fs.mkdirSync(out, { recursive: true });
  const secretsBefore = secretHashes();
  const dataBefore = dataSnapshot();
  const rollbackServices = {};
  for (const service of updatedServices) {
    const tag = `wk-member-permission-tree-rollback-${service}:${stamp}`;
    docker(['tag', before[service].Image, tag]);
    rollbackServices[service] = { image: tag };
  }
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: rollbackServices }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ dataBefore, rollbackServices }, null, 2));

  let replacing = false;
  try {
    run(['compose', 'build', ...updatedServices]);
    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', ...updatedServices]);
    await waitFor('http://127.0.0.1:3100/api/users', 401);
    await waitFor('http://127.0.0.1:5174/', 200);
    await waitFor('http://127.0.0.1:5176/', 200);
    for (const service of ['web', 'admin-web', 'ota-https']) {
      docker(['compose', 'exec', '-T', service, 'nginx', '-t']);
      docker(['compose', 'exec', '-T', service, 'nginx', '-s', 'reload']);
    }
    await waitFor('http://127.0.0.1:5174/api/users', 401);
    await waitFor('http://127.0.0.1:5176/api/users', 401);

    const tenantHtml = await (await fetch('http://127.0.0.1:5174/')).text();
    const tenantAsset = tenantHtml.match(/src="([^"]+\.js)"/)?.[1];
    assert.ok(tenantAsset, 'Tenant frontend JavaScript asset was not found');
    const tenantJavascript = await (await fetch(new URL(tenantAsset, 'http://127.0.0.1:5174'))).text();
    for (const marker of ['菜单、页面、按钮', '管理设备分组', '新增与复制模板', '下一步']) {
      assert.ok(tenantJavascript.includes(marker), `Tenant frontend is missing marker: ${marker}`);
    }
    const apiSmoke = docker(['compose', 'exec', '-T', 'api', 'node', '-e', `const p=require('./dist/common/user-permissions'); const result=p.normalizeTenantUserPermissions(['DEVICE_DELETE','MODEL_EDIT','ALARM_RESOLVE']); if(JSON.stringify(result)!==JSON.stringify(['DEVICE_VIEW','DEVICE_DELETE','MODEL_VIEW','MODEL_EDIT','ALARM_VIEW','ALARM_RESOLVE'])) process.exit(2); console.log(JSON.stringify(result));`]).toString().trim();

    assert.equal(secretHashes(), secretsBefore, 'Protected secret files changed (values redacted)');
    assert.deepEqual(dataSnapshot(), dataBefore, 'Existing tenant, account, permission or device records changed');
    const currentApiEnvironment = environment(inspect('api'));
    for (const key of Object.keys(composeConfig.services.api.environment)) assert.equal(currentApiEnvironment[key], liveApiEnvironment[key], `API setting changed (value redacted): ${key}`);
    for (const service of preservedServices) assert.equal(inspect(service).Id, before[service].Id, `Unrelated container replaced: ${service}`);
    for (const service of updatedServices) {
      const state = inspect(service).State;
      assert.equal(state.Running, true, `${service} is not running`);
      if (state.Health) assert.equal(state.Health.Status, 'healthy', `${service} is not healthy`);
    }
    const result = {
      deployedAt: new Date().toISOString(),
      updatedServices,
      migrationsApplied: [],
      tenantFrontendAsset: tenantAsset,
      permissionTreeUiVerified: true,
      actionPermissionApiVerified: true,
      apiSmoke,
      unauthenticatedRequestsRejected: true,
      credentialsUnchanged: true,
      existingDataUnchanged: true,
      unrelatedServicesUnchanged: true,
      rollbackFile,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', ...updatedServices]);
      docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-s', 'reload']);
      console.error('Application rollback completed.');
    }
    throw error;
  }
}

main().catch(error => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `commercial-plan-first-batch-deploy-${stamp}`);
const migration = '20260924170000_commercial_plan_first_batch';
const updatedServices = ['api', 'web', 'admin-web', 'ingest'];
const preservedServices = ['postgres', 'timescaledb', 'redis', 'emqx', 'goview-designer', 'ota-https'];
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
  'tenantCount',(SELECT count(*) FROM "Tenant"),
  'userCount',(SELECT count(*) FROM "User"),
  'deviceCount',(SELECT count(*) FROM "Device"),
  'subscriptionCount',(SELECT count(*) FROM "TenantSubscription"),
  'orderCount',(SELECT count(*) FROM "SubscriptionOrder"),
  'usersHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'tenantId',"tenantId",'username',username,'role',role,'status',status)::text,'' ORDER BY id),'')) FROM "User"),
  'tenantsHash',(SELECT md5(COALESCE(string_agg(jsonb_build_object('id',id,'name',name,'status',status,'subscriptionPlanId',"subscriptionPlanId")::text,'' ORDER BY id),'')) FROM "Tenant")
);`));

async function waitFor(url, expectedStatus) {
  for (let attempt = 0; attempt < 80; attempt++) {
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(3000) });
      if (response.status === expectedStatus) return response;
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(`HTTP verification failed: ${url}`);
}

async function frontendAsset(baseUrl, markers) {
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
  const ingestEnvironmentBefore = environment(before.ingest);
  for (const [key, value] of Object.entries(composeConfig.services.api.environment)) {
    assert.equal(apiEnvironmentBefore[key] ?? '', String(value ?? ''), `API configuration mismatch (value redacted): ${key}`);
  }
  for (const [key, value] of Object.entries(composeConfig.services.ingest.environment)) {
    assert.equal(ingestEnvironmentBefore[key] ?? '', String(value ?? ''), `Ingest configuration mismatch (value redacted): ${key}`);
  }
  assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE finished_at IS NULL AND rolled_back_at IS NULL;`), '0', 'Unresolved migration exists');
  assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='${migration}' AND finished_at IS NOT NULL AND rolled_back_at IS NULL;`), '1', 'Commercial plan migration is not applied');
  const dataBefore = stableSnapshot();
  const secretsBefore = secretHashes();

  fs.mkdirSync(out, { recursive: true });
  const backup = JSON.parse(cp.execFileSync(process.execPath, ['tools/backup-production-batch-deploy.cjs'], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 4 * 1024 * 1024,
  }).trim());
  assert.equal(backup.encryptedBackupVerified, true);
  const rollbackServices = {};
  for (const service of updatedServices) {
    const tag = `wk-commercial-plan-rollback-${service}:${stamp}`;
    docker(['tag', before[service].Image, tag]);
    rollbackServices[service] = { image: tag };
  }
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: rollbackServices }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ dataBefore, databaseBackupDirectory: backup.backupDirectory, rollbackServices }, null, 2));
  console.log(JSON.stringify({ stage: 'backup-verified', out, databaseBackupDirectory: backup.backupDirectory }));

  let replacing = false;
  try {
    run(['compose', 'build', ...updatedServices]);
    run(['compose', 'run', '--rm', '--no-deps', 'api', 'npx', 'prisma', 'migrate', 'deploy']);
    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', 'api', 'web', 'admin-web']);
    await waitFor('http://127.0.0.1:3100/api/subscriptions/plans', 401);
    run(['compose', 'up', '-d', '--no-deps', 'ingest']);
    const tenantAsset = await frontendAsset('http://127.0.0.1:5174', ['商务报价', '云点位', '联系商务']);
    const adminAsset = await frontendAsset('http://127.0.0.1:5176', ['可售周期', '商务报价', '扩容包']);
    for (const service of ['web', 'admin-web', 'ota-https']) docker(['compose', 'exec', '-T', service, 'nginx', '-t']);
    await waitFor('http://127.0.0.1:5174/api/subscriptions/plans', 401);
    await waitFor('http://127.0.0.1:5176/api/subscriptions/plans', 401);

    const catalog = JSON.parse(query(`SELECT json_build_object(
      'plans',(SELECT json_object_agg("code", json_build_object('devices',"deviceLimit",'points',"pointLimit",'scada',"scadaProjectLimit",'users',"userLimit",'annual',"priceCents",'cycles',"billingCycles")) FROM "SubscriptionPlan" WHERE "code" IN ('TRIAL','BASIC','PROFESSIONAL','ENTERPRISE')),
      'addons',(SELECT json_object_agg("code", json_build_object('quantity',quantity,'price',"priceCents",'quoteOnly',"quoteOnly")) FROM "QuotaAddon" WHERE "code" LIKE 'POINT_%' AND enabled=true)
    );`));
    assert.equal(Object.keys(catalog.plans).length, 4);
    assert.equal(Object.keys(catalog.addons).length, 5);
    assert.equal(catalog.addons.POINT_100000_QUOTE.quoteOnly, true);
    assert.deepEqual(catalog.plans.BASIC.cycles, ['YEARLY']);
    assert.deepEqual(stableSnapshot(), dataBefore, 'Accounts, tenants, devices, subscriptions or orders changed during application replacement');
    assert.equal(secretHashes(), secretsBefore, 'Protected secret files changed (values redacted)');
    const currentApiEnvironment = environment(inspect('api'));
    const currentIngestEnvironment = environment(inspect('ingest'));
    for (const key of Object.keys(composeConfig.services.api.environment)) assert.equal(currentApiEnvironment[key], apiEnvironmentBefore[key], `API setting changed (value redacted): ${key}`);
    for (const key of Object.keys(composeConfig.services.ingest.environment)) assert.equal(currentIngestEnvironment[key], ingestEnvironmentBefore[key], `Ingest setting changed (value redacted): ${key}`);
    for (const service of preservedServices) assert.equal(inspect(service).Id, before[service].Id, `Unrelated service was recreated: ${service}`);

    const result = {
      deployedAt: new Date().toISOString(), migration, updatedServices, tenantAsset, adminAsset,
      plansVerified: 4, pointAddonsVerified: 5, quoteOnlyGuardConfigured: true, yearlyOnlyConfigured: true,
      encryptedBackupVerified: true, credentialsUnchanged: true, existingDataUnchanged: true,
      unrelatedServicesUnchanged: true, eg100Updated: false, databaseBackupDirectory: backup.backupDirectory, rollbackFile,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', ...updatedServices]);
      console.error('Application services rolled back; the backward-compatible database migration was retained.');
    }
    throw error;
  }
})().catch(error => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

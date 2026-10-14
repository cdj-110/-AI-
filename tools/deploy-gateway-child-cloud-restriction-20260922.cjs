const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `gateway-child-cloud-restriction-deploy-${stamp}`);
const migration = '20260922110000_persist_gateway_child_cloud_restrictions';
const updatedServices = ['api', 'web', 'admin-web', 'ingest'];
const preservedServices = ['postgres', 'timescaledb', 'redis', 'emqx', 'ota-https'];

const docker = (args) => cp.execFileSync('docker', args, { cwd: root, maxBuffer: 192 * 1024 * 1024 });
const run = (args) => {
  const result = cp.spawnSync('docker', args, { cwd: root, stdio: 'inherit' });
  if (result.status !== 0) throw new Error(`Deployment step failed: docker ${args.slice(0, 4).join(' ')}`);
};
const inspect = (service) => {
  const id = docker(['compose', 'ps', '-q', service]).toString().trim();
  assert.ok(id, `Service is not running: ${service}`);
  return JSON.parse(docker(['inspect', id]).toString())[0];
};
const query = (sql) => docker([
  'compose', 'exec', '-T', 'postgres', 'psql', '-X', '-v', 'ON_ERROR_STOP=1',
  '-U', 'weikong', '-d', 'weikong_iot', '-At', '-c', sql,
]).toString().trim();
const env = (item) => Object.fromEntries(item.Config.Env.map((line) => {
  const index = line.indexOf('=');
  return [line.slice(0, index), line.slice(index + 1)];
}));

async function waitForHttp(url, expectedStatus, attempts = 60) {
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      const response = await fetch(url);
      if (response.status === expectedStatus) return true;
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  return false;
}

async function main() {
  const config = JSON.parse(docker(['compose', 'config', '--format', 'json']).toString());
  const before = Object.fromEntries([...updatedServices, ...preservedServices].map((service) => [service, inspect(service)]));

  const protectedApiKeys = [
    'DATABASE_URL', 'JWT_SECRET', 'FACTORY_CREDENTIAL_ENCRYPTION_KEY',
    'ACTIVATION_SIGNING_PRIVATE_KEY', 'ACTIVATION_SIGNING_KEY_ID',
    'MQTT_INGEST_USERNAME', 'MQTT_INGEST_PASSWORD', 'MQTT_PUBLIC_HOST',
    'REMOTE_ACCESS_PUBLIC_URL', 'OTA_TRUSTED_PUBLIC_KEYS', 'OTA_DOWNLOAD_SECRET',
  ];
  const liveApiEnv = env(before.api);
  for (const key of protectedApiKeys) {
    assert.equal(String(config.services.api.environment[key] ?? ''), liveApiEnv[key] ?? '', `Deployment would change protected API setting: ${key}`);
  }
  const protectedIngestKeys = ['DATABASE_URL', 'TIMESCALE_DATABASE_URL', 'MQTT_URL', 'MQTT_USERNAME', 'MQTT_INGEST_PASSWORD'];
  const liveIngestEnv = env(before.ingest);
  for (const key of protectedIngestKeys) {
    assert.equal(String(config.services.ingest.environment[key] ?? ''), liveIngestEnv[key] ?? '', `Deployment would change protected ingest setting: ${key}`);
  }

  const applied = JSON.parse(query(`SELECT COALESCE(json_agg(json_build_object('name',migration_name,'checksum',checksum)),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
  const migrationRoot = path.join(root, 'apps', 'api', 'prisma', 'migrations');
  const local = fs.readdirSync(migrationRoot).filter((name) => fs.existsSync(path.join(migrationRoot, name, 'migration.sql')));
  for (const item of applied) {
    assert.ok(local.includes(item.name), `Applied migration missing locally: ${item.name}`);
    const checksum = crypto.createHash('sha256').update(fs.readFileSync(path.join(migrationRoot, item.name, 'migration.sql'))).digest('hex');
    assert.equal(checksum, item.checksum, `Historical migration changed: ${item.name}`);
  }
  const pending = local.filter((name) => !applied.some((item) => item.name === name));
  assert.deepEqual(pending, [migration], `Unexpected pending migrations: ${pending.join(', ')}`);

  fs.mkdirSync(out, { recursive: true });
  const backup = JSON.parse(cp.execFileSync('node', ['tools/backup-production-batch-deploy.cjs'], {
    cwd: root, encoding: 'utf8', maxBuffer: 4 * 1024 * 1024,
  }).trim());
  assert.ok(backup.encryptedBackupVerified, 'Database backup verification failed');

  const rollbackServices = {};
  for (const service of updatedServices) {
    const tag = `wk-child-restriction-rollback-${service}:${stamp}`;
    docker(['tag', before[service].Image, tag]);
    rollbackServices[service] = { image: tag };
  }
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: rollbackServices }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({
    databaseBackupDirectory: backup.backupDirectory,
    databaseBackupVerified: backup.encryptedBackupVerified,
    pendingMigrations: pending,
    rollbackServices,
    preservedContainerIds: Object.fromEntries(preservedServices.map((service) => [service, before[service].Id])),
  }, null, 2));
  console.log(JSON.stringify({ stage: 'backup-verified', out, pendingMigrations: pending, credentialsUnchanged: true }));

  let replacing = false;
  try {
    run(['compose', 'build', ...updatedServices]);
    run(['compose', 'run', '--rm', '--no-deps', 'api', 'npx', 'prisma', 'migrate', 'deploy']);
    assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='${migration}' AND finished_at IS NOT NULL AND rolled_back_at IS NULL;`), '1');
    assert.equal(query(`SELECT to_regclass('public."GatewayChildCloudRestriction"') IS NOT NULL;`), 't');
    const missingBackfill = query(`SELECT count(*) FROM "Device" d WHERE d."deviceType"='GATEWAY_CHILD' AND d."gatewayId" IS NOT NULL AND d."cloudAccessEnabled"=false AND NOT EXISTS (SELECT 1 FROM "GatewayChildCloudRestriction" r WHERE r."gatewayDeviceId"=d."gatewayId" AND r."childDeviceKey"=d."deviceKey");`);
    assert.equal(missingBackfill, '0', 'Existing disabled child devices were not fully backfilled');

    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', 'api', 'web', 'admin-web']);
    assert.ok(await waitForHttp('http://127.0.0.1:3100/api/devices', 401), 'API did not recover');
    assert.ok(await waitForHttp('http://127.0.0.1:5174/', 200), 'Tenant frontend did not recover');
    assert.ok(await waitForHttp('http://127.0.0.1:5176/', 200), 'Admin frontend did not recover');
    run(['compose', 'up', '-d', '--no-deps', 'ingest']);

    let ingestionReady = false;
    for (let attempt = 0; attempt < 60; attempt += 1) {
      const logs = docker(['compose', 'logs', '--since', '3m', '--tail', '200', 'ingest']).toString();
      if (inspect('ingest').State.Running && logs.includes('[ingest] subscribed to all MQTT ingestion topics')) {
        ingestionReady = true;
        break;
      }
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
    assert.ok(ingestionReady, 'Ingest did not restore MQTT subscriptions');

    const currentApi = inspect('api');
    const currentApiEnv = env(currentApi);
    for (const key of protectedApiKeys) assert.equal(currentApiEnv[key] ?? '', liveApiEnv[key] ?? '', `Protected API setting changed: ${key}`);
    const currentIngest = inspect('ingest');
    const currentIngestEnv = env(currentIngest);
    for (const key of protectedIngestKeys) assert.equal(currentIngestEnv[key] ?? '', liveIngestEnv[key] ?? '', `Protected ingest setting changed: ${key}`);
    for (const service of preservedServices) assert.equal(inspect(service).Id, before[service].Id, `Unrelated service was recreated: ${service}`);

    const after = JSON.parse(query(`SELECT json_build_object('factoryCount',(SELECT count(*) FROM "FactoryGateway"),'deviceCount',(SELECT count(*) FROM "Device"),'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t));`));
    for (const key of ['factoryCount', 'deviceCount', 'usersHash', 'tenantsHash']) assert.equal(after[key], backup.before[key], `Preserved information changed: ${key}`);
    docker(['exec', inspect('web').Id, 'nginx', '-t']);
    docker(['exec', inspect('admin-web').Id, 'nginx', '-t']);

    const result = {
      deployedAt: new Date().toISOString(),
      migrationsApplied: pending,
      databaseSchemaUpToDate: true,
      apiUpdated: true,
      ingestUpdated: true,
      tenantFrontendUpdated: true,
      adminFrontendUpdated: true,
      disabledChildRestrictionsBackfilled: true,
      credentialsUnchanged: true,
      accountsUnchanged: true,
      tenantsUnchanged: true,
      deviceCountUnchanged: true,
      unrelatedServicesUnchanged: true,
      physicalPointWrites: false,
      backup: out,
      databaseBackupDirectory: backup.backupDirectory,
      rollbackServices,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', ...updatedServices]);
      console.error('Application services rolled back. The additive database migration was retained; no existing data was removed.');
    }
    throw error;
  }
}

main().catch((error) => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

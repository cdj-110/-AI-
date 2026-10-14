const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `license-hardware-profile-deploy-${stamp}`);
const migration = '20260922130000_complete_eg100_hardware_profile';
const preservedServices = ['postgres', 'timescaledb', 'redis', 'emqx', 'ota-https', 'ingest', 'web', 'admin-web'];

const docker = (args) => cp.execFileSync('docker', args, { cwd: root, maxBuffer: 192 * 1024 * 1024 });
const run = (args) => {
  const result = cp.spawnSync('docker', args, { cwd: root, stdio: 'inherit' });
  if (result.status !== 0) throw new Error(`Deployment step failed: docker ${args.slice(0, 5).join(' ')}`);
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

async function waitForHttp(url, expectedStatus, attempts = 90) {
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
  const beforeApi = inspect('api');
  const beforePreserved = Object.fromEntries(preservedServices.map((service) => [service, inspect(service)]));
  const protectedKeys = [
    'DATABASE_URL', 'JWT_SECRET', 'FACTORY_CREDENTIAL_ENCRYPTION_KEY',
    'ACTIVATION_SIGNING_PRIVATE_KEY', 'ACTIVATION_SIGNING_KEY_ID',
    'MQTT_INGEST_USERNAME', 'MQTT_INGEST_PASSWORD', 'MQTT_PUBLIC_HOST',
    'REMOTE_ACCESS_PUBLIC_URL', 'OTA_TRUSTED_PUBLIC_KEYS', 'OTA_DOWNLOAD_SECRET',
  ];
  const liveEnv = env(beforeApi);
  for (const key of protectedKeys) {
    assert.equal(String(config.services.api.environment[key] ?? ''), liveEnv[key] ?? '', `Deployment would change protected API setting: ${key}`);
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

  const rollbackTag = `wk-license-hardware-profile-api-rollback:${stamp}`;
  docker(['tag', beforeApi.Image, rollbackTag]);
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: { api: { image: rollbackTag } } }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({
    databaseBackupDirectory: backup.backupDirectory,
    databaseBackupVerified: true,
    pendingMigrations: pending,
    rollbackImage: rollbackTag,
    preservedContainerIds: Object.fromEntries(Object.entries(beforePreserved).map(([key, item]) => [key, item.Id])),
  }, null, 2));
  console.log(JSON.stringify({ stage: 'backup-verified', out, pendingMigrations: pending }));

  let replacing = false;
  try {
    run(['compose', 'build', 'api']);
    run(['compose', 'run', '--rm', '--no-deps', 'api', 'npx', 'prisma', 'migrate', 'deploy']);
    assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='${migration}' AND finished_at IS NOT NULL AND rolled_back_at IS NULL;`), '1');
    const profile = JSON.parse(query(`SELECT json_build_object('ethernetPortCount',"ethernetPortCount",'serialPortCount',"serialPortCount",'rs232PortCount',"rs232PortCount",'rs485PortCount',"rs485PortCount",'canPortCount',"canPortCount",'supports4G',"supports4G",'supportsWifi',"supportsWifi") FROM "ProductModel" WHERE "code"='EG100';`));
    assert.deepEqual(profile, { ethernetPortCount: 2, serialPortCount: 4, rs232PortCount: 2, rs485PortCount: 2, canPortCount: 0, supports4G: true, supportsWifi: true });

    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', 'api']);
    assert.ok(await waitForHttp('http://127.0.0.1:3100/api/devices', 401), 'API did not recover');

    const currentApi = inspect('api');
    const currentEnv = env(currentApi);
    for (const key of protectedKeys) assert.equal(currentEnv[key] ?? '', liveEnv[key] ?? '', `Protected API setting changed: ${key}`);
    for (const [service, item] of Object.entries(beforePreserved)) assert.equal(inspect(service).Id, item.Id, `Unrelated service was recreated: ${service}`);

    const after = JSON.parse(query(`SELECT json_build_object('factoryCount',(SELECT count(*) FROM "FactoryGateway"),'deviceCount',(SELECT count(*) FROM "Device"),'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t));`));
    for (const key of ['factoryCount', 'deviceCount', 'usersHash', 'tenantsHash']) assert.equal(after[key], backup.before[key], `Preserved information changed: ${key}`);

    const result = {
      deployedAt: new Date().toISOString(), migrationsApplied: pending,
      databaseSchemaUpToDate: true, apiUpdated: true, eg100ProductProfile: profile,
      credentialsUnchanged: true, accountsUnchanged: true, tenantsUnchanged: true,
      deviceCountUnchanged: true, unrelatedServicesUnchanged: true,
      databaseBackupDirectory: backup.backupDirectory, rollbackImage: rollbackTag, evidence: out,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', 'api']);
      console.error('API rolled back. The additive product-profile migration was retained.');
    }
    throw error;
  }
}

main().catch((error) => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

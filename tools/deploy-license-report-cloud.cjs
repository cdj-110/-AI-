const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..');
const docker = args => cp.execFileSync('docker', args, { cwd: root, maxBuffer: 192 * 1024 * 1024 });
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const config = JSON.parse(docker(['compose', 'config', '--format', 'json']));
const current = JSON.parse(docker(['inspect', 'weikong-iot-platform-api-1']))[0];
const live = Object.fromEntries(current.Config.Env.map(value => { const split = value.indexOf('='); return [value.slice(0, split), value.slice(split + 1)]; }));
for (const key of ['DATABASE_URL', 'JWT_SECRET', 'FACTORY_CREDENTIAL_ENCRYPTION_KEY', 'ACTIVATION_SIGNING_PRIVATE_KEY', 'ACTIVATION_SIGNING_KEY_ID', 'MQTT_PUBLIC_HOST', 'REMOTE_ACCESS_PUBLIC_URL']) {
  assert.equal(String(config.services.api.environment[key] ?? ''), live[key] ?? '', `Deployment would unexpectedly change ${key}`);
}
assert.ok(live.ACTIVATION_SIGNING_PRIVATE_KEY && live.FACTORY_CREDENTIAL_ENCRYPTION_KEY);
const rollbackImages = {};
const rollbackDirectory = path.join(root, 'artifacts', `license-report-runtime-backup-${stamp}`);
fs.mkdirSync(rollbackDirectory, { recursive: true });
for (const service of ['api', 'ingest', 'admin-web', 'web']) {
  const item = JSON.parse(docker(['inspect', `weikong-iot-platform-${service}-1`]))[0];
  const tag = `wk-license-report-rollback-${service}:${stamp}`;
  const found = cp.spawnSync('docker', ['image', 'inspect', item.Image], { cwd: root, stdio: 'ignore' });
  if (found.status === 0) docker(['tag', item.Image, tag]);
  else {
    // Missing image content can also prevent commit/export. Back up only static
    // runtime paths: no .env, database data or injected environment secrets.
    const paths = service === 'api' ? ['/app/dist', '/app/prisma', '/app/node_modules', '/app/package.json', '/app/package-lock.json']
      : service === 'ingest' ? ['/app/dist', '/app/node_modules', '/app/package.json', '/app/package-lock.json']
      : ['/usr/share/nginx/html', '/etc/nginx/conf.d/default.conf'];
    const archive = docker(['exec', item.Id, 'tar', '-czf', '-', ...paths]);
    assert.equal(archive[0], 0x1f); assert.equal(archive[1], 0x8b);
    const location = path.join(rollbackDirectory, `${service}.tar.gz`);
    fs.writeFileSync(location, archive, { flag: 'wx' });
    rollbackImages[service] = { runtimeArchive: location, oldImage: item.Image };
    continue;
  }
  rollbackImages[service] = tag;
}
console.log(JSON.stringify({ stage: 'deployment-preflight', credentialsUnchanged: true, rollbackImages }));
docker(['compose', 'run', '--rm', '--no-deps', 'api', 'npx', 'prisma', 'migrate', 'deploy']);
console.log('License reporting migration applied.');
docker(['compose', 'up', '-d', '--no-deps', 'api', 'ingest', 'admin-web', 'web']);
const out = path.join(root, 'artifacts/license-report-deployment-20260917');
fs.mkdirSync(out, { recursive: true });
fs.writeFileSync(path.join(out, 'cloud-deployment.json'), JSON.stringify({ deployedAt: new Date().toISOString(), migration: '20260917160000_license_reporting', rollbackImages, credentialsUnchanged: true }, null, 2));
console.log('Cloud services updated; rollback image references saved.');

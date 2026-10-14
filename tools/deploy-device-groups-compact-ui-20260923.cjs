const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `device-groups-compact-ui-deploy-${stamp}`);
const preservedServices = ['api', 'admin-web', 'postgres', 'timescaledb', 'redis', 'emqx', 'ingest', 'ota-https'];
const docker = args => cp.execFileSync('docker', args, { cwd: root, maxBuffer: 128 * 1024 * 1024 });
const run = args => {
  const result = cp.spawnSync('docker', args, { cwd: root, stdio: 'inherit' });
  assert.equal(result.status, 0, `Deployment step failed: docker ${args.join(' ')}`);
};
const inspect = service => {
  const id = docker(['compose', 'ps', '-q', service]).toString().trim();
  assert.ok(id && !id.includes('\n'), `Missing service: ${service}`);
  return JSON.parse(docker(['inspect', id]))[0];
};
async function waitFor(url, expectedStatus = 200) {
  for (let attempt = 0; attempt < 30; attempt++) {
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(3000) });
      if (response.status === expectedStatus) return response;
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(`Health verification failed: ${url}`);
}

(async () => {
  const preserved = Object.fromEntries(preservedServices.map(service => [service, inspect(service).Id]));
  const previous = inspect('web');
  const rollbackTag = `wk-device-groups-compact-ui-rollback:${stamp}`;
  fs.mkdirSync(out, { recursive: true });
  docker(['tag', previous.Image, rollbackTag]);
  const archive = docker(['exec', previous.Id, 'tar', '-czf', '-', '/usr/share/nginx/html', '/etc/nginx/conf.d/default.conf']);
  assert.equal(archive[0], 0x1f, 'Frontend backup is not gzip data');
  assert.equal(archive[1], 0x8b, 'Frontend backup is not gzip data');
  fs.writeFileSync(path.join(out, 'web.original.tar.gz'), archive, { flag: 'wx' });
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: { web: { image: rollbackTag } } }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ oldContainerId: previous.Id, oldImage: previous.Image, rollbackTag, preserved }, null, 2));
  console.log(JSON.stringify({ stage: 'backup-verified', out, rollbackTag }));

  let replacing = false;
  try {
    run(['compose', 'build', 'web']);
    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', 'web']);
    const current = inspect('web');
    assert.equal(current.State.Running, true, 'Tenant frontend is not running');
    docker(['exec', current.Id, 'nginx', '-t']);

    // The HTTPS proxy resolves the Compose service name at reload time.
    docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-t']);
    docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-s', 'reload']);

    const response = await waitFor('http://127.0.0.1:5174/');
    const html = await response.text();
    const assetPath = html.match(/src="([^"]+\.js)"/)?.[1];
    assert.ok(assetPath, 'Tenant frontend JavaScript asset was not found');
    const assetResponse = await waitFor(new URL(assetPath, 'http://127.0.0.1:5174').toString());
    const asset = await assetResponse.text();
    assert.ok(asset.includes('搜索分组') && asset.includes('调整分组') && asset.includes('设备分组'), 'New compact device-group UI is missing from deployed assets');
    await waitFor('http://127.0.0.1:5174/api/device-groups', 401);

    for (const [service, id] of Object.entries(preserved)) {
      assert.equal(inspect(service).Id, id, `Unrelated service was recreated: ${service}`);
    }
    const result = {
      deployedAt: new Date().toISOString(),
      service: 'web',
      oldContainerId: previous.Id,
      newContainerId: current.Id,
      oldImage: previous.Image,
      newImage: current.Image,
      frontendAsset: assetPath,
      compactDeviceGroupUiVerified: true,
      unauthenticatedApiRejected: true,
      unrelatedServicesUnchanged: true,
      migrationRequired: false,
      eg100Updated: false,
      backupDirectory: out,
      rollbackFile,
      rollbackTag,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', 'web']);
      docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-s', 'reload']);
      console.error('Tenant frontend rolled back to the verified previous image.');
    }
    throw error;
  }
})().catch(error => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

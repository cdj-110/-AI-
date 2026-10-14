const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `member-device-transfer-ui-deploy-${stamp}`);
const updatedServices = ['web', 'admin-web'];
const preservedServices = ['api', 'postgres', 'timescaledb', 'redis', 'emqx', 'ingest', 'ota-https'];
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
async function waitFor(url, expectedStatus = 200) {
  for (let attempt = 0; attempt < 50; attempt++) {
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(3000) });
      if (response.status === expectedStatus) return response;
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(`Health verification failed: ${url}`);
}
async function verifyFrontend(baseUrl, markers) {
  const html = await (await waitFor(`${baseUrl}/`)).text();
  const assetPath = html.match(/src="([^"]+\.js)"/)?.[1];
  assert.ok(assetPath, `Frontend JavaScript asset was not found: ${baseUrl}`);
  const asset = await (await waitFor(new URL(assetPath, baseUrl).toString())).text();
  for (const marker of markers) assert.ok(asset.includes(marker), `Deployed asset is missing marker: ${marker}`);
  return assetPath;
}

(async () => {
  const before = Object.fromEntries([...updatedServices, ...preservedServices].map(service => [service, inspect(service)]));
  fs.mkdirSync(out, { recursive: true });
  const rollbackServices = {};
  for (const service of updatedServices) {
    const tag = `wk-member-device-transfer-rollback-${service}:${stamp}`;
    docker(['tag', before[service].Image, tag]);
    rollbackServices[service] = { image: tag };
  }
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: rollbackServices }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ rollbackServices, containerIds: Object.fromEntries(Object.entries(before).map(([key, value]) => [key, value.Id])) }, null, 2));

  let replacing = false;
  try {
    run(['compose', 'build', ...updatedServices]);
    replacing = true;
    run(['compose', 'up', '-d', '--no-deps', ...updatedServices]);
    for (const service of updatedServices) {
      const current = inspect(service);
      assert.equal(current.State.Running, true, `${service} is not running`);
      docker(['exec', current.Id, 'nginx', '-t']);
    }
    docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-t']);
    docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-s', 'reload']);

    const markers = ['未添加设备', '已添加设备', '支持拖拽', '按设备分组授权', '未命名设备'];
    const tenantAsset = await verifyFrontend('http://127.0.0.1:5174', markers);
    const adminAsset = await verifyFrontend('http://127.0.0.1:5176', markers);
    await waitFor('http://127.0.0.1:5174/api/users', 401);
    await waitFor('http://127.0.0.1:5176/api/users', 401);
    for (const service of preservedServices) assert.equal(inspect(service).Id, before[service].Id, `Unrelated service was recreated: ${service}`);

    const result = {
      deployedAt: new Date().toISOString(),
      updatedServices,
      tenantAsset,
      adminAsset,
      transferButtonsVerified: true,
      dragAndDropUiVerified: true,
      unauthenticatedApiRejected: true,
      unrelatedServicesUnchanged: true,
      migrationRequired: false,
      rollbackFile,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replacing) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', ...updatedServices]);
      docker(['compose', 'exec', '-T', 'ota-https', 'nginx', '-s', 'reload']);
      console.error('Frontend rollback completed.');
    }
    throw error;
  }
})().catch(error => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

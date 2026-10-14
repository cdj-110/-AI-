const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const assert = require('node:assert/strict');

const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const out = path.join(root, 'artifacts', `commercial-status-filter-header-deploy-${stamp}`);
const docker = args => cp.execFileSync('docker', args, { cwd: root, maxBuffer: 128 * 1024 * 1024 });
const inspect = service => {
  const id = docker(['compose', 'ps', '-q', service]).toString().trim();
  assert.ok(id, `Missing service: ${service}`);
  return JSON.parse(docker(['inspect', id]))[0];
};
const run = args => {
  const result = cp.spawnSync('docker', args, { cwd: root, stdio: 'inherit' });
  assert.equal(result.status, 0, `Deployment failed: docker ${args.join(' ')}`);
};
async function waitFor(url, status) {
  for (let attempt = 0; attempt < 60; attempt++) {
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(3000) });
      if (response.status === status) return response;
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(`HTTP verification failed: ${url}`);
}

(async () => {
  fs.mkdirSync(out, { recursive: true });
  const previous = inspect('admin-web');
  const preserved = Object.fromEntries(
    ['api', 'ingest', 'web', 'postgres', 'timescaledb', 'redis', 'emqx', 'goview-designer', 'ota-https']
      .map(service => [service, inspect(service).Id]),
  );
  const archive = docker(['exec', previous.Id, 'tar', '-czf', '-', '/usr/share/nginx/html', '/etc/nginx/conf.d/default.conf']);
  assert.equal(archive[0], 0x1f);
  assert.equal(archive[1], 0x8b);
  fs.writeFileSync(path.join(out, 'admin-web.original.tar.gz'), archive, { flag: 'wx' });
  const rollbackTag = `wk-commercial-status-filter-header-rollback:${stamp}`;
  docker(['tag', previous.Image, rollbackTag]);
  const rollbackFile = path.join(out, 'compose.rollback.json');
  fs.writeFileSync(rollbackFile, JSON.stringify({ services: { 'admin-web': { image: rollbackTag } } }, null, 2));
  fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ previousImage: previous.Image, rollbackTag, rollbackFile, preserved }, null, 2));
  let replaced = false;
  try {
    run(['compose', 'build', 'admin-web']);
    replaced = true;
    run(['compose', 'up', '-d', '--no-deps', 'admin-web']);
    const current = inspect('admin-web');
    assert.equal(current.State.Running, true);
    docker(['exec', current.Id, 'nginx', '-t']);
    const html = await (await waitFor('http://127.0.0.1:5176/', 200)).text();
    const assetPath = html.match(/src="([^"]+\.js)"/)?.[1];
    assert.ok(assetPath, 'Admin JavaScript asset was not found');
    const javascript = await (await waitFor(new URL(assetPath, 'http://127.0.0.1:5176').toString(), 200)).text();
    for (const marker of ['quota-addons', '扩容包管理', '全部状态', '按套餐状态筛选', '按销售状态筛选']) {
      assert.ok(javascript.includes(marker), `Missing production marker: ${marker}`);
    }
    assert.equal((await waitFor('http://127.0.0.1:5176/quota-addons', 200)).status, 200);
    assert.equal((await waitFor('http://127.0.0.1:5176/api/subscriptions/plans', 401)).status, 401);
    for (const [service, id] of Object.entries(preserved)) {
      assert.equal(inspect(service).Id, id, `Unrelated service was recreated: ${service}`);
    }
    const result = {
      deployedAt: new Date().toISOString(),
      service: 'admin-web',
      assetPath,
      defaultAllStatusesVerified: true,
      headerStatusFiltersVerified: true,
      unrelatedServicesUnchanged: true,
      rollbackTag,
      backupDirectory: out,
    };
    fs.writeFileSync(path.join(out, 'deployment.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } catch (error) {
    if (replaced) {
      run(['compose', '-f', 'docker-compose.yml', '-f', rollbackFile, 'up', '-d', '--no-deps', '--no-build', 'admin-web']);
    }
    throw error;
  }
})().catch(error => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});

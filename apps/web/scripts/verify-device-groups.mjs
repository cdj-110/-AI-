import assert from 'node:assert/strict';
import { createServer } from 'vite';
import vue from '@vitejs/plugin-vue';
import { mkdir } from 'node:fs/promises';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const server = await createServer({ configFile: false, plugins: [vue()], optimizeDeps: { entries: ['scripts/fixtures/device-groups.html'] }, server: { host: '127.0.0.1', port: 0 }, logLevel: 'error' });
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true, channel: 'msedge' });
  const page = await browser.newPage({ viewport: { width: 1500, height: 1000 } });
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  let groups = [{ id: 'g1', name: '一号车间' }, { id: 'g2', name: '办公区' }];
  const devices = Array.from({ length: 12 }, (_, i) => ({ id: 'd' + i, deviceKey: 'WK-TEST-' + i, name: i === 0 ? '车间网关' : '传感器' + i, tenantId: 'test-tenant', tenant: { id: 'test-tenant', name: '测试租户' }, deviceType: i === 0 ? 'GATEWAY' : 'GATEWAY_CHILD', gatewayId: i === 0 ? null : 'd0', protocol: 'MQTT', status: 'ONLINE', cloudAccessEnabled: true, groupId: i < 2 ? 'g1' : null }));
  let assignments = [], failAssign = false;
  await page.route('**/api/**', async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname;
    if (!path.startsWith('/api/')) return route.continue();
    const payload = request.method() === 'GET' ? null : request.postDataJSON();
    let data;
    if (path === '/api/device-groups' && request.method() === 'GET') data = { items: groups.map(g => ({ ...g, _count: { devices: devices.filter(d => d.groupId === g.id).length } })), totalDevices: devices.length, ungroupedDevices: devices.filter(d => !d.groupId).length };
    else if (path === '/api/device-groups' && request.method() === 'POST') {
      if (groups.some(g => g.name === payload.name)) return route.fulfill({ status: 409, json: { message: '该租户已存在同名分组' } });
      data = { id: 'g' + (groups.length + 1), name: payload.name }; groups.push(data);
    } else if (path === '/api/device-groups/membership') {
      if (failAssign) return route.fulfill({ status: 409, json: { message: '设备已发生变化，请刷新后重试' } });
      assignments.push(payload);
      for (const d of devices) if (payload.deviceIds.includes(d.id)) d.groupId = payload.groupId;
      data = { count: payload.deviceIds.length };
    } else if (path.startsWith('/api/device-groups/')) {
      const id = path.split('/').at(-1);
      if (request.method() === 'DELETE') { groups = groups.filter(g => g.id !== id); for (const d of devices) if (d.groupId === id) d.groupId = null; data = {}; }
      else { data = groups.find(g => g.id === id); data.name = payload.name; }
    } else if (path === '/api/devices') {
      const groupId = url.searchParams.get('groupId'), keyword = url.searchParams.get('keyword'), type = url.searchParams.get('deviceType');
      const rows = devices.filter(d => (!groupId || (groupId === 'ungrouped' ? !d.groupId : d.groupId === groupId)) && (!keyword || d.name.includes(keyword) || d.deviceKey.includes(keyword)) && (!type || d.deviceType === type));
      const offset = (Number(url.searchParams.get('page') || 1) - 1) * 10;
      data = { items: rows.slice(offset, offset + 10).map(d => ({ ...d, group: groups.find(g => g.id === d.groupId) ?? null })), total: rows.length };
    } else throw Error('Unexpected request ' + path);
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  const url = `http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/device-groups.html`;
  const table = page.locator('.el-table__body-wrapper');
  const groupPopover = page.locator('.device-group-popover');
  const groupTrigger = () => page.getByRole('button', { name: /设备分组：/ });
  const openGroups = async () => {
    if (!await groupPopover.isVisible().catch(() => false)) await groupTrigger().click();
    await groupPopover.waitFor();
    await page.waitForFunction(() => {
      const popper = document.querySelector('.device-group-popover')?.closest('.el-popper');
      return popper && getComputedStyle(popper).opacity === '1';
    });
  };
  const groupButton = name => groupPopover.locator('button.group-item').filter({ hasText: name });
  const chooseGroup = async name => { await openGroups(); await groupButton(name).click(); };
  await page.goto(url); await table.getByRole('link', { name: '车间网关', exact: true }).waitFor();
  await chooseGroup('一号车间'); await page.waitForFunction(() => document.querySelectorAll('.el-table__body-wrapper tbody tr').length === 2);
  await page.getByRole('button', { name: '设备分组：一号车间', exact: true }).waitFor();
  await openGroups();
  await groupPopover.getByRole('button', { name: '新建分组', exact: true }).click();
  let dialog = page.getByRole('dialog', { name: '新建设备分组', exact: true });
  await dialog.getByLabel('分组名称', { exact: true }).fill('一号车间'); await dialog.getByRole('button', { name: '保存', exact: true }).click();
  await page.getByText('该租户已存在同名分组', { exact: true }).waitFor(); assert.equal(await dialog.isVisible(), true);
  await dialog.getByLabel('分组名称', { exact: true }).fill('仓储区'); await dialog.getByRole('button', { name: '保存', exact: true }).click();
  await dialog.waitFor({ state: 'hidden' }); await openGroups(); await groupButton('仓储区').waitFor();
  await groupButton('仓储区').click();
  await openGroups(); await groupPopover.getByRole('button', { name: '重命名仓储区', exact: true }).click();
  dialog = page.getByRole('dialog', { name: '重命名分组', exact: true });
  await dialog.getByLabel('分组名称', { exact: true }).fill('仓库'); await dialog.getByRole('button', { name: '保存', exact: true }).click();
  await dialog.waitFor({ state: 'hidden' }); await openGroups(); await groupButton('仓库').waitFor();
  await groupButton('一号车间').click(); await table.getByRole('link', { name: '车间网关', exact: true }).waitFor();
  await table.locator('tbody tr').nth(0).locator('.el-checkbox').click();
  await table.locator('tbody tr').nth(1).locator('.el-checkbox').click();
  await page.getByRole('button', { name: '调整分组（2）', exact: true }).click();
  dialog = page.getByRole('dialog', { name: '调整设备分组', exact: true });
  await dialog.locator('.el-select').click(); await page.getByRole('option', { name: '仓库', exact: true }).click();
  failAssign = true; await dialog.getByRole('button', { name: '确定', exact: true }).click();
  await page.getByText('设备已发生变化，请刷新后重试', { exact: true }).waitFor(); assert.equal(await dialog.isVisible(), true); assert.equal(assignments.length, 0);
  failAssign = false; await dialog.getByRole('button', { name: '确定', exact: true }).click(); await dialog.waitFor({ state: 'hidden' });
  assert.deepEqual(assignments[0], { deviceIds: ['d0', 'd1'], groupId: 'g3' });
  await chooseGroup('仓库'); await table.getByRole('link', { name: '车间网关', exact: true }).waitFor();
  await mkdir('artifacts/device-groups', { recursive: true });
  await page.waitForFunction(() => !document.querySelector('.el-message'));
  await page.screenshot({ path: 'artifacts/device-groups/desktop.png', fullPage: true });
  await openGroups();
  await page.screenshot({ path: 'artifacts/device-groups/dropdown.png' });
  await groupPopover.getByRole('button', { name: '删除仓库', exact: true }).click();
  let confirm = page.getByRole('dialog', { name: '删除分组', exact: true });
  await confirm.getByRole('button', { name: '取消', exact: true }).click(); assert.ok(groups.some(g => g.id === 'g3'));
  await openGroups();
  await groupPopover.getByRole('button', { name: '删除仓库', exact: true }).click();
  confirm = page.getByRole('dialog', { name: '删除分组', exact: true });
  await confirm.getByRole('button', { name: '删除分组', exact: true }).click(); await confirm.waitFor({ state: 'hidden' });
  await page.getByRole('button', { name: '设备分组：未分组', exact: true }).waitFor();
  assert.equal(devices.length, 12); assert.equal(devices[1].gatewayId, 'd0');
  await page.getByPlaceholder('搜索设备名称或编号').fill('传感器11'); await page.getByRole('button', { name: '查询', exact: true }).click();
  await page.waitForFunction(() => document.querySelectorAll('.el-table__body-wrapper tbody tr').length === 1);
  assert.equal(await table.getByRole('link', { name: '传感器11', exact: true }).count(), 1);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForFunction(() => !document.querySelector('.el-message'));
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), false);
  await page.screenshot({ path: 'artifacts/device-groups/mobile.png', fullPage: true });
  await page.goto(url + '?readonly'); await openGroups(); await groupButton('全部设备').waitFor();
  assert.equal(await page.getByRole('button', { name: '新建分组', exact: true }).count(), 0);
  assert.equal(await page.getByRole('button', { name: /调整分组/ }).count(), 0);
  await groupButton('办公区').click(); await openGroups(); assert.equal(await page.getByRole('button', { name: /重命名办公区|删除办公区/ }).count(), 0);
  assert.deepEqual(errors, []);
  console.log('PASS device groups UI: create/rename, duplicate/error recovery, filter/search, bulk assignment, safe delete/cancel, readonly roles and responsive layout.');
} finally { await browser?.close(); await server.close(); }

import assert from 'node:assert/strict';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const base = process.env.IDENTITY_PREVIEW_URL || 'http://127.0.0.1:5196';
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  const writes = [];
  const user = { id: 'product-ui-test', username: 'test-admin', role: 'SUPER_ADMIN', tenantId: null };
  const products = [{ code: 'EG100', name: 'EG100 边缘网关', ethernetPortCount: 2, serialPortCount: 4, rs232PortCount: 2, rs485PortCount: 2, canPortCount: null, supports4G: true, supportsWifi: true, enabled: true, description: 'CAN 数量待确认', _count: { gateways: 1 } }];
  products.push({ code: 'LEGACY', name: '历史规格', ethernetPortCount: null, serialPortCount: 8, rs232PortCount: null, rs485PortCount: null, canPortCount: null, supports4G: null, supportsWifi: null, enabled: false, description: null, _count: { gateways: 0 } });
  await page.addInitScript(user => { localStorage.setItem('accessToken', 'test-only-token'); localStorage.setItem('userInfo', JSON.stringify(user)); }, user);
  page.on('pageerror', error => errors.push(error.message));
  // All writes are intercepted; this test never touches real product records.
  await page.route('**/api/**', async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    let data = { items: [], total: 0 };
    if (path === '/api/auth/profile') data = user;
    if (path === '/api/products') {
      if (request.method() === 'POST') {
        const body = request.postDataJSON(); writes.push({ method: 'POST', body });
        data = { ...body, _count: { gateways: 0 } }; products.push(data);
      } else data = products;
    }
    if (path.startsWith('/api/products/')) {
      const code = decodeURIComponent(path.split('/').at(-1));
      const index = products.findIndex(product => product.code === code);
      if (request.method() === 'PATCH') {
        const body = request.postDataJSON(); writes.push({ method: 'PATCH', body });
        Object.assign(products[index], body); data = products[index];
      } else if (request.method() === 'DELETE') { writes.push({ method: 'DELETE', code }); data = products.splice(index, 1)[0]; }
    }
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto(`${base}/products`);
  const table = page.locator('.el-table');
  const eg100 = table.getByRole('row').filter({ hasText: 'EG100 边缘网关' });
  await eg100.waitFor();
  await table.getByText('历史串口总数 8，接口类型待确认', { exact: true }).waitFor();
  assert.equal(await eg100.getByText('待确认', { exact: true }).count(), 1);
  assert.equal(await eg100.getByText('2 路', { exact: true }).count(), 2);
  assert.equal(await eg100.getByText('支持', { exact: true }).count(), 2);
  assert.ok(await eg100.getByRole('button', { name: '删除', exact: true }).isDisabled());
  await page.getByRole('button', { name: '新增产品型号', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('textbox', { name: '型号编码', exact: true }).fill('EG200');
  await dialog.getByRole('textbox', { name: '产品名称', exact: true }).fill('EG200 测试网关');
  await dialog.getByRole('spinbutton', { name: '网口数量', exact: true }).fill('4');
  await dialog.getByRole('spinbutton', { name: 'RS232 数量', exact: true }).fill('2');
  await dialog.getByRole('spinbutton', { name: 'RS485 数量', exact: true }).fill('4');
  await dialog.getByRole('spinbutton', { name: 'CAN 数量', exact: true }).fill('1');
  await dialog.getByRole('combobox', { name: '4G 支持', exact: true }).focus();
  await dialog.getByRole('combobox', { name: '4G 支持', exact: true }).press('ArrowDown');
  const cellularList = await dialog.getByRole('combobox', { name: '4G 支持', exact: true }).getAttribute('aria-controls');
  await page.locator(`[id="${cellularList}"]`).getByRole('option', { name: '支持', exact: true }).click();
  await dialog.getByRole('combobox', { name: 'WiFi 支持', exact: true }).focus();
  await dialog.getByRole('combobox', { name: 'WiFi 支持', exact: true }).press('ArrowDown');
  const wifiList = await dialog.getByRole('combobox', { name: 'WiFi 支持', exact: true }).getAttribute('aria-controls');
  await page.locator(`[id="${wifiList}"]`).getByRole('option', { name: '不支持', exact: true }).click();
  await dialog.getByRole('button', { name: '保存', exact: true }).click();
  const row = table.getByRole('row').filter({ hasText: 'EG200 测试网关' });
  await row.waitFor();
  assert.equal(writes[0].body.code, 'EG200');
  assert.equal(writes[0].body.ethernetPortCount, 4);
  assert.equal(writes[0].body.rs232PortCount, 2);
  assert.equal(writes[0].body.rs485PortCount, 4);
  assert.equal(writes[0].body.canPortCount, 1);
  assert.equal(writes[0].body.serialPortCount, undefined, 'Must not overwrite historical untyped totals');
  assert.equal(writes[0].body.supports4G, true);
  assert.equal(writes[0].body.supportsWifi, false);
  await row.getByRole('button', { name: '编辑', exact: true }).click();
  assert.ok(await dialog.getByRole('textbox', { name: '型号编码', exact: true }).isDisabled());
  await dialog.getByRole('spinbutton', { name: 'CAN 数量', exact: true }).fill('0');
  await dialog.locator('.el-switch').click();
  await dialog.getByRole('button', { name: '保存', exact: true }).click();
  await row.getByText('停用', { exact: true }).waitFor();
  assert.equal(writes[1].body.code, undefined);
  assert.equal(writes[1].body.enabled, false);
  assert.equal(writes[1].body.canPortCount, 0);
  await row.getByText('0 路', { exact: true }).waitFor();
  await row.getByRole('button', { name: '删除', exact: true }).click();
  await page.getByRole('button', { name: '删除', exact: true }).last().click();
  await row.waitFor({ state: 'detached' });
  await page.getByRole('textbox', { name: '搜索产品', exact: true }).fill('NOT-EXIST');
  await table.getByText('暂无产品型号，请新增产品后再登记出厂网关').waitFor();
  await page.getByRole('textbox', { name: '搜索产品', exact: true }).fill('');
  // No active products: new registration must be blocked with a library link.
  products[0].enabled = false;
  await page.goto(`${base}/factory-gateways`);
  await page.getByRole('button', { name: '登记出厂网关', exact: true }).click();
  await dialog.getByText('暂无可登记型号', { exact: false }).waitFor();
  assert.ok(await dialog.getByRole('button', { name: '登记并生成交付资料', exact: true }).isDisabled());
  assert.equal(writes.length, 3);
  assert.deepEqual(errors, []);
  console.log('Product UI checks passed: create/edit/disable/delete, wireless flags, immutable model code, referenced deletion guard, search, no-active-model registration guard; all API writes mocked.');
} finally { await browser.close(); }

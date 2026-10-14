import assert from 'node:assert/strict';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const base = process.env.IDENTITY_PREVIEW_URL || 'http://127.0.0.1:5196';
const artifactDir = new URL('../artifacts/factory-handover/', import.meta.url);
await mkdir(artifactDir, { recursive: true });
try {
  const context = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  const user = { id: 'layout-test', username: 'test-admin', role: 'SUPER_ADMIN', tenantId: null };
  await page.addInitScript(user => {
    localStorage.setItem('accessToken', 'test-only-mocked-token');
    localStorage.setItem('userInfo', JSON.stringify(user));
  }, user);
  const hardwareId = '0123456789ABCDEF0123456789ABCDEF';
  const chipUsid = '0123456789ABCDEF';
  const activationFile = { hardwareId, chipUsid, sn: 'TEST-EG100-001', signature: 'TEST-ONLY-NOT-A-VALID-LICENSE' };
  const mockGateway = {id:'mock-gateway',hardwareId,chipUsid,model:'EG100',sn:'TEST-EG100-001',status:'UNBOUND'};
  // All API traffic is intercepted: no real gateways or credentials are created.
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    let data = { items: [], total: 0 };
    if (path === '/api/auth/profile') data = user;
    if (path === '/api/products') data = [{ code: 'EG100', name: 'EG100', enabled: true, rs232PortCount: 2, rs485PortCount: 2, canPortCount: null, ethernetPortCount: 2, supports4G: true, supportsWifi: true }];
    if (path === '/api/factory-gateways' && route.request().method() === 'GET') data = {items:[mockGateway],total:1};
    if (path === '/api/factory-gateways' && route.request().method() === 'POST') {
      data = {...mockGateway,bindCode:'123456',batchNo:'20260917-EG100-001',activationFile};
    }
    if (path === '/api/factory-gateways/mock-gateway/activation/reset') data = {...mockGateway,activationFile};
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.setViewportSize({ width: 1366, height: 768 });
  await page.goto(`${base}/factory-gateways`);
  await page.getByRole('button', { name: '登记出厂网关', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('textbox', { name: '粘贴完整设备信息', exact: true }).fill(`HardwareId：${hardwareId}\n芯片 USID：${chipUsid}`);
  await dialog.getByRole('button', { name: '登记并生成交付资料' }).click();
  await dialog.getByRole('button', { name: '资料已保存，关闭' }).waitFor();
  await dialog.locator('.label-preview img').waitFor();
  for (const [name, width, height] of [['desktop', 1366, 768], ['wide', 1920, 1080], ['mobile', 390, 844]]) {
    await page.setViewportSize({ width, height });
    await page.waitForTimeout(250);
    await page.screenshot({ path: fileURLToPath(new URL(`${name}.png`, artifactDir)), fullPage: true });
    const bounds = await dialog.boundingBox();
    assert.ok(bounds.y >= 0 && bounds.y + bounds.height <= height + 1, `${name}: dialog stays inside viewport`);
    const footer = await dialog.getByRole('button', { name: '资料已保存，关闭' }).boundingBox();
    assert.ok(footer.y + footer.height <= height, `${name}: footer remains visible`);
    const dimensions = await dialog.evaluate(el => ({ scroll: el.scrollWidth, client: el.clientWidth }));
    assert.ok(dimensions.scroll <= dimensions.client + 1, `${name}: no horizontal overflow`);
    if (width > 640) {
      const body = await dialog.locator('.el-dialog__body').evaluate(el => ({ scroll: el.scrollHeight, client: el.clientHeight }));
      assert.ok(body.scroll <= body.client + 1, `${name}: default delivery information fits without scrolling (${body.scroll}/${body.client})`);
      const left = await dialog.locator('.activation-handover').boundingBox();
      const right = await dialog.locator('.customer-handover').boundingBox();
      const qr = await dialog.locator('.handover-qr').boundingBox();
      assert.ok(Math.abs(left.y - right.y) <= 1 && qr.y >= Math.max(left.y + left.height, right.y + right.height), `${name}: two summary cards above QR section`);
    }
    await page.screenshot({ path: fileURLToPath(new URL(`${name}.png`, artifactDir)), fullPage: true });
  }
  await page.setViewportSize({ width: 1366, height: 768 });
  await dialog.getByRole('button', { name: '复制客户交付信息' }).click();
  assert.equal((await page.evaluate(() => navigator.clipboard.readText())).replaceAll('\r\n', '\n'), '网关 SN：TEST-EG100-001\n绑定码：123456');
  const downloadPromise = page.waitForEvent('download');
  await dialog.getByRole('button', { name: '下载授权文件（.lic）', exact: true }).click();
  const licenseDownload = await downloadPromise;
  assert.equal(licenseDownload.suggestedFilename(), 'activation-TEST-EG100-001.lic');
  const licensePath = await licenseDownload.path();
  const { readFile } = await import('node:fs/promises');
  const licenseContent = JSON.parse(await readFile(licensePath, 'utf8'));
  assert.equal(licenseContent.signature, 'TEST-ONLY-NOT-A-VALID-LICENSE');
  assert.equal(licenseContent.hardwareId, hardwareId);
  assert.equal(licenseContent.chipUsid, chipUsid);
  assert.deepEqual(licenseContent, activationFile, 'Changing the extension must not change signed content');
  await dialog.getByText('查看文件内容', { exact: true }).click();
  await dialog.locator('.activation-collapse pre').waitFor({ state: 'visible' });
  assert.match(await dialog.locator('.activation-collapse pre').textContent(), /TEST-ONLY-NOT-A-VALID-LICENSE/);
  await dialog.getByRole('button', { name: '资料已保存，关闭' }).click();
  await dialog.waitFor({state:'hidden'});
  await page.locator('.factory-gateway-table').getByRole('button',{name:'更多',exact:true}).click();
  await page.getByText('重发授权文件',{exact:true}).click();
  await page.getByRole('button',{name:'确认重新生成',exact:true}).click();
  const resetDownloadPromise = page.waitForEvent('download');
  await page.getByRole('button',{name:'下载新激活文件',exact:true}).click();
  const resetDownload = await resetDownloadPromise;
  assert.equal(resetDownload.suggestedFilename(),'activation-TEST-EG100-001.lic');
  assert.deepEqual(JSON.parse(await readFile(await resetDownload.path(),'utf8')),activationFile);
  assert.deepEqual(errors, []);
  console.log('Handover layout passed: desktop/mobile, clipboard, .lic registration/reissuance downloads with unchanged signed JSON content; all API traffic mocked.');
} finally {
  await browser.close();
}

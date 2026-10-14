import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';

const require = createRequire(new URL('../apps/web/package.json', import.meta.url));
const ts = require('typescript');
const source = await readFile(new URL('../apps/web/src/utils/gateway-identity.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText;
const { parseGatewayIdentity } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const hardwareId = '0123456789ABCDEF0123456789ABCDEF';
const chipUsid = '0123456789ABCDEF';
const copied = `网关：TEST-GATEWAY\nHardwareId：${hardwareId}\n芯片 USID：${chipUsid}`;
assert.deepEqual(parseGatewayIdentity(copied), { hardwareId, chipUsid, gatewayKey: 'TEST-GATEWAY' });
assert.deepEqual(parseGatewayIdentity(copied.toLowerCase().replaceAll('\n', '\r\n').replaceAll('：', ': ')), { hardwareId, chipUsid, gatewayKey: 'test-gateway' });
assert.deepEqual(parseGatewayIdentity(`\uFEFF\n chipUsid: ${chipUsid}\n HardwareId: ${hardwareId}\n`), { hardwareId, chipUsid, gatewayKey: undefined });
for (const invalid of ['', hardwareId, `HardwareId：${hardwareId}`, copied.replace(chipUsid + '\n', 'x\n'), copied.replace('芯片 USID：' + chipUsid, '芯片 USID：-'), copied.replace(hardwareId, hardwareId.slice(1)), `${copied}\nHardwareId：${'A'.repeat(32)}`]) {
  assert.throws(() => parseGatewayIdentity(invalid));
}
console.log('Device identity parser checks passed.');

if (!process.env.IDENTITY_PREVIEW_URL) process.exit(0);
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_CHANNEL || 'chrome', headless: true });
try {
  const page = await browser.newPage();
  const errors = [];
  let registrations = 0;
  page.on('pageerror', error => errors.push(error.message));
  const user = { id: 'identity-ui-test', username: 'test-admin', role: 'SUPER_ADMIN', tenantId: null };
  await page.addInitScript(userInfo => {
    localStorage.setItem('accessToken', 'test-only-mocked-token');
    localStorage.setItem('userInfo', JSON.stringify(userInfo));
  }, user);
  // No requests reach the real API, and no factory gateway is registered.
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    if (route.request().method() === 'POST' && path === '/api/factory-gateways') registrations++;
    const catalog = ['EG100', 'EG200'].map(code => ({ code, name: code, ethernetPortCount: 2, serialPortCount: 4, rs232PortCount: 2, rs485PortCount: 2, canPortCount: null, supports4G: true, supportsWifi: true, enabled: true, description: null }));
    await route.fulfill({ json: { code: 0, message: 'ok', data: path === '/api/auth/profile' ? user : path === '/api/products' ? catalog : { items: [], total: 0 } } });
  });
  await page.goto(`${process.env.IDENTITY_PREVIEW_URL}/factory-gateways`);
  await page.getByRole('button', { name: '登记出厂网关', exact: true }).click();
  const dialog = page.getByRole('dialog');
  const fullText = dialog.getByRole('textbox', { name: '粘贴完整设备信息', exact: true });
  const hardware = dialog.getByRole('textbox', { name: 'HardwareId（32 位硬件指纹）', exact: true });
  const chip = dialog.getByRole('textbox', { name: 'RJGT102 芯片 USID（16 位）', exact: true });
  const model = dialog.getByRole('combobox', { name: '网关型号', exact: true });
  const batch = dialog.locator('input[maxlength="64"]').last();
  await dialog.getByText('网口 2 · RS232 2 · RS485 2 · CAN 待确认 · 4G 支持 · WiFi 支持', { exact: true }).waitFor();
  assert.match(await batch.getAttribute('placeholder'), /\d{8}-EG100-001/);
  await model.focus();
  await model.press('ArrowDown');
  await page.getByRole('option', { name: 'EG200 · EG200', exact: true }).click();
  assert.match(await batch.getAttribute('placeholder'), /\d{8}-EG200-001/);
  await fullText.fill(copied);
  assert.equal(await hardware.inputValue(), hardwareId);
  assert.equal(await chip.inputValue(), chipUsid);
  assert.ok(await dialog.getByRole('status').textContent().then(text => text.includes('已识别设备身份')));
  await fullText.fill('HardwareId：不完整');
  await dialog.getByRole('button', { name: '登记并生成交付资料' }).click();
  assert.equal(registrations, 0, 'Malformed identity must not submit stale previous fields');
  await fullText.fill('');
  const second = copied.replace(hardwareId, 'B'.repeat(32)).replace('芯片 USID：' + chipUsid, '芯片 USID：' + 'C'.repeat(16));
  for (const input of [hardware, chip]) {
    await input.evaluate((element, text) => {
      const clipboardData = new DataTransfer();
      clipboardData.setData('text/plain', text);
      element.dispatchEvent(new ClipboardEvent('paste', { clipboardData, bubbles: true, cancelable: true }));
    }, second);
    assert.equal(await hardware.inputValue(), 'B'.repeat(32));
    assert.equal(await chip.inputValue(), 'C'.repeat(16));
  }
  await dialog.getByRole('button', { name: '取消', exact: true }).click();
  await page.getByRole('button', { name: '登记出厂网关', exact: true }).click();
  assert.equal(await fullText.inputValue(), '');
  assert.equal(await hardware.inputValue(), '');
  assert.equal(await chip.inputValue(), '');
  assert.match(await batch.getAttribute('placeholder'), /\d{8}-EG100-001/);
  await hardware.fill(hardwareId);
  await chip.fill(chipUsid);
  assert.equal(await hardware.inputValue(), hardwareId);
  assert.equal(await chip.inputValue(), chipUsid);
  assert.deepEqual(errors, []);
  assert.equal(registrations, 0);
  console.log('Browser checks passed: full paste, both field paste handlers, malformed-input guard, reset, manual entry; no real API writes.');
} finally {
  await browser.close();
}

const assert = require('node:assert/strict');
const { mkdtempSync, writeFileSync, rmSync } = require('node:fs');
const { join } = require('node:path');
const { tmpdir } = require('node:os');
const { MqttConnectionAdminService } = require('../dist/mqtt/mqtt-connection-admin.service');

async function main() {
  const directory = mkdtempSync(join(tmpdir(), 'weikong-emqx-key-test-'));
  const previousFetch = global.fetch;
  const keys = ['EMQX_API_URL', 'EMQX_API_KEY', 'EMQX_API_SECRET', 'EMQX_API_CREDENTIALS_FILE', 'EMQX_DASHBOARD_USERNAME', 'EMQX_DASHBOARD_PASSWORD'];
  const previous = Object.fromEntries(keys.map(key => [key, process.env[key]]));
  const calls = [], warnings = [];
  let status = 204;
  global.fetch = async (url, options) => { calls.push({ url, options }); return { ok: status === 204, status }; };
  const service = new MqttConnectionAdminService();
  service.logger = { warn: message => warnings.push(message) };
  try {
    keys.forEach(key => delete process.env[key]);
    Object.assign(process.env, { EMQX_API_URL: 'http://broker.test/api/v5/', EMQX_DASHBOARD_USERNAME: 'legacy', EMQX_DASHBOARD_PASSWORD: 'legacy-secret' });
    assert.equal(await service.disconnect('device'), 'unavailable');
    assert.equal(calls.length, 0, 'missing API credentials must not attempt dashboard login');
    Object.assign(process.env, { EMQX_API_KEY: 'test-key', EMQX_API_SECRET: 'test-secret' });
    assert.equal(await service.disconnect('device/a'), 'disconnected');
    assert.equal(calls[0].url, 'http://broker.test/api/v5/clients/device%2Fa');
    assert.equal(calls[0].options.method, 'DELETE');
    assert.equal(calls[0].options.headers.Authorization, 'Basic ' + Buffer.from('test-key:test-secret').toString('base64'));
    for (const [code, expected] of [[404, 'not_found'], [401, 'unavailable'], [403, 'unavailable'], [500, 'unavailable']]) {
      status = code; assert.equal(await service.disconnect('device'), expected);
    }
    const file = join(directory, 'credentials.json');
    process.env.EMQX_API_CREDENTIALS_FILE = file;
    const count = calls.length;
    assert.equal(await service.disconnect('device'), 'unavailable');
    writeFileSync(file, '{bad json');
    assert.equal(await service.disconnect('device'), 'unavailable');
    writeFileSync(file, JSON.stringify({ api_key: 'file-key', api_secret: 123 }));
    assert.equal(await service.disconnect('device'), 'unavailable');
    assert.equal(calls.length, count, 'file failure must not fall back to environment or dashboard');
    writeFileSync(file, JSON.stringify({ api_key: 'file-key', api_secret: 'file-secret' }));
    status = 204;
    await Promise.all(Array.from({ length: 6 }, (_, i) => service.disconnect('batch-' + i)));
    assert.equal(calls.length, count + 6);
    assert(calls.slice(count).every(call => call.options.headers.Authorization === 'Basic ' + Buffer.from('file-key:file-secret').toString('base64')));
    assert(calls.every(call => call.options.method === 'DELETE' && !call.url.endsWith('/login')));
    assert(warnings.every(message => !/test-secret|file-secret|legacy-secret/.test(message)));
    console.log('Connection API key tests passed: no dashboard fallback, Basic authentication, batch isolation, file credentials, error handling and secret-safe logs.');
  } finally {
    global.fetch = previousFetch;
    for (const key of keys) previous[key] === undefined ? delete process.env[key] : process.env[key] = previous[key];
    rmSync(directory, { recursive: true });
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });

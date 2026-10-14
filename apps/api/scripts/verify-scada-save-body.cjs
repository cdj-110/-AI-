require('reflect-metadata');
const assert = require('node:assert/strict');
const { NestFactory } = require('@nestjs/core');
const { Module, Controller, Post, Body } = require('@nestjs/common');
const { scadaSaveBodyParser } = require('../dist/scada-projects/scada-save-body-parser');
const { HttpExceptionFilter } = require('../dist/common/filters/http-exception.filter');
const { ScadaProjectsService } = require('../dist/scada-projects/scada-projects.service');
class TestController { save(body) { return { size: JSON.stringify(body).length }; } }
Controller()(TestController);
Post('conf/config')(TestController.prototype, 'save', Object.getOwnPropertyDescriptor(TestController.prototype, 'save'));
Body()(TestController.prototype, 'save', 0);
class TestModule {}
Module({ controllers: [TestController] })(TestModule);
async function main() {
  const app = await NestFactory.create(TestModule, { logger: false });
  app.use(scadaSaveBodyParser); app.setGlobalPrefix('api'); app.useGlobalFilters(new HttpExceptionFilter());
  await app.listen(0, '127.0.0.1');
  const base = await app.getUrl();
  const post = (path, payload) => fetch(base + path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: payload });
  try {
    let response = await post('/api/conf/config', JSON.stringify({ main_data: { text: 'x'.repeat(120000) }, thumbnail: 'x'.repeat(100000) }));
    assert.equal(response.status, 201, 'old 100 KB boundary must no longer reject scada saves');
    response = await post('/api/conf/config', JSON.stringify({ main_data: { text: '中'.repeat(1700000) }, thumbnail: 'x'.repeat(490000) }));
    assert.equal(response.status, 201, 'valid Chinese config + thumbnail must fit byte limit');
    response = await post('/api/conf/config', JSON.stringify({ text: 'x'.repeat(11 * 1024 * 1024) }));
    assert.equal(response.status, 413); assert.match((await response.json()).message, /请求内容过大/);
    response = await post('/api/unrelated', JSON.stringify({ text: 'x'.repeat(120000) }));
    assert.equal(response.status, 413, 'unrelated endpoints retain original size limit');
    response = await post('/api/conf/config', '{invalid');
    assert.equal(response.status, 400); assert.match((await response.json()).message, /JSON/);
    let writes = 0, saved;
    const service = new ScadaProjectsService({ scadaProject: { update: async args => { writes++; saved = args.data.config; } } });
    service.getAccessible = async () => ({ id: 'synthetic-project' });
    const actor = { sub: 'synthetic-user' };
    await assert.rejects(service.saveGoViewConfig(actor, 'synthetic-project', { text: 'x'.repeat(2000001) }), /2 MB/);
    await assert.rejects(service.saveGoViewConfig(actor, 'synthetic-project', {}, 'x'.repeat(500001)), /500 KB/);
    assert.equal(writes, 0);
    await service.saveGoViewConfig(actor, 'synthetic-project', { item: [{ componentList: [{ attr: { x: -300, y: 1400 } }] }] });
    assert.equal(writes, 1); assert.equal(saved.main_data.item[0].componentList[0].attr.x, -300);
    console.log('PASS: Nest scoped parser, >100 KB and Chinese config requests, 413/400 errors, unchanged business limits and outside-canvas coordinates (synthetic server, no database)');
  } finally { await app.close(); }
}
main().catch(error => { console.error(error); process.exitCode = 1; });

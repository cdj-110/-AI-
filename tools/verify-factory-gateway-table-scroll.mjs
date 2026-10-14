import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFile, mkdir} from 'node:fs/promises';
import {resolve, extname} from 'node:path';
const {chromium} = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const root = resolve('apps/web/dist/admin');
const user = {id:'layout-test', username:'layout-test', role:'SUPER_ADMIN', tenantId:null};
const gateway = {id:'layout-test-gateway', sn:'WK20265AF2F5C6', hardwareId:'3AF104FF59D83AB08D4880783218BA75', chipUsid:'EE38111700000000', status:'UNBOUND', licenseSequence:6, authorizationState:'ACTIVE', batchNo:'20260917-EG100-005', model:'EG100', lastSeenAt:'2026-09-17T07:39:22Z'};
gateway.product = {code:'EG100',name:'工业网关',ethernetPortCount:2,rs232PortCount:2,rs485PortCount:2,canPortCount:0,supports4G:true,supportsWifi:true};
gateway.issuedLicense = {licenseSequence:6,licenseVersion:2,keyId:'test-key',issuedAt:'2026-09-17T07:00:00Z',expiresAt:'',features:['collection','cloud','remote-ssh']};
gateway.reportedLicense = {...gateway.issuedLicense};
gateway.licenseReportedAt = '2026-09-17T07:39:22Z';
gateway.licenseActivatedAt = gateway.licenseReportedAt;
gateway.authorizationReportFresh = true;
let currentGateway = gateway;
const server = createServer(async (req,res) => {
  try {
    const path = new URL(req.url,'http://localhost').pathname;
    const file = resolve(root, path.startsWith('/assets/') ? '.'+path : 'index.html');
    assert.ok(file.startsWith(root));
    const content = await readFile(file);
    res.setHeader('Content-Type', {'.js':'text/javascript','.css':'text/css','.html':'text/html'}[extname(file)] || 'application/octet-stream');
    res.end(content);
  } catch {res.writeHead(404);res.end();}
});
await new Promise(r=>server.listen(0,'127.0.0.1',r));
let browser;
try {
  browser = await chromium.launch({headless:true,channel:'chrome'});
  const context = await browser.newContext();
  await context.addInitScript(user=>{localStorage.setItem('accessToken','isolated-layout-test');localStorage.setItem('userInfo',JSON.stringify(user));},user);
  await context.route('**/api/**', async route=>{
    assert.equal(route.request().method(),'GET','Layout test must not mutate data');
    const path = new URL(route.request().url()).pathname;
    const data = path === '/api/auth/profile' ? user : path === '/api/factory-gateways' ? {items:[currentGateway],total:1} : path === '/api/products' ? [] : {items:[],total:0,unreadTotal:0};
    await route.fulfill({json:{code:0,message:'ok',data}});
  });
  const page = await context.newPage();
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await mkdir('artifacts/factory-table-scroll',{recursive:true});
  for (const width of [1440,1000]) {
    await page.setViewportSize({width,height:900});
    await page.goto(`${process.env.WK_FACTORY_TABLE_URL || `http://127.0.0.1:${server.address().port}`}/factory-gateways`);
    const table = page.locator('.factory-gateway-table');
    const more = table.getByRole('button',{name:'更多',exact:true});
    await more.waitFor();
    await table.locator('.el-loading-mask').waitFor({state:'hidden'});
    assert.deepEqual((await table.getByRole('columnheader').allTextContents()).map(t=>t.trim()).filter(Boolean),['SN','产品型号','绑定状态','授权状态','归属租户','最近心跳','操作']);
    const wrap = table.locator('.el-scrollbar__wrap');
    await page.waitForFunction(()=>{const e=document.querySelector('.factory-gateway-table .el-scrollbar__wrap');return e && e.scrollWidth>e.clientWidth;});
    const box = await more.boundingBox();
    assert.ok(box && box.x>=0 && box.x+box.width<=width, 'Operation buttons must be visible before scrolling');
    await table.locator('.el-scrollbar__bar.is-horizontal').waitFor({state:'visible'});
    const area = await wrap.boundingBox();
    await page.mouse.move(area.x+area.width/3,area.y+area.height/2);
    await page.mouse.wheel(650,0);
    await page.waitForFunction(()=>document.querySelector('.factory-gateway-table .el-scrollbar__wrap').scrollLeft>0);
    const after = await more.boundingBox();
    assert.ok(Math.abs(after.x-box.x)<2,'Fixed operations must stay in place while scrolling');
    await more.click();
    await page.getByText('重发授权文件',{exact:true}).waitFor({state:'visible'});
    await page.keyboard.press('Escape');
    await page.screenshot({path:`artifacts/factory-table-scroll/width-${width}.png`,fullPage:true});
    await wrap.evaluate(e=>{e.scrollLeft=0;});
    await table.getByRole('button',{name:gateway.sn,exact:true}).click();
    const drawer = page.getByRole('dialog',{name:'出厂网关详情'});
    await drawer.waitFor();
    await page.waitForFunction(()=>{const e=document.querySelector('.factory-details-drawer');return e && getComputedStyle(e).transform==='none';});
    for(const name of ['设备身份','生产信息','授权信息','绑定信息','连接信息']) await drawer.getByRole('heading',{name,exact:true}).waitFor();
    await drawer.getByText(gateway.hardwareId,{exact:true}).waitFor();
    await drawer.getByText(gateway.batchNo,{exact:true}).waitFor();
    await drawer.getByText(/RS232 2 · RS485 2/).waitFor();
    assert.ok((await drawer.textContent()).includes('远程 SSH'));
    assert.equal(await drawer.getByText('永久有效',{exact:true}).count(),2,'issued and reported perpetual expiry must both be visible');
    await page.screenshot({path:`artifacts/factory-table-scroll/details-${width}.png`,fullPage:true});
    // Refresh replaces the selected row rather than displaying stale detail data.
    currentGateway = {...gateway,authorizationState:'UPDATE_PENDING',reportedLicense:{...gateway.issuedLicense,licenseSequence:5},licenseReportedAt:'2026-09-17T07:39:22Z',authorizationReportFresh:false};
    await drawer.getByRole('button',{name:'刷新详情',exact:true}).click();
    await drawer.getByText('待更新确认',{exact:true}).waitFor();
    await drawer.getByText('回执暂未刷新',{exact:true}).waitFor();
    await page.setViewportSize({width:390,height:844});
    await page.waitForFunction(()=>document.querySelector('.factory-details-drawer')?.getBoundingClientRect().width<=390);
    assert.ok(await drawer.evaluate(e=>e.getBoundingClientRect().width<=390));
    await page.screenshot({path:'artifacts/factory-table-scroll/details-mobile.png',fullPage:true});
    await page.keyboard.press('Escape');
    await drawer.waitFor({state:'hidden'});
    currentGateway = gateway;
  }
  assert.deepEqual(errors,[]);
  console.log('Passed: compact seven-column list; fixed actions and scrolling; SN detail drawer with five groups, full identities, product specifications, signed/reported licenses, stale report refresh and mobile sizing. Isolated mock data; no production changes.');
} finally {await browser?.close();await new Promise(r=>server.close(r));}

const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {chromium}=require(process.argv[2]||'playwright'),root=path.resolve('dist/tenant');
async function main(){
 const server=http.createServer((req,res)=>{let file=path.resolve(root,'.'+decodeURIComponent((req.url||'/').split('?')[0]));if(!file.startsWith(root+path.sep)||!fs.existsSync(file)||fs.statSync(file).isDirectory())file=path.join(root,'index.html');res.setHeader('Content-Type',file.endsWith('.js')?'application/javascript':file.endsWith('.css')?'text/css':file.endsWith('.png')?'image/png':'text/html');res.end(fs.readFileSync(file));});await new Promise(r=>server.listen(0,'127.0.0.1',r));
 const browser=await chromium.launch({channel:'chrome',headless:true});
 try{
  const context=await browser.newContext({viewport:{width:1920,height:1000}}),page=await context.newPage(),errors=[],writes=[];
  const user={id:'synthetic-admin',role:'TENANT_ADMIN',tenantId:'synthetic-tenant',username:'测试管理员'};
  await context.addInitScript(user=>{localStorage.setItem('accessToken','synthetic-test-token');localStorage.setItem('userInfo',JSON.stringify(user));localStorage.setItem('sessionLastActivityAt',String(Date.now()));localStorage.setItem('sessionIdleTimeoutSeconds','3600');localStorage.setItem('sessionAbsoluteExpiresAt','2099-01-01T00:00:00Z');},user);
  page.on('pageerror',e=>errors.push(e.message));
  let retention={label:'9个自然月',queryEarliestAt:'2026-01-14T00:00:00+08:00',addonMonths:0};
  await context.route('**/api/**',async route=>{const req=route.request(),url=new URL(req.url());if(req.method()!=='GET')writes.push(url.pathname);let data=[];
   if(url.pathname==='/api/subscriptions/current')data={plan:{id:'enterprise',name:'企业版',description:'适合大规模设备接入和企业级应用',sortOrder:3},subscription:{status:'ACTIVE',startedAt:'2026-09-24T00:00:00+08:00',expiresAt:'2027-09-24T00:00:00+08:00',graceEndsAt:'2027-10-01T00:00:00+08:00'},usage:{devices:{used:5,limit:510},points:{used:7,limit:100000},scadaProjects:{used:2,limit:50},users:{used:2,limit:100}},historyRetention:retention,notifications:[],entitlementChanges:[]};
   else if(url.pathname.includes('/auth/'))data={...user,user,accessToken:'synthetic-test-token',idleTimeoutSeconds:3600,absoluteExpiresAt:'2099-01-01T00:00:00Z'};
   else if(url.pathname.includes('notification'))data={items:[],unreadTotal:0};
   else if(url.pathname.includes('/stream')){await route.fulfill({status:200,contentType:'text/event-stream',body:''});return;}
   await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:0,data})});
  });
  await page.goto(process.argv.includes('--deployed')?'http://127.0.0.1:5174/subscription':`http://127.0.0.1:${server.address().port}/subscription`);
  const card=page.locator('.history-quota-card');await card.getByText('9个自然月',{exact:true}).waitFor();assert.equal(await page.locator('.usage>div').count(),5);assert.equal(await page.locator('.current-card+.el-alert').count(),0);
  const backgrounds=await page.locator('.usage>div').evaluateAll(nodes=>nodes.map(n=>getComputedStyle(n).backgroundColor));assert.equal(new Set(backgrounds).size,1);
  await card.getByRole('button',{name:'历史保存时间说明'}).hover();await page.getByText(/升级套餐也不会恢复已清理数据/).waitFor({state:'visible'});
  const output=path.resolve('../../outputs/subscription-history-card-20261014');fs.mkdirSync(output,{recursive:true});await page.mouse.move(100,100);await page.screenshot({path:path.join(output,'desktop.png'),fullPage:true});
  retention={...retention,label:'12个自然月',addonMonths:3,addonExpiresAt:'2027-09-24T00:00:00+08:00',graceEndsAt:'2026-10-21T00:00:00+08:00'};await page.reload();await card.getByText('12个自然月',{exact:true}).waitFor();await card.getByRole('button',{name:'历史保存时间说明'}).hover();await page.getByText(/扩展权益到期/).waitFor({state:'visible'});await page.getByText(/当前清理宽限至/).waitFor({state:'visible'});
  await page.setViewportSize({width:390,height:844});await page.mouse.move(0,0);await page.screenshot({path:path.join(output,'mobile.png'),fullPage:true});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  assert.deepEqual(errors,[]);assert.deepEqual(writes,[]);console.log('PASS subscription history card: fifth matching card, no separate alert, hover explanation/addon/grace, mobile layout, no page errors; all APIs mocked, no writes.');
 }finally{await browser.close();await new Promise(r=>server.close(r));}
}
main().catch(e=>{console.error(e);process.exitCode=1});

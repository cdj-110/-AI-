const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {chromium}=require(process.argv[2]||'playwright');
const root=path.resolve('dist/tenant');
async function main(){
 const server=http.createServer((req,res)=>{let file=path.resolve(root,'.'+decodeURIComponent((req.url||'/').split('?')[0]));if(!file.startsWith(root+path.sep)||!fs.existsSync(file)||fs.statSync(file).isDirectory())file=path.join(root,'index.html');res.setHeader('Content-Type',file.endsWith('.js')?'application/javascript':file.endsWith('.css')?'text/css':file.endsWith('.png')?'image/png':'text/html');res.end(fs.readFileSync(file));});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const browser=await chromium.launch({channel:'chrome',headless:true});
 try{
  const context=await browser.newContext({viewport:{width:1440,height:1050},acceptDownloads:true});const page=await context.newPage(),errors=[],requests=[];
  page.on('pageerror',e=>errors.push(e.message));
  const user={id:'test-user',role:'TENANT_USER',tenantId:'test-tenant',username:'测试用户',permissions:['DEVICE_VIEW','DASHBOARD_VIEW'],deviceScopeMode:'CUSTOM'};
  await context.addInitScript(user=>{localStorage.setItem('accessToken','synthetic-test-token');localStorage.setItem('userInfo',JSON.stringify(user));localStorage.setItem('sessionLastActivityAt',String(Date.now()));localStorage.setItem('sessionIdleTimeoutSeconds','3600');localStorage.setItem('sessionAbsoluteExpiresAt','2099-01-01T00:00:00Z');},user);
  const ids=['aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa','bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb'];
  await page.route('**/api/**',async route=>{
   const req=route.request(),url=new URL(req.url());let data=[];
   if(url.pathname==='/api/devices')data={items:[{id:ids[0],name:'噪声变送器',deviceKey:'noise-meter'},{id:ids[1],name:'西门子S7',deviceKey:'s7'}],total:2};
   else if(url.pathname.endsWith('/metrics'))data=[{identifier:'value',name:'测量值',unit:'dB',enabled:true},{identifier:'disabled',name:'停用点位',enabled:false}];
   else if(url.pathname==='/api/devices/data/retention')data={label:'7天',queryEarliestAt:new Date(Date.now()-7*86400000).toISOString()};
   else if(url.pathname==='/api/devices/data/history'){const body=req.postDataJSON();requests.push(body);data={dataset:{dimensions:[{title:'时间',key:'timestamp'},{title:'设备',key:'deviceName'},{title:'噪声变送器 / 测量值',key:'metric_0'},{title:'西门子S7 / 测量值',key:'metric_1'}],source:[{timestamp:'2026-10-13 12:00:00',deviceName:'噪声变送器',metric_0:41.2,metric_1:null}]},total:201,page:body.page,pageSize:body.pageSize};}
   else if(url.pathname==='/api/devices/data/export'){requests.push({export:req.postDataJSON()});data={csv:'\uFEFF"时间","测量值"\r\n"2026-10-13","41.2"',total:201,exported:201,limited:false};}
   else if(url.pathname.includes('/auth/'))data={...user,user,accessToken:'synthetic-test-token',idleTimeoutSeconds:3600,absoluteExpiresAt:'2099-01-01T00:00:00Z'};
   else if(url.pathname.includes('notification'))data={items:[],unreadTotal:0};
   else if(url.pathname.includes('/stream')){await route.fulfill({status:200,contentType:'text/event-stream',body:''});return;}
   await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:200,message:'ok',data})});
  });
  await page.goto(process.argv.includes('--deployed')?'http://127.0.0.1:5174/data-center':`http://127.0.0.1:${server.address().port}/data-center`);
  await page.getByRole('heading',{name:'数据中心',exact:true}).waitFor();
  await page.locator('.device-select .el-select__wrapper').click();
  await page.getByRole('option',{name:'噪声变送器 · noise-meter',exact:true}).click();
  await page.getByRole('option',{name:'西门子S7 · s7',exact:true}).click();await page.keyboard.press('Escape');
  await page.locator('.point-row').nth(0).getByRole('button',{name:'全选点位'}).click();
  await page.locator('.point-row').nth(1).getByRole('button',{name:'全选点位'}).click();
  await page.getByText('已选 2 / 20 个点位',{exact:true}).waitFor();
  await page.getByRole('button',{name:'查询数据',exact:true}).click();await page.getByText('共 201 条',{exact:true}).waitFor();
  assert.equal(requests[0].pairs.length,2);assert.ok(requests[0].pairs.every(p=>p.value2==='value'));
  const download=page.waitForEvent('download');await page.getByRole('button',{name:'导出 CSV',exact:true}).click();assert.match((await download).suggestedFilename(),/历史数据.*\.csv/);assert.equal(requests.at(-1).export.pairs.length,2);
  const output=path.resolve(`../../outputs/data-center-20261013${process.argv.includes('--deployed')?'-deployed':''}`);fs.mkdirSync(output,{recursive:true});await page.screenshot({path:path.join(output,'desktop.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.waitForTimeout(400);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,'mobile page does not overflow horizontally');
  await page.screenshot({path:path.join(output,'mobile.png'),fullPage:true});
  assert.equal(errors.length,0,errors.join('\n'));
  console.log('PASS data center browser: DEVICE_VIEW without SCADA permission, two devices, enabled points only, query payload, CSV download and no page errors. All API requests mocked.');
 }finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
}
main().catch(e=>{console.error(e);process.exitCode=1});

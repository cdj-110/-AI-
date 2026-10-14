const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {chromium}=require(process.argv[2]||'playwright'),root=path.resolve('dist/tenant');
async function main(){
 const server=http.createServer((req,res)=>{let file=path.resolve(root,'.'+decodeURIComponent((req.url||'/').split('?')[0]));if(!file.startsWith(root+path.sep)||!fs.existsSync(file)||fs.statSync(file).isDirectory())file=path.join(root,'index.html');res.setHeader('Content-Type',file.endsWith('.js')?'application/javascript':file.endsWith('.css')?'text/css':file.endsWith('.png')?'image/png':'text/html');res.end(fs.readFileSync(file));});await new Promise(r=>server.listen(0,'127.0.0.1',r));
 const browser=await chromium.launch({channel:'chrome',headless:true});
 try{
  const context=await browser.newContext({viewport:{width:1920,height:911}}),page=await context.newPage(),errors=[],writes=[];
  const user={id:'synthetic-admin',role:'TENANT_ADMIN',tenantId:'synthetic-tenant',username:'测试管理员'};
  await context.addInitScript(user=>{localStorage.setItem('accessToken','synthetic-test-token');localStorage.setItem('userInfo',JSON.stringify(user));localStorage.setItem('sessionLastActivityAt',String(Date.now()));localStorage.setItem('sessionIdleTimeoutSeconds','3600');localStorage.setItem('sessionAbsoluteExpiresAt','2099-01-01T00:00:00Z');},user);
  page.on('pageerror',e=>errors.push(e.message));
  const source={id:'aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa',name:'噪声变送器',deviceKey:'noise-source',deviceType:'DIRECT'},target={id:'bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb',name:'开关设备',deviceKey:'switch-target',deviceType:'GATEWAY_CHILD'};
  const rules=[{id:'rule-a',name:'噪声超限关闭设备',sourceDeviceId:source.id,sourceName:source.name,sourceMetric:'noise',operator:'GT',threshold:40,targetDeviceId:target.id,targetName:target.name,targetMetric:'switch',actionValue:false,enabled:false}];
  await context.route('**/api/**',async route=>{const req=route.request(),url=new URL(req.url());if(req.method()!=='GET')writes.push({url:url.pathname,method:req.method(),body:req.postDataJSON()});let data=[];
   if(url.pathname==='/api/device-linkage'){
    if(req.method()==='POST'){const value=req.postDataJSON();assert.equal(value.enabled,undefined);data={id:'rule-new',...value,sourceName:source.name,targetName:target.name,enabled:false};rules.unshift(data);}else data={items:rules,total:rules.length,page:1};
   }else if(url.pathname.endsWith('/enabled')){const rule=rules.find(r=>url.pathname.includes(r.id));const value=req.postDataJSON();assert.equal(value.confirmAutomaticControl,true);rule.enabled=value.enabled;data=rule;}
   else if(url.pathname.endsWith('/runs'))data={items:[{id:'run1',sampleAt:'2026-10-14T04:00:00Z',observedValue:41,actionValue:false,status:'TIMEOUT',message:'30秒内未收到网关回执，结果未知'}],total:1,page:1};
   else if(url.pathname==='/api/devices')data={items:url.searchParams.get('deviceType')?[target]:[source,target],total:2};
   else if(url.pathname.endsWith('/metrics'))data=[{identifier:'noise',name:'噪声',dataType:'NUMBER',enabled:true,ignored:false}];
   else if(url.pathname.endsWith('/commands/capabilities'))data={points:[{identifier:'switch',name:'开关',dataType:'BOOLEAN'}],available:true,reason:''};
   else if(url.pathname.includes('/auth/'))data={...user,user,accessToken:'synthetic-test-token',idleTimeoutSeconds:3600,absoluteExpiresAt:'2099-01-01T00:00:00Z'};
   else if(url.pathname.includes('notification')||url.pathname==='/api/alarms')data={items:[],total:0,unreadTotal:0};
   else if(url.pathname.includes('/stream')){await route.fulfill({status:200,contentType:'text/event-stream',body:''});return;}
   await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:0,data})});
  });
  await page.goto(process.argv.includes('--deployed')?'http://127.0.0.1:5174/device-linkage':`http://127.0.0.1:${server.address().port}/device-linkage`);
  await page.getByRole('heading',{name:'设备联动',exact:true}).waitFor();await page.getByText('噪声超限关闭设备',{exact:true}).waitFor();
  await page.getByRole('button',{name:'新增联动规则'}).click();const dialog=page.getByRole('dialog',{name:'新增联动规则'});
  await dialog.getByPlaceholder('例如：噪声超限关闭设备').waitFor();await page.waitForTimeout(350);const emptyOutput=path.resolve('../../outputs/device-linkage-20261014');fs.mkdirSync(emptyOutput,{recursive:true});await page.screenshot({path:path.join(emptyOutput,'editor-empty-desktop.png'),fullPage:true});
  await dialog.getByPlaceholder('例如：噪声超限关闭设备').fill('测试新规则');
  await dialog.getByRole('button',{name:'查看联动规则说明'}).click();await page.getByText('运行与安全说明',{exact:true}).waitFor();await dialog.getByPlaceholder('例如：噪声超限关闭设备').click();
  await dialog.locator('.el-form-item').filter({hasText:'来源设备'}).locator('.el-select').click();await page.getByRole('option',{name:'噪声变送器 · noise-source',exact:true}).click();
  await dialog.locator('.el-form-item').filter({hasText:'数值点位'}).locator('.el-select').click();await page.getByRole('option',{name:'噪声 / noise',exact:true}).click();
  await dialog.locator('.el-form-item').filter({hasText:'目标采集子设备'}).locator('.el-select').click();await page.getByRole('option',{name:'开关设备 · switch-target',exact:true}).click();
  await dialog.locator('.el-form-item').filter({hasText:'可写点位'}).locator('.el-select').click();await page.getByRole('option',{name:'开关 / switch',exact:true}).click();
  await dialog.locator('.rule-preview').getByText(/下发 false/).waitFor();
  const editorOutput=path.resolve('../../outputs/device-linkage-20261014');fs.mkdirSync(editorOutput,{recursive:true});await dialog.locator('.editor-scroll').evaluate(el=>{el.scrollTop=0;});await page.waitForTimeout(350);await page.screenshot({path:path.join(editorOutput,'editor-desktop.png'),fullPage:true});
  await dialog.getByRole('button',{name:'保存规则'}).click();await page.getByText('测试新规则',{exact:true}).waitFor();
  const created=writes.find(w=>w.method==='POST'&&w.url==='/api/device-linkage');assert.equal(created.body.actionValue,false);assert.equal(created.body.threshold,40);assert.equal(created.body.sourceDeviceId,source.id);
  const row=page.locator('.el-table__row').filter({hasText:'测试新规则'});
  await row.getByRole('button',{name:'启用',exact:true}).click();await page.getByRole('button',{name:'取消',exact:true}).click();assert.equal(writes.filter(w=>w.url.endsWith('/enabled')).length,0,'cancel authorization sends no mutation');
  await row.getByRole('button',{name:'启用',exact:true}).click();await page.getByRole('button',{name:'授权并启用',exact:true}).click();await row.getByRole('button',{name:'停用',exact:true}).waitFor();assert.equal(await row.getByRole('button',{name:'编辑',exact:true}).isDisabled(),true);
  await row.getByRole('button',{name:'执行记录'}).click();await page.getByRole('dialog').getByText('结果未知',{exact:true}).waitFor();await page.getByRole('dialog').getByText('30秒内未收到网关回执，结果未知',{exact:true}).waitFor();
  const output=path.resolve('../../outputs/device-linkage-20261014');fs.mkdirSync(output,{recursive:true});await page.screenshot({path:path.join(output,'execution-records.png'),fullPage:true});
  await page.getByRole('dialog').getByRole('button',{name:'Close this dialog'}).click();await page.getByRole('dialog').waitFor({state:'hidden'});await page.screenshot({path:path.join(output,'rules.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.getByRole('button',{name:'新增联动规则'}).click();await page.getByRole('dialog',{name:'新增联动规则'}).getByPlaceholder('例如：噪声超限关闭设备').waitFor();await page.waitForTimeout(400);await page.screenshot({path:path.join(output,'mobile-editor.png'),fullPage:true});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  const mobile=page.getByRole('dialog',{name:'新增联动规则'}),before=await mobile.getByRole('button',{name:'保存规则'}).boundingBox();assert.ok(before.y>=0&&before.y+before.height<=844,'save stays visible without scrolling');await mobile.locator('.editor-scroll').evaluate(el=>{el.scrollTop=el.scrollHeight;});const after=await mobile.getByRole('button',{name:'保存规则'}).boundingBox();assert.equal(Math.round(before.y),Math.round(after.y),'footer does not scroll with fields');await mobile.getByRole('button',{name:'取消',exact:true}).click();await mobile.waitFor({state:'hidden'});
  assert.equal(writes.some(w=>/\/commands$/.test(w.url)),false,'browser never directly posts device commands');assert.deepEqual(errors,[]);
  await context.close();console.log('PASS mocked browser: rules/create defaults false, device/point identities, consent cancellation/enable, enabled edit protection, unknown receipts, mobile editor and no direct device command calls. No real APIs/devices used.');
 }finally{await browser.close();await new Promise(r=>server.close(r));}
}
main().catch(e=>{console.error(e);process.exitCode=1});

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
  const gatewayId='dddddddd-dddd-4ddd-dddd-dddddddddddd';
  let task;
  const taskId='cccccccc-cccc-4ccc-cccc-cccccccccccc';
  await page.route('**/api/**',async route=>{
   const req=route.request(),url=new URL(req.url());let data=[];
   if(url.pathname==='/api/devices')data={items:[{id:ids[0],name:'噪声变送器',deviceKey:'noise-meter'},{id:ids[1],name:'西门子S7',deviceKey:'s7'},{id:gatewayId,name:'测试网关',deviceKey:'gateway'}],total:3};
   else if(url.pathname===`/api/devices/${gatewayId}/metrics`)data=ids.map((id,i)=>({deviceId:id,deviceName:i?'西门子S7':'噪声变送器',identifier:'value',name:'测量值',enabled:true,ignored:false}));
   else if(url.pathname.endsWith('/metrics'))data=[{identifier:'value',name:'测量值',unit:'dB',enabled:true},{identifier:'disabled',name:'停用点位',enabled:false}];
   else if(url.pathname==='/api/devices/data/retention')data={label:'7天',queryEarliestAt:new Date(Date.now()-7*86400000).toISOString()};
   else if(url.pathname==='/api/history-data-tasks'&&req.method()==='POST'){const body=req.postDataJSON();requests.push({saved:body});task={...body,id:taskId,startAt:body.start,endAt:body.end,createdAt:new Date().toISOString(),createdByName:'测试用户',deviceNames:['噪声变送器','西门子S7'],deviceCount:2,pointCount:body.pairs.length,canView:true,devices:[{id:ids[0],name:'噪声变送器',deviceKey:'noise-meter'},{id:ids[1],name:'西门子S7',deviceKey:'s7'}]};data=task;}
   else if(url.pathname==='/api/history-data-tasks'){data={items:task?[task]:[],total:task?1:0,page:1,pageSize:20};}
   else if(url.pathname==='/api/history-data-tasks/'+taskId&&req.method()==='PATCH'){const body=req.postDataJSON();task={...task,...body,startAt:body.start,endAt:body.end};data=task;}
   else if(url.pathname==='/api/history-data-tasks/'+taskId&&req.method()==='DELETE'){task=undefined;data={success:true};}
   else if(url.pathname==='/api/history-data-tasks/'+taskId)data=task;
   else if(url.pathname==='/api/history-data-tasks/'+taskId+'/records'){const body={...Object.fromEntries(url.searchParams),pairs:task.pairs,page:Number(url.searchParams.get('page'))||1,pageSize:Number(url.searchParams.get('pageSize'))||20};requests.push(body);data={dataset:{dimensions:[{title:'时间',key:'timestamp'},{title:'设备',key:'deviceName'},{title:'噪声变送器 / 测量值',key:'metric_0'},{title:'西门子S7 / 测量值',key:'metric_1'}],source:[{timestamp:'2026-10-13 12:00:00',deviceName:'噪声变送器',metric_0:41.2,metric_1:null}]},total:201,page:body.page,pageSize:body.pageSize};}
   else if(url.pathname==='/api/history-data-tasks/'+taskId+'/export'){requests.push({export:{taskId,pairs:task.pairs,filters:req.postDataJSON()}});data={csv:'\uFEFF"时间","测量值"\r\n"2026-10-13","41.2"',total:201,exported:201,limited:false};}
   else if(url.pathname.includes('/auth/'))data={...user,user,accessToken:'synthetic-test-token',idleTimeoutSeconds:3600,absoluteExpiresAt:'2099-01-01T00:00:00Z'};
   else if(url.pathname.includes('notification'))data={items:[],unreadTotal:0};
   else if(url.pathname.includes('/stream')){await route.fulfill({status:200,contentType:'text/event-stream',body:''});return;}
   await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:200,message:'ok',data})});
  });
  await page.goto(process.argv.includes('--deployed')?'http://127.0.0.1:5174/data-center':`http://127.0.0.1:${server.address().port}/data-center`);
  await page.getByRole('heading',{name:'数据中心',exact:true}).waitFor();
  assert.equal(await page.locator('.filter-panel').count(),0,'task list does not contain the entire query form');
  await page.getByRole('button',{name:'新增历史数据任务',exact:true}).click();
  await page.getByPlaceholder('例如：噪声设备每日历史查询').fill('历史数据任务验证');
  await page.locator('.device-select .el-select__wrapper').click();
  await page.getByRole('option',{name:'噪声变送器 · noise-meter',exact:true}).click();
  await page.getByRole('option',{name:'西门子S7 · s7',exact:true}).click();await page.keyboard.press('Escape');
  await page.locator('.device-select .el-select__wrapper').click();await page.getByRole('option',{name:'测试网关 · gateway',exact:true}).click();await page.keyboard.press('Escape');
  await page.locator('.point-row').nth(2).getByRole('button',{name:'全选点位'}).click();
  await page.locator('.point-row').nth(0).getByRole('button',{name:'全选点位'}).click();
  await page.locator('.point-row').nth(1).getByRole('button',{name:'全选点位'}).click();
  await page.getByText('已选 2 / 20 个点位',{exact:true}).waitFor();
  await page.getByRole('button',{name:'保存任务',exact:true}).click();
  await page.getByRole('button',{name:'历史数据任务验证',exact:true}).waitFor();
  assert.equal(requests[0].saved.pairs.length,2);
  assert.deepEqual(requests[0].saved.pairs.map(p=>p.value1).sort(),[...ids].sort(),'gateway selections persist child owners and duplicate selections are removed');
  await page.getByRole('button',{name:'历史数据任务验证',exact:true}).click();
  await page.getByRole('heading',{name:'历史数据任务验证',exact:true}).waitFor();await page.getByText('共 201 条',{exact:true}).waitFor();
  assert.match(page.url(),/\/data-center$/);await page.getByRole('dialog',{name:'历史数据任务详情'}).waitFor();assert.equal(await page.locator('.filter-panel').count(),0,'detail dialog shows records instead of editing form');
  await page.getByRole('button',{name:'关闭',exact:true}).click();await page.getByRole('dialog',{name:'历史数据任务详情'}).waitFor({state:'hidden'});
  await page.reload();await page.getByRole('button',{name:'历史数据任务验证',exact:true}).click();await page.getByText('共 201 条',{exact:true}).waitFor();
  const timeInputs=page.locator('.record-filters .el-range-input');assert.ok(await timeInputs.nth(0).inputValue(),'filter range defaults to valid task time');
  await timeInputs.nth(0).click();await page.locator('.el-date-range-picker:visible').waitFor();assert.ok(await page.locator('.el-date-range-picker:visible td.disabled').count(),'out-of-task dates are disabled');await page.keyboard.press('Escape');
  await timeInputs.nth(0).fill('2020-01-01 00:00:00');await timeInputs.nth(0).press('Enter');await page.getByRole('heading',{name:'筛选历史记录',exact:true}).click();
  await page.getByPlaceholder('搜索设备名称或点位值').fill('41.2');const filteredResponse=page.waitForResponse(r=>r.url().includes('/records')&&r.url().includes('keyword=41.2'));await page.getByRole('button',{name:'筛选',exact:true}).click();await filteredResponse;assert.equal(requests.at(-1).keyword,'41.2');assert.equal(requests.at(-1).page,1);
  assert.ok(new Date(requests.at(-1).start)>=new Date(task.startAt)&&new Date(requests.at(-1).end)<=new Date(task.endAt)&&new Date(requests.at(-1).start)<new Date(requests.at(-1).end),'manual out-of-range input never reaches API');
  await page.locator('.record-filter-fields .el-select').nth(1).locator('.el-select__wrapper').click();await page.getByRole('option',{name:'噪声变送器 / value',exact:true}).click();await page.getByPlaceholder('数值下限（含）').fill('0');await page.getByPlaceholder('数值上限（含）').fill('42');
  const numericResponse=page.waitForResponse(r=>r.url().includes('/records')&&r.url().includes('valueMin=0'));await page.getByRole('button',{name:'筛选',exact:true}).click();await numericResponse;assert.equal(requests.at(-1).valueMax,'42');assert.equal(requests.at(-1).pointKey,JSON.stringify([ids[0],'value']));
  const pagedResponse=page.waitForResponse(r=>r.url().includes('/records')&&r.url().includes('page=2'));await page.locator('.results-panel .el-pager li').filter({hasText:/^2$/}).click();await pagedResponse;assert.equal(requests.at(-1).keyword,'41.2');assert.equal(requests.at(-1).valueMin,'0');
  const filteredDownload=page.waitForEvent('download');await page.getByRole('button',{name:'导出 CSV',exact:true}).click();await filteredDownload;assert.equal(requests.at(-1).export.filters.keyword,'41.2');assert.equal(requests.at(-1).export.filters.valueMin,'0');
  const resetResponse=page.waitForResponse(r=>r.url().includes('/records')&&!r.url().includes('keyword='));await page.getByRole('button',{name:'重置筛选',exact:true}).click();await resetResponse;assert.equal(requests.at(-1).keyword,undefined);
  await page.getByRole('button',{name:'编辑任务',exact:true}).click();await page.getByPlaceholder('例如：噪声设备每日历史查询').fill('已修改任务');await page.getByRole('button',{name:'保存任务',exact:true}).click();await page.getByRole('heading',{name:'已修改任务',exact:true}).waitFor();await page.getByText('共 201 条',{exact:true}).waitFor();
  const download=page.waitForEvent('download');await page.getByRole('button',{name:'导出 CSV',exact:true}).click();assert.match((await download).suggestedFilename(),/历史数据.*\.csv/);assert.equal(requests.at(-1).export.pairs.length,2);
  const output=path.resolve(`../../outputs/history-data-tasks-20261013${process.argv.includes('--deployed')?'-deployed':''}`);fs.mkdirSync(output,{recursive:true});await page.screenshot({path:path.join(output,'desktop.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.waitForTimeout(400);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,'mobile page does not overflow horizontally');
  await page.screenshot({path:path.join(output,'mobile.png'),fullPage:true});
  await page.getByRole('button',{name:'关闭',exact:true}).click();await page.getByRole('dialog',{name:'历史数据任务详情'}).waitFor({state:'hidden'});await page.getByRole('heading',{name:'数据中心',exact:true}).waitFor();
  await page.getByRole('button',{name:'删除',exact:true}).click();await page.getByRole('button',{name:'删除任务',exact:true}).click();await page.getByText('暂无历史数据任务，点击右上角新增',{exact:true}).waitFor();
  assert.equal(errors.length,0,errors.join('\n'));
  console.log('PASS history task browser: task list, create dialog, child ownership/deduplication, detail dialog without navigation, close/reload/reopen, edit, CSV download, delete configuration, mobile layout, no page errors. All APIs mocked.');
 }finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
}
main().catch(e=>{console.error(e);process.exitCode=1});

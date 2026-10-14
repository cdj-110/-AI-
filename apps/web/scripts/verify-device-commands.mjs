import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {mkdir} from 'node:fs/promises'
import {fileURLToPath} from 'node:url'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({root:fileURLToPath(new URL('..',import.meta.url)),configFile:false,plugins:[vue()],optimizeDeps:{entries:['scripts/fixtures/device-commands.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1400,height:1000}}),errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 let status='SENT',posts=0,available=true,task,expectedMetric='setpoint',expectedValue=45.15
 await page.route('**/api/**',async route=>{
  const request=route.request(),path=new URL(request.url()).pathname
  if(!path.startsWith('/api/'))return route.continue()
  assert.ok(path.startsWith('/api/devices/test-child/commands'))
  let data
  if(request.method()==='POST'){
   posts++;const payload=request.postDataJSON()
   assert.equal(payload.metric,expectedMetric);assert.equal(payload.value,expectedValue)
   assert.match(payload.requestId,/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
   task={id:'task-'+posts,...payload,status,message:'正在等待回执',createdAt:new Date().toISOString()};data=task
  }else if(path.endsWith('/capabilities'))data={available,reason:available?'':'设备或网关离线，暂时不能下发',points:[{identifier:'setpoint',name:'目标值',dataType:'NUMBER',unit:'V'},{identifier:'bool-point',name:'设备开关',dataType:'BOOLEAN'}]}
  else data=task?[{...task,status,message:status==='SUCCEEDED'?'网关已执行点位写入':status==='FAILED'?'设备拒绝写入':status==='TIMEOUT'?'30 秒内未收到网关回执，结果未知；不要盲目重试':'正在等待网关回执'}]:[]
  await route.fulfill({json:{code:0,message:'ok',data}})
 })
 await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/device-commands.html`)
 // Explicitly exercise LAN HTTP compatibility: randomUUID is unavailable there.
 await page.evaluate(()=>Object.defineProperty(crypto,'randomUUID',{value:undefined,configurable:true}))
 await page.getByRole('button',{name:'下发',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'点位下发',exact:true})
 const closeDialog=async()=>{await dialog.getByRole('button',{name:'Close this dialog'}).click();await dialog.waitFor({state:'hidden'})}
 assert.equal(await dialog.locator('.el-select').count(),0,'card downlink must lock the chosen point')
 assert.equal(await page.locator('.metric-card').nth(1).getByRole('button',{name:'下发',exact:true}).count(),0)
 await dialog.getByPlaceholder('输入数值（不会自动四舍五入）').fill('45.15')
 await dialog.getByRole('button',{name:'确认下发',exact:true}).click()
 let confirm=page.getByRole('dialog',{name:'确认下发',exact:true})
 await confirm.getByRole('button',{name:'取消',exact:true}).click();assert.equal(posts,0)
 await dialog.getByRole('button',{name:'确认下发',exact:true}).click()
 confirm=page.getByRole('dialog',{name:'确认下发',exact:true});await confirm.getByRole('button',{name:'确认下发',exact:true}).click()
 await page.waitForFunction(()=>document.body.textContent.includes('已提交，请查看回执'));assert.equal(posts,1)
 await page.waitForTimeout(100)
 assert.equal(await dialog.getByPlaceholder('输入数值（不会自动四舍五入）').isDisabled(),true)
 assert.equal(await dialog.getByRole('button',{name:'已提交，请查看回执'}).isDisabled(),true)
 for(const [next,label]of [['SUCCEEDED','执行成功'],['FAILED','执行失败'],['TIMEOUT','结果未确认']]){
  status=next;await dialog.getByRole('button',{name:'查询下发结果'}).click();await dialog.locator('.el-alert').filter({hasText:label}).waitFor();assert.equal(posts,1)
 }
 await mkdir('artifacts/device-commands-20260918',{recursive:true})
 await dialog.screenshot({path:'artifacts/device-commands-20260918/command-results.png'})
 assert.equal(await dialog.locator('.recent-row').count(),1,'dialog should show only a compact recent summary')
 await dialog.getByRole('button',{name:'查看历史 →'}).click()
 await dialog.waitFor({state:'hidden'})
 const historyDialog=page.getByRole('dialog',{name:'下发记录',exact:true})
 await historyDialog.waitFor()
 assert.equal(await historyDialog.locator('.command-history-table .el-table__body tr').count(),1,'full history belongs in a separate dialog')
 await historyDialog.screenshot({path:'artifacts/device-commands-20260918/command-history.png'})
 await historyDialog.getByRole('button',{name:'Close this dialog'}).click()
 await historyDialog.waitFor({state:'hidden'})
 available=false;task=undefined
 await page.getByRole('button',{name:'下发',exact:true}).click()
 await dialog.getByText('设备或网关离线，暂时不能下发',{exact:true}).waitFor()
 assert.equal(await dialog.getByRole('button',{name:'确认下发',exact:true}).isDisabled(),true)
 await closeDialog()
 available=true;await page.evaluate(()=>window.openCommand('read-only-input'))
 await dialog.getByText('该点位当前不可写，请检查网关点位配置',{exact:true}).waitFor()
 assert.equal(await dialog.getByRole('button',{name:'确认下发',exact:true}).isDisabled(),true,'missing requested point must never select another writable point')
 assert.equal(posts,1)
 await closeDialog()
 expectedMetric='bool-point'
 task=undefined;status='SENT';expectedValue=true
 const cardSwitch=page.locator('.boolean-card .el-switch')
 await cardSwitch.waitFor()
 assert.equal(await cardSwitch.count(),1,'writable boolean card should show a switch')
 await cardSwitch.click()
 let boolConfirm=page.getByRole('dialog',{name:'确认切换',exact:true})
 await boolConfirm.getByRole('button',{name:'取消',exact:true}).click()
 assert.equal(posts,1,'cancelled switch must not send a command')
 assert.equal(await cardSwitch.evaluate(element=>element.classList.contains('is-checked')),false,'cancelled switch must retain reported state')
 await cardSwitch.click()
 boolConfirm=page.getByRole('dialog',{name:'确认切换',exact:true})
 assert.ok((await boolConfirm.innerText()).includes('开启（true）'))
 await boolConfirm.getByRole('button',{name:'确认下发',exact:true}).click()
 await page.waitForFunction(()=>document.querySelector('.boolean-card .el-switch')?.classList.contains('is-disabled'))
 assert.equal(posts,2);assert.equal(task.value,true);assert.equal(typeof task.value,'boolean')
 assert.equal(await cardSwitch.evaluate(element=>element.classList.contains('is-checked')),false,'switch must wait for reported telemetry')
 status='SUCCEEDED';await page.evaluate(()=>window.refreshCommands())
 await page.waitForFunction(()=>!document.querySelector('.boolean-card .el-switch')?.classList.contains('is-disabled'))
 assert.equal(await cardSwitch.evaluate(element=>element.classList.contains('is-checked')),false,'receipt alone must not fake live state')
 await page.evaluate(()=>window.setBoolLive(true))
 await page.waitForFunction(()=>document.querySelector('.boolean-card .el-switch')?.classList.contains('is-checked'))
 task=undefined;status='SENT';expectedValue=false
 await cardSwitch.click()
 boolConfirm=page.getByRole('dialog',{name:'确认切换',exact:true})
 assert.ok((await boolConfirm.innerText()).includes('关闭（false）'))
 await boolConfirm.getByRole('button',{name:'确认下发',exact:true}).click()
 assert.equal(posts,3);assert.equal(task.value,false)
 status='FAILED';await page.evaluate(()=>window.refreshCommands())
 await page.waitForFunction(()=>!document.querySelector('.boolean-card .el-switch')?.classList.contains('is-disabled'))
 assert.equal(await cardSwitch.evaluate(element=>element.classList.contains('is-checked')),true,'failed switch must retain reported state')
 await page.evaluate(()=>window.openCommand('bool-point'))
 const modalSwitch=dialog.locator('.el-switch')
 await modalSwitch.waitFor()
 assert.equal(await dialog.locator('.el-radio-button').count(),0,'boolean inputs must use switches, not two radio buttons')
 assert.equal(await dialog.locator('.el-input').count(),0,'boolean inputs must not use text fields')
 assert.equal(await dialog.getByRole('button',{name:'确认下发',exact:true}).isDisabled(),true,'modal switch must require a deliberate change')
 await modalSwitch.click()
 assert.equal(await dialog.getByRole('button',{name:'确认下发',exact:true}).isDisabled(),false)
 await dialog.screenshot({path:'artifacts/device-commands-20260918/boolean-switch.png'})
 await closeDialog()
 assert.deepEqual(errors,[])
 console.log('Cloud command UI passed: card boolean switch, confirmation/cancel, telemetry-backed state, failure rollback, modal switch, LAN HTTP UUID fallback, typed value and duplicate prevention. All API traffic mocked; no physical device writes.')
}finally{await browser?.close();await server.close()}

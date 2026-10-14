import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {mkdir} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const config={resources:[],channels:[],devices:[{deviceKey:'d1',name:'温度设备',protocol:'modbus-tcp',points:[{metric:'temp',name:'温度',dataType:'float64'}]}]}
const base={ruleId:'r1',ruleName:'自动恢复规则',deviceKey:'d1',metric:'temp',level:'WARNING',state:'ACTIVE',acknowledged:false,value:20,unit:'℃',quality:'GOOD',message:'温度超限',occurredAt:new Date().toISOString(),recoveryHint:'恢复需有效值 ≤ 10'}
let active=[{...base,eventId:'e1'},{...base,eventId:'e2',ruleId:'r2',ruleName:'允许手动规则',allowManualRecovery:true}]
let duration=10
let rules=[],saved=[],calls=[],failManual=false,permissions=['config.view','alarms.view','alarms.manage']
const server=await createServer({configFile:false,plugins:[vue()],server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen();browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHANNEL?{channel:process.env.PLAYWRIGHT_CHANNEL}:{})})
 const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[]
 page.on('pageerror',e=>errors.push(e.message))
 await page.route('**/api/**',async route=>{
  const req=route.request(),path=new URL(req.url()).pathname
  let data={}
  if(path==='/api/session')data={username:'admin',permissions}
  else if(path.startsWith('/api/config'))data={content:JSON.stringify(config),largeMode:false}
  else if(path==='/api/alarms')data={popupDurationSeconds:duration,active,activeCount:active.length,unackedCount:active.filter(e=>!e.acknowledged).length,criticalCount:0,history:[]}
  else if(path==='/api/alarms/settings'){if(req.method()==='PUT')duration=req.postDataJSON().popupDurationSeconds;data={popupDurationSeconds:duration}}
  else if(path==='/api/alarm-rules'){if(req.method()==='PUT'){saved=req.postDataJSON().rules;rules=saved}data={rules}}
  else if(path==='/api/alarms/history')data={items:[],page:1,pageSize:50,total:0,totalPages:1}
  else if(path==='/api/point-status')data=[]
  else if(path==='/api/alarms/acknowledge'){calls.push(path);active=active.map(e=>e.eventId===req.postDataJSON().eventId?{...e,acknowledged:true}:e);data={ok:true}}
  else if(path==='/api/alarms/manual-recover'){calls.push(path);if(failManual)return route.fulfill({status:500,body:'保存手动恢复记录失败，告警仍保留'});active=active.filter(e=>e.eventId!==req.postDataJSON().eventId);data={ok:true}}
  return route.fulfill({json:data})
 })
 await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/?page=alarms`)
 const view=page.locator('.alarm-view');await view.waitFor()
 const row=view.locator('tbody tr').filter({hasText:'自动恢复规则'}),manual=view.locator('tbody tr').filter({hasText:'允许手动规则'})
 const names=()=>view.locator('tbody tr td:nth-child(2)').allTextContents()
 const originalOrder=await names()
 active=[...active].reverse().map(e=>({...e,value:21,updatedAt:new Date().toISOString()}))
 await view.getByRole('button',{name:'刷新',exact:true}).click()
 await row.locator('.mono').filter({hasText:'21'}).waitFor()
 assert.deepEqual(await names(),originalOrder,'same-time alarms stay ordered when API order changes')
 assert.equal(await row.locator('.alarm-actions-cell').getByText('仅自动恢复').count(),0)
 assert.ok(await row.locator('.alarm-state-inline').getByText('仅自动恢复',{exact:true}).isVisible())
 const rowHeight=(await row.boundingBox()).height,manualHeight=(await manual.boundingBox()).height
 assert.ok(Math.abs(rowHeight-manualHeight)<2,'automatic recovery hint must not enlarge the row')
 const ackButton=row.getByRole('button',{name:'确认知晓',exact:true}),recoverButton=manual.getByRole('button',{name:'手动恢复',exact:true})
 assert.equal(await ackButton.evaluate(el=>getComputedStyle(el).color),'rgb(29, 78, 216)')
 assert.equal(await recoverButton.evaluate(el=>getComputedStyle(el).color),'rgb(154, 52, 18)')
 await mkdir('artifacts/alarm-recovery',{recursive:true});await page.screenshot({path:'artifacts/alarm-recovery/active-alarm-actions.png'})
 assert.equal(await row.getByRole('button',{name:'手动恢复',exact:true}).count(),0)
 await row.getByRole('button',{name:'确认知晓',exact:true}).click()
 await row.getByRole('button',{name:'已确认',exact:true}).waitFor()
 assert.match(await row.textContent(),/告警中.*已确认/)
 assert.deepEqual(await names(),originalOrder,'acknowledging must not move the row')
 await manual.getByRole('button',{name:'手动恢复',exact:true}).click()
 const confirm=page.getByRole('alertdialog',{name:'手动恢复告警',exact:true})
 await confirm.waitFor();await confirm.getByRole('button',{name:'取消',exact:true}).click()
 assert.equal(calls.filter(p=>p.endsWith('manual-recover')).length,0)
 failManual=true
 await manual.getByRole('button',{name:'手动恢复',exact:true}).click();await confirm.getByRole('button',{name:'手动恢复',exact:true}).click()
 await view.locator('.alarm-message').filter({hasText:'保存手动恢复记录失败'}).waitFor();assert.equal(await manual.count(),1)
 failManual=false
 await manual.getByRole('button',{name:'手动恢复',exact:true}).click();await confirm.getByRole('button',{name:'手动恢复',exact:true}).click();await manual.waitFor({state:'detached'})
 await view.getByRole('button',{name:/告警规则/}).click();await view.getByRole('button',{name:'新增规则',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'告警规则',exact:true}),handle=dialog.locator('h3')
 await dialog.waitFor();assert.equal(await dialog.getByRole('switch',{name:'允许手动恢复',exact:true}).isChecked(),false)
 await mkdir('artifacts/alarm-rule-layout',{recursive:true})
 for(const size of [{width:1920,height:911},{width:1366,height:768},{width:1280,height:720}]){
  await page.setViewportSize(size)
  const dims=await dialog.evaluate(el=>{const body=el.querySelector('.alarm-form');return{scroll:body.scrollHeight,client:body.clientHeight,outerScroll:el.scrollHeight,outerClient:el.clientHeight}})
  assert.ok(dims.scroll<=dims.client+1,`all rule fields fit without scrolling at ${size.width}x${size.height}: ${JSON.stringify(dims)}`)
  assert.ok(dims.outerScroll<=dims.outerClient+1,'outer dialog must not scroll')
  const bounds=await dialog.boundingBox(),save=await dialog.getByRole('button',{name:'确定',exact:true}).boundingBox()
  assert.ok(bounds.x>=0&&bounds.y>=0&&bounds.x+bounds.width<=size.width&&bounds.y+bounds.height<=size.height)
  assert.ok(save.y+save.height<=size.height,'save always visible')
  await page.screenshot({path:`artifacts/alarm-rule-layout/desktop-${size.width}.png`})
 }
 await dialog.getByLabel('阈值模式',{exact:true}).selectOption('single')
 assert.ok(await dialog.locator('.alarm-form').evaluate(el=>el.scrollHeight<=el.clientHeight+1),'legacy single threshold also fits')
 await dialog.getByLabel('阈值模式',{exact:true}).selectOption('limits')
 await page.setViewportSize({width:390,height:780})
 assert.ok(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth),'mobile has no horizontal overflow')
 const mobileSave=await dialog.getByRole('button',{name:'确定',exact:true}).boundingBox()
 assert.ok(mobileSave.y+mobileSave.height<=780,'mobile footer stays visible while form scrolls')
 await dialog.getByRole('switch',{name:/规则状态/}).scrollIntoViewIfNeeded()
 await page.screenshot({path:'artifacts/alarm-rule-layout/mobile.png'})
 await page.setViewportSize({width:1440,height:1000})
 await dialog.locator('.alarm-form').evaluate(el=>el.scrollTop=0)
 assert.equal(await dialog.getByLabel('阈值模式',{exact:true}).inputValue(),'limits')
 assert.equal(await dialog.getByRole('switch',{name:'弹窗提醒',exact:true}).isChecked(),true)
 await dialog.getByRole('switch',{name:'弹窗提醒',exact:true}).uncheck({force:true})
 const help=dialog.getByRole('button',{name:'回差说明',exact:true}),tip=dialog.getByRole('tooltip')
 assert.equal(await tip.count(),0)
 await help.hover();await tip.waitFor();assert.match(await tip.textContent(),/降到 95 或以下/);assert.match(await tip.textContent(),/升到 23 或以上/)
 await tip.hover();assert.ok(await tip.isVisible(),'hoverable explanation stays open')
 await mkdir('artifacts/alarm-recovery',{recursive:true});await page.screenshot({path:'artifacts/alarm-recovery/hysteresis-help.png'})
 await page.mouse.move(20,20);await tip.waitFor({state:'detached'})
 await help.focus();await tip.waitFor();await page.keyboard.press('Escape');await tip.waitFor({state:'detached'})
 await page.keyboard.press('Tab');assert.equal(await dialog.getByLabel('回差',{exact:true}).evaluate(el=>el===document.activeElement),true)
 await dialog.getByLabel('回差',{exact:true}).fill('5')
 await dialog.getByLabel('上限阈值',{exact:true}).fill('10')
 for(const [name,value] of [['下下限','-20'],['下限','-10'],['上上限','20']]){await dialog.getByLabel(`启用${name}`,{exact:true}).check();await dialog.getByLabel(`${name}阈值`,{exact:true}).fill(value)}
 assert.equal(await dialog.getByLabel('上上限告警等级',{exact:true}).inputValue(),'CRITICAL')
 await dialog.getByLabel('下限阈值',{exact:true}).fill('15');await dialog.getByRole('button',{name:'确定',exact:true}).click()
 await dialog.getByRole('alert').filter({hasText:'下限必须小于上限'}).waitFor()
 await dialog.getByLabel('下限阈值',{exact:true}).fill('-10')
 await dialog.getByRole('alert').waitFor({state:'detached'})
 await handle.scrollIntoViewIfNeeded()
 const before=await dialog.boundingBox(),head=await handle.boundingBox()
 await page.mouse.move(head.x+100,head.y+20);await page.mouse.down();await page.mouse.move(head.x+210,head.y+60,{steps:8});await page.mouse.up()
 const after=await dialog.boundingBox();assert.ok(after.x-before.x>100&&after.y>=before.y,'dialog drag moves panel within viewport')
 await dialog.getByLabel('规则名称',{exact:true}).fill('新增手动规则')
 await dialog.getByRole('switch',{name:'允许手动恢复',exact:true}).check({force:true})
 await dialog.evaluate(async el=>{await Promise.all(el.getAnimations({subtree:true}).map(a=>a.finished.catch(()=>{})))})
 assert.equal(await dialog.locator('.alarm-form-state').filter({hasText:'允许手动恢复'}).locator('i').evaluate(el=>getComputedStyle(el).backgroundColor),'rgb(22, 163, 74)')
 await mkdir('artifacts/alarm-recovery',{recursive:true});await page.screenshot({path:'artifacts/alarm-recovery/rule-dialog.png'})
 await dialog.getByRole('button',{name:'确定',exact:true}).click();await view.getByRole('button',{name:'保存规则',exact:true}).click()
 await view.locator('.alarm-message').filter({hasText:'已保存并热加载'}).waitFor();assert.equal(saved[0].allowManualRecovery,true)
 assert.equal(saved.length,1);assert.equal(saved[0].limits.highHigh.threshold,20);assert.equal(saved[0].limits.lowLow.threshold,-20)
 assert.equal(saved[0].popupEnabled,false,'disabled popup persists with rule')
 await view.getByRole('button',{name:'编辑',exact:true}).click();await dialog.waitFor()
 assert.equal(await dialog.getByRole('switch',{name:'弹窗提醒',exact:true}).isChecked(),false)
 assert.equal(await dialog.getByLabel('上上限阈值',{exact:true}).inputValue(),'20');await dialog.getByRole('button',{name:'取消',exact:true}).click()
 await view.getByRole('button',{name:'新增规则',exact:true}).click();await dialog.waitFor()
 assert.ok(Math.abs((await dialog.boundingBox()).x-before.x)<2,'reopened dialog recenters')
 await handle.focus();await page.keyboard.press('ArrowLeft');assert.ok((await dialog.boundingBox()).x<before.x)
 await page.keyboard.press('Home');assert.ok(Math.abs((await dialog.boundingBox()).x-before.x)<2)
 await page.setViewportSize({width:900,height:720});await handle.focus();for(let i=0;i<50;i++)await page.keyboard.press('ArrowRight')
 const small=await dialog.boundingBox();assert.ok(small.x+small.width<=900&&small.y>=0)
 await dialog.getByRole('button',{name:'取消',exact:true}).click();await page.setViewportSize({width:1440,height:1000})
 // Existing events do not toast; a newly arriving unacknowledged event does.
 active.push({...base,eventId:'no-popup',ruleId:'no-popup',ruleName:'不弹窗规则',popupEnabled:false})
 await page.evaluate(()=>window.dispatchEvent(new Event('gateway-alarms-changed')))
 await page.waitForResponse(response=>new URL(response.url()).pathname==='/api/alarms')
 assert.equal(await page.getByRole('article',{name:'告警：不弹窗规则',exact:true}).count(),0)
 active.push({...base,eventId:'toast-new',ruleId:'toast-new',ruleName:'新的告警'})
 await page.evaluate(()=>window.dispatchEvent(new Event('gateway-alarms-changed')))
 const toast=page.getByRole('dialog',{name:'告警面板',exact:true})
 await toast.waitFor();assert.deepEqual(await toast.boundingBox(),{x:0,y:0,width:1440,height:1000})
 await toast.locator('.panel-close').click();await toast.waitFor({state:'detached'})
 assert.ok(active.some(e=>e.eventId==='toast-new'),'closing full-page panel does not recover alarms')
 await view.getByRole('button',{name:'弹窗设置',exact:true}).click()
 await view.getByLabel('显示时长（秒）',{exact:true}).fill('0')
 await view.getByRole('button',{name:'保存弹窗设置',exact:true}).click()
 await view.locator('.alarm-message').filter({hasText:'弹窗设置已保存'}).waitFor();assert.equal(duration,0)
 await view.getByLabel('显示时长（秒）',{exact:true}).fill('1')
 await view.getByRole('button',{name:'保存弹窗设置',exact:true}).click()
 await view.locator('.alarm-message').filter({hasText:'弹窗设置已保存'}).waitFor();assert.equal(duration,1)
 await page.screenshot({path:'artifacts/alarm-recovery/popup-settings.png'})
 permissions=['config.view','alarms.view'];await page.reload();await view.waitFor()
 assert.equal(await view.getByRole('button',{name:'确认知晓',exact:true}).count(),0)
 assert.deepEqual(errors,[])
 console.log('Alarm UI passed: manual policy, acknowledge, cancel/error/success, settings persistence, rule dialog dragging/full-page panel, resize, viewer access')
}finally{await browser?.close();await server.close()}

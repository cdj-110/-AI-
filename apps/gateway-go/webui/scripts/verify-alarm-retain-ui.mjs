import assert from 'node:assert/strict'
import {mkdir} from 'node:fs/promises'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const {expect}=await import(process.env.PLAYWRIGHT_MODULE?new URL('./test.mjs',process.env.PLAYWRIGHT_MODULE).href:'playwright/test')
const config={resources:[],channels:[],devices:[{deviceKey:'d1',name:'温度设备',points:[{metric:'temp',name:'温度',dataType:'float64'}]}]}
const rule={ruleId:'retain',name:'温度保留提醒',enabled:true,deviceKey:'d1',metric:'temp',type:'threshold',operator:'>',threshold:10,level:'CRITICAL',popupEnabled:true,retainAfterAcknowledged:true}
const event=(eventId,extra={})=>({eventId,ruleId:'retain',ruleName:'温度保留提醒',deviceKey:'d1',metric:'temp',pointName:'温度',level:'CRITICAL',state:'ACTIVE',acknowledged:false,popupEnabled:true,retainAfterAcknowledged:true,value:20,quality:'GOOD',message:'温度高于阈值',occurredAt:'2026-09-14T08:00:00Z',...extra})
let active=[event('e1'),event('e2',{ruleId:'legacy',ruleName:'旧规则',retainAfterAcknowledged:false})],rules=[rule],saved=[],calls=[]
const errors=[]
const server=await createServer({configFile:false,plugins:[vue()],server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen()
 browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
 const context=await browser.newContext({viewport:{width:1366,height:768}})
 await context.addInitScript(()=>localStorage.setItem('gateway-alarm-panel-mode-v1:test','page'))
 await context.route('**/api/**',async route=>{
  const req=route.request(),path=new URL(req.url()).pathname
  let data={}
  if(path==='/api/session')data={username:'test',permissions:['config.view','alarms.view','alarms.manage']}
  else if(path.startsWith('/api/config'))data={content:JSON.stringify(config),largeMode:false}
  else if(path==='/api/alarms')data={active,history:[],activeCount:active.length,unackedCount:active.filter(e=>!e.acknowledged).length,criticalCount:active.length}
  else if(path==='/api/alarm-rules'){if(req.method()==='PUT'){saved=req.postDataJSON().rules;rules=saved}data={rules}}
  else if(path==='/api/point-status')data=[]
  else if(path==='/api/alarms/history')data={items:[],page:1,pageSize:50,total:0,totalPages:1}
  else if(path==='/api/alarms/acknowledge'){const id=req.postDataJSON().eventId;calls.push(id);active=active.map(e=>e.eventId===id?{...e,acknowledged:true}:e);data={ok:true}}
  await route.fulfill({json:data})
 })
 const origin=`http://127.0.0.1:${server.httpServer.address().port}`
 const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message))
 await page.goto(origin+'/?page=alarms')
 const activeTable=page.locator('.alarm-view > .alarm-table-wrap table')
 const currentValueCell=activeTable.locator('tbody tr').filter({hasText:'温度保留提醒'}).locator('td').nth(3)
 const initialValueWidth=(await currentValueCell.boundingBox()).width
 active[0]={...active[0],valueText:'123456789012345678901234567890.123456789 kWh'}
 await expect(currentValueCell).toContainText('123456789012345678901234567890.123456789 kWh',{timeout:10000})
 const changedValueWidth=(await currentValueCell.boundingBox()).width
 assert.ok(Math.abs(changedValueWidth-initialValueWidth)<0.5,`current-value column width must stay fixed: ${initialValueWidth} -> ${changedValueWidth}`)
 active[0]={...active[0],valueText:'20'}
 await page.getByRole('button',{name:'打开告警面板',exact:true}).click()
 const card=page.getByRole('article',{name:'告警：温度保留提醒',exact:true}),legacy=page.getByRole('article',{name:'告警：旧规则',exact:true})
 await card.waitFor()
 assert.notEqual(await card.evaluate(el=>getComputedStyle(el).animationName),'none')
 await card.getByRole('button',{name:'确认知晓',exact:true}).click()
 await expect(card.getByText('已知晓 · 等待恢复',{exact:true})).toBeVisible()
 await expect(card.getByRole('button',{name:'已知晓',exact:true})).toBeDisabled()
 assert.equal(await card.evaluate(el=>getComputedStyle(el).animationName),'none')
 await legacy.getByRole('button',{name:'确认知晓',exact:true}).click()
 await expect(legacy).toHaveCount(0)
 const second=await context.newPage();second.on('pageerror',e=>errors.push(e.message))
 await second.goto(origin+'/?alarmPanel=1')
 await expect(second.getByText('已知晓 · 等待恢复',{exact:true})).toBeVisible()
 await page.locator('.panel-close').click()
 await page.getByRole('button',{name:'打开告警面板',exact:true}).click()
 await expect(card.getByText('已知晓 · 等待恢复',{exact:true})).toBeVisible()
 await mkdir('artifacts/alarm-retain',{recursive:true})
 await page.screenshot({path:'artifacts/alarm-retain/acknowledged-card.png'})
 active=[]
 await expect(card).toHaveCount(0,{timeout:10000})
 await expect(second.locator('.alarm-panel-card')).toHaveCount(0,{timeout:10000})
 await second.close()
 await page.locator('.panel-close').click()
 await page.locator('.alarm-view').getByRole('button',{name:/告警规则/}).click()
 const row=page.locator('.alarm-rule-row').filter({hasText:'温度保留提醒'})
 await row.getByRole('button',{name:'编辑',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'告警规则',exact:true})
 await expect(dialog.getByRole('switch',{name:'确认后保留卡片',exact:true})).toBeChecked()
 const statusBox=await dialog.getByRole('switch',{name:/规则状态/}).boundingBox(),nameBox=await dialog.getByLabel('规则名称').boundingBox()
 assert.ok(statusBox.y<nameBox.y,'rule status is shown in the dialog header above basic fields')
 assert.ok(statusBox.x<nameBox.x+300,'rule status is aligned with the left-side fields')
 const panelReminder=dialog.getByRole('switch',{name:'面板提醒',exact:true})
 await panelReminder.locator('..').click()
 const soundOnly=dialog.getByRole('checkbox',{name:'仍开启声音提醒',exact:true})
 await expect(soundOnly).toBeVisible()
 await expect(soundOnly).not.toBeChecked()
 await soundOnly.check()
 await page.screenshot({path:'artifacts/alarm-retain/sound-only-option.png'})
 await panelReminder.locator('..').click()
 await expect(soundOnly).toHaveCount(0)
 for(const size of [{width:1920,height:911},{width:1366,height:768},{width:1280,height:720}]){
  await page.setViewportSize(size)
  const dims=await dialog.locator('.alarm-form').evaluate(el=>({scroll:el.scrollHeight,client:el.clientHeight}))
  assert.ok(dims.scroll<=dims.client+1,`rule form fits ${size.width}x${size.height}: ${JSON.stringify(dims)}`)
 }
 await page.screenshot({path:'artifacts/alarm-retain/rule-switch.png'})
 await dialog.getByRole('switch',{name:'确认后保留卡片',exact:true}).locator('..').click()
 await expect(dialog.getByRole('switch',{name:'确认后保留卡片',exact:true})).not.toBeChecked()
 await dialog.getByRole('button',{name:'确定',exact:true}).click()
 await page.getByRole('button',{name:'保存规则',exact:true}).click()
 await expect.poll(()=>saved.length).toBe(1)
 assert.equal(saved[0].retainAfterAcknowledged,false)
 assert.equal(saved[0].soundWhenPopupDisabled,true)
 await row.getByRole('button',{name:'编辑',exact:true}).click()
 await expect(dialog.getByRole('switch',{name:'确认后保留卡片',exact:true})).not.toBeChecked()
 await dialog.getByRole('switch',{name:'确认后保留卡片',exact:true}).press('Space')
 await expect(dialog.getByRole('switch',{name:'确认后保留卡片',exact:true})).toBeChecked()
 await dialog.getByRole('button',{name:'确定',exact:true}).click()
 await page.getByRole('button',{name:'保存规则',exact:true}).click()
 await expect.poll(()=>saved[0]?.retainAfterAcknowledged).toBe(true)
 assert.deepEqual(calls,['e1','e2'])
 assert.deepEqual(errors,[])
 console.log('PASS browser: main/standalone retained cards, acknowledgement UI, no flash after ack, legacy hide, reopen, recovery, rule editing and save, desktop layout')
}finally{await browser?.close();await server.close()}

import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const {expect}=await import(process.env.PLAYWRIGHT_MODULE?new URL('./test.mjs',process.env.PLAYWRIGHT_MODULE).href:'playwright/test')
const event=(eventId,extra={})=>({eventId,ruleId:'noise',ruleName:'噪声值 告警',deviceKey:'noise-meter-1',metric:'NOISE_DB',pointName:'噪声值',level:'CRITICAL',state:'ACTIVE',acknowledged:false,popupEnabled:true,retainAfterAcknowledged:true,value:42.1,quality:'GOOD',message:'噪声值高于阈值',occurredAt:'2026-09-14T08:00:00Z',...extra})
let active=[event('high',{ruleId:'limits:high',parentRuleId:'limits',parentRuleName:'点位 告警',acknowledged:true}),event('highHigh',{ruleId:'limits:highHigh',parentRuleId:'limits',parentRuleName:'点位 告警',acknowledged:true}),event('noise')],acknowledgedCalls=[]
const server=await createServer({configFile:false,plugins:[vue()],server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:'chrome'})
 const context=await browser.newContext({viewport:{width:1920,height:911}}),errors=[]
 await context.addInitScript(()=>localStorage.setItem('gateway-alarm-panel-mode-v1:test','page'))
 await context.route('**/api/**',async route=>{
  const path=new URL(route.request().url()).pathname
  let data={}
  if(path==='/api/session')data={username:'test',permissions:['config.view','alarms.view','alarms.manage']}
  else if(path.startsWith('/api/config'))data={content:JSON.stringify({resources:[],channels:[],devices:[]}),largeMode:false}
  else if(path==='/api/alarms')data={active,history:[],activeCount:active.length,unackedCount:active.filter(e=>!e.acknowledged).length,criticalCount:active.length}
  else if(path==='/api/alarms/acknowledge'){const id=route.request().postDataJSON().eventId;acknowledgedCalls.push(id);active=active.map(e=>e.eventId===id?{...e,acknowledged:true}:e);data={ok:true}}
  else if(path==='/api/alarm-rules')data={rules:[]}
  else if(path==='/api/point-status')data=[]
  else if(path==='/api/alarms/history')data={items:[],page:1,total:0}
  await route.fulfill({json:data})
 })
 const origin=`http://127.0.0.1:${server.httpServer.address().port}`
 const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message))
 await page.goto(origin+'/?page=alarms')
 await page.getByRole('button',{name:'打开告警面板',exact:true}).click()
 const card=page.getByRole('article',{name:'告警：噪声值 告警',exact:true})
 await card.waitFor()
 const acknowledged=page.getByRole('article',{name:'告警：点位 告警',exact:true})
 await expect(acknowledged.getByRole('button',{name:'已知晓，无需静音',exact:true})).toBeDisabled()
 const grip=card.locator('.alarm-card-drag'),box=await grip.boundingBox()
 await page.mouse.move(box.x+30,box.y+12);await page.mouse.down();await page.mouse.move(box.x+170,box.y+50,{steps:6});await page.mouse.up()
 await card.getByRole('button',{name:'静音此告警',exact:true}).click()
 await expect(card.getByRole('button',{name:'取消静音',exact:true})).toHaveAttribute('aria-pressed','true')
 await expect(card.getByText('已静音',{exact:true})).toBeVisible()
 const muted=()=>page.evaluate(()=>JSON.parse(localStorage.getItem('gateway-alarm-presentation-v1:test')||'{}').muted)
 assert.deepEqual(await muted(),['noise'])
 await expect.poll(async()=>{await page.waitForTimeout(2200);return muted()}).toEqual(['noise'])
 const second=await context.newPage();second.on('pageerror',e=>errors.push(e.message));await second.goto(origin+'/?alarmPanel=1')
 const secondCard=second.getByRole('article',{name:'告警：噪声值 告警',exact:true})
 await expect(secondCard.getByRole('button',{name:'取消静音',exact:true})).toHaveAttribute('aria-pressed','true')
 await secondCard.getByRole('button',{name:'取消静音',exact:true}).click()
 await expect(card.getByRole('button',{name:'静音此告警',exact:true})).toHaveAttribute('aria-pressed','false')
 assert.deepEqual(await muted(),[])
 await card.getByRole('button',{name:'确认知晓',exact:true}).click()
 await expect(card.getByRole('button',{name:'已知晓',exact:true})).toBeDisabled()
 assert.deepEqual(acknowledgedCalls,['noise'])
 active=active.map(e=>e.eventId==='noise'?{...e,eventId:'noise-new',acknowledged:false}:e)
 await expect.poll(async()=>{await page.waitForTimeout(2200);return card.getByRole('button',{name:'静音此告警',exact:true}).count()}).toBe(1)
 await card.getByRole('button',{name:'静音此告警',exact:true}).click()
 assert.deepEqual(await muted(),['noise-new'])
 assert.deepEqual(errors,[])
 console.log('PASS per-card mute/unmute, dragged card, retained acknowledged neighbors, polling and cross-tab sync')
}finally{await browser?.close();await server.close()}

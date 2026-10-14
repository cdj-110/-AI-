import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {mkdir} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const event={eventId:'e1',ruleId:'r1',ruleName:'温度告警',deviceKey:'d1',metric:'temperature',pointName:'温度',level:'CRITICAL',state:'ACTIVE',acknowledged:false,popupEnabled:true,value:80,occurredAt:'2026-09-11T10:00:00Z'}
let active=[],browser
const errors=[],writes=[]
const server=await createServer({configFile:false,plugins:[vue()],server:{host:'127.0.0.1',port:0},logLevel:'error'})
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
 const context=await browser.newContext({viewport:{width:1440,height:900}})
 context.on('page',p=>p.on('pageerror',e=>errors.push(e.message)))
 await context.addInitScript(()=>{
  window.__starts=0
  window.AudioContext=class{state='running';currentTime=0;destination={};resume(){return Promise.resolve()};close(){return Promise.resolve()};createGain(){return {gain:{setValueAtTime(){},exponentialRampToValueAtTime(){}},connect(){},disconnect(){}}};createOscillator(){return {frequency:{setValueAtTime(){}},connect(){},disconnect(){},start(){window.__starts++},stop(){}}}}
 })
 await context.route('**/api/**',async route=>{
  if(route.request().method()!=='GET')writes.push(route.request().url())
  const path=new URL(route.request().url()).pathname
  let data={}
  if(path==='/api/session')data={username:'admin',permissions:['config.view','alarms.view','alarms.manage']}
  else if(path.startsWith('/api/config'))data={content:JSON.stringify({resources:[],channels:[],devices:[]}),largeMode:false}
  else if(path==='/api/alarms')data={active,history:[],activeCount:active.length,unackedCount:active.length,criticalCount:active.length,popupDurationSeconds:0}
  else if(path==='/api/alarm-rules')data={rules:[]}
  else if(path==='/api/alarms/settings')data={popupDurationSeconds:0}
  else if(path==='/api/point-status')data=[]
  await route.fulfill({json:data})
 })
 const page=await context.newPage()
 await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/?page=alarms`);await page.locator('.alarm-view').waitFor()
 const refresh=async p=>{await Promise.all([p.waitForResponse(r=>new URL(r.url()).pathname==='/api/alarms'),p.evaluate(()=>window.dispatchEvent(new Event('gateway-alarms-changed')))]);await p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(resolve)))}
 const panel=page.getByRole('dialog',{name:'告警面板',exact:true})
 active=[event];await refresh(page);await panel.waitFor()
 const mode=panel.getByRole('switch',{name:'新标签页打开告警面板'})
 assert.equal(await mode.getAttribute('aria-checked'),'false')
 await mode.click();assert.equal(await mode.getAttribute('aria-checked'),'true')
 await refresh(page);assert.equal(await panel.count(),1,'switch only changes next opening')
 await panel.locator('.panel-close').click()
 await page.evaluate(()=>{window.__nativeOpen=window.open;window.__openAttempts=0;window.open=()=>{window.__openAttempts++;return null}})
 active=[{...event,eventId:'e2'}];await refresh(page)
 await page.locator('.alarm-tab-notice').waitFor();assert.equal(await panel.count(),0,'blocked popup never falls back to an intrusive modal')
 active=[{...event,eventId:'e3'}];await refresh(page);assert.equal(await page.evaluate(()=>window.__openAttempts),1,'blocked opening is not retried every alarm')
 await page.evaluate(()=>window.open=window.__nativeOpen)
 const [child]=await Promise.all([page.waitForEvent('popup'),page.getByRole('button',{name:'打开告警标签页',exact:true}).click()])
 const childPanel=child.getByRole('dialog',{name:'告警面板',exact:true}),card=child.getByRole('article',{name:'告警：温度告警',exact:true})
 await card.waitFor();assert.ok(child.url().includes('alarmPanel=1'));assert.equal(await child.locator('.main-nav').count(),0,'dedicated tab contains no background application')
 assert.equal(await childPanel.getByRole('switch',{name:'新标签页打开告警面板'}).getAttribute('aria-checked'),'true')
 await page.locator('.alarm-tab-notice').waitFor({state:'detached'})
 await page.getByRole('button',{name:'声音设置',exact:true}).click()
 await page.getByLabel('严重告警颜色').evaluate(el=>{el.value='#7c3aed';el.dispatchEvent(new Event('input',{bubbles:true}))})
 await page.getByRole('button',{name:'保存设置',exact:true}).click()
 await child.waitForFunction(()=>document.documentElement.style.getPropertyValue('--alarm-critical-color')==='#7c3aed')
 assert.equal(await childPanel.locator('.level').first().evaluate(el=>getComputedStyle(el).backgroundColor),'rgb(245, 239, 254)','custom palette syncs into independent tab')
 const parentStarts=await page.evaluate(()=>window.__starts)
 active=[{...event,eventId:'e4'}];await refresh(page);await refresh(child)
 assert.equal(context.pages().length,2,'new alarm reuses the same tab')
 assert.equal(await panel.count(),0)
 await child.waitForFunction(()=>window.__starts>0)
 await refresh(page);assert.equal(await page.evaluate(()=>window.__starts),parentStarts,'main page stops audio while alarm tab owns it')
 await card.getByRole('button',{name:'静音此告警',exact:true}).click()
 await card.getByRole('button',{name:'置顶此告警',exact:true}).click()
 await page.waitForFunction(()=>{const v=JSON.parse(localStorage.getItem('gateway-alarm-presentation-v1:admin'));return v?.muted?.includes('e4')&&v.pinned.length===1})
 await page.reload();await page.locator('.alarm-view').waitFor();await refresh(page)
 assert.equal(context.pages().length,2,'parent reload detects existing alarm tab');assert.equal(await panel.count(),0)
 const starts=await page.evaluate(()=>window.__starts);assert.equal(starts,0,'reloaded main page does not duplicate sound')
 await mkdir('artifacts/alarm-window',{recursive:true});await child.screenshot({path:'artifacts/alarm-window/independent.png'})
 await childPanel.getByRole('switch',{name:'新标签页打开告警面板'}).click()
 await page.waitForFunction(()=>localStorage.getItem('gateway-alarm-panel-mode-v1:admin')==='page')
 await Promise.all([child.waitForEvent('close'),childPanel.locator('.panel-close').click()])
 active=[{...event,eventId:'e5'}];await refresh(page);await panel.waitFor()
 assert.equal(await mode.getAttribute('aria-checked'),'false','mode change synchronizes back to main')
 assert.equal(await panel.getByRole('button',{name:'取消置顶',exact:true}).getAttribute('aria-pressed'),'true','pin preference is shared')
 await mode.click();await panel.locator('.panel-close').click()
 const [child2]=await Promise.all([page.waitForEvent('popup'),page.getByRole('button',{name:'打开告警面板',exact:true}).click()]);await child2.getByRole('dialog',{name:'告警面板',exact:true}).waitFor()
 await child2.close()
 active=[{...event,eventId:'e6'}];await refresh(page);await refresh(page)
 assert.equal(context.pages().length,1,'closing alarm tab suppresses repeated reopening');assert.equal(await panel.count(),0)
 await page.reload();await page.locator('.alarm-view').waitFor();await refresh(page)
 active=[{...event,eventId:'e7'}];await refresh(page);assert.equal(context.pages().length,1,'closed state survives main reload')
 assert.deepEqual(writes,[]);assert.deepEqual(errors,[])
 console.log('Alarm window UI passed: next-opening switch, persistence, blocked popup notice, singleton reuse, no forced modal, parent reload, shared mute/pin, sound owner, mode switching and close suppression')
}finally{await browser?.close();await server.close()}

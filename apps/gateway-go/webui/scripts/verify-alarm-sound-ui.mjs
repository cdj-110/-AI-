import assert from 'node:assert/strict'
import {mkdir} from 'node:fs/promises'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const high={eventId:'h1',ruleId:'r1::high',parentRuleId:'r1',parentRuleName:'温度告警',ruleName:'温度上限',deviceKey:'d1',metric:'temperature',pointName:'温度',level:'WARNING',state:'ACTIVE',acknowledged:false,value:90,occurredAt:'2026-09-11T10:00:00Z'}
const hh={...high,eventId:'hh1',ruleId:'r1::highHigh',level:'CRITICAL'}
const other={...high,eventId:'o1',ruleId:'r2',parentRuleId:undefined,parentRuleName:undefined,ruleName:'湿度告警',metric:'humidity'}
let active=[],browser,username='admin'
const writes=[],errors=[]
const server=await createServer({configFile:false,plugins:[vue()],server:{host:'127.0.0.1',port:0},logLevel:'error'})
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
 const page=await browser.newPage({viewport:{width:1440,height:900}})
 page.on('pageerror',e=>errors.push(e.message))
 await page.addInitScript(()=>{
  window.__sound={starts:[],stops:0,allow:false}
  window.AudioContext=class{
   state='suspended';currentTime=0;destination={}
   resume(){if(!window.__sound.allow)return Promise.reject(new Error('blocked'));this.state='running';return Promise.resolve()}
   close(){this.state='closed';return Promise.resolve()}
   createGain(){return {gain:{setValueAtTime(){},exponentialRampToValueAtTime(){}},connect(){},disconnect(){}}}
   createOscillator(){let hz;return {frequency:{setValueAtTime(v){hz=v}},connect(){},disconnect(){},start(){window.__sound.starts.push(hz)},stop(){window.__sound.stops++}}}
  }
 })
 await page.route('**/api/**',async route=>{
  const path=new URL(route.request().url()).pathname
  if(route.request().method()!=='GET')writes.push(path)
  let data={}
  if(path==='/api/session')data={username,permissions:['config.view','alarms.view','alarms.manage']}
  else if(path.startsWith('/api/config'))data={content:JSON.stringify({resources:[],channels:[],devices:[]}),largeMode:false}
  else if(path==='/api/alarms')data={active,history:[],activeCount:active.length,unackedCount:active.filter(e=>!e.acknowledged).length,criticalCount:active.filter(e=>e.level==='CRITICAL').length,popupDurationSeconds:0}
  else if(path==='/api/alarm-rules')data={rules:[]}
  else if(path==='/api/alarms/settings')data={popupDurationSeconds:0}
  else if(path==='/api/point-status')data=[]
  await route.fulfill({json:data})
 })
 await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/?page=alarms`)
 await page.locator('.alarm-view').waitFor()
 await page.clock.install()
 const refresh=async()=>{await Promise.all([page.waitForResponse(r=>new URL(r.url()).pathname==='/api/alarms'),page.evaluate(()=>window.dispatchEvent(new Event('gateway-alarms-changed')))]);await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(resolve)))}
 const count=()=>page.evaluate(()=>window.__sound.starts.length)
 const panel=page.getByRole('dialog',{name:'告警面板',exact:true}),card=page.getByRole('article',{name:'告警：温度告警',exact:true}),neighbour=page.getByRole('article',{name:'告警：湿度告警',exact:true})
 active=[high,hh,other];await refresh();await card.waitFor()
 assert.equal(await card.locator('.card-shortcuts button svg').count(),2,'mute and pin use icons')
 assert.equal((await card.locator('.card-shortcuts').textContent()).trim(),'','shortcut buttons have no visible text')
 assert.ok((await card.getByRole('button',{name:'静音此告警',exact:true}).getAttribute('title')).startsWith('静音此告警'))
 assert.equal(await count(),0,'autoplay is not assumed')
 await panel.getByRole('button',{name:'启用声音',exact:true}).waitFor()
 await page.evaluate(()=>window.__sound.allow=true)
 await panel.getByRole('button',{name:'启用声音',exact:true}).click()
 assert.equal(await count(),4,'highest severity plays one pattern, not overlapping rules')
 let n=await count();await page.clock.fastForward(2100);assert.ok(await count()>n,'loop continues without new events')
 await card.getByRole('button',{name:'静音此告警',exact:true}).click()
 assert.equal(await card.getByRole('button',{name:'取消静音',exact:true}).getAttribute('aria-pressed'),'true')
 assert.ok((await card.getByRole('button',{name:'取消静音',exact:true}).getAttribute('title')).startsWith('取消静音'))
 await neighbour.getByRole('button',{name:'静音此告警',exact:true}).click()
 n=await count();await page.clock.fastForward(4500);assert.equal(await count(),n,'muting all groups stops loop immediately')
 await panel.locator('.panel-close').click()
 await page.getByRole('button',{name:'声音设置',exact:true}).click()
 const settings=page.getByRole('region',{name:'告警声音设置'})
 await settings.waitFor();assert.equal(await settings.getByLabel('严重播放方式').inputValue(),'loop')
 assert.equal(await page.locator('.alarm-tabs button.active').textContent(),'声音设置','sound tab is selected')
 await settings.getByLabel('提示提示音',{exact:true}).selectOption('buzzer')
 await settings.getByLabel('提示告警音量').fill('25')
 await settings.getByLabel('警告提示音',{exact:true}).selectOption('bell')
 assert.equal(await settings.getByLabel('严重提示音',{exact:true}).inputValue(),'industrial','changing another level does not overwrite severe sound')
 for(const [label,tones] of [['提示试听',1],['警告试听',2],['严重试听',4]]){const before=await count();await settings.getByRole('button',{name:label,exact:true}).click();assert.equal(await count(),before+tones,'preview uses its own severity and tone');await settings.getByRole('button',{name:'停止试听',exact:true}).click()}
 const beforeColor=await count()
 await settings.getByLabel('严重告警颜色').evaluate(el=>{el.value='#7c3aed';el.dispatchEvent(new Event('input',{bubbles:true}))})
 assert.equal(await count(),beforeColor,'changing color never plays sound')
 await page.getByRole('button',{name:/^当前告警/}).click()
 await page.getByRole('alertdialog').getByRole('button',{name:'保存并离开',exact:true}).click()
 assert.equal(await page.locator('.alarm-view .level.critical').first().evaluate(el=>getComputedStyle(el).backgroundColor),'rgb(245, 239, 254)','list badge uses custom palette')
 await page.getByRole('button',{name:'声音设置',exact:true}).click()
 await settings.getByLabel('严重提示音',{exact:true}).selectOption('bell')
 await settings.getByLabel('严重告警音量').fill('35')
 await settings.getByRole('button',{name:'保存设置',exact:true}).click()
 n=await count();await settings.getByRole('button',{name:'严重试听',exact:true}).click();assert.equal(await count(),n+2)
 await settings.getByRole('button',{name:'停止试听',exact:true}).click()
 await page.getByRole('button',{name:'打开告警面板',exact:true}).click()
 const colors=await card.evaluate(el=>{const a=el.getAnimations()[0];a.pause();a.currentTime=1000;const s=getComputedStyle(el),result={border:s.borderTopColor,background:s.backgroundColor};a.play();return result})
 assert.equal(colors.border,'rgb(124, 58, 237)');assert.equal(colors.background,'rgb(255, 255, 255)','only custom border pulses')
 await card.getByRole('button',{name:'取消静音',exact:true}).click()
 n=await count();await panel.locator('.panel-close').click();await page.clock.fastForward(2100);assert.ok(await count()>n,'closing panel does not mute')
 await settings.getByLabel('警告播放方式').selectOption('once')
 await settings.getByLabel('严重播放方式').selectOption('once')
 await settings.getByRole('button',{name:'保存设置',exact:true}).click()
 n=await count();await page.clock.fastForward(6000);assert.equal(await count(),n,'once mode never loops on unchanged polls')
 active=active.map(e=>e.eventId==='hh1'?{...e,acknowledged:true}:e);await refresh();assert.equal(await count(),n,'once mode does not replay on severity downgrade or acknowledgment')
 await settings.getByLabel('严重播放方式').selectOption('loop')
 await settings.getByLabel('告警声音开关').selectOption('off')
 await settings.getByRole('button',{name:'保存设置',exact:true}).click()
 n=await count();await page.clock.fastForward(4100);assert.equal(await count(),n,'global switch stops ongoing sound')
 await settings.getByLabel('告警声音开关').selectOption('on')
 await settings.getByLabel('警告声音开关').uncheck()
 await settings.getByLabel('提示声音开关').uncheck()
 await settings.getByRole('button',{name:'保存设置',exact:true}).click()
 active=[high,other];await refresh();n=await count();await page.clock.fastForward(2100);assert.equal(await count(),n,'critical recovery silences critical-only mode')
 active=[high,{...hh,eventId:'hh2'},other];await refresh();await card.waitFor()
 assert.ok(await count()>n,'new severe episode sounds again')
 await card.getByRole('button',{name:'静音此告警',exact:true}).click()
 active=[{...high,acknowledged:true},{...hh,eventId:'hh2',acknowledged:true},other];await refresh();await card.waitFor({state:'detached'})
 active=[{...high,eventId:'h3'},{...hh,eventId:'hh3'},other];await refresh();await card.waitFor()
 assert.equal(await card.getByRole('button',{name:'静音此告警',exact:true}).getAttribute('aria-pressed'),'false','new episode is not silenced by previous mute')
 await panel.locator('.panel-close').click();await page.getByRole('button',{name:'打开告警面板',exact:true}).click();await neighbour.waitFor()
 await card.getByRole('button',{name:'置顶此告警',exact:true}).click()
 assert.equal(await panel.locator('.alarm-panel-card').first().getAttribute('aria-label'),'告警：温度告警')
 const handle=card.getByRole('button',{name:'拖动告警卡片：温度告警',exact:true})
 await handle.focus();await handle.press('ArrowDown');await handle.press('ArrowRight')
 const position=await card.boundingBox()
 await neighbour.getByRole('button',{name:'拖动告警卡片：湿度告警',exact:true}).press('ArrowDown')
 assert.ok(await card.locator('..').evaluate(el=>Number(getComputedStyle(el).zIndex))>await neighbour.locator('..').evaluate(el=>Number(getComputedStyle(el).zIndex)),'ordinary dragging cannot cover pinned card')
 await card.getByRole('button',{name:'静音此告警',exact:true}).click()
 await page.reload();await page.locator('.alarm-view').waitFor();await page.getByRole('button',{name:'打开告警面板',exact:true}).locator('b').waitFor()
 await page.getByRole('button',{name:'打开告警面板',exact:true}).click();await card.waitFor()
 assert.equal(await card.getByRole('button',{name:'取消置顶',exact:true}).getAttribute('aria-pressed'),'true','pin restored after reload')
 assert.equal(await card.getByRole('button',{name:'取消静音',exact:true}).getAttribute('aria-pressed'),'true','same episode stays muted after reload')
 await page.waitForFunction(p=>{const r=document.querySelector('[aria-label="告警：温度告警"]')?.getBoundingClientRect();return r&&Math.abs(r.x-p.x)<2&&Math.abs(r.y-p.y)<2},position)
 await mkdir('artifacts/alarm-sound',{recursive:true});await page.screenshot({path:'artifacts/alarm-sound/cards.png'})
 await card.getByRole('button',{name:'取消置顶',exact:true}).click()
 await panel.locator('.panel-close').click();await page.getByRole('button',{name:'声音设置',exact:true}).click()
 assert.equal(await settings.getByLabel('严重提示音',{exact:true}).inputValue(),'bell');assert.equal(await settings.getByLabel('严重告警音量').inputValue(),'35')
 assert.equal(await settings.getByLabel('提示提示音',{exact:true}).inputValue(),'buzzer');assert.equal(await settings.getByLabel('提示告警音量').inputValue(),'25')
 assert.equal(await settings.getByLabel('严重告警颜色').inputValue(),'#7c3aed','custom color survives reload')
 await page.screenshot({path:'artifacts/alarm-sound/settings.png'})
 await settings.getByRole('button',{name:'严重恢复默认',exact:true}).click()
 await settings.getByRole('button',{name:'保存设置',exact:true}).click()
 assert.equal(await settings.getByLabel('严重告警颜色').inputValue(),'#e5484d')
 assert.equal(await settings.getByLabel('提示告警音量').inputValue(),'25','reset only affects the selected level')
 active=[];await refresh();n=await count();await page.clock.fastForward(4100);assert.equal(await count(),n,'all recovered has no stale sound')
 username='operator';await page.reload();await page.locator('.alarm-view').waitFor();await page.getByRole('button',{name:'声音设置',exact:true}).click()
 assert.equal(await settings.getByLabel('严重提示音',{exact:true}).inputValue(),'industrial','preferences isolated by account')
 assert.deepEqual(writes,[],'sound, mute and pin controls never change backend state')
 assert.deepEqual(errors,[])
 console.log('Alarm sound UI passed: separate tab, autoplay unlock, loop/once, tone/volume/scope/switch, preview, per-episode mute/unmute, recovery/ack, pin/stacking/position persistence, account isolation and no business writes')
}finally{await browser?.close();await server.close()}

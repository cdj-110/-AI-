import assert from 'node:assert/strict'
import {createServer} from 'vite'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({optimizeDeps:{entries:['scripts/fixtures/global-draft-guard.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen()
 browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
 const page=await browser.newPage({viewport:{width:1600,height:1000}}),errors=[]
 await page.routeWebSocket('**/api/ws',()=>{})
 page.on('pageerror',error=>errors.push(error.message))
 const base=`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/global-draft-guard.html`
 const modal=page.locator('.global-confirm-modal')
 const cases=[
  {view:'ai',prepare:async()=>page.getByRole('button',{name:'模型设置',exact:true}).click()},
  {view:'wifi'}, {view:'cellular'}, {view:'maintenance',tabs:'.maintenance-tabs button'},
  {view:'users',tabs:'#panel-users .sub-tabs button',prepare:async()=>page.getByRole('button',{name:'编辑',exact:true}).first().click()},
  {view:'cloud',tabs:'#panel-cloud [role=tab]'},
  {view:'storage',tabs:'.history-tabs button',prepare:async()=>page.getByRole('tab',{name:'存储设置',exact:true}).click(),numeric:true},
  {view:'remote',numeric:true}, {view:'edge',tabs:'.edge-editor-tabs button'},
  {view:'alarms',tabs:'#panel-alarms .sub-tabs button',prepare:async()=>{await page.getByRole('button',{name:/^告警规则/}).click();await page.getByRole('button',{name:'编辑',exact:true}).first().click()}},
 ]
 for(const item of cases){
  await page.goto(base+'?view='+item.view)
  await page.waitForTimeout(350)
  await item.prepare?.()
  await page.locator('#navigate').click()
  await page.waitForFunction(()=>document.querySelector('#left').textContent==='1')
  assert.equal(await modal.count(),0,`${item.view}: opening editor or automatic defaults is not an edit`)
  const field=page.locator(item.numeric?'input[type=number]:not(:disabled):visible':'input:not([type=checkbox]):not([type=number]):not([type=password]):not([type=hidden]):not(:disabled):not([readonly]):visible').first()
  const old=await field.inputValue(),edited=item.numeric?String(Number(old)+1):old+'编辑'
  await field.fill(edited)
  await page.locator('#navigate').click();await modal.waitFor()
  await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
  assert.equal(await page.locator('#left').textContent(),'1')
  assert.equal(await field.inputValue(),edited)
  await page.evaluate(()=>window.failSave=true)
  await page.locator('#navigate').click();await modal.waitFor()
  await modal.getByRole('button',{name:'保存修改并继续',exact:true}).click()
  await page.waitForTimeout(220)
  assert.equal(await page.locator('#left').textContent(),'1',`${item.view}: failed save must not navigate`)
  if(['users','alarms'].includes(item.view))assert.ok((await page.locator('main').textContent()).includes(edited),`${item.view}: failed save keeps committed editor in page draft`)
  else assert.equal(await field.inputValue(),edited,`${item.view}: failed save must keep draft`)
  await page.evaluate(()=>window.failSave=false)
  await page.locator('#navigate').click();await modal.waitFor()
  await modal.getByRole('button',{name:'放弃修改并离开',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('#left').textContent==='2')
  if(item.view==='users'||item.view==='alarms')await item.prepare()
  const nextField=page.locator(item.numeric?'input[type=number]:not(:disabled):visible':'input:not([type=checkbox]):not([type=number]):not([type=password]):not([type=hidden]):not(:disabled):not([readonly]):visible').first()
  assert.equal(await nextField.inputValue(),old)
  await nextField.fill(edited)
  await page.locator('#navigate').click();await modal.waitFor()
  await modal.getByRole('button',{name:'保存修改并继续',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('#left').textContent==='3')
  await page.locator('#navigate').click()
  await page.waitForFunction(()=>document.querySelector('#left').textContent==='4')
  assert.equal(await modal.count(),0,`${item.view}: saved draft must be clean`)
  console.log(`${item.view}: clean/cancel/save failure/discard/save success verified`)
  if(item.tabs){
   await page.goto(base+'?view='+item.view);await page.waitForTimeout(200);await item.prepare?.()
   const field2=page.locator(item.numeric?'input[type=number]:not(:disabled):visible':'input:not([type=checkbox]):not([type=number]):not([type=password]):not([type=hidden]):not(:disabled):not([readonly]):visible').first()
   await field2.fill(edited)
   // Editors overlay tabs; test page navigation there instead of clicking through a modal.
   if(!['users','alarms'].includes(item.view)){
    const inactive=page.locator(item.tabs+':not(.active)').first()
    await inactive.click();await modal.waitFor()
    await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
    assert.equal(await field2.inputValue(),edited)
    await inactive.click();await modal.waitFor()
    await modal.getByRole('button',{name:'放弃修改并离开',exact:true}).click()
    await page.waitForTimeout(150)
    assert.equal(await modal.count(),0)
    console.log(`${item.view}: internal tab switch protected`)
   }
  }
 }
 for(const item of cases.filter(item=>['wifi','cellular','cloud','storage','edge'].includes(item.view))){
  await page.goto(base+'?view='+item.view);await page.waitForTimeout(300);await item.prepare?.()
  const reload=page.getByRole('button',{name:'重新加载',exact:true})
  if(!await reload.count())continue
  const field=page.locator(item.numeric?'input[type=number]:not(:disabled):visible':'input:not([type=checkbox]):not([type=number]):not([type=password]):not([type=hidden]):not(:disabled):not([readonly]):visible').first()
  const old=await field.inputValue(),edited=item.numeric?String(Number(old)+1):old+'编辑'
  await field.fill(edited);await reload.click();await modal.waitFor()
  await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
  assert.equal(await field.inputValue(),edited)
  await reload.click();await modal.waitFor();await modal.getByRole('button',{name:'放弃修改并离开',exact:true}).click()
  await page.waitForTimeout(150);assert.equal(await field.inputValue(),old)
  console.log(`${item.view}: reload protects pending edits`)
 }
 assert.deepEqual(errors,[])
}finally{await browser?.close();await server.close()}

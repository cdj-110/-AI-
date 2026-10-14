import assert from 'node:assert/strict'
import {createServer} from 'vite'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({optimizeDeps:{entries:['scripts/fixtures/config-draft-guard.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
 const page=await browser.newPage({viewport:{width:1600,height:1000}}),errors=[]
 await page.routeWebSocket('**/api/ws',()=>{});page.on('pageerror',error=>errors.push(error.message))
 const base=`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/config-draft-guard.html`
 for(const protocol of ['modbus-rtu','opcda','iec104','modbus-tcp-slave','iec61850-mms-server']){
  await page.goto(base+'?protocol='+protocol);await page.locator('.legacy-resource-workspace').waitFor()
  await page.locator('#edit').click();await page.locator('#device').click();await page.locator('.legacy-device-workspace').waitFor()
  const tabs=page.locator('.device-tabs button'),modal=page.locator('.global-confirm-modal')
  await tabs.first().click()
  await tabs.nth(1).click();await tabs.first().click()
  await page.locator('#leave').click();await page.waitForFunction(()=>document.querySelector('#left').textContent==='1')
  assert.equal(await modal.count(),0,'opening protocol forms and browsing tabs must stay clean')
  const field=page.locator('.legacy-device-workspace input:not(:disabled):not([readonly]):not([type=number]):not([type=checkbox]):visible').first()
  const old=await field.inputValue()
  await field.fill(old+'编辑')
  await tabs.nth(1).click();await modal.waitFor()
  await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
  assert.ok((await tabs.first().getAttribute('class'))?.includes('active'))
  assert.equal(await field.inputValue(),old+'编辑')
  await page.evaluate(()=>window.failSave=true)
  await tabs.nth(1).click();await modal.waitFor();await modal.getByRole('button',{name:'保存修改并继续',exact:true}).click()
  await page.waitForTimeout(160)
  assert.ok((await tabs.first().getAttribute('class'))?.includes('active'),'failed save must keep device tab')
  assert.equal(await field.inputValue(),old+'编辑')
  await page.evaluate(()=>window.failSave=false)
  await tabs.nth(1).click();await modal.waitFor();await modal.getByRole('button',{name:'保存修改并继续',exact:true}).click()
  await page.waitForFunction(()=>document.querySelectorAll('.device-tabs button')[1].classList.contains('active'))
  assert.equal(await page.evaluate(()=>window.saveCount),2)
  assert.equal((await page.evaluate(()=>window.draftState())).devices.concat((await page.evaluate(()=>window.draftState())).forwardDevices).find(item=>item.deviceKey==='device').name,old+'编辑')
  assert.equal(await modal.count(),0)
  // Edit a point, then attempt the reverse tab switch and a different workspace.
  const pointName=page.locator('.legacy-device-workspace tbody .point-name-cell input:not(:disabled):visible').first()
  await pointName.fill('点位实际修改')
  await tabs.first().click();await modal.waitFor();await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
  assert.ok((await tabs.nth(1).getAttribute('class'))?.includes('active'))
  await page.locator('#channel').click();await modal.waitFor();await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
  assert.equal(await page.locator('.legacy-device-workspace').count(),1)
  await page.locator('#channel').click();await modal.waitFor();await modal.getByRole('button',{name:'放弃修改并离开',exact:true}).click()
  await page.locator('.legacy-channel-workspace').waitFor()
  assert.notEqual((await page.evaluate(()=>window.draftState())).devices.concat((await page.evaluate(()=>window.draftState())).forwardDevices).find(item=>item.deviceKey==='device').points[0].name,'点位实际修改')
  console.log(`${protocol}: device/point tab switches, failed/successful save and workspace selection verified`)
 }
 assert.deepEqual(errors,[])
}finally{await browser?.close();await server.close()}

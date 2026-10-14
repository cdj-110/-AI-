import assert from 'node:assert/strict'
import {createServer} from 'vite'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({optimizeDeps:{entries:['scripts/fixtures/config-draft-guard.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
 const page=await browser.newPage({viewport:{width:1600,height:1000}}),errors=[]
 await page.routeWebSocket('**/api/ws',()=>{});page.on('pageerror',error=>errors.push(error.message))
 const base=`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/config-draft-guard.html?large&protocol=`
 for(const protocol of ['modbus-rtu','opcda','iec104','modbus-tcp-slave','iec61850-mms-server']){
  await page.goto(base+protocol);await page.locator('.legacy-resource-workspace').waitFor()
  await page.locator('#edit').click();await page.locator('#device').click()
  const table=page.locator('.large-device-workspace'),modal=page.locator('.global-confirm-modal'),tabs=table.locator('.device-tabs button')
  await table.locator('.metric-inline-trigger').first().click()
  await table.locator('.metric-inline-editor input').fill('改动后的标识符')
  await tabs.first().click();await modal.waitFor();await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
  assert.equal(await table.locator('.metric-inline-editor input').inputValue(),'改动后的标识符')
  await page.evaluate(()=>window.failSave=true)
  await tabs.first().click();await modal.waitFor();await modal.getByRole('button',{name:'保存修改并继续',exact:true}).click();await page.waitForTimeout(150)
  assert.ok((await tabs.nth(1).getAttribute('class'))?.includes('active'))
  await page.evaluate(()=>window.failSave=false)
  await page.locator('#device2').click();await modal.waitFor();await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
  assert.ok((await table.textContent()).includes('采集设备1')||(await table.textContent()).includes('转发设备1'))
  await page.locator('#device2').click();await modal.waitFor();await modal.getByRole('button',{name:'放弃修改并离开',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('.large-device-workspace .device-workspace-head h3')?.textContent==='另一台设备')
  await page.locator('#device').click();await table.locator('.metric-inline-trigger').first().click()
  await table.locator('.metric-inline-editor input').fill('新的标识符')
  await tabs.first().click();await modal.waitFor();await modal.getByRole('button',{name:'保存修改并继续',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('.large-device-workspace .device-tabs button')?.classList.contains('active'))
  await page.locator('#leave').click();await page.waitForFunction(()=>document.querySelector('#left').textContent==='1')
  assert.equal(await modal.count(),0,'persisted point counters must not count as unsaved metadata')
  console.log(`${protocol}: large-table pending metric edit, failure/cancel/discard/save/device change verified`)
 }
 // OPC DA has editable row fields in addition to the explicit metric editor.
 await page.goto(base+'opcda');await page.locator('.legacy-resource-workspace').waitFor();await page.locator('#edit').click();await page.locator('#device').click()
 const table=page.locator('.large-device-workspace'),modal=page.locator('.global-confirm-modal')
 await table.locator('tbody .opcda-access-column select').first().selectOption('read_write')
 await table.locator('.device-tabs button').first().click();await modal.waitFor();await modal.getByRole('button',{name:'保存修改并继续',exact:true}).click()
 await page.waitForFunction(()=>document.querySelector('.large-device-workspace .device-tabs button')?.classList.contains('active'))
 await table.locator('.device-tabs button').nth(1).click()
 assert.equal(await table.locator('tbody .opcda-access-column select').first().inputValue(),'read_write')
 await table.locator('.metric-inline-trigger').first().click();await table.locator('.metric-inline-editor input').fill('待保存的标识符')
 await table.getByRole('button',{name:'重新加载',exact:true}).click();await modal.waitFor()
 await modal.getByRole('button',{name:'继续编辑',exact:true}).click()
 assert.equal(await table.locator('.metric-inline-editor input').inputValue(),'待保存的标识符')
 await table.getByRole('button',{name:'重新加载',exact:true}).click();await modal.waitFor()
 await modal.getByRole('button',{name:'放弃修改并离开',exact:true}).click();await page.waitForTimeout(200)
 assert.equal(await table.locator('.metric-inline-editor').count(),0)
 assert.deepEqual(errors,[])
 console.log('OPC DA row draft is saved before tab navigation')
}finally{await browser?.close();await server.close()}

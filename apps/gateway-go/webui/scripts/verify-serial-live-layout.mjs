import assert from 'node:assert/strict'
import {mkdir} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const browser=await chromium.launch({headless:true,channel:'chrome'})
try{
 const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 await page.goto((process.env.WK_SERIAL_TEST_URL||'http://127.0.0.1:5199')+'/scripts/fixtures/serial-live-regression.html')
 const help=page.getByRole('button',{name:'协议说明：该串口总线下所有采集设备统一使用此协议',exact:true})
 await help.waitFor()
 assert.match(await help.getAttribute('title'),/统一使用此协议/)
 assert.equal(await page.locator('.legacy-field-grid small').count(),0,'explanatory text still occupies a form row')
 const bounds=await page.locator('.legacy-field-grid>label').evaluateAll(labels=>labels.slice(0,4).map(label=>{const b=label.querySelector('input,select').getBoundingClientRect();return{top:b.top,height:b.height}}))
 assert.ok(bounds.every(b=>Math.abs(b.top-bounds[0].top)<1),'serial form first row is not aligned: '+JSON.stringify(bounds))
 await page.evaluate(()=>{document.documentElement.dataset.gatewayDetail='device'})
 await page.getByRole('button',{name:/点位配置/}).click()
 await page.getByRole('button',{name:/保持寄存器 03/}).click()
 const cell=page.locator('.legacy-device-workspace .point-table-row .point-live-value').first()
 await cell.waitFor()
 const width=(await cell.boundingBox()).width
 await page.evaluate(()=>{window.__visibleProbeValues=[];const cell=document.querySelector('.point-table-row .point-live-value');new MutationObserver(()=>window.__visibleProbeValues.push(cell.textContent.trim())).observe(cell,{subtree:true,childList:true,characterData:true})})
 for(const value of [43.300000000000004,43.3,43.400000000000006,43.4,43.3]){
  await page.evaluate(value=>window.updateProbeValue(value),value)
  await page.waitForTimeout(80)
  assert.ok(['43.3','43.4'].includes((await cell.innerText()).trim()),'raw floating tail displayed')
  assert.ok(Math.abs((await cell.boundingBox()).width-width)<1,'live value update changed column width')
 }
 assert.ok((await page.evaluate(()=>window.__visibleProbeValues)).every(value=>['43.3','43.4'].includes(value)),'a transient floating tail was rendered')
 await mkdir('artifacts/serial-live-layout',{recursive:true})
 await page.screenshot({path:'artifacts/serial-live-layout/desktop.png',fullPage:true})
 await page.setViewportSize({width:390,height:844})
 assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'mobile layout overflows')
 await page.screenshot({path:'artifacts/serial-live-layout/mobile.png',fullPage:true})
 assert.deepEqual(errors,[])
 console.log('Serial layout and real-time display passed: tooltip, aligned fields, transient update observer, stable column widths and mobile.')
}finally{await browser.close()}

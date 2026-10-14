import assert from 'node:assert/strict'
import {mkdir,writeFile} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
const out='artifacts/license-layout-20260917'
try{
 const page=await browser.newPage({viewport:{width:1920,height:911}}),errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 await page.goto('http://127.0.0.1:5196/scripts/fixtures/license-management.html')
 const button=page.getByRole('button',{name:'更换授权文件',exact:true})
 await button.waitFor()
 assert.equal(await page.locator('.license-feature').count(),9)
 assert.ok((await page.locator('.license-feature-list').innerText()).includes('远程 SSH'))
 const choose=page.waitForEvent('filechooser');await button.click();await (await choose).setFiles([])
 await page.getByRole('button',{name:'刷新授权状态',exact:true}).click()
 await page.waitForFunction(()=>{const button=document.querySelector('.device-license-panel .primary');return !!button&&!button.disabled})
 await mkdir(out,{recursive:true})
 await page.screenshot({path:out+'/desktop.png',fullPage:true})
 await page.locator('.device-license-panel').screenshot({path:out+'/desktop-panel.png'})
 for(const width of [1024,768,390]){
  await page.setViewportSize({width,height:844})
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`Overflow at ${width}px`)
  assert.ok(await button.isEnabled())
  if(width<=900)await page.waitForFunction(()=>document.querySelector('.sidebar').getBoundingClientRect().right<=0)
  await page.screenshot({path:out+`/viewport-${width}.png`,fullPage:true})
  if(width===390){await button.scrollIntoViewIfNeeded();await page.screenshot({path:out+'/mobile-actions.png'});await page.locator('.device-license-panel').screenshot({path:out+'/mobile-panel.png'})}
 }
 assert.deepEqual(errors,[])
 await writeFile(out+'/verification.json',JSON.stringify({desktop:true,tablet:true,mobile:true,featureLabels:true,chooserCancel:true,refresh:true,pageErrors:errors},null,2))
 console.log('License layout: desktop/tablet/mobile, feature labels, chooser cancel, refresh and no page errors passed.')
}finally{await browser.close()}

import assert from 'node:assert/strict'
import {mkdir,writeFile} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const base='http://192.168.1.214:8088',out='artifacts/eg100-point-table-fixes-20260917'
const browser=await chromium.launch({headless:true,channel:'chrome'})
try{
 const context=await browser.newContext({viewport:{width:1600,height:1000}})
 const page=await context.newPage(),errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 await page.goto(base+'/login')
 await page.locator('input[name=gateway_user]').fill('admin')
 await page.locator('input[name=gateway_pass]').fill(process.env.WK_EG100_WEB_PASSWORD)
 await page.getByRole('button',{name:'登录',exact:true}).click()
 await page.locator('.app-shell').waitFor()
 await page.locator('.tree-device').first().click()
 await page.getByRole('button',{name:/点位配置/}).click()
 await page.getByRole('button',{name:/保持寄存器 03/}).click()
 const table=page.locator('table.point-sheet-table:visible').first()
 await table.waitFor()
 const header=table.locator('th').nth(2)
 await header.click({position:{x:20,y:12}})
 assert.ok(await table.locator('.is-column-selected').count()>0)
 await page.locator('.app-shell .header').click({position:{x:400,y:20}})
 await page.waitForTimeout(1200)
 assert.equal(await table.locator('.is-column-selected, .is-row-selected').count(),0)
 await table.locator('tbody td.point-sheet-index').first().click()
 assert.ok(await table.locator('.is-row-selected').count()>0)
 await page.locator('.app-shell .header').click({position:{x:400,y:20}})
 await page.waitForTimeout(1200)
 assert.equal(await table.locator('.is-column-selected, .is-row-selected').count(),0)
 assert.deepEqual(errors,[])
 await mkdir(out,{recursive:true})
 await page.screenshot({path:out+'/point-table.png',fullPage:true})
 const result={verifiedAt:new Date().toISOString(),loginWorks:true,columnOutsideDeselect:true,rowOutsideDeselect:true,realtimeDoesNotRestoreSelection:true,pageErrors:errors}
 await context.request.post(base+'/logout')
 await writeFile(out+'/verification.json',JSON.stringify(result,null,2))
 console.log(JSON.stringify(result))
}finally{await browser.close()}

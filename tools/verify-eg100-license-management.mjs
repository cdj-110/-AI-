import assert from 'node:assert/strict'
import {mkdir,writeFile} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const base='http://192.168.1.214:8088',out=process.env.WK_EG100_QA_OUTPUT||'artifacts/eg100-license-management-20260917'
assert.ok(process.env.WK_EG100_WEB_PASSWORD,'Web password must be provided explicitly')
let browser
try{
 browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'chrome'})
 const context=await browser.newContext({viewport:{width:1440,height:1000}})
 assert.equal((await context.request.get(base+'/api/license/status')).status(),401,'Licensed device exposes public status')
 assert.equal((await context.request.get(base+'/api/license/details')).status(),401,'License details bypass login')
 const page=await context.newPage(),errors=[],mutations=[]
 page.on('pageerror',cause=>errors.push(cause.message))
 page.on('request',request=>{if(new URL(request.url()).pathname.startsWith('/api/activation'))mutations.push(request.method())})
 await page.addInitScript(()=>{
  window.__activationMounts=0
  new MutationObserver(records=>{
   for(const record of records)for(const node of record.addedNodes){
    if(node.nodeType===1&&(node.matches('.activation-page')||node.querySelector('.activation-page')))window.__activationMounts++
   }
  }).observe(document,{childList:true,subtree:true})
 })
 let releaseLicense,releaseSession
 const licenseGate=new Promise(resolve=>{releaseLicense=resolve}),sessionGate=new Promise(resolve=>{releaseSession=resolve})
 await page.route('**/api/license/status',async route=>{await licenseGate;await route.fulfill({response:await route.fetch()})})
 await page.route('**/api/session',async route=>{await sessionGate;await route.fulfill({response:await route.fetch()})})
 await page.goto(base+'/login?next='+encodeURIComponent('/?page=maintenance'))
 await page.locator('input[name=gateway_user]').fill('admin')
 await page.locator('input[name=gateway_pass]').fill(process.env.WK_EG100_WEB_PASSWORD)
 await page.getByRole('button',{name:'登录',exact:true}).click()
 await page.getByRole('status').filter({hasText:'正在加载网关'}).waitFor()
 assert.equal(await page.locator('.activation-page').count(),0,'Activation page appeared while authorization loaded')
 assert.equal(await page.locator('.app-shell').count(),0,'Business shell appeared before authorization')
 const sessionRequested=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/session')
 releaseLicense();await sessionRequested
 assert.equal(await page.locator('.activation-page').count(),0,'Activation page appeared while session loaded')
 assert.equal(await page.locator('.app-shell').count(),0,'Business shell appeared before session was ready')
 releaseSession();await page.locator('.app-shell').waitFor()
 await page.getByRole('tab',{name:'设备授权',exact:true}).click()
 await page.getByRole('button',{name:'更换授权文件',exact:true}).waitFor()
 await page.getByRole('heading',{name:'授权信息',exact:true}).waitFor()
 await page.getByRole('heading',{name:'设备身份与签名',exact:true}).waitFor()
 assert.equal(await page.locator('.license-feature').count(),(await (await context.request.get(base+'/api/license/details')).json()).authorization.features?.length||0)
 assert.equal(await page.evaluate(()=>window.__activationMounts),0,'Activation page briefly mounted after login')
 const response=await context.request.get(base+'/api/license/details')
 assert.equal(response.status(),200)
 const details=await response.json()
 assert.equal(details.license.valid,true)
 assert.equal(details.license.restricted,false)
 assert.equal(details.canReplace,true)
 for(const forbidden of ['deviceSecret','signature','broker','cloudApi','remoteAccessEndpoint','file','content'])assert.ok(!(forbidden in details.authorization),`Summary exposes ${forbidden}`)
 const choose=page.waitForEvent('filechooser')
 await page.getByRole('button',{name:'更换授权文件',exact:true}).click()
 await (await choose).setFiles([])
 assert.deepEqual(mutations,[],'Cancelled file choice sent activation requests')
 const refreshed=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/license/details'&&response.status()===200)
 await page.getByRole('button',{name:'刷新授权状态',exact:true}).click()
 await refreshed
 await page.getByRole('button',{name:'更换授权文件',exact:true}).waitFor({state:'visible'})
 await page.waitForFunction(()=>{const button=document.querySelector('.device-license-panel .primary');return !!button&&!button.disabled})
 await page.getByRole('button',{name:'更换授权文件',exact:true}).click({trial:true})
 await mkdir(out,{recursive:true})
 await page.screenshot({path:out+'/desktop.png',fullPage:true})
 await page.setViewportSize({width:390,height:844})
 await page.waitForFunction(()=>document.querySelector('.sidebar').getBoundingClientRect().right<=0)
 assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'Mobile license panel overflows')
 await page.screenshot({path:out+'/mobile.png',fullPage:true})
 await page.getByRole('button',{name:'更换授权文件',exact:true}).scrollIntoViewIfNeeded()
 await page.screenshot({path:out+'/mobile-actions.png'})
 await page.reload()
 await page.locator('.app-shell').waitFor()
 await page.getByRole('tab',{name:'设备授权',exact:true}).click()
 await page.getByRole('button',{name:'更换授权文件',exact:true}).waitFor()
 assert.equal(await page.evaluate(()=>window.__activationMounts),0,'Page reload flashed activation')
 assert.deepEqual(mutations,[])
 assert.deepEqual(errors,[])
 const result={realGatewayVerified:true,licensedLoginWorks:true,noActivationFlash:true,delayedLicenseAndSessionVerified:true,licenseMaintenanceTab:true,summaryWithoutSecrets:true,replacementChooserWorks:true,cancelledChoiceMakesNoActivationRequests:true,refreshWorks:true,mobileNoOverflow:true,authorizationNotChanged:true,pageErrors:errors}
 await context.request.post(base+'/logout')
 assert.equal((await context.request.get(base+'/api/license/details')).status(),401,'Verification session did not log out')
 await writeFile(out+'/page-verification.json',JSON.stringify(result,null,2))
 console.log(JSON.stringify(result))
}finally{await browser?.close()}

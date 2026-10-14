import assert from 'node:assert/strict'
import {createServer} from 'node:http'
import {readFile,mkdir} from 'node:fs/promises'
import {resolve,extname} from 'node:path'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const root=resolve('../internal/web/frontend/vue-dist')
const server=createServer(async(req,res)=>{
 try{
  const path=resolve(root,`.`+new URL(req.url,'http://localhost').pathname)
  if(!path.startsWith(root+'\\')&&!path.startsWith(root+'/')&&path!==root){res.writeHead(403).end();return}
  const file=extname(path)?path:resolve(root,'index.html')
  const types={'.js':'text/javascript','.css':'text/css','.html':'text/html','.png':'image/png'}
  res.setHeader('Content-Type',types[extname(file)]||'application/octet-stream');res.end(await readFile(file))
 }catch{res.writeHead(404).end()}
})
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
let browser
try{
 browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHANNEL?{channel:process.env.PLAYWRIGHT_CHANNEL}:{})})
 const page=await browser.newPage({viewport:{width:1440,height:1000}})
 let permissions=['cloud.manage'], unavailable=false, authenticated=true, initialActivationAllowed=false, licensed=false
 let releaseLicense,releaseSession
 let licenseGate=new Promise(resolve=>{releaseLicense=resolve}),sessionGate=Promise.resolve()
 let replaceSucceeds=false,replacementCommits=0,detailsUnavailable=false,licenseSequence=1
 let firstActivationSucceeds=false,perpetual=false
 let restartRequired=false,restartTriggered=false,restartBoot='test-boot',releaseRestart
 let restartGate=Promise.resolve()
 const businessRequests=[],errors=[]
 await page.addInitScript(()=>{
  window.__activationMounts=0
  new MutationObserver(records=>{
   for(const record of records)for(const node of record.addedNodes){
    if(node.nodeType===1&&(node.matches('.activation-page')||node.querySelector('.activation-page')))window.__activationMounts++
   }
  }).observe(document,{childList:true,subtree:true})
 })
 page.on('pageerror',e=>errors.push(e.message))
 await page.route('**/api/**',async route=>{
  const url=new URL(route.request().url()),method=route.request().method()
  if(url.pathname==='/api/session'){assert.ok(authenticated,'anonymous pending entry requested protected session');await sessionGate;return route.fulfill({json:{username:'admin',roleKey:'admin',permissions}})}
  if(url.pathname==='/api/license/status'){
   await licenseGate
   return unavailable?route.fulfill({status:503,body:'授权状态暂时不可用'}):route.fulfill({json:{authenticated,initialActivationAllowed,gatewayKey:'WK-DEMO',license:{valid:licensed,restricted:!licensed,reason:'尚未导入有效设备授权'},hardwareIdentity:{id:'A'.repeat(32),available:true},secureChipIdentity:{usid:'B'.repeat(16),available:true}}})
  }
  if(url.pathname==='/api/license/details')return detailsUnavailable?route.fulfill({status:503,body:'授权信息暂时不可用'}):route.fulfill({json:{gatewayKey:'WK-DEMO',canReplace:permissions.includes('*')||permissions.includes('cloud.manage'),license:{valid:true,restricted:false},authorization:{sn:'WK-DEMO',hardwareId:'A'.repeat(32),chipUsid:'B'.repeat(16),licenseVersion:2,keyId:'production-v2',licenseSequence,issuedAt:'2026-09-16T00:00:00Z',expiresAt:perpetual?'':'2030-09-16T00:00:00Z',features:['collection','cloud'],signed:true}}})
  if(url.pathname==='/api/readyz'){if(restartTriggered)await restartGate;return route.fulfill({json:{ready:true,bootId:restartBoot}})}
  if(['/api/activation','/api/activation/initial'].includes(url.pathname)){
   assert.equal(url.pathname,initialActivationAllowed?'/api/activation/initial':'/api/activation','wrong import endpoint')
   if(method==='POST')return route.fulfill({json:{sn:'WK-DEMO',keyId:'demo',licenseSequence:licensed?2:1,expiresAt:perpetual?'':'2030-09-16T08:30:18.940Z'}})
   if(method==='PUT'){
    if(!licensed&&firstActivationSucceeds)return route.fulfill({json:{ok:true,restartRequired:true}})
    if(licensed){replacementCommits++;if(replaceSucceeds){licenseSequence=2;restartTriggered=restartRequired;return route.fulfill({json:{ok:true,restartRequired}})}}
    return route.fulfill({status:400,body:'授权校验失败，原授权未修改'})
   }
  }
  businessRequests.push(url.pathname);return route.fulfill({json:{}})
 })
 const url=`http://127.0.0.1:${server.address().port}/`
 await page.goto(url)
 await page.getByRole('status').filter({hasText:'正在加载网关'}).waitFor()
 assert.equal(await page.locator('.activation-page').count(),0,'unknown authorization flashed activation page')
 assert.equal(await page.locator('.app-shell').count(),0,'unknown authorization mounted business shell')
 assert.deepEqual(businessRequests,[],'unknown authorization started business requests')
 releaseLicense()
 await page.getByRole('heading',{name:'网关待激活'}).waitFor()
 assert.equal(await page.locator('.app-shell').count(),0,'business shell mounted without authorization')
 assert.equal(await page.getByRole('button',{name:'导入授权文件'}).isEnabled(),true)
 assert.deepEqual(businessRequests,[],'business requests started before activation')
 const headerStyle=await page.locator('.activation-header').evaluate(el=>{const s=getComputedStyle(el);return{background:s.backgroundColor,border:s.borderBottomWidth,position:s.position,backdrop:s.backdropFilter,width:el.getBoundingClientRect().width,left:el.getBoundingClientRect().left}})
 assert.equal(headerStyle.background,'rgb(255, 255, 255)','full-width header is not white')
 assert.equal(headerStyle.border,'1px')
 assert.equal(headerStyle.left,0)
 assert.equal(headerStyle.width,1440,'white header does not span the viewport')
 assert.equal((await page.locator('.activation-header').boundingBox()).height,56,'header height differs from the gateway shell')
 assert.equal(headerStyle.position,'static')
 assert.equal(headerStyle.backdrop,'none')
 const brandStyle=await page.locator('.activation-brand').evaluate(el=>{const s=getComputedStyle(el),r=el.getBoundingClientRect();return{background:s.backgroundColor,width:r.width,height:r.height,left:r.left,top:r.top}})
 assert.equal(brandStyle.background,'rgb(255, 255, 255)')
 assert.ok(brandStyle.width<220)
 assert.equal(brandStyle.height,56)
 assert.equal(brandStyle.left,24,'brand strip is not at the top left of the page')
 assert.ok(brandStyle.top<=24)
 await mkdir('artifacts/activation-page',{recursive:true})
 await page.screenshot({path:'artifacts/activation-page/desktop.png',fullPage:true})
 await page.setViewportSize({width:390,height:844})
 assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth),'mobile page overflows')
 await page.screenshot({path:'artifacts/activation-page/mobile.png',fullPage:true})
 await page.locator('input[type=file]').setInputFiles({name:'license.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify({signature:'test-signature',hardwareId:'A'.repeat(32),chipUsid:'B'.repeat(16),sn:'WK-DEMO'}))})
 await page.locator('.confirmation-fields').waitFor()
 assert.equal(await page.locator('.confirmation-fields>div').count(),3)
 assert.match(await page.locator('.confirmation-fields').innerText(),/2030/)
 assert.ok(!(await page.locator('.confirmation-fields').innerText()).includes('T08:30:18.940Z'),'raw expiry timestamp shown')
 assert.ok(await page.locator('.license-confirm-dialog').evaluate(el=>el.scrollWidth<=el.clientWidth),'timed modal overflows')
 await page.screenshot({path:'artifacts/activation-page/confirm-timed-mobile.png',fullPage:true})
 await page.getByRole('button',{name:'确认激活',exact:true}).click()
 await page.getByRole('alert').filter({hasText:'授权校验失败'}).waitFor()
 assert.equal(await page.locator('.app-shell').count(),0)
 permissions=[];await page.reload();await page.getByRole('heading',{name:'网关待激活'}).waitFor()
 assert.equal(await page.getByRole('button',{name:'导入授权文件'}).isDisabled(),true)
 unavailable=true;await page.reload();await page.getByRole('alert').filter({hasText:'授权状态暂时不可用'}).waitFor()
 assert.equal(await page.locator('.app-shell').count(),0)
 assert.equal(await page.locator('.activation-page').count(),0,'authorization failure incorrectly displayed activation page')
 unavailable=false;await page.getByRole('button',{name:'重新检查'}).click();await page.getByRole('heading',{name:'网关待激活'}).waitFor()
 authenticated=false;initialActivationAllowed=true;await page.reload();await page.getByRole('heading',{name:'网关待激活'}).waitFor()
 await page.setViewportSize({width:1920,height:912})
 await page.screenshot({path:'artifacts/activation-page/desktop-full-width-header.png',fullPage:true})
 await page.setViewportSize({width:390,height:844})
 assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth),'white brand layout overflows mobile viewport')
 assert.equal((await page.locator('.activation-brand').boundingBox()).x,14)
 assert.equal((await page.locator('.activation-header').boundingBox()).width,390)
 assert.equal((await page.locator('.activation-header').boundingBox()).height,56,'mobile header height differs from the gateway shell')
 assert.equal((await page.locator('.activation-header').boundingBox()).x,0)
 await page.screenshot({path:'artifacts/activation-page/mobile-full-width-header.png',fullPage:true})
 assert.equal(await page.getByRole('button',{name:'退出登录'}).count(),0,'anonymous page showed logout')
 assert.equal(await page.getByRole('button',{name:'导入授权文件'}).isEnabled(),true)
 await page.getByRole('button',{name:'复制设备身份'}).click()
 await page.getByRole('status').filter({hasText:'设备身份已复制'}).waitFor()
 const firstChoice=page.waitForEvent('filechooser')
 await page.getByRole('button',{name:'导入授权文件'}).click()
 const firstChooser=await firstChoice
 assert.equal(await page.getByRole('dialog',{name:'管理员身份验证'}).count(),0,'first activation requested administrator credentials')
 await firstChooser.setFiles({name:'first-license.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify({signature:'test-signature',hardwareId:'A'.repeat(32),chipUsid:'B'.repeat(16),sn:'WK-DEMO'}))})
 await page.getByRole('button',{name:'确认激活',exact:true}).click()
 await page.getByRole('alert').filter({hasText:'授权校验失败'}).waitFor()
 assert.equal(page.url(),url,'first activation failure redirected to login')
 firstActivationSucceeds=true;perpetual=true
 assert.equal(await page.locator('input[type=file]').getAttribute('accept'),'.lic,.json,application/json')
 await page.locator('input[type=file]').setInputFiles({name:'success-test.lic',mimeType:'application/octet-stream',buffer:Buffer.from(JSON.stringify({signature:'test-signature',hardwareId:'A'.repeat(32),chipUsid:'B'.repeat(16),sn:'WK-DEMO'}))})
 await page.locator('.confirmation-fields dd.perpetual').waitFor()
 assert.equal(await page.locator('.confirmation-fields dd.perpetual').innerText(),'永久有效')
 assert.ok(await page.locator('.license-confirm-dialog').evaluate(el=>el.scrollWidth<=el.clientWidth),'perpetual modal overflows mobile')
 await page.screenshot({path:'artifacts/activation-page/confirm-perpetual-mobile.png',fullPage:true})
 await page.setViewportSize({width:1440,height:1000})
 await page.screenshot({path:'artifacts/activation-page/confirm-perpetual-desktop.png',fullPage:true})
 await page.setViewportSize({width:390,height:844})
 await page.getByRole('button',{name:'确认激活',exact:true}).click()
 await page.getByRole('status').filter({hasText:'正在重启网关'}).waitFor()
 assert.equal(await page.getByRole('heading',{name:'授权验证成功',exact:true}).count(),1)
 assert.equal(await page.locator('.activation-reason').count(),0,'successful activation kept a stale unlicensed warning')
 assert.equal(await page.locator('.activation-card').getAttribute('aria-busy'),'true')
 for(const name of ['等待进入登录…','复制设备身份','重新检查'])assert.equal(await page.getByRole('button',{name,exact:true}).isDisabled(),true,'duplicate activation action enabled')
 assert.match(await page.locator('.activation-spinner').evaluate(el=>getComputedStyle(el).animationName),/^activation-spin(?:-|$)/)
 await page.screenshot({path:'artifacts/activation-page/restart-progress-mobile.png',fullPage:true})
 await page.setViewportSize({width:1440,height:1000})
 await page.screenshot({path:'artifacts/activation-page/restart-progress-desktop.png',fullPage:true})
 await page.emulateMedia({reducedMotion:'reduce'})
 assert.equal(await page.locator('.activation-spinner').evaluate(el=>getComputedStyle(el).animationName),'none')
 assert.equal(await page.locator('.activation-progress-track span').evaluate(el=>getComputedStyle(el).animationName),'none')
 await page.emulateMedia({reducedMotion:'no-preference'})
 firstActivationSucceeds=false
 initialActivationAllowed=false;await page.reload();await page.getByRole('heading',{name:'网关待激活'}).waitFor()
 await page.getByRole('button',{name:'导入授权文件'}).click()
 const verification=page.getByRole('dialog',{name:'管理员身份验证'})
 await verification.waitFor()
 assert.equal(page.url(),url,'unlicensed import navigated to login')
 let rejectCredentials=true
 await page.route('**/login',route=>rejectCredentials?route.fulfill({status:401,json:{message:'账号或密码错误'}}):route.fulfill({json:{mfaRequired:true,next:'/login/verify'}}))
 await page.route('**/login/verify',route=>{authenticated=true;permissions=['cloud.manage'];return route.fulfill({json:{next:'/'}})})
 await verification.getByLabel('管理员账号').fill('test-admin')
 await verification.getByLabel('管理员密码').fill('test-only-password')
 await verification.getByRole('button',{name:'验证并继续'}).click()
 await verification.getByRole('alert').filter({hasText:'账号或密码错误'}).waitFor()
 assert.equal(page.url(),url,'incorrect password redirected unlicensed gateway')
 rejectCredentials=false
 await verification.getByRole('button',{name:'验证并继续'}).click()
 await verification.getByLabel('动态验证码或恢复码').fill('123456')
 const choose=page.waitForEvent('filechooser')
 await verification.getByRole('button',{name:'验证并继续'}).click()
 const chooser=await choose
 await chooser.setFiles({name:'test-license.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify({signature:'test-signature',hardwareId:'A'.repeat(32),chipUsid:'B'.repeat(16),sn:'WK-DEMO'}))})
 await page.getByRole('button',{name:'确认激活',exact:true}).click()
 await page.getByRole('alert').filter({hasText:'授权校验失败'}).waitFor()
 assert.equal(page.url(),url,'inline verification navigated away from activation')
 assert.deepEqual(businessRequests,[],'anonymous entry started business requests')
 // Simulate returning from login with valid authorization. Hold both responses
 // independently and observe every activation mount, not only the final frame.
 licensed=true;authenticated=true;permissions=[];initialActivationAllowed=false
 licenseGate=new Promise(resolve=>{releaseLicense=resolve})
 sessionGate=new Promise(resolve=>{releaseSession=resolve})
 await page.reload()
 await page.getByRole('status').filter({hasText:'正在加载网关'}).waitFor()
 assert.equal(await page.locator('.activation-page').count(),0,'login return flashed activation before license response')
 assert.equal(await page.locator('.app-shell').count(),0)
 const sessionRequested=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/session')
 releaseLicense();await sessionRequested
 assert.equal(await page.locator('.activation-page').count(),0,'login return flashed activation while session loaded')
 assert.equal(await page.locator('.app-shell').count(),0,'business shell mounted before session was ready')
 releaseSession();await page.locator('.app-shell').waitFor()
 assert.equal(await page.locator('.activation-page').count(),0)
 assert.equal(await page.evaluate(()=>window.__activationMounts),0,'activation page briefly mounted after login')
 // A cloud/license manager can enter maintenance without unrelated time/SSH
 // privileges; a maintenance viewer sees metadata but cannot import.
 permissions=['cloud.manage'];await page.goto(url+'?page=maintenance')
 await page.getByRole('tab',{name:'设备授权',exact:true}).click()
 await page.getByRole('button',{name:'更换授权文件',exact:true}).waitFor()
 assert.ok((await page.locator('.license-information').innerText()).includes('WK-DEMO'))
 assert.equal(await page.locator('.license-information>div').filter({hasText:'有效期'}).locator('dd').innerText(),'永久有效')
 assert.equal(await page.locator('.activation-page').count(),0)
 const replacementChoice=page.waitForEvent('filechooser')
 await page.getByRole('button',{name:'更换授权文件',exact:true}).click()
 await (await replacementChoice).setFiles([])
 assert.equal(replacementCommits,0,'cancelled chooser submitted a replacement')
 const replacementFile={name:'replacement.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify({signature:'test-signature',hardwareId:'A'.repeat(32),chipUsid:'B'.repeat(16),sn:'WK-DEMO'}))}
 await page.locator('.device-license-panel input[type=file]').setInputFiles(replacementFile)
 await page.getByRole('button',{name:'确认更换',exact:true}).click()
 await page.getByRole('alert').filter({hasText:'授权校验失败'}).waitFor()
 assert.equal(replacementCommits,1)
 assert.equal(await page.locator('.license-information>div').filter({hasText:'授权序号'}).locator('dd').innerText(),'1')
 await page.locator('.device-license-panel input[type=file]').setInputFiles(replacementFile)
 await page.getByRole('button',{name:'取消',exact:true}).click()
 assert.equal(replacementCommits,1,'cancelled confirmation submitted a replacement')
 replaceSucceeds=true
 assert.equal(await page.locator('.device-license-panel input[type=file]').getAttribute('accept'),'.lic,.json,application/json')
 await page.locator('.device-license-panel input[type=file]').setInputFiles({...replacementFile,name:'replacement.lic',mimeType:'application/octet-stream'})
 await page.getByRole('button',{name:'确认更换',exact:true}).click()
 await page.getByRole('status').filter({hasText:'设备授权更换成功'}).waitFor()
 assert.equal(await page.locator('.license-information>div').filter({hasText:'授权序号'}).locator('dd').innerText(),'2')
 await page.setViewportSize({width:390,height:844})
 assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'license management overflows mobile viewport')
 await page.screenshot({path:'artifacts/activation-page/license-management-mobile.png',fullPage:true})
 await page.setViewportSize({width:1440,height:1000})
 await page.screenshot({path:'artifacts/activation-page/license-management-desktop.png',fullPage:true})
 restartRequired=true;restartGate=new Promise(resolve=>{releaseRestart=resolve})
 await page.locator('.device-license-panel input[type=file]').setInputFiles(replacementFile)
 await page.getByRole('button',{name:'确认更换',exact:true}).click()
 await page.getByRole('status').filter({hasText:'正在等待网关重启恢复'}).waitFor()
 const restartProbe=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/readyz')
 await restartProbe
 assert.equal(await page.getByRole('button',{name:'等待服务恢复…',exact:true}).isDisabled(),true)
 const reloadedSummary=page.waitForRequest(request=>new URL(request.url()).pathname==='/api/license/details')
 restartBoot='test-boot-new';releaseRestart();await reloadedSummary
 await page.getByRole('button',{name:'更换授权文件',exact:true}).waitFor()
 assert.equal(await page.evaluate(()=>window.__activationMounts),0,'restart recovery flashed activation for licensed gateway')
 permissions=['maintenance.view'];await page.reload()
 await page.getByRole('heading',{name:'设备授权',exact:true}).waitFor()
 await page.locator('.license-information').waitFor()
 assert.equal(await page.getByRole('button',{name:'更换授权文件',exact:true}).count(),0,'readonly maintenance user could replace license')
 permissions=['cloud.manage'];detailsUnavailable=true;await page.reload()
 await page.getByRole('alert').filter({hasText:'授权信息暂时不可用'}).waitFor()
 assert.equal(await page.getByRole('button',{name:'更换授权文件',exact:true}).count(),0,'failed metadata left replacement enabled')
 assert.deepEqual(errors,[])
 console.log('Activation/login verified without flashes. Restart progress: animated spinner, disabled duplicate actions, success state, desktop/mobile and reduced motion passed. Maintenance license entry: permissions, cancellation, replacement and fail-closed metadata passed.')
}finally{await browser?.close();await new Promise(resolve=>server.close(resolve))}

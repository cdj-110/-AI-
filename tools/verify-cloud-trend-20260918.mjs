import assert from 'node:assert/strict'
import {execFileSync} from 'node:child_process'
import {mkdir,writeFile} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const source=`const {PrismaClient}=require('@prisma/client'),{JwtService}=require('@nestjs/jwt');
(async()=>{const p=new PrismaClient();try{
const device=await p.device.findFirst({where:{metrics:{some:{identifier:'mbrtu-P01'}}},select:{id:true,tenantId:true}})||await p.device.findFirstOrThrow({where:{metrics:{some:{}}},select:{id:true,tenantId:true}});
const user=await p.user.findFirstOrThrow({where:{tenantId:device.tenantId,role:'TENANT_ADMIN',status:'ACTIVE'},select:{id:true,username:true,nickname:true,role:true,tenantId:true}});
const token=new JwtService().sign({sub:user.id,username:user.username,role:user.role,tenantId:user.tenantId},{secret:process.env.JWT_SECRET,expiresIn:'3m'});
console.log(JSON.stringify({user,token,deviceId:device.id}));
}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`
// Verification credentials exist only in this process and isolated browser memory.
const session=JSON.parse(execFileSync('docker',['compose','exec','-T','api','node'],{input:source,encoding:'utf8'}))
const out=process.env.CLOUD_TREND_VERIFICATION_OUT||'artifacts/cloud-trend-verification-20260918',base='http://127.0.0.1:5174'
let browser
try{
 browser=await chromium.launch({headless:true,channel:'chrome'})
 const context=await browser.newContext({viewport:{width:1600,height:1000}})
 await context.addInitScript(({token,user})=>{localStorage.setItem('accessToken',token);localStorage.setItem('userInfo',JSON.stringify(user))},session)
 const page=await context.newPage(),errors=[],failedApi=[]
 let trendItems=0,mutations=0
 page.on('pageerror',error=>errors.push(error.message))
 page.on('request',request=>{if(new URL(request.url()).pathname.startsWith('/api/')&&!['GET','HEAD','OPTIONS'].includes(request.method()))mutations++})
 page.on('response',response=>{
  const path=new URL(response.url()).pathname
  if(path.startsWith('/api/')&&response.status()>=400)failedApi.push({path,status:response.status()})
  if(path.endsWith('/telemetry/trend')&&response.ok())void response.json().then(value=>{trendItems=value.data.items.length})
 })
 await page.goto(`${base}/devices/${session.deviceId}`)
 await page.locator('.detail-tabs').waitFor()
 await page.waitForFunction(()=>document.querySelector('.detail-tabs button:nth-child(2) span')?.textContent!=='0')
 await page.waitForTimeout(700)
 assert.equal(await page.locator('.trend-chart canvas').count(),0,'hidden monitor should not initialize a zero-size chart')
 const monitor=page.locator('.detail-tabs button').filter({hasText:'实时数据'})
 const overview=page.locator('.detail-tabs button').filter({hasText:'设备概览'})
 const assertChart=async()=>{
  await page.waitForFunction(()=>{const host=document.querySelector('.trend-chart'),canvas=host?.querySelector('canvas');return host?.clientWidth>0&&canvas?.width>500&&Math.abs(canvas.getBoundingClientRect().width-host.clientWidth)<2})
  const pixels=await page.locator('.trend-chart canvas').first().evaluate(canvas=>{
   const data=canvas.getContext('2d').getImageData(0,0,canvas.width,canvas.height).data
   let visible=0;for(let index=3;index<data.length;index+=4)if(data[index])visible++
   return visible
  })
  assert.ok(pixels>100,'chart canvas must contain visible axes or trend lines')
 }
 await monitor.click();await assertChart()
 for(let index=0;index<3;index++){await overview.click();await monitor.click();await assertChart()}
 await page.setViewportSize({width:1280,height:950});await assertChart()
 await page.getByText('15分钟',{exact:true}).click();await assertChart();await page.waitForTimeout(700)
 assert.ok(trendItems>0,'actual device trend data should be available')
 const chartBox=await page.locator('.trend-chart').boundingBox()
 await page.mouse.move(chartBox.x+100,chartBox.y+100)
 await page.waitForFunction(()=>document.querySelector('.trend-chart')?.innerText?.trim().length>0)
 const tooltipText=await page.locator('.trend-chart').innerText()
 assert.ok(!/\d+\.\d{10,}/.test(tooltipText),'production tooltip must not expose floating-point tails')
 assert.match(tooltipText,/\d+\.\d{2}(?!\d)/,'configured metric precision should appear in production tooltip')
 assert.equal(mutations,0,'acceptance must not change configuration or records')
 assert.deepEqual(errors,[]);assert.deepEqual(failedApi,[])
 await mkdir(out,{recursive:true});await page.locator('.chart-panel').screenshot({path:out+'/trend-chart.png'})
 const result={verifiedAt:new Date().toISOString(),actualDeviceData:true,trendDataAvailable:true,firstMonitorEntryRenders:true,repeatedTabSwitchesRender:true,windowResizeRenders:true,rangeSwitchRenders:true,canvasHasVisiblePixels:true,tooltipUsesConfiguredPrecision:true,tooltipHasNoFloatingTails:true,noConfigurationWrites:true,pageErrors:errors,failedApi}
 await writeFile(out+'/verification.json',JSON.stringify(result,null,2));console.log(JSON.stringify(result))
}finally{await browser?.close()}

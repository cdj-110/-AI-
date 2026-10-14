import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {mkdir} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({configFile:false,plugins:[vue()],optimizeDeps:{entries:['scripts/fixtures/device-trend.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1500,height:1000}}),errors=[],warnings=[]
 page.on('pageerror',error=>errors.push(error.message))
 page.on('console',message=>{if(message.type()==='warning'&&message.text().includes('ECharts'))warnings.push(message.text())})
 await page.routeWebSocket('**/api/realtime',()=>{})
 await page.route('**/api/**',async route=>{
  if(!new URL(route.request().url()).pathname.startsWith('/api/'))return route.continue()
  assert.equal(route.request().method(),'GET','regression must not mutate external data')
  const url=new URL(route.request().url()),id=url.pathname.split('/')[3],empty=id==='empty'
  const now=Date.now(),items=empty?[]:[{time:new Date(now-2000).toISOString(),metrics:{'mbrtu-P01':43.300000000000004,mbrtu_P01:'44.300000000000004','s7-P102':true,'s7-P103':false}},{time:new Date(now-1000).toISOString(),metrics:{'mbrtu-P01':45.150000000000006,mbrtu_P01:'44.300000000000004','s7-P102':true,'s7-P103':false}}]
  let data=null
  if(url.pathname.endsWith('/commands/capabilities'))data={points:id==='fallback'||empty?[]:[{identifier:'mbrtu-P01',name:'噪声值',dataType:'NUMBER',unit:'dB'},{identifier:'s7-P102',name:'s7-P102',dataType:'BOOLEAN'},{identifier:'s7-P103',name:'s7-P103',dataType:'BOOLEAN'}],available:true,reason:''}
  else if(url.pathname.endsWith('/commands'))data=[]
  else if(url.pathname.endsWith('/telemetry/trend'))data={items:empty?[]:[{time:new Date(now-2000).toISOString(),value:45.150000000000006},{time:new Date(now-1000).toISOString(),value:43.300000000000004}]}
  else if(url.pathname.endsWith('/telemetry'))data={deviceKey:'device-001',items}
  else if(url.pathname.endsWith('/metrics'))data=empty||id==='fallback'?[]:[{id:'metric-1',identifier:'mbrtu-P01',name:'噪声值',unit:'dB',dataType:'float64',decimals:2,accessMode:'r',ignored:false,sortOrder:0}]
  else if(url.pathname.endsWith('/alarm-rules'))data=[]
  else if(url.pathname==='/api/device-logs')data={items:[],total:0}
  else if(url.pathname.startsWith('/api/devices/')&&!url.pathname.endsWith('/credentials')&&!url.pathname.endsWith('/topology-draft'))data={id,deviceKey:'device-001',name:'趋势测试设备',deviceType:'GATEWAY_CHILD',protocol:'modbus-rtu',status:'ONLINE',tenant:{name:'测试租户'}}
  await route.fulfill({json:{code:0,message:'ok',data}})
 })
 await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/device-trend.html`)
 await page.getByRole('heading',{name:'趋势测试设备',exact:true}).waitFor()
 await page.waitForTimeout(400)
 assert.equal((await page.evaluate(()=>window.trendState())).chartWidth,undefined,'hidden panel must not initialize at zero size')
 const monitor=page.locator('.detail-tabs button').filter({hasText:'实时数据'})
 const overview=page.locator('.detail-tabs button').filter({hasText:'设备概览'})
 const assertRendered=async()=>{
  await page.waitForFunction(()=>{const s=window.trendState();return s.hostWidth>0&&s.chartWidth===s.hostWidth&&s.chartHeight===s.hostHeight&&s.series?.length===2})
  const state=await page.evaluate(()=>window.trendState());assert.ok(state.chartWidth>500);return state
 }
 await monitor.click();const first=await assertRendered()
 await page.locator('.metric-card').getByRole('button',{name:'下发',exact:true}).click()
 const commandDialog=page.getByRole('dialog',{name:'点位下发',exact:true})
 await commandDialog.locator('.command-point').filter({hasText:'mbrtu-P01'}).waitFor()
 assert.equal(await page.locator('.monitor-actions').getByRole('button',{name:'下发',exact:true}).count(),0)
 await commandDialog.getByRole('button',{name:'Close this dialog'}).click()
 await commandDialog.waitFor({state:'hidden'})
 await mkdir('artifacts/device-trend-20260918',{recursive:true})
 await page.locator('.metrics-section').screenshot({path:'artifacts/device-trend-20260918/point-card-downlink.png'})
 for(const [key,raw,label,checked] of [['s7-P102','true','开启',true],['s7-P103','false','关闭',false]]){
  const card=page.locator('.metric-card').filter({has:page.locator('.metric-boolean-state').filter({hasText:raw})}).filter({hasText:key})
  assert.equal(await card.count(),1)
  const cardText=await card.innerText()
  assert.equal(cardText.split(key).length-1,1,'identifier must appear once when it is also the display name')
  assert.ok(cardText.includes(raw)&&cardText.includes(label),'boolean card must show raw and readable status')
  assert.equal(await card.locator('.el-switch').evaluate(element=>element.classList.contains('is-checked')),checked)
  assert.equal(await card.getByRole('button',{name:'下发',exact:true}).count(),0,'boolean card uses a switch instead of a downlink button')
  assert.ok((await card.boundingBox()).height<150,'metric cards should stay compact')
 }
 assert.equal(first.series[0],45.150000000000006,'display formatting must preserve raw chart data')
 assert.ok((await page.locator('.metric-card strong').first().textContent()).includes('45.15'))
 const history=await page.locator('.table-panel.history').textContent()
 assert.ok(history.includes('mbrtu_P01: 44.3')&&!history.includes('44.300000000000004'),'recent reports must also format gateway numeric text')
 const stats=await page.locator('.trend-stats').textContent()
 assert.ok(stats.includes('45.15')&&stats.includes('43.30')&&stats.includes('44.23'))
 await page.evaluate(()=>window.showTrendTip());await page.waitForTimeout(100)
 const tooltip=await page.locator('.trend-chart').textContent()
 assert.ok(tooltip.includes('45.15dB')&&!tooltip.includes('000000000000006'),'tooltip must obey metric decimals and suppress floating tails')
 for(let index=0;index<3;index++){await overview.click();await monitor.click();await assertRendered()}
 await page.evaluate(()=>document.querySelector('#detail-frame').style.width='850px');await assertRendered()
 assert.ok((await page.evaluate(()=>window.trendState())).chartWidth<first.chartWidth,'container-only layout changes must resize chart')
 await page.getByText('15分钟',{exact:true}).click();await assertRendered()
 await page.evaluate(()=>window.goDevice('empty'))
 await monitor.click();await page.getByText('暂无可绘制的数值指标',{exact:true}).waitFor()
 await page.evaluate(()=>window.goDevice('with-data-again'))
 await monitor.click();const recreated=await assertRendered()
 assert.notEqual(recreated.chartId,first.chartId,'recreated host must get a new live chart instance')
 await page.evaluate(()=>window.goDevice('fallback'));await monitor.click();await assertRendered()
 assert.equal(await page.locator('.metric-card').getByRole('button',{name:'下发',exact:true}).count(),0,'read-only metric must have no downlink action')
 await page.evaluate(()=>window.showTrendTip());await page.waitForTimeout(100)
 const fallbackTip=await page.locator('.trend-chart').textContent()
 assert.ok(fallbackTip.includes('45.15')&&!fallbackTip.includes('000000000000006'),'unconfigured metrics must also suppress floating tails')
 await mkdir('artifacts/device-trend-20260918',{recursive:true})
 await page.screenshot({path:'artifacts/device-trend-20260918/trend-chart.png',fullPage:true})
 assert.deepEqual(errors,[]);assert.deepEqual(warnings,[])
 console.log('Trend chart passed: visibility/resize lifecycle, metric precision across cards/stats/tooltip, floating-tail fallback, and unchanged raw series. No page errors or ECharts size warnings.')
}finally{await browser?.close();await server.close()}

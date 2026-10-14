import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {mkdir} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({configFile:false,plugins:[vue()],optimizeDeps:{entries:['scripts/fixtures/remote-storage-layout.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
  await server.listen()
  browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'msedge'})
  const page=await browser.newPage({viewport:{width:1440,height:900}}),errors=[]
  page.on('pageerror',e=>errors.push(e.message))
  let config={storagePath:'/mnt/udisk',storageUUID:'test-uuid',maxSizeMB:16,targets:['postgresql','mysql','sqlserver'].map((driver,i)=>({id:'task-'+i,name:'EG100 '+['PostgreSQL','MySQL','SQL Server'][i]+' 实库联调',enabled:true,driver,host:'192.168.1.141',port:[5432,3306,1433][i],database:'eg100_integration',schema:['public','','dbo'][i],table:'eg100_samples',username:'test_writer',password:'***',tlsMode:'verify-full',caFile:'',createTable:false,intervalSeconds:5,batchSize:100,timeoutSeconds:10,points:[{deviceKey:'test',metric:'temperature'}]}))}
  let statuses=config.targets.map((t,i)=>({id:t.id,enabled:true,cacheReady:true,pending:0,bytes:0,sent:3632-i*100,dropped:0,connected:true,lastSuccess:'2026-09-12T08:08:55Z'}))
  let writes=0
  await page.route('**/api/**',async route=>{
    const req=route.request(),path=new URL(req.url()).pathname
    if(req.method()==='PUT'){writes++;config=req.postDataJSON()}
    await route.fulfill({json:path==='/api/remote-storage/status'?{statuses}:path==='/api/history-storage/points'?{total:1,points:[{deviceKey:'test',metric:'temperature',name:'温度'}]}:{config,revision:'test-revision',statuses,devices:[{device:'/dev/sda4',mountPath:'/mnt/udisk',uuid:'test-uuid',freeBytes:19e9}],pointGroups:[{deviceKey:'test',name:'测试设备',count:1}]}})
  })
  const url=`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/remote-storage-layout.html`
  await page.goto(url)
  await page.locator('.remote-task').nth(2).waitFor()
  assert.equal(await page.locator('.state-badge.healthy').count(),3)
  assert.equal(await page.locator('.cache-note').getAttribute('open'),null)
  assert.deepEqual(await page.locator('.task-head').evaluateAll(nodes=>nodes.map(n=>getComputedStyle(n).position)),['static','static','static'])
  const card=await page.locator('.remote-task').first().boundingBox()
  assert.ok(card.height<180,'desktop cards should stay compact')
  await mkdir('artifacts/remote-storage-layout',{recursive:true})
  await page.screenshot({path:'artifacts/remote-storage-layout/desktop.png',fullPage:true})
  const toggle=page.getByRole('switch').first()
  await toggle.uncheck()
  assert.equal(writes,0,'toggle is a draft, not an immediate server write')
  assert.equal(await page.getByText('有未保存的修改',{exact:true}).isVisible(),true)
  await page.getByRole('button',{name:'保存设置',exact:true}).click()
  await page.waitForFunction(()=>!document.querySelector('.draft-indicator'))
  assert.equal(writes,1);assert.equal(config.targets[0].enabled,false)
  assert.equal(config.targets[0].password,'***')
  await page.getByRole('button',{name:'编辑',exact:true}).first().click()
  await page.getByRole('dialog').waitFor()
  assert.equal(await page.getByLabel('任务名称',{exact:true}).inputValue(),config.targets[0].name)
  await page.getByRole('button',{name:'取消',exact:true}).click()
  statuses[1]={...statuses[1],connected:false,lastError:'连接超时：等待数据库响应',pending:24}
  statuses[2]={...statuses[2],cacheError:'外部缓存卷未挂载',dropped:5}
  await page.getByRole('button',{name:'重新加载',exact:true}).click()
  await page.locator('.state-badge.failed').waitFor()
  assert.equal(await page.locator('.state-badge.warning').count(),1)
  assert.equal(await page.getByText('外部缓存卷未挂载',{exact:true}).isVisible(),true)
  await page.screenshot({path:'artifacts/remote-storage-layout/states.png',fullPage:true})
  await page.setViewportSize({width:390,height:844})
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,'mobile must not overflow horizontally')
  await page.screenshot({path:'artifacts/remote-storage-layout/mobile.png',fullPage:true})
  await page.goto(url+'?readonly')
  await page.locator('.remote-task').first().waitFor()
  assert.equal(await page.getByRole('switch').count(),0)
  assert.equal(await page.getByRole('button',{name:'编辑',exact:true}).count(),0)
  assert.deepEqual(errors,[])
  console.log('PASS compact cards, status colors, switch drafts, save/edit, readonly permissions and mobile layout')
}finally{await browser?.close();await server.close()}

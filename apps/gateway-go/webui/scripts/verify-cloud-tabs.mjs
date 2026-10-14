import assert from 'node:assert/strict'
import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'
import { mkdir } from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({configFile:false,plugins:[vue()],optimizeDeps:{entries:['scripts/fixtures/cloud-tabs.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try {
  await server.listen()
  browser=await chromium.launch({headless:true,channel:process.env.PLAYWRIGHT_CHANNEL||'msedge'})
  const page=await browser.newPage({viewport:{width:1440,height:900}})
  const errors=[],writes=[]
  let config={gatewayKey:'test',mqttChannels:[{name:'测试 MQTT',enabled:true,broker:'tcp://localhost:1883',clientId:'test',payloadMode:'flat'}],remoteAccess:{enabled:true,endpoint:'wss://example.test/tunnel'},devices:[]}
  let activation={enabled:true,hardwareId:'test-hardware',sn:'test-sn',broker:'tcp://localhost:1883',deviceSecret:'***'}
  let cache={configured:true,active:true,maxSizeMB:16,storagePath:'/media/udisk1',devices:[]}
  page.on('pageerror',error=>errors.push(error.message))
  await page.route('**/api/**',async route=>{
    const request=route.request(),path=new URL(request.url()).pathname
    if(request.method()==='PUT'){
      writes.push(path)
      const body=request.postDataJSON()
      if(path==='/api/config')config=body
      if(path==='/api/activation')activation=JSON.parse(body.content)
      if(path==='/api/storage')cache={...cache,...body,configured:body.enabled}
    }
    const json=path.startsWith('/api/config')?{content:JSON.stringify(config)}:path==='/api/activation'?{content:JSON.stringify(activation)}:path==='/api/storage'?cache:{}
    await route.fulfill({json})
  })
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/cloud-tabs.html`)
  await page.getByLabel('名称',{exact:true}).waitFor()
  assert.equal(await page.getByRole('tab').count(),3)
  assert.equal(await page.locator('[role=tabpanel]:visible').count(),1)
  await page.getByLabel('名称',{exact:true}).fill('保留 MQTT 草稿')
  await page.getByRole('tab',{name:'微控云连接',exact:true}).click()
  assert.equal(await page.getByRole('heading',{name:'云端远程访问'}).isVisible(),true)
  await page.getByLabel('SN',{exact:true}).fill('保留云草稿')
  await page.getByRole('tab',{name:'断线续存',exact:true}).click()
  await page.getByLabel('容量上限（MB）',{exact:true}).fill('15')
  await page.getByRole('tab',{name:'手动 MQTT',exact:true}).click()
  assert.equal(await page.getByLabel('名称',{exact:true}).inputValue(),'保留 MQTT 草稿')
  assert.deepEqual(writes,[],'tab switching must not save or reload drafts')
  assert.equal(await page.locator('#dirty').textContent(),'true')
  await page.getByRole('button',{name:'保存配置',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('#dirty')?.textContent==='false')
  assert.deepEqual(writes,['/api/config','/api/storage','/api/activation'])
  assert.equal(config.mqttChannels[0].name,'保留 MQTT 草稿')
  assert.equal(activation.sn,'保留云草稿')
  assert.equal(cache.maxSizeMB,15)
  await page.getByRole('tab',{name:'手动 MQTT',exact:true}).focus()
  await page.keyboard.press('End')
  assert.equal(await page.getByRole('tab',{name:'断线续存',exact:true}).getAttribute('aria-selected'),'true')
  await mkdir('artifacts/cloud-tabs',{recursive:true})
  for(const name of ['手动 MQTT','微控云连接','断线续存']){
    await page.getByRole('tab',{name,exact:true}).click()
    await page.waitForFunction(label=>document.querySelector('[role=tab].active')?.textContent===label,name)
    assert.equal(await page.locator('[role=tabpanel]:visible').count(),1)
    await page.screenshot({path:`artifacts/cloud-tabs/${name}.png`,fullPage:true})
  }
  assert.deepEqual(errors,[])
  console.log('PASS 云平台三个标签、远程访问归组、草稿保留、统一保存、键盘切换')
} finally {await browser?.close();await server.close()}

import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {mkdir} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const config={resources:[{resourceKey:'eth0',name:'网口1',type:'network',network:{interface:'eth0'}},{resourceKey:'rs485',name:'RS485-1',type:'serial',serial:{port:'/dev/ttyS1'}}],channels:[
  {channelKey:'f1',name:'转发通道1',role:'forward',resourceKey:'eth0',protocol:'modbus-tcp-slave',forwardProtocol:'modbus-tcp-slave'},
  {channelKey:'c1',name:'采集通道1',role:'collect',resourceKey:'eth0',protocol:'opcua'},
  {channelKey:'f2',name:'转发通道2',role:'forward',resourceKey:'eth0',protocol:'iec104-server',forwardProtocol:'iec104-server'},
  {channelKey:'c2',name:'采集通道2',role:'collect',resourceKey:'eth0',protocol:'modbus-tcp'},
  {channelKey:'s1',name:'串口采集',role:'collect',resourceKey:'rs485',protocol:'modbus-rtu'}],
  devices:[{deviceKey:'d1',name:'采集设备',channelKey:'c1',protocol:'opcua',points:[]}],forwardDevices:[{deviceKey:'fd1',name:'转发设备',channelKey:'f1',protocol:'modbus-tcp-slave',points:[]}]}
const server=await createServer({configFile:false,plugins:[vue()],server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
  await server.listen()
  browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHANNEL?{channel:process.env.PLAYWRIGHT_CHANNEL}:{})})
  const page=await browser.newPage({viewport:{width:1440,height:1000}})
  const errors=[]
  page.on('pageerror',error=>errors.push(error.message))
  await page.route('**/api/**',route=>{
    const path=new URL(route.request().url()).pathname
    const data=path==='/api/session'?{username:'admin',permissions:['config.view','config.manage']}:path.startsWith('/api/config')?{content:JSON.stringify(config),largeMode:false}:path==='/api/point-status'?[]:{}
    return route.fulfill({json:data})
  })
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/?page=config`)
  const resource=page.locator('.gateway-tree-resource[data-resource="eth0"]')
  const collect=resource.locator('[data-role="collect"]'),forward=resource.locator('[data-role="forward"]')
  await collect.waitFor()
  assert.equal(await resource.locator('.resource-row > button').count(),2,'network resource has only navigation and fold buttons')
  assert.equal(await page.locator('[data-resource="rs485"] .resource-row button[title="新增设备"]').count(),1,'serial add device stays available')
  const roleStyle=await resource.locator('.gateway-tree-role-row').evaluateAll(rows=>rows.map(row=>({height:row.getBoundingClientRect().height,bg:getComputedStyle(row).backgroundColor,weight:getComputedStyle(row.querySelector('.gateway-tree-role-label')).fontWeight,badge:getComputedStyle(row.querySelector('.gateway-tree-role-count')).backgroundColor})))
  assert.ok(roleStyle.every(style=>style.height>=30&&style.height<=32&&Number(style.weight)>=600&&style.badge!=='rgba(0, 0, 0, 0)'),'group rows need prominent labels, badges and click targets')
  assert.notEqual(roleStyle[0].bg,roleStyle[1].bg,'collection and forwarding must be visually distinguishable')
  assert.equal(await resource.locator('.gateway-tree-role-icon').count(),2)
  assert.equal(await resource.locator('.tree-channel em').count(),0,'role tags are redundant under role groups')
  assert.match(await collect.locator('.tree-channel small').first().textContent(),/OPC UA.*1 台设备/,'keep protocol and device count')
  assert.deepEqual(await collect.locator('.tree-channel b').allTextContents(),['采集通道1','采集通道2'])
  assert.deepEqual(await forward.locator('.tree-channel b').allTextContents(),['转发通道1','转发通道2'])
  assert.equal(await page.locator('.gateway-nav-tree > .gateway-tree-resource').count(),2,'resources appear only once, above role groups')
  await collect.getByRole('button',{name:'折叠采集',exact:true}).click()
  assert.equal(await collect.locator('.tree-channel').count(),0)
  assert.equal(await forward.locator('.tree-channel').count(),2,'folding shared NIC in collect must not fold forwarding')
  await page.getByRole('button',{name:'全部折叠',exact:true}).click()
  assert.equal(await page.locator('.gateway-nav-tree .tree-channel').count(),0)
  await page.getByRole('button',{name:'全部展开',exact:true}).click()
  assert.equal(await page.locator('.gateway-nav-tree .tree-channel').count(),4)
  await forward.locator('.tree-device').click()
  await page.locator('.legacy-device-workspace').waitFor()
  assert.match(await page.locator('.legacy-device-workspace').textContent(),/转发设备/)
  await resource.locator('.gateway-tree-group').first().click()
  const detail=page.locator('.resource-role-groups')
  await detail.waitFor()
  assert.equal(await page.getByRole('button',{name:'新增通道',exact:true}).count(),0,'network creation goes through role groups')
  assert.deepEqual(await detail.locator('[data-role="collect"] .channel-structure-card strong').allTextContents(),['采集通道1','采集通道2'])
  assert.deepEqual(await detail.locator('[data-role="forward"] .channel-structure-card strong').allTextContents(),['转发通道1','转发通道2'])
  for(const [role,label,protocol] of [['forward','转发','modbus-tcp-slave'],['collect','采集','modbus-tcp']]){
    await resource.locator(`[data-role="${role}"] .gateway-tree-role-create`).click()
    const dialog=page.locator('.dialog-panel').filter({has:page.getByRole('heading',{name:`新增${label}通道`,exact:true})})
    await dialog.waitFor()
    assert.equal(await dialog.getByLabel('通道用途',{exact:true}).count(),0,'role is fixed by the entry group')
    assert.equal(await dialog.getByLabel('协议',{exact:true}).inputValue(),protocol)
    await dialog.getByRole('button',{name:'确定新增',exact:true}).click()
    assert.equal(await resource.locator(`[data-role="${role}"] .tree-channel`).count(),3,'created channel belongs to selected resource and role')
  }
  await resource.locator('.gateway-tree-group').click()
  await mkdir('artifacts/channel-groups',{recursive:true})
  await page.screenshot({path:'artifacts/channel-groups/desktop.png',fullPage:true})
  await collect.getByRole('button',{name:'折叠采集',exact:true}).click()
  await forward.getByRole('button',{name:'折叠转发',exact:true}).click()
  await page.locator('.gateway-nav-tree').screenshot({path:'artifacts/channel-groups/collapsed.png'})
  assert.deepEqual(errors,[])
  console.log('Channel grouping UI: sidebar, independent folding, all-fold, device navigation and resource detail passed')
}finally{await browser?.close();await server.close()}

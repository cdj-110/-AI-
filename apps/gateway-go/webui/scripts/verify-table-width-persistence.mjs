import assert from 'node:assert/strict'
import {createServer} from 'vite'

const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({configFile:false,optimizeDeps:{entries:['scripts/fixtures/table-widths.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
  await server.listen()
  browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHANNEL?{channel:process.env.PLAYWRIGHT_CHANNEL}:{})})
  const page=await browser.newPage({viewport:{width:1900,height:1000}})
  const errors=[]
  page.on('pageerror',error=>errors.push(error.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/table-widths.html`)
  async function checkTable(id){
    const header=page.locator(`#${id} th`).nth(2)
    // Hover first so initialization and hover/resize classes match a real drag.
    await header.hover()
    const initial=await header.boundingBox()
    const x=initial.x+initial.width-2,y=initial.y+initial.height/2
    await page.mouse.move(x,y)
    await page.mouse.down()
    await page.mouse.move(x+90,y,{steps:8})
    await page.mouse.up()
    await page.locator('#outside').hover()
    await page.waitForTimeout(250)
    const resized=await header.boundingBox()
    assert.ok(Math.abs(resized.width-initial.width-90)<=2,`${id}: resized width reverted: ${initial.width} -> ${resized.width}`)
    await header.click({position:{x:40,y:12}})
    assert.ok(await page.locator(`#${id} .is-column-selected`).count()>0,`${id}: header must select column`)
    await page.locator('#outside').click()
    await page.waitForTimeout(200)
    assert.equal(await page.locator(`#${id} .is-column-selected, #${id} .is-row-selected`).count(),0,`${id}: outside click must clear column across realtime updates`)
    await page.locator(`#${id} tbody td`).nth(2).click({position:{x:40,y:12}})
    assert.equal(await page.locator(`#${id} .is-row-selected`).count(),1,`${id}: cell must select row`)
    await page.mouse.click(5,5)
    await page.waitForTimeout(200)
    assert.equal(await page.locator(`#${id} .is-column-selected, #${id} .is-row-selected`).count(),0,`${id}: document blank must clear row across realtime updates`)
    const checkbox=page.locator(`#${id} tbody input[type=checkbox]`)
    await checkbox.check()
    const input=page.locator(`#${id} .point-metric-cell input`)
    await input.fill('edited-P01')
    await input.click()
    assert.equal(await input.inputValue(),'edited-P01',`${id}: in-table editing must remain available`)
    await page.locator('#outside').click()
    await page.waitForTimeout(200)
    assert.equal(await checkbox.isChecked(),true,`${id}: clearing visual highlight must preserve batch selection`)
    assert.equal(await input.inputValue(),'edited-P01',`${id}: clearing visual highlight must preserve editing`)
    assert.equal(await page.locator(`#${id} .is-column-selected, #${id} .is-row-selected`).count(),0,`${id}: stopped outside event must still clear highlights`)
    assert.ok(Math.abs((await header.boundingBox()).width-resized.width)<=1,`${id}: selection/realtime must retain width`)
    const rect=await header.boundingBox()
    await page.mouse.dblclick(rect.x+rect.width-2,rect.y+12)
    const fitted=(await header.boundingBox()).width
    await page.locator('#outside').hover()
    await page.waitForTimeout(250)
    assert.ok(Math.abs((await header.boundingBox()).width-fitted)<=1,`${id}: auto-fit reverted`)
    assert.ok(fitted>=240)
    return fitted
  }
  for(const protocol of ['modbus','s7','opcua','iec61850','iec104','goose','dlt645']){
    await page.selectOption('#protocol',protocol)
    await checkTable('generic')
    console.log(`${protocol}: widths, outside deselection, realtime, checkbox and editing passed`)
  }
  const fitted=await checkTable('opcda')
  await page.locator('#advanced').click()
  await page.locator('#advanced').click()
  await page.setViewportSize({width:1500,height:1000})
  await page.waitForTimeout(200)
  assert.ok(Math.abs((await page.locator('#opcda th').nth(2).boundingBox()).width-fitted)<=1)
  assert.ok(Number(await page.locator('#pulse').textContent())>10,'fixture must refresh realtime values during resizing')
  assert.deepEqual(errors,[])
  console.log('OPC DA: drag, auto-fit, advanced toggle and container resize passed')
}finally{
  await browser?.close()
  await server.close()
}

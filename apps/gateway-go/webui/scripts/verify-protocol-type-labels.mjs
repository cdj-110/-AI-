import assert from 'node:assert/strict'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {dataTypeLabel} from '../src/dataTypeLabels.ts'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({configFile:false,plugins:[vue()],optimizeDeps:{entries:['scripts/fixtures/dlt645-layout.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try {
  await server.listen()
  browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHANNEL?{channel:process.env.PLAYWRIGHT_CHANNEL}:{})})
  const page=await browser.newPage({viewport:{width:1920,height:1000}})
  const errors=[]
  await page.route('**/api/dlt645/dictionary',route=>route.fulfill({json:{items:[]}}))
  page.on('pageerror',error=>errors.push(error.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/dlt645-layout.html`)
  await page.locator('#mode').click()
  for(const protocol of ['modbus-rtu','siemens-s7','opcua','iec104','iec61850','iec61850-goose','opcda']){
    await page.selectOption('#protocol',protocol)
    await page.getByRole('button',{name:/点位配置/}).click()
    if(protocol==='modbus-rtu')await page.getByRole('button',{name:/保持寄存器/}).click()
    if(protocol==='opcda')await page.locator('.opcda-advanced-toggle:visible').click()
    const select=page.locator('.point-sheet-table select.point-data-type-select:visible').first()
    await select.waitFor()
    const options=await select.locator('option').evaluateAll(options=>options.map(option=>({value:option.value,label:option.textContent,title:option.title})))
    for(const option of options){
      assert.equal(option.label,dataTypeLabel(option.value),protocol)
      assert.equal(option.title,option.value,protocol)
    }
    const chosen=options.find(option=>option.value==='uint16')||options.at(-1)
    await select.selectOption(chosen.value)
    await page.waitForTimeout(150)
    assert.equal(await select.inputValue(),chosen.value)
    assert.equal(await select.getAttribute('title'),chosen.value)
    if(protocol!=='opcda'){
      assert.equal(JSON.parse(await page.locator('#types').textContent())[0],chosen.value,'original code, not Chinese label, is written to point config')
    }
    const width=await select.evaluate(el=>({select:el.getBoundingClientRect().width,cell:el.closest('td').getBoundingClientRect().width}))
    assert.ok(width.select<=width.cell+1,protocol+' type selector must stay in its column')
    assert.ok(width.cell>=156,protocol+' Chinese names require readable width')
    const indexCells=await select.locator('xpath=ancestor::table').locator('.point-sheet-index').evaluateAll(cells=>cells.map(cell=>({width:cell.getBoundingClientRect().width,minWidth:getComputedStyle(cell).minWidth,align:getComputedStyle(cell).textAlign})).filter(cell=>cell.width>0))
    // Tables with fewer data columns distribute surplus width differently;
    // all protocols must escape the generic 88px minimum and center their index.
    assert.ok(indexCells.every(cell=>parseFloat(cell.minWidth)<=42&&cell.width>=42&&cell.align==='center'),protocol+' sequence column must use the shared compact layout: '+JSON.stringify(indexCells))
    const offsets=await select.locator('xpath=ancestor::table').locator('.point-sheet-select').evaluateAll(cells=>cells.flatMap(cell=>{
      const input=cell.querySelector('input[type="checkbox"]')
      if(!input)return []
      const a=cell.getBoundingClientRect(),b=input.getBoundingClientRect()
      return a.width>0?[Math.abs((a.left+a.right-b.left-b.right)/2)]:[]
    }))
    assert.ok(offsets.length>1&&offsets.every(offset=>offset<1.1),protocol+' header and row checkboxes must share the same center')
    console.log(protocol+': Chinese options, type-code tooltips, unchanged model codes and readable widths passed')
  }
  assert.deepEqual(errors,[])
  await page.screenshot({path:'artifacts/dlt645-layout/protocol-types.png',fullPage:true})
}finally{
  await browser?.close()
  await server.close()
}

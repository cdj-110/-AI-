import assert from 'node:assert/strict'
import {createServer} from 'vite'
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright')
const server=await createServer({optimizeDeps:{entries:['scripts/fixtures/point-group-defaults.html']},server:{host:'127.0.0.1',port:0},logLevel:'error'})
let browser
try{
 await server.listen();browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1600,height:1000}}),errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 const base=`http://127.0.0.1:${server.httpServer.address().port}/scripts/fixtures/point-group-defaults.html`
 async function open(protocol,scenario='only03',large=false){await page.goto(`${base}?protocol=${protocol}&scenario=${scenario}&large=${large?1:0}`);await page.locator('.device-tabs').waitFor()}
 async function activeGroup(text){await page.waitForFunction(text=>Array.from(document.querySelectorAll('.point-groups button.active, .iec104-point-tabs button.active')).some(button=>button.textContent.includes(text)),text)}
 for(const large of [false,true]){
  for(const protocol of ['modbus-rtu','modbus-tcp','modbus-tcp-slave']){
   await open(protocol,'only03',large);await activeGroup('03')
   await page.locator('tbody tr.point-table-row, tbody tr[data-row-key]').first().waitFor()
   await page.locator('.point-groups button, .iec104-point-tabs button').filter({hasText:'01'}).click();await activeGroup('01')
   if(!large)await page.locator('#tick').click()
   await page.waitForTimeout(100);await activeGroup('01')
   await open(protocol,'priority',large);await activeGroup('01')
   await open(protocol,'empty',large)
   await page.waitForFunction(()=>Array.from(document.querySelectorAll('.device-tabs button.active')).some(button=>button.textContent.includes('设备配置')))
  }
  for(const protocol of ['iec104','iec104-server','siemens-s7','opcua','opcda','iec61850','iec61850-mms-server','iec61850-goose','iec61850-goose-publisher','dlt645']){
   await open(protocol,'only03',large)
   await page.waitForFunction(()=>Array.from(document.querySelectorAll('.device-tabs button.active')).some(button=>button.textContent.includes('点位配置')))
   if(!large&&protocol.startsWith('iec104'))await activeGroup('遥信')
  }
 }
 await open('iec104','yk');await activeGroup('遥控')
 await open('iec104','priority');await activeGroup('遥测')
 await open('modbus-rtu','late');await page.evaluate(()=>window.addLatePoints());await activeGroup('03')
 await open('modbus-rtu','slow',true)
 await page.locator('.point-groups button').filter({hasText:'04'}).click();await page.waitForTimeout(500);await activeGroup('04')
 await open('modbus-rtu','slow',true);await page.evaluate(()=>window.switchProbeDevice());await activeGroup('01');await page.waitForTimeout(500);await activeGroup('01')
 assert.deepEqual(errors,[])
 console.log('Navigation passed: every protocol, ordered populated groups, regular/large tables, empty/late points, manual selection and stale-device requests.')
}finally{await browser?.close();await server.close()}

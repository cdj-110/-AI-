import {chromium} from 'file:///C:/Users/18507/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs'
const browser=await chromium.launch({headless:true,channel:'chrome'})
try{
 const context=await browser.newContext(),page=await context.newPage()
 await page.goto('http://192.168.1.214:8088/login')
 await page.locator('input[name=gateway_user]').fill('admin')
 await page.locator('input[name=gateway_pass]').fill(process.env.WK_EG100_WEB_PASSWORD)
 await page.getByRole('button',{name:'登录',exact:true}).click()
 await page.locator('.app-shell').waitFor()
 const response=await context.request.get('http://192.168.1.214:8088/api/config')
 const original=JSON.parse((await response.json()).content)
 await page.locator('.tree-device').first().click()
 await page.waitForTimeout(1000)
 const diagnostics=await page.evaluate(original=>{
  const app=document.querySelector('#app').__vue_app__
  const pinia=Object.values(app._context.provides).find(value=>value?._s instanceof Map)||Object.getOwnPropertySymbols(app._context.provides).map(key=>app._context.provides[key]).find(value=>value?._s instanceof Map)
  const current=pinia?._s.get('gateway-config')?.value
  if(!current)return{storeFound:false}
  const changes=[]
  function diff(a,b,path=''){
   if(a&&b&&typeof a==='object'&&typeof b==='object'){
    for(const key of new Set([...Object.keys(a),...Object.keys(b)]))diff(a[key],b[key],path+'.'+key)
   }else if(JSON.stringify(a)!==JSON.stringify(b)){
    const safe=/\.(protocol|address|decimals)$/.test(path)
    changes.push({path,beforePresent:a!==undefined,afterPresent:b!==undefined,...(safe?{before:a,after:b}:{})})
   }
  }
  diff(original,current)
  return{storeFound:true,changes}
 },original)
 await page.getByRole('button',{name:'运行状态',exact:true}).click()
 await page.waitForTimeout(400)
 diagnostics.unsavedDialog=await page.getByRole('heading',{name:'尚有未保存的修改',exact:true}).count()>0
 console.log(JSON.stringify(diagnostics))
 await context.request.post('http://192.168.1.214:8088/logout')
}finally{await browser.close()}

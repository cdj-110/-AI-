import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright');
const base='http://192.168.1.214:8088';
const out='artifacts/eg100-lic-deploy-20260917/verification';
let browser;
try{
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const context=await browser.newContext({viewport:{width:1440,height:950}});
  const status=await context.request.get(base+'/api/license/status');
  const page=await context.newPage(),errors=[],mutations=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/activation'))mutations.push(r.method());});
  let initial;
  if(status.status()===200){
    const payload=await status.json();assert.equal(payload.license.restricted,true);initial=true;
    await page.goto(base+'/');await page.getByRole('heading',{name:'网关待激活',exact:true}).waitFor();
    assert.equal(await page.locator('.activation-actions input[type=file]').getAttribute('accept'),'.lic,.json,application/json');
  }else{
    assert.equal(status.status(),401);initial=false;
    assert.ok(process.env.WK_EG100_WEB_PASSWORD,'Web administrator password required');
    await page.goto(base+'/login?next='+encodeURIComponent('/?page=maintenance'));
    await page.locator('input[name=gateway_user]').fill('admin');
    await page.locator('input[name=gateway_pass]').fill(process.env.WK_EG100_WEB_PASSWORD);
    await page.getByRole('button',{name:'登录',exact:true}).click();
    await page.locator('.app-shell').waitFor();
    await page.getByRole('tab',{name:'设备授权',exact:true}).click();
    await page.getByRole('button',{name:'更换授权文件',exact:true}).waitFor();
    assert.equal(await page.locator('.device-license-panel input[type=file]').getAttribute('accept'),'.lic,.json,application/json');
  }
  await mkdir(out,{recursive:true});
  await page.screenshot({path:out+'/page.png',fullPage:true});
  assert.deepEqual(errors,[]);assert.deepEqual(mutations,[]);
  const result={verifiedAt:new Date().toISOString(),gatewayMode:initial?'unlicensed':'licensed',licAndJsonFilePickerVerified:true,activationUnchanged:true,pageErrors:errors};
  await writeFile(out+'/verification.json',JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}finally{await browser?.close();}

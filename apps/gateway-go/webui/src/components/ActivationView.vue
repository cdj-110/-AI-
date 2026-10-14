<script lang="ts">
export interface ActivationStatus {
 authenticated?:boolean
 initialActivationAllowed?:boolean
 license:{valid:boolean;restricted:boolean;reason?:string}
 gatewayKey:string
 hardwareIdentity?:{id:string;available:boolean;message?:string}
 secureChipIdentity?:{usid?:string;available:boolean;message?:string}
}
</script>
<script setup lang="ts">
import {ref,computed,onBeforeUnmount} from 'vue'
import {api} from '../api'
import {confirmAction} from '../confirm'
import {formatLicenseExpiry} from '../license-expiry'
import brandLogo from '../../../internal/web/frontend/brand-logo.png'
const props=defineProps<{status:ActivationStatus|null;error:string;canImport:boolean;authenticated:boolean}>()
const emit=defineEmits<{refresh:[]}>()
const upload=ref<HTMLInputElement>()
const busy=ref(false)
const transitioning=ref(false),restarting=ref(false)
const notice=ref('')
const failure=ref('')
let restartTimer=0
const verificationOpen=ref(false),verifying=ref(false),verificationError=ref('')
const username=ref(''),password=ref(''),code=ref(''),mfaRequired=ref(false)
const verified=ref(false),verifiedPermission=ref(false)
const isAuthenticated=computed(()=>props.authenticated||verified.value)
const initialActivation=computed(()=>props.status?.initialActivationAllowed===true)
const mayImport=computed(()=>initialActivation.value||props.canImport||verifiedPermission.value)
function beginImport(){
 if(busy.value||transitioning.value)return
 if(!initialActivation.value&&!isAuthenticated.value){verificationOpen.value=true;verificationError.value='';return}
 upload.value?.click()
}
function closeVerification(){verificationOpen.value=false;password.value='';code.value='';mfaRequired.value=false}
async function verifyAdministrator(){
 verifying.value=true;verificationError.value=''
 try{
  const response=await fetch(mfaRequired.value?'/login/verify':'/login',{method:'POST',headers:{Accept:'application/json','Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams(mfaRequired.value?{code:code.value}:{gateway_user:username.value,gateway_pass:password.value,next:'/'})})
  const result=await response.json()
  if(!response.ok)throw new Error(result.message||'管理员身份验证失败，请重试')
  password.value='';code.value=''
  if(result.mfaRequired){mfaRequired.value=true;return}
  if(result.passwordChangeRequired){window.location.assign(result.next);return}
  const session=await api<{permissions:string[]}>('/api/session',{redirectOnUnauthorized:false})
  verified.value=true;verifiedPermission.value=session.permissions.some(permission=>permission==='*'||permission==='cloud.manage')
  closeVerification();emit('refresh')
  if(!verifiedPermission.value)throw new Error('当前账号没有授权导入权限，请使用具有云平台管理权限的管理员账号')
  notice.value='管理员身份已验证，请选择授权文件'
  upload.value?.click()
 }catch(cause){verificationError.value=cause instanceof Error?cause.message:'身份验证失败';if(!verificationOpen.value)failure.value=verificationError.value}
 finally{verifying.value=false}
}
async function copyIdentity(){
 const text=`网关：${props.status?.gatewayKey||'-'}\nHardwareId：${props.status?.hardwareIdentity?.id||'-'}\n芯片 USID：${props.status?.secureChipIdentity?.usid||'-'}`
 try{
  if(window.isSecureContext&&navigator.clipboard)await navigator.clipboard.writeText(text)
  else{
   const field=document.createElement('textarea');field.value=text;field.readOnly=true;field.style.cssText='position:fixed;left:-9999px;top:0';document.body.appendChild(field)
   try{field.select();if(!document.execCommand('copy'))throw new Error('copy unavailable')}finally{field.remove()}
  }
  notice.value='设备身份已复制'
 }
 catch{failure.value='复制失败，请手动复制下方设备身份'}
}
async function importLicense(event:Event){
 const input=event.target as HTMLInputElement;const file=input.files?.[0];input.value=''
 if(!file||!mayImport.value||busy.value||transitioning.value)return
 busy.value=true;notice.value='';failure.value=''
 try{
  if(file.size>1024*1024)throw new Error('授权文件不能超过 1 MB')
  const content=await file.text();const parsed=JSON.parse(content)
  if(!parsed.signature||!parsed.hardwareId||!parsed.chipUsid||!parsed.sn)throw new Error('请导入管理平台签发的完整授权文件')
  const options={headers:{'Content-Type':'application/json'},body:JSON.stringify({content}),timeoutMs:15000,redirectOnUnauthorized:false}
  const endpoint=initialActivation.value?'/api/activation/initial':'/api/activation'
  const preview=await api<{sn:string;keyId:string;licenseSequence:number;expiresAt:string}>(endpoint,{...options,method:'POST'})
  if(!await confirmAction({kind:'license',title:'确认激活网关',message:'请确认设备与授权信息',fields:[{label:'授权设备',value:preview.sn},{label:'授权序号',value:String(preview.licenseSequence||'旧版')},{label:'有效期',value:formatLicenseExpiry(preview.expiresAt)}],detail:'激活成功后将进入登录页面。验证失败不会替换原授权。',confirmText:'确认激活'}))return
  const result=await api<{restartRequired?:boolean}>(endpoint,{...options,method:'PUT'})
  restarting.value=!!result.restartRequired
  transitioning.value=true
  notice.value=result.restartRequired?'授权验证成功，正在重启网关，随后进入登录页面…':'授权导入成功，正在进入登录页面…'
  restartTimer=window.setTimeout(async()=>{
   if(isAuthenticated.value){try{await fetch('/logout',{method:'POST'})}catch{}}
   window.location.assign('/')
  },result.restartRequired?5000:1000)
 }catch(cause){failure.value=cause instanceof Error?cause.message:'授权导入失败，请检查文件后重试';if(failure.value==='登录已失效'){verified.value=false;verifiedPermission.value=false;verificationOpen.value=true}}
 finally{busy.value=false}
}
onBeforeUnmount(()=>window.clearTimeout(restartTimer))
</script>

<template>
 <main class="activation-page">
  <header class="activation-header"><div class="activation-brand"><img :src="brandLogo" alt="微控网关"/><span>微控网关<small>Edge Gateway</small></span></div><form v-if="isAuthenticated" method="post" action="/logout"><button type="submit" :disabled="transitioning">退出登录</button></form></header>
  <section class="activation-card" aria-labelledby="activation-title" :aria-busy="busy||transitioning">
   <div class="activation-symbol" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M12 3l8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3z"/><rect x="8.5" y="10" width="7" height="6" rx="1"/><path d="M10 10V8a2 2 0 0 1 4 0v2"/></svg></div>
   <span class="activation-badge" :class="{confirmed:transitioning}">{{transitioning?'授权验证通过':!status?'授权状态检查':status.license.restricted?'等待有效授权':'待激活'}}</span>
   <h1 id="activation-title">{{transitioning?'授权验证成功':!status?'正在检查设备授权':'网关待激活'}}</h1>
   <p class="activation-intro">{{transitioning?'设备授权已保存，正在准备登录页面，请稍候。':'请导入管理平台签发的设备授权文件，激活后即可使用采集、转发及云平台连接等已授权功能。'}}</p>
   <div v-if="error||failure" class="activation-error" role="alert">{{failure||error}}</div>
   <div v-else-if="!transitioning&&status?.license.reason" class="activation-reason">{{status.license.reason}}</div>
   <dl class="activation-identity">
    <div><dt>网关编号</dt><dd>{{status?.gatewayKey||'—'}}</dd></div>
    <div><dt>HardwareId</dt><dd>{{status?.hardwareIdentity?.id||'未读取到设备身份'}}</dd></div>
    <div><dt>芯片 USID</dt><dd>{{status?.secureChipIdentity?.usid||'未读取到芯片身份'}}</dd></div>
   </dl>
   <p v-if="status?.secureChipIdentity&&!status.secureChipIdentity.available" class="activation-hint">芯片身份不可用：{{status.secureChipIdentity.message||'请联系管理员检查设备'}}。未完成设备身份验证不能激活生产网关。</p>
   <div class="activation-actions"><input ref="upload" type="file" accept=".lic,.json,application/json" hidden @change="importLicense"/><button class="primary" :disabled="busy||transitioning||(isAuthenticated&&!mayImport)||!status" @click="beginImport">{{transitioning?'等待进入登录…':busy?'正在验证授权…':'导入授权文件'}}</button><button :disabled="busy||transitioning||!status" @click="copyIdentity">复制设备身份</button><button :disabled="busy||transitioning" @click="emit('refresh')">重新检查</button></div>
   <p v-if="initialActivation" class="activation-hint">首次激活无需登录，授权导入后默认开启云平台连接，并进入登录页面。</p>
   <p v-else-if="!isAuthenticated&&status" class="activation-hint">设备已有授权记录，更换授权需要管理员验证；有效授权恢复后进入正常登录流程。</p>
   <p v-else-if="!mayImport&&status" class="activation-hint">当前账号没有授权导入权限，请使用具有云平台管理权限的管理员账号。</p>
   <div v-if="transitioning" class="activation-progress" role="status" aria-live="polite" aria-atomic="true">
    <div class="activation-progress-content"><span class="activation-spinner" aria-hidden="true"></span><div><strong>{{restarting?'正在重启网关':'正在进入登录页面'}}</strong><p>{{restarting?'授权验证成功，服务恢复后将进入登录页面，请勿关闭页面。':'授权导入成功，即将进入登录页面，请稍候。'}}</p></div></div>
    <div class="activation-progress-track" aria-hidden="true"><span></span></div>
   </div>
   <p v-else-if="notice" class="activation-success" role="status">{{notice}}</p>
   <footer>设备身份绑定 · 数字签名校验 · 授权文件防重放</footer>
  </section>
  <div v-if="verificationOpen" class="activation-verification-mask">
   <section class="activation-verification" role="dialog" aria-modal="true" aria-labelledby="verification-title">
    <h2 id="verification-title">管理员身份验证</h2><p>验证完成后继续导入授权文件，不会进入业务管理页面。</p>
    <form @submit.prevent="verifyAdministrator">
     <template v-if="!mfaRequired"><label>管理员账号<input v-model="username" autocomplete="username" required/></label><label>管理员密码<input v-model="password" type="password" autocomplete="current-password" required/></label></template>
     <label v-else>动态验证码或恢复码<input v-model="code" autocomplete="one-time-code" required/></label>
     <p v-if="verificationError" class="activation-error" role="alert">{{verificationError}}</p>
     <div class="activation-actions"><button type="button" :disabled="verifying" @click="closeVerification">取消</button><button class="primary" type="submit" :disabled="verifying">{{verifying?'正在验证…':'验证并继续'}}</button></div>
    </form>
   </section>
  </div>
 </main>
</template>

<style scoped>
.activation-verification-mask{position:fixed;inset:0;z-index:1200;display:grid;place-items:center;padding:20px;background:#14243e66}.activation-verification{width:min(420px,100%);max-height:90dvh;overflow:auto;box-sizing:border-box;background:#fff;border-radius:18px;padding:28px;text-align:left;box-shadow:0 24px 70px #14243e33}.activation-verification h2{margin:0;font-size:20px}.activation-verification p{font-size:13px;line-height:1.7;color:#6b7d98}.activation-verification label{display:grid;gap:8px;margin-top:16px;font-size:13px}.activation-verification input{box-sizing:border-box;width:100%;min-height:42px;padding:10px;border:1px solid #d7e2f1;border-radius:8px}.activation-verification input:focus{outline:2px solid #2478f533;border-color:#2478f5}
.activation-page{min-height:100dvh;background:radial-gradient(ellipse at 20% 15%,#e5efff,transparent 55%),radial-gradient(ellipse at 85% 80%,#e5f6f4,transparent 45%),#f5f8fc;color:#14243e;padding:0 24px 48px;box-sizing:border-box}.activation-header{height:88px;max-width:1240px;margin:auto;display:flex;align-items:center;justify-content:space-between}.activation-header>div{display:flex;gap:12px;align-items:center}.activation-header img{width:42px;height:42px;object-fit:contain}.activation-header span{font-size:19px;font-weight:700}.activation-header small{display:block;margin-top:3px;color:#8393b0;font-size:11px;font-weight:500}.activation-header a{color:#637494;text-decoration:none;font-size:13px}.activation-card{max-width:650px;margin:clamp(20px,6vh,72px) auto 0;padding:40px 44px 26px;background:#ffffffed;border:1px solid #dfe8f5;border-radius:24px;box-shadow:0 24px 70px #26487812;text-align:center}.activation-symbol{display:grid;place-items:center;margin:0 auto 20px;width:76px;height:76px;border-radius:23px;background:linear-gradient(145deg,#eaf2ff,#dcecff);color:#2478f5}.activation-symbol svg{width:38px;height:38px}.activation-badge{font-size:12px;color:#966514;background:#fff5de;padding:5px 12px;border-radius:20px}h1{font-size:30px;margin:18px 0 12px;letter-spacing:.5px}.activation-intro{max-width:500px;margin:0 auto 26px;color:#6b7d98;line-height:1.9;font-size:14px}.activation-identity{margin:22px 0;text-align:left;border:1px solid #e3eaf4;border-radius:12px;background:#f8faff;padding:6px 18px}.activation-identity>div{display:grid;grid-template-columns:114px minmax(0,1fr);gap:10px;padding:13px 0;align-items:start}.activation-identity>div+div{border-top:1px solid #e7edf6}.activation-identity dt{font-size:12px;color:#7a8ba7}.activation-identity dd{margin:0;font:13px ui-monospace,Consolas,monospace;overflow-wrap:anywhere;user-select:all}.activation-actions{display:flex;gap:10px;justify-content:center;flex-wrap:wrap;margin-top:24px}.activation-actions button{min-height:42px;padding:10px 17px;border-radius:9px}.activation-error,.activation-reason{padding:12px 16px;font-size:13px;line-height:1.7;text-align:left;border-radius:9px;overflow-wrap:anywhere}.activation-error{background:#fff0f0;color:#b32626}.activation-reason{background:#fff8e8;color:#91682e}.activation-hint{font-size:12px;line-height:1.8;color:#8994a8;text-align:left}.activation-success{font-size:13px;color:#17805a;line-height:1.8}footer{margin-top:28px;padding-top:20px;border-top:1px solid #edf1f7;color:#93a1b7;font-size:11px}@media(max-width:600px){.activation-page{padding:0 14px 28px}.activation-header{height:72px}.activation-card{padding:28px 20px 22px;border-radius:18px;margin-top:20px}.activation-identity>div{grid-template-columns:90px minmax(0,1fr)}h1{font-size:26px}.activation-actions button.primary{width:100%}}
</style>

<style scoped>
/* Full-bleed white header with the same compact brand as the gateway shell. */
.activation-header{
 position:static;
 z-index:auto;
 top:auto;
 height:56px;
 min-height:56px;
 width:calc(100% + 48px);
 max-width:none;
 margin:0 -24px;
 padding:0 24px;
 border:0;
 border-bottom:1px solid #e8edf4;
 background:#fff;
 backdrop-filter:none;
 box-shadow:none;
}
.activation-header .activation-brand{
 width:auto;
 height:56px;
 padding:8px 0;
 gap:9px;
 flex-shrink:0;
 background:#fff;
 border:0;
 border-radius:0;
 box-shadow:none;
}
.activation-header img{width:34px;height:34px;mix-blend-mode:normal}
.activation-header span{font-size:14px;line-height:18px;color:#0f172a;font-weight:800}
.activation-header small{margin-top:2px;font-size:10px;line-height:14px;color:#94a3b8;font-weight:700;letter-spacing:.04em}
.activation-header form{margin:0}
.activation-header button{
 min-height:36px;
 padding:7px 12px;
 color:#637494;
 border:1px solid #d7e2f1;
 border-radius:9px;
 background:transparent;
 box-shadow:none;
}
.activation-header button:hover{color:#1769e0;background:#ffffff70}
.activation-card{margin:24px auto 0}
.activation-badge.confirmed{background:#e8f7ef;color:#17805a}
.activation-progress{margin-top:18px;padding:17px 18px 15px;border:1px solid #dce9fc;border-radius:12px;background:#f4f8ff;text-align:left}
.activation-progress-content{display:flex;align-items:center;gap:14px}
.activation-progress-content>div{min-width:0}
.activation-progress strong{display:block;color:#20589f;font-size:14px;font-weight:600}
.activation-progress p{margin:5px 0 0;font-size:12px;line-height:1.8;color:#7388a6}
.activation-spinner{box-sizing:border-box;flex:none;width:28px;height:28px;border:3px solid #dce8fb;border-top-color:#2478f5;border-radius:50%;animation:activation-spin 1s linear infinite}
.activation-progress-track{height:3px;overflow:hidden;background:#e0eafb;border-radius:4px;margin-top:14px}
.activation-progress-track span{display:block;width:36%;height:100%;border-radius:4px;background:linear-gradient(90deg,#9cc9ff,#2478f5);animation:activation-travel 1.8s ease-in-out infinite}
@keyframes activation-spin{to{transform:rotate(360deg)}}
@keyframes activation-travel{from{transform:translateX(-110%)}to{transform:translateX(390%)}}
@media(prefers-reduced-motion:reduce){.activation-spinner,.activation-progress-track span{animation:none}.activation-progress-track span{width:100%;opacity:.45}}
@media(max-width:600px){
 .activation-header{height:56px;min-height:56px;width:calc(100% + 28px);margin:0 -14px;padding:0 14px;gap:10px}
 .activation-header .activation-brand{width:auto;padding:8px 0}
 .activation-card{margin-top:18px}
}
</style>

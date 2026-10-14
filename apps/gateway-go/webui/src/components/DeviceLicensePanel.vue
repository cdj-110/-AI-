<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,ref} from 'vue'
import {api} from '../api'
import {confirmAction} from '../confirm'
import {formatLicenseExpiry} from '../license-expiry'
import {IconShieldCheck,IconFingerprint,IconCertificate,IconCheck,IconRefresh,IconUpload,IconLock} from '@tabler/icons-vue'
type LicenseDetails={gatewayKey:string;canReplace:boolean;license:{valid:boolean;restricted:boolean;reason?:string};authorization:{sn:string;hardwareId:string;chipUsid:string;licenseVersion:number;keyId:string;licenseSequence:number;issuedAt:string;expiresAt:string;features:string[]|null;signed:boolean}}
const details=ref<LicenseDetails>(),loading=ref(false),busy=ref(false),restarting=ref(false),error=ref(''),notice=ref('')
const upload=ref<HTMLInputElement>()
let restartTimer=0,restartDeadline=0,restartStarted=0,previousBoot='',disposed=false
const canReplace=computed(()=>details.value?.canReplace===true&&!loading.value&&!busy.value&&!restarting.value)
const featureNames:Record<string,string>={collection:'数据采集',forward:'数据转发',forwarding:'数据转发',cloud:'云平台连接','remote-access':'远程访问','remote-storage':'远程数据库存储','remote-ssh':'远程 SSH',remote_access:'远程访问',remoteAccess:'远程访问',ssh:'远程 SSH',edge:'边缘计算',history:'历史存储',alarms:'告警管理'}
function formatTime(value?:string){if(!value)return'未设置';const date=new Date(value);return Number.isNaN(date.getTime())?value:date.toLocaleString('zh-CN',{hour12:false})}
async function loadDetails(){
 loading.value=true;error.value=''
 try{details.value=await api<LicenseDetails>('/api/license/details')}
 catch(cause){details.value=undefined;error.value=cause instanceof Error?cause.message:'授权信息读取失败'}
 finally{loading.value=false}
}
async function readBoot(){
 const response=await fetch(`/api/readyz?license_probe=${Date.now()}`,{cache:'no-store',signal:AbortSignal.timeout(2500)})
 const status=await response.json() as {ready?:boolean;bootId?:string}
 if(!response.ok||!status.ready)throw new Error('not ready')
 return status.bootId||''
}
async function probeRestart(){
 if(disposed||!restarting.value)return
 try{const boot=await readBoot();if(disposed||!restarting.value)return;if(previousBoot?boot&&boot!==previousBoot:Date.now()-restartStarted>=5000){window.location.reload();return}}catch{}
 if(disposed||!restarting.value)return
 if(Date.now()>=restartDeadline){restarting.value=false;error.value='授权已保存，但暂未检测到服务恢复，请稍后刷新页面。';return}
 restartTimer=window.setTimeout(probeRestart,1500)
}
async function replaceLicense(event:Event){
 const input=event.target as HTMLInputElement,file=input.files?.[0];input.value=''
 if(!file||!canReplace.value)return
 busy.value=true;error.value='';notice.value=''
 try{
  if(file.size>1024*1024)throw new Error('授权文件不能超过 1 MB')
  const content=await file.text(),document=JSON.parse(content)
  if(!document.signature||!document.hardwareId||!document.chipUsid||!document.sn)throw new Error('请使用管理平台签发的完整授权文件')
  const options={headers:{'Content-Type':'application/json'},body:JSON.stringify({content}),timeoutMs:15000}
  const preview=await api<{sn:string;licenseSequence:number;expiresAt:string}>('/api/activation',{...options,method:'POST'})
  if(!await confirmAction({kind:'license',title:'确认更换设备授权',message:'请确认新授权信息',fields:[{label:'授权设备',value:preview.sn},{label:'新授权序号',value:String(preview.licenseSequence||'旧版')},{label:'有效期',value:formatLicenseExpiry(preview.expiresAt)}],detail:'云连接信息将随授权更新，服务可能短暂重启。校验失败不会替换当前授权。',confirmText:'确认更换'}))return
  try{previousBoot=await readBoot()}catch{previousBoot=''}
  const result=await api<{restartRequired?:boolean}>('/api/activation',{...options,method:'PUT'})
  if(disposed)return
  if(result.restartRequired){notice.value='授权更换成功，正在等待网关重启恢复…';restarting.value=true;restartStarted=Date.now();restartDeadline=restartStarted+120000;restartTimer=window.setTimeout(probeRestart,1500)}
  else{notice.value='设备授权更换成功';await loadDetails()}
 }catch(cause){error.value=cause instanceof Error?cause.message:'授权更换失败，请检查文件后重试'}
 finally{busy.value=false}
}
onMounted(loadDetails)
onBeforeUnmount(()=>{disposed=true;restarting.value=false;window.clearTimeout(restartTimer)})
</script>

<template>
 <section class="device-license-panel" aria-labelledby="device-license-title">
  <header class="license-overview"><div class="license-heading"><span class="license-emblem"><IconShieldCheck :size="28" stroke="1.6" aria-hidden="true"/></span><div><h3 id="device-license-title">设备授权</h3><p>管理当前网关的授权信息与可用功能</p></div></div><span v-if="details" class="license-status" :class="details.license.valid&&!details.license.restricted?'valid':'restricted'"><IconShieldCheck :size="17" aria-hidden="true"/>{{details.license.valid&&!details.license.restricted?'授权有效':'授权受限'}}</span></header>
  <p v-if="loading" class="meta" role="status">正在读取设备授权…</p>
  <template v-if="details">
   <div class="license-details-grid"><section class="license-detail-card"><header class="license-card-heading"><IconCertificate :size="20" aria-hidden="true"/><h4>授权信息</h4><span>LICENSE</span></header><dl class="license-information">
    <div><dt>授权设备 SN</dt><dd>{{details.authorization.sn||details.gatewayKey}}</dd></div><div><dt>授权序号</dt><dd>{{details.authorization.licenseSequence||'旧版授权'}}</dd></div>
    <div><dt>签发时间</dt><dd>{{formatTime(details.authorization.issuedAt)}}</dd></div><div><dt>有效期</dt><dd>{{formatLicenseExpiry(details.authorization.expiresAt)}}</dd></div>
   </dl></section><section class="license-detail-card"><header class="license-card-heading"><IconFingerprint :size="20" aria-hidden="true"/><h4>设备身份与签名</h4><span>IDENTITY</span></header><dl class="license-identity">
    <div><dt>HardwareId</dt><dd class="mono">{{details.authorization.hardwareId||'未设置'}}</dd></div><div><dt>芯片 USID</dt><dd class="mono">{{details.authorization.chipUsid||'未设置'}}</dd></div>
    <div class="license-signature"><dt>授权版本 / 数字签名</dt><dd><span class="license-version">V{{details.authorization.licenseVersion||1}}</span><span class="signature-state"><IconCheck v-if="details.authorization.signed" :size="14" aria-hidden="true"/>{{details.authorization.signed?'已签名':'未签名'}}</span></dd></div><div><dt>签名密钥版本</dt><dd class="license-key">{{details.authorization.keyId||'旧版'}}</dd></div>
   </dl></section></div>
   <section class="license-features"><header class="license-card-heading"><IconCheck :size="20" aria-hidden="true"/><h4>已授权功能</h4><span>{{details.authorization.features?.length||0}} 项</span></header><div class="license-feature-list"><span v-for="feature in details.authorization.features||[]" :key="feature" class="license-feature"><IconCheck :size="14" aria-hidden="true"/>{{featureNames[feature]||feature}}</span><span v-if="!details.authorization.features?.length" class="meta">未列明</span></div></section>
   <p v-if="details.license.reason" class="meta">{{details.license.reason}}</p>
  </template>
  <p v-if="error" class="inline-result bad" role="alert">{{error}}</p><p v-if="notice" class="inline-result good" role="status">{{notice}}</p>
  <div class="license-footer"><div class="license-security-note"><IconLock :size="18" aria-hidden="true"/><p><strong>安全校验保护</strong><span>更换前校验签名、设备身份、有效期与序号；校验失败不会替换当前授权。</span></p></div><div class="point-actions"><input ref="upload" type="file" accept=".lic,.json,application/json" hidden @change="replaceLicense"/><button :disabled="loading||busy||restarting" @click="loadDetails"><IconRefresh :size="16" aria-hidden="true"/>刷新授权状态</button><button v-if="details?.canReplace" class="primary" :disabled="!canReplace" @click="upload?.click()"><IconUpload :size="16" aria-hidden="true"/>{{busy?'正在校验授权…':restarting?'等待服务恢复…':'更换授权文件'}}</button></div></div>
  <p v-if="details&&!details.canReplace" class="meta">当前账号仅可查看授权，更换授权请使用有授权管理权限的账号。</p>
 </section>
</template>

<style scoped>
.device-license-panel{padding:16px 0 0;color:#243955}
.device-license-panel header{position:static;z-index:auto;height:auto;top:auto;box-shadow:none;backdrop-filter:none}.device-license-panel .license-card-heading{padding:0;background:transparent;border:0;justify-content:flex-start}
.license-overview{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:22px 24px;background:#f3f7ff;border:1px solid #e2ebfa;border-radius:14px;margin-bottom:18px}
.license-heading{display:flex;align-items:center;gap:14px;min-width:0}.license-emblem{display:flex;align-items:center;justify-content:center;width:50px;height:50px;flex-shrink:0;border-radius:14px;background:#e4edff;color:#2875ec}.license-heading h3{margin:0 0 6px;font-size:20px;font-weight:700;line-height:1.3}.license-heading p{margin:0;font-size:13px;color:#687f9d;line-height:1.6}
.license-status{display:inline-flex;align-items:center;gap:7px;flex-shrink:0;border:1px solid #c8eadb;border-radius:8px;padding:8px 12px;font-size:13px;font-weight:600;background:#edf9f3;color:#137449}.license-status.restricted{background:#fff6e7;border-color:#f4ddb3;color:#925b13}
.license-details-grid{display:grid;grid-template-columns:1.05fr 1fr;gap:16px;margin-bottom:16px}.license-detail-card,.license-features{min-width:0;border:1px solid #e3eaf4;border-radius:12px;background:#fff;padding:20px 22px}.license-card-heading{display:flex;align-items:center;gap:9px;color:#5d7b9e;margin-bottom:12px}.license-card-heading h4{margin:0;font-size:14px;font-weight:600;color:#2b405c}.license-card-heading>span{margin-left:auto;font-size:10px;color:#7e91ab;letter-spacing:1px}
.license-information,.license-identity{margin:0;display:grid;grid-template-columns:1fr}.license-information>div{display:grid;grid-template-columns:100px minmax(0,1fr);gap:14px;align-items:start;padding:15px 0;border-bottom:1px solid #edf1f6}.license-information>div:last-child{border:0}.license-information dt,.license-identity dt{font-size:12px;color:#6e819b;line-height:1.65}.license-information dd,.license-identity dd{margin:0;font-size:13px;color:#263e5e;line-height:1.65;overflow-wrap:anywhere;user-select:all;font-variant-numeric:tabular-nums}.license-information>div:first-child dd{font-weight:700;font-size:15px;color:#215fac}.license-identity{grid-template-columns:repeat(2,minmax(0,1fr));gap:18px 20px;padding-top:7px}.license-identity>div:first-child,.license-identity>div:last-child{grid-column:1/-1}.license-identity dt{margin-bottom:6px}.license-identity .mono{font-size:12px;letter-spacing:.15px}.license-signature dd{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.license-version{padding:1px 7px;border:1px solid #dbe5f5;border-radius:5px;font-size:11px;color:#3d6ca8;background:#f5f8fd}.signature-state{display:inline-flex;align-items:center;gap:4px;font-size:12px;color:#536d8b}.license-key{font-size:12px!important;color:#536d8b!important}
.license-features{margin-bottom:16px;padding-bottom:20px}.license-feature-list{display:flex;flex-wrap:wrap;gap:8px;padding-top:4px}.license-feature{display:inline-flex;align-items:center;gap:6px;padding:8px 11px;font-size:12px;line-height:1.2;color:#3f5e7e;background:#f4f7fb;border:1px solid #e5ebf4;border-radius:7px;font-weight:500}.license-feature svg{color:#25845e;flex-shrink:0}
.license-footer{display:flex;justify-content:space-between;align-items:center;gap:20px;padding:18px 20px;background:#f8fafd;border:1px solid #e6ecf5;border-radius:10px}.license-security-note{display:flex;align-items:center;gap:10px;min-width:0;color:#6c82a0}.license-security-note>svg{flex-shrink:0}.license-security-note p{margin:0;line-height:1.6}.license-security-note strong{display:block;font-size:12px;font-weight:600;color:#506b8b;margin-bottom:3px}.license-security-note span{display:block;font-size:12px}.license-footer .point-actions{display:flex;gap:8px;flex-shrink:0}.license-footer button{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:36px;font-size:12px;white-space:nowrap}.license-footer button:focus-visible{outline:2px solid #327cf2;outline-offset:3px}
@media(max-width:1100px){.license-footer{align-items:flex-start;flex-direction:column}.license-footer .point-actions{align-self:flex-end}.license-information>div{grid-template-columns:88px minmax(0,1fr);gap:10px}}
@media(max-width:760px){.license-details-grid{grid-template-columns:1fr}.license-overview{padding:18px 16px;align-items:flex-start;flex-wrap:wrap}.license-heading{gap:10px}.license-heading h3{font-size:18px}.license-emblem{width:42px;height:42px;border-radius:11px}.license-status{padding:6px 10px}.license-detail-card,.license-features{padding:18px 16px}.license-identity{gap:16px}.license-footer{padding:16px}.license-footer .point-actions{align-self:stretch;flex-wrap:wrap}.license-footer button{flex:1;min-height:40px}}
</style>

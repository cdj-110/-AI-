<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import { confirmAction } from '../confirm'
import type { CollectionIncidentPage } from '../types'

const props=defineProps<{canManage?:boolean}>()
const page=ref<CollectionIncidentPage>({items:[],page:1,pageSize:30,total:0,totalPages:0})
const allTotal=ref(0)
const pageNumber=ref(1)
const state=ref('ALL')
const protocol=ref('')
const keyword=ref('')
const loading=ref(false)
const error=ref('')
const message=ref('')
const refreshedAt=ref<Date>()
let refreshTimer=0

const visibleActiveCount=computed(()=>page.value.items.filter(item=>item.state==='ACTIVE').length)
const protocolOptions=computed(()=>Array.from(new Set(page.value.items.map(item=>item.protocol).filter(Boolean))).sort())

function formatTime(value?:string){return value?new Date(value).toLocaleString():'-'}
function stateText(value:string){return ({ACTIVE:'异常中',RECOVERED:'已恢复',CHANGED:'原因变化',INTERRUPTED:'重启中断'} as Record<string,string>)[value]||value}
function stateClass(value:string){return value==='ACTIVE'?'bad':value==='RECOVERED'?'ok':value==='CHANGED'?'warn':''}

async function load(reset=false){
  if(reset)pageNumber.value=1
  loading.value=true
  try{
    const params=new URLSearchParams({page:String(pageNumber.value),pageSize:'30',state:state.value})
    if(protocol.value)params.set('protocol',protocol.value)
    if(keyword.value.trim())params.set('keyword',keyword.value.trim())
    page.value=await api<CollectionIncidentPage>(`/api/collection-incidents?${params}`,{timeoutMs:10000})
    if(state.value==='ALL'&&!protocol.value&&!keyword.value.trim())allTotal.value=page.value.total
    error.value=''
    refreshedAt.value=new Date()
  }catch(reason){error.value=reason instanceof Error?reason.message:'采集异常记录读取失败'}
  finally{loading.value=false}
}

async function clearRecords(){
  if(!props.canManage||!allTotal.value)return
  if(!await confirmAction({title:'清空采集异常记录',message:`确认清空全部 ${allTotal.value} 条采集异常记录？`,detail:'该操作只删除采集异常历史，不会影响设备、点位、工程配置和历史数据；删除后不可恢复。',confirmText:'确认清空',danger:true}))return
  loading.value=true;error.value='';message.value=''
  try{
    await api('/api/collection-incidents/clear',{method:'DELETE',timeoutMs:10000})
    allTotal.value=0
    pageNumber.value=1
    await load()
    message.value='采集异常记录已清空。'
  }catch(reason){error.value=reason instanceof Error?reason.message:'采集异常记录清理失败'}
  finally{loading.value=false}
}

async function changePage(step:number){
  const next=pageNumber.value+step
  if(next<1||next>Math.max(1,page.value.totalPages))return
  pageNumber.value=next
  await load()
}

function resetFilters(){keyword.value='';state.value='ALL';protocol.value='';void load(true)}

onMounted(()=>{void load();refreshTimer=window.setInterval(()=>load(),10000)})
onBeforeUnmount(()=>window.clearInterval(refreshTimer))
</script>

<template>
  <section class="collection-incidents-page">
    <section class="card collection-incidents-card">
      <div class="collection-incidents-head">
        <div><h2>采集异常记录</h2><p>记录各协议采集失败、原因变化与恢复过程</p></div>
        <div class="collection-incidents-summary"><span><b>{{page.total}}</b>筛选结果</span><span :class="{danger:visibleActiveCount>0}"><b>{{visibleActiveCount}}</b>本页异常中</span><button :disabled="loading" @click="load()">{{loading?'刷新中…':'刷新'}}</button><button v-if="canManage" class="danger-outline" :disabled="loading||!allTotal" @click="clearRecords">清空记录</button></div>
      </div>
      <div class="collection-incident-toolbar">
        <input v-model="keyword" placeholder="搜索设备、点位、地址或异常原因" @keyup.enter="load(true)"/>
        <select v-model="state" @change="load(true)"><option value="ALL">全部状态</option><option value="ACTIVE">异常中</option><option value="RECOVERED">已恢复</option><option value="CHANGED">原因变化</option><option value="INTERRUPTED">重启中断</option></select>
        <select v-model="protocol" @change="load(true)"><option value="">全部协议</option><option v-for="item in protocolOptions" :key="item" :value="item">{{item}}</option></select>
        <button @click="resetFilters">重置</button>
      </div>
      <div class="collection-incidents-meta"><span>异常记录每 10 秒自动刷新</span><span>{{refreshedAt?`最近刷新：${refreshedAt.toLocaleTimeString()}`:'尚未刷新'}}</span></div>
      <p v-if="message" class="notice ok">{{message}}</p>
      <p v-if="error" class="notice bad">{{error}}</p>
      <p v-if="page.lastError" class="notice bad">记录数据库最近写入失败：{{page.lastError}}</p>
      <div class="table-wrap collection-incident-table"><table><thead><tr><th>状态</th><th>发生 / 恢复时间</th><th>设备 / 点位</th><th>协议 / 地址</th><th>异常原因</th></tr></thead><tbody>
        <tr v-if="loading&&!page.items.length"><td colspan="5" class="empty">正在读取异常记录…</td></tr>
        <tr v-else-if="!page.items.length"><td colspan="5" class="empty">暂无符合条件的采集异常记录</td></tr>
        <tr v-for="item in page.items" :key="item.id"><td><span class="state-tag" :class="stateClass(item.state)">{{stateText(item.state)}}</span></td><td><strong>{{formatTime(item.occurredAt)}}</strong><small>{{item.recoveredAt?`恢复：${formatTime(item.recoveredAt)}`:`最近：${formatTime(item.lastSeenAt)}`}}</small></td><td><strong>{{item.deviceKey}}</strong><small>{{item.pointName||item.metric}} · {{item.metric}}</small></td><td><strong>{{item.protocol||'-'}}</strong><small class="mono">{{item.address||'-'}}</small></td><td class="collection-incident-reason"><strong>{{item.explanation}}</strong><code :title="item.reason">原始错误：{{item.reason}}</code></td></tr>
      </tbody></table></div>
      <div class="collection-incident-pagination"><span>共 {{page.total}} 条 · 第 {{page.page}} / {{Math.max(1,page.totalPages)}} 页</span><button :disabled="page.page<=1||loading" @click="changePage(-1)">上一页</button><button :disabled="page.page>=page.totalPages||loading" @click="changePage(1)">下一页</button></div>
    </section>
  </section>
</template>

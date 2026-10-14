<script setup lang="ts">
import {nextTick,onBeforeUnmount,ref,watch} from 'vue'
import {IconSitemap,IconMaximize,IconMinimize,IconX} from '@tabler/icons-vue'
import StatusView from './StatusView.vue'
import TopologyView from './TopologyView.vue'
import type {GatewayStatus} from '../types'
import type {TopologySelection} from '../topology'

const props=defineProps<{active:'status'|'topology';canStatus:boolean;canTopology:boolean;canConfig?:boolean;canIncidents?:boolean;status?:GatewayStatus|null}>()
const emit=defineEmits<{navigate:[key:'status'|'topology'|'config'|'collection-incidents'];openDevice:[selection:TopologySelection]}>()
const dialog=ref<HTMLDialogElement>(),opened=ref(false),expanded=ref(false)
let previousOverflow:string|undefined
function unlockScroll(){if(previousOverflow!==undefined){document.body.style.overflow=previousOverflow;previousOverflow=undefined}}
async function openTopology(){
  if(!props.canTopology||opened.value)return
  opened.value=true
  await nextTick()
  if(!opened.value||!dialog.value)return
  previousOverflow=document.body.style.overflow
  document.body.style.overflow='hidden'
  dialog.value.showModal()
}
function closeTopology(returnToStatus=true){
  opened.value=false
  expanded.value=false
  dialog.value?.close()
  unlockScroll()
  if(returnToStatus&&props.active==='topology'&&props.canStatus)emit('navigate','status')
}
function backdropClick(event:MouseEvent){
  if(event.target!==dialog.value||!dialog.value)return
  const bounds=dialog.value.getBoundingClientRect()
  if(event.clientX<bounds.left||event.clientX>bounds.right||event.clientY<bounds.top||event.clientY>bounds.bottom)closeTopology()
}
function navigate(key:'config'|'collection-incidents'|'topology'){
  if(key==='topology')void openTopology()
  else emit('navigate',key)
}
function openDevice(selection:TopologySelection){closeTopology(false);emit('openDevice',selection)}
watch(()=>[props.active,props.canTopology] as const,([active,allowed])=>{
  if(!allowed)closeTopology()
  else if(active==='topology')void openTopology()
  else if(opened.value)closeTopology(false)
},{immediate:true,flush:'post'})
onBeforeUnmount(()=>{opened.value=false;dialog.value?.close();unlockScroll()})
</script>

<template>
  <section class="status-workspace">
    <StatusView v-if="canStatus" :can-topology="canTopology" :can-config="canConfig" :can-incidents="canIncidents" @navigate="navigate" @open-device="emit('openDevice',$event)"/>
    <section v-else-if="canTopology" class="card topology-entry-only">
      <p class="meta">查看网关资源与设备连接关系</p>
      <button type="button" @click="openTopology"><IconSitemap :size="18" aria-hidden="true"/>设备拓扑</button>
    </section>
      <dialog ref="dialog" class="topology-dialog" :class="{expanded}" aria-label="设备拓扑" @cancel.prevent="closeTopology()" @click="backdropClick">
        <TopologyView v-if="opened&&canTopology" :status="status" @open-device="openDevice">
          <template #actions>
            <button type="button" class="topology-window-action" :aria-label="expanded?'退出全屏':'全屏查看'" :title="expanded?'退出全屏':'全屏查看'" :aria-pressed="expanded" @click="expanded=!expanded"><component :is="expanded?IconMinimize:IconMaximize" :size="19" aria-hidden="true"/></button>
            <button type="button" class="topology-window-action" aria-label="关闭设备拓扑" title="关闭" autofocus @click="closeTopology()"><IconX :size="20" aria-hidden="true"/></button>
          </template>
        </TopologyView>
      </dialog>
  </section>
</template>

<style scoped>
.status-workspace{min-width:0}
.topology-entry-only button{display:inline-flex;align-items:center;gap:8px}
.topology-dialog{box-sizing:border-box;width:min(1600px,94vw);max-width:none;height:90dvh;max-height:none;margin:auto;padding:0;border:1px solid #dfe8f5;border-radius:16px;background:#fff;color:#172b4d;box-shadow:0 24px 80px #0f234333;overflow:auto;overscroll-behavior:contain}
.topology-dialog::backdrop{background:rgba(15,30,52,.4)}
.topology-dialog.expanded{width:100vw;height:100dvh;border:0;border-radius:0}
.topology-dialog :deep(.topology-page){min-height:100%;margin:0;border:0;border-radius:0;box-shadow:none}
.topology-window-action{display:inline-flex;align-items:center;justify-content:center;flex-shrink:0;width:36px;height:36px;padding:0;color:#61738e}
.topology-window-action:focus-visible{outline:2px solid #1677ff;outline-offset:2px}
@media(max-width:650px){.topology-dialog{width:100vw;height:100dvh;border:0;border-radius:0}}
</style>

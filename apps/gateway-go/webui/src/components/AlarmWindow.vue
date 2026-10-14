<script setup lang="ts">
import {onBeforeUnmount,onMounted,provide,ref} from 'vue'
import {api} from '../api'
import AlarmNotificationPanel from './AlarmNotificationPanel.vue'
import {alarmPresentationKey,useAlarmPresentation} from '../alarmPresentation.ts'
import {alarmPanelWindowKey,useAlarmPanelWindow} from '../composables/useAlarmPanelWindow.ts'
import {detectNewActiveAlarms,updatePersistentAlarmNotifications,type AlarmEvent,type AlarmNotification,type AlarmSnapshot} from '../alarmNotifications.ts'
const username=ref(''),authorized=ref(false),error=ref(''),loading=ref(true)
const target=useAlarmPanelWindow(()=>username.value,true)
const audio=useAlarmPresentation(()=>username.value,()=>target.ownsAudio.value,target.setAudioReady,()=>target.audioAvailable.value)
provide(alarmPanelWindowKey,target);provide(alarmPresentationKey,audio)
const notifications=ref<AlarmNotification[]>([]),activeEvents=ref<AlarmEvent[]>([]),canAcknowledge=ref(false)
let timer=0,busy=false,initialized=false,known=new Set<string>(),disposed=false
async function refresh(){
 if(busy||!authorized.value)return
 busy=true
 try{const next=await api<AlarmSnapshot>('/api/alarms?activeOnly=1');if(disposed)return
  const detected=detectNewActiveAlarms(known,next,initialized);known=detected.eventIds
  activeEvents.value=next.active
  notifications.value=updatePersistentAlarmNotifications(notifications.value,next.active)
  audio.update(next.active,detected.events);initialized=true;error.value=''
 }catch{error.value='告警更新失败，正在重试；当前展示为上次成功读取的数据。'}finally{busy=false}
}
function acknowledged(eventIds:string[]){
 const ids=new Set(eventIds)
 activeEvents.value=activeEvents.value.map(e=>ids.has(e.eventId)?{...e,acknowledged:true}:e)
 notifications.value=updatePersistentAlarmNotifications(notifications.value,activeEvents.value)
 audio.update(activeEvents.value,[])
 window.dispatchEvent(new Event('gateway-alarms-changed'))
}
function close(){window.close();if(!window.closed)window.location.replace('/?page=alarms')}
function center(){window.location.assign('/?page=alarms')}
function unlock(){void audio.unlock()}
onMounted(async()=>{
 document.title='告警面板 · 微控网关'
 try{const session=await api<{username:string;permissions:string[]}>('/api/session');if(disposed)return
  // Match the app's existing implied alarm-view permissions.
  authorized.value=(session.permissions||[]).some(p=>['*','alarms.view','alarms.manage','status.view','config.view','config.manage','edge.view','edge.manage'].includes(p))
  canAcknowledge.value=(session.permissions||[]).some(p=>['*','alarms.manage','config.manage'].includes(p))
  if(authorized.value){username.value=session.username;await refresh();timer=window.setInterval(refresh,2000)}else error.value='当前账号没有告警查看权限。'
 }catch{error.value='无法读取登录信息，请重新登录。'}finally{loading.value=false}
 window.addEventListener('pointerdown',unlock,{passive:true});window.addEventListener('keydown',unlock);window.addEventListener('gateway-alarms-changed',refresh)
})
onBeforeUnmount(()=>{disposed=true;window.clearInterval(timer);window.removeEventListener('pointerdown',unlock);window.removeEventListener('keydown',unlock);window.removeEventListener('gateway-alarms-changed',refresh)})
</script>
<template>
 <AlarmNotificationPanel v-if="authorized" :notifications="notifications" :can-acknowledge="canAcknowledge" @acknowledged="acknowledged" :layout-scope="username" :status-message="error" @close="close" @open="center"/>
 <main v-else class="alarm-window-loading"><h1>告警面板</h1><p>{{loading?'正在加载…':error}}</p><a v-if="!loading" href="/">返回网关首页</a></main>
</template>
<style scoped>.alarm-window-loading{padding:32px;font-family:inherit;color:#425570}.alarm-window-loading h1{font-size:22px}</style>

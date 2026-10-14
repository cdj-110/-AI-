<script setup lang="ts">
import {computed,inject,onMounted,onBeforeUnmount,ref} from 'vue'
import {alarmPresentationKey} from '../alarmPresentation.ts'
import {alarmPanelWindowKey} from '../composables/useAlarmPanelWindow.ts'
import type {AlarmNotification} from '../alarmNotifications.ts'
import AlarmNotificationCard from './AlarmNotificationCard.vue'
import {clearAlarmCardPositions} from '../alarmCardPositions.ts'
const props=defineProps<{notifications:AlarmNotification[];canAcknowledge?:boolean;layoutScope?:string;statusMessage?:string}>()
const emit=defineEmits<{close:[];open:[];acknowledged:[eventIds:string[]]}>()
const audio=inject(alarmPresentationKey)
const target=inject(alarmPanelWindowKey)
const ordered=computed(()=>[...props.notifications].sort((a,b)=>Number(audio?.pinned.value.includes(b.groupId)||false)-Number(audio?.pinned.value.includes(a.groupId)||false)))
const element=ref<HTMLDialogElement>()
const list=ref<HTMLElement>(),layoutRevision=ref(0)
function resetLayout(){clearAlarmCardPositions(props.layoutScope||'');layoutRevision.value++}
const critical=computed(()=>props.notifications.filter(n=>n.event.level==='CRITICAL').length)
const warning=computed(()=>props.notifications.filter(n=>n.event.level==='WARNING').length)
// A native modal makes the covered workspace inert and keeps keyboard focus inside.
onMounted(()=>element.value?.showModal())
onBeforeUnmount(()=>element.value?.close())
</script>

<template>
  <dialog ref="element" class="alarm-notification-panel" aria-label="告警面板" @cancel.prevent="emit('close')">
    <header class="panel-heading">
      <div class="panel-title"><span class="panel-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/></svg></span><div><h1>告警面板</h1><p>集中查看告警提醒 · 同一规则合并展示</p></div></div>
      <div class="panel-heading-actions"><button v-if="audio" type="button" class="panel-mute-all" :class="{muted:audio.allMuted.value}" :disabled="!audio.hasActive.value" :aria-pressed="audio.allMuted.value" :aria-label="audio.allMuted.value?'取消全部静音':'全部静音'" :title="audio.allMuted.value?'取消当前全部告警静音；仍受声音总开关控制':'静音当前全部告警，不确认或恢复；新告警仍会提醒'" @click="audio.toggleMuteAll"><svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M11 5 6 9H3v6h3l5 4z"/><path v-if="audio.allMuted.value" d="m16 9 6 6m0-6-6 6"/><path v-else d="M15 8a6 6 0 0 1 0 8m3-11a10 10 0 0 1 0 14"/></svg></button><label v-if="target" class="panel-target"><span>下次打开</span><span :class="{chosen:target.mode.value==='page'}">本页展示</span><button type="button" role="switch" aria-label="新标签页打开告警面板" :aria-checked="target.mode.value==='tab'" :class="{enabled:target.mode.value==='tab'}" title="仅改变下次打开方式，当前面板保持不变" @click="target.setMode(target.mode.value==='tab'?'page':'tab')"><i/></button><span :class="{chosen:target.mode.value==='tab'}">新标签页</span></label>
      <button type="button" class="panel-close" autofocus :aria-label="target?.standalone.value?'关闭告警页':'返回原页面'" :title="target?.standalone.value?'关闭此标签页，不确认或恢复告警':'返回原页面，不确认或恢复告警'" @click="emit('close')"><svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5m7-7-7 7 7 7"/></svg>{{target?.standalone.value?'关闭告警页':'返回原页面'}}</button></div>
    </header>
    <div class="panel-summary"><span role="status"><b>{{notifications.length}}</b> 组提醒</span><span class="critical-count"><i/>严重 <b>{{critical}}</b></span><span class="warning-count"><i/>警告 <b>{{warning}}</b></span><small>拖动位置自动记忆 · 详细原因可展开查看</small><button type="button" class="layout-reset" title="清除当前账号在此浏览器保存的全部告警卡片位置" @click="resetLayout">重置布局</button></div>
    <div v-if="statusMessage||target?.notice.value" class="sound-blocked" role="status">{{statusMessage||target?.notice.value}}</div>
    <div v-if="audio?.blocked.value" class="sound-blocked" role="status">当前没有已获播放许可的告警页面。请点击一次启用声音。<button type="button" @click="audio.unlock">启用声音</button></div>
    <div v-else-if="audio?.allMuted.value" class="audio-status" role="status">当前告警已全部静音，新告警仍会提醒；可点击卡片喇叭单独取消静音。</div>
    <div v-else-if="target?.audioAvailable.value&&!target.ownsAudio.value&&audio?.settings.value.enabled&&audio.audible.value.length" class="audio-status" role="status">声音由另一个已启用声音的网关标签页继续播放，本页静音操作会同步生效。</div>
    <div ref="list" class="panel-list" tabindex="0" aria-label="告警通知列表">
      <AlarmNotificationCard v-for="notification in ordered" :key="notification.groupId" :notification="notification" :can-acknowledge="canAcknowledge" @acknowledged="emit('acknowledged',$event)" :bounds="list" :reset-key="layoutRevision" :layout-scope="layoutScope"/>
      <div v-if="!notifications.length" class="panel-empty"><span aria-hidden="true">✓</span><h2>暂无待展示的告警</h2><p>仅展示已启用面板提醒的未恢复告警；已知晓卡片按规则决定是否保留。</p><button type="button" @click="emit('close')">返回原页面</button></div>
    </div>
    <footer><span>未恢复告警按规则持续显示<small>已知晓卡片可保留至恢复 · 静音不隐藏卡片 · 关闭面板不确认或恢复告警</small></span><button type="button" @click="emit('open')">告警管理 →</button></footer>
  </dialog>
</template>

<style scoped>
.panel-mute-all{display:grid;place-items:center;width:36px;height:36px;padding:0;border:1px solid #d9e3f0;border-radius:8px;color:#425570;background:#fff;cursor:pointer}.panel-mute-all.muted{color:#1765c1;background:#edf4ff;border-color:#93bdf1}.panel-mute-all:disabled{opacity:.45;cursor:not-allowed}.audio-status{padding:8px 32px;font-size:12px;color:#526780;background:#edf4ff}
.panel-heading-actions{display:flex;align-items:center;justify-content:flex-end;gap:18px;flex-wrap:wrap}.panel-target{display:flex;align-items:center;gap:8px;font-size:12px;color:#718099;white-space:nowrap}.panel-target span{font-weight:400}.panel-target .chosen{font-weight:600;color:#1765c1}.panel-target button{position:relative;width:34px;height:20px;padding:0;border:0;border-radius:12px;background:#a9b8cc;cursor:pointer}.panel-target button.enabled{background:#1677ff}.panel-target i{display:block;position:absolute;top:3px;left:3px;width:14px;height:14px;border-radius:50%;background:#fff}.panel-target .enabled i{left:17px}@media(max-width:900px){.panel-heading-actions{gap:10px}.panel-target{font-size:11px;gap:5px}}@media(max-width:600px){.alarm-notification-panel>.panel-heading{flex-wrap:wrap}.panel-heading-actions{width:100%;justify-content:space-between}.panel-target{gap:7px}}

.sound-blocked{padding:8px 32px;font-size:12px;color:#946000;background:#fff7e6}.sound-blocked button{margin-left:12px;border:1px solid #dfc799;background:white;border-radius:5px;padding:5px 10px;color:inherit;cursor:pointer}
.layout-reset{border:1px solid #d9e3f0;border-radius:6px;background:#fff;color:#526780;padding:6px 10px;font-size:11px;cursor:pointer}.layout-reset:disabled{opacity:.5;cursor:default}
.alarm-notification-panel{position:fixed;inset:0;margin:0;padding:0;width:100vw;height:100dvh;max-width:none;max-height:none;border:0;border-radius:0;overflow:hidden;background:#f4f7fb;color:#203047;font-family:inherit;box-sizing:border-box}
.alarm-notification-panel[open]{display:flex;flex-direction:column}.alarm-notification-panel::backdrop{background:#f4f7fb}
.alarm-notification-panel>.panel-heading{position:static;z-index:auto;display:flex;align-items:center;justify-content:space-between;gap:20px;height:auto;min-height:96px;padding:20px 32px;background:#fff;border-bottom:1px solid #e1e8f1;backdrop-filter:none;box-shadow:none;flex-shrink:0}
.panel-title{display:flex;align-items:center;gap:14px;min-width:0}.panel-title h1{margin:0;font-size:23px;color:#142740;line-height:1.4}.panel-title p{margin:5px 0 0;font-size:12px;color:#718099}.panel-icon{display:grid;place-items:center;width:46px;height:46px;border-radius:12px;color:#326cb5;background:#edf4ff;flex-shrink:0}.panel-icon svg{width:25px;height:25px}
.panel-close{display:flex;align-items:center;gap:16px;flex-shrink:0;border:1px solid #d9e3f0;background:#fff;color:#425570;padding:9px 12px;border-radius:7px;font-size:12px;cursor:pointer}.panel-close span{font-size:20px;line-height:20px;font-weight:400}
.panel-summary{display:flex;align-items:center;gap:24px;flex-wrap:wrap;padding:18px 32px;font-size:12px;color:#64748b;flex-shrink:0}.panel-summary>span{display:flex;align-items:center;gap:7px}.panel-summary b{font-size:18px;color:#25384f;font-variant-numeric:tabular-nums}.panel-summary i{width:7px;height:7px;border-radius:50%;background:currentColor}.critical-count{color:#c73541}.warning-count{color:#a16c13}.panel-summary small{margin-left:auto;font-size:11px;color:#748399}
.panel-list{flex:1;min-height:0;display:grid;grid-template-columns:repeat(auto-fill,minmax(min(100%,340px),1fr));align-content:start;align-items:start;gap:16px;overflow:auto;overscroll-behavior:contain;padding:0 32px 24px;scrollbar-gutter:stable}.panel-list:has(.panel-empty){display:flex;align-items:center;justify-content:center}
.panel-list :deep(.alarm-panel-card){min-width:0;padding:18px;border-radius:10px}.panel-list :deep(header b){font-size:15px}.panel-list :deep(.point){margin:16px 0 12px}.panel-list :deep(.point strong){font-size:28px}.panel-list :deep(.meta){margin-top:14px}.panel-list :deep(details){margin-top:10px}
footer{display:flex;align-items:center;justify-content:space-between;gap:16px;border-top:1px solid #e1e8f1;background:#fff;padding:16px 32px;flex-shrink:0;font-size:12px;color:#64748b}footer small{display:block;font-size:11px;color:#7b889a;margin-top:4px}footer button,.panel-empty button{flex-shrink:0;border:1px solid #cbdcf3;background:#f2f7ff;color:#1765c1;border-radius:7px;padding:10px 16px;font-size:13px;cursor:pointer}
.panel-empty{text-align:center;padding:24px;font-size:13px;color:#6b7b91}.panel-empty>span{display:inline-grid;place-items:center;width:56px;height:56px;border-radius:50%;font-size:24px;color:#368368;background:#eaf5f0}.panel-empty h2{font-size:19px;color:#42536c;margin:18px 0 10px}.panel-empty p{line-height:1.8;margin:0 0 20px}
button:hover{filter:brightness(.98)}button:focus-visible,[tabindex]:focus-visible{outline:2px solid #1677ff;outline-offset:-2px}
@media(max-width:600px){.alarm-notification-panel>.panel-heading{min-height:86px;padding:16px;gap:12px}.panel-title{gap:10px}.panel-title h1{font-size:19px}.panel-title p{font-size:10px;max-width:190px}.panel-icon{display:none}.panel-close{padding:7px 9px;gap:8px;font-size:11px}.panel-summary{padding:14px 16px;gap:16px}.panel-summary small{width:100%;margin:0}.panel-list{grid-template-columns:minmax(0,1fr);padding:0 16px 16px;gap:12px}footer{padding:12px 16px;font-size:11px}footer small{font-size:10px;max-width:210px}footer button{padding:9px;font-size:12px}}
</style>

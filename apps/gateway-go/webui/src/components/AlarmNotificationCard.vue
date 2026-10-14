<script setup lang="ts">
import {computed,inject,ref} from 'vue'
import {api} from '../api'
import {acknowledgeCardEvents} from '../alarmCardAcknowledge.ts'
import {alarmPresentationKey} from '../alarmPresentation.ts'
import {alarmLimitText,alarmValueText,type AlarmNotification} from '../alarmNotifications.ts'
import {useAlarmCardDrag} from '../composables/useAlarmCardDrag.ts'
const props=defineProps<{notification:AlarmNotification;canAcknowledge?:boolean;bounds?:HTMLElement;resetKey?:number;layoutScope?:string}>()
const emit=defineEmits<{acknowledged:[eventIds:string[]]}>()
const acknowledging=ref(false),acknowledgeError=ref('')
async function acknowledge(){
  if(!props.canAcknowledge||acknowledging.value)return
  acknowledging.value=true;acknowledgeError.value=''
  try{
    const result=await acknowledgeCardEvents(props.notification.events,eventId=>api('/api/alarms/acknowledge',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({eventId})}))
    acknowledgeError.value=result.error
    if(result.acknowledged.length)emit('acknowledged',result.acknowledged)
  }finally{acknowledging.value=false}
}
const audio=inject(alarmPresentationKey)
const pinned=computed(()=>audio?.pinned.value.includes(props.notification.groupId)||false)
const muted=computed(()=>audio?.isMutedEvents(props.notification.events)||false)
const {element,slot,style,slotStyle,dragging,position,start,keyboard,raise}=useAlarmCardDrag(()=>props.bounds,()=>props.resetKey||0,()=>props.layoutScope||'',()=>props.notification.groupId,()=>pinned.value)
const event=computed(()=>props.notification.event)
const allAcknowledged=computed(()=>props.notification.events.every(item=>item.acknowledged))
const acknowledgedCount=computed(()=>props.notification.events.filter(item=>item.acknowledged).length)
function toggleCardMute(){
  if(!audio||allAcknowledged.value)return
  audio.toggleMuteEvents(props.notification.events)
  // Controls do not bubble pointerdown into dragging or global audio unlock.
  // Explicitly retain the user gesture needed when unmuting in a fresh tab.
  void audio.unlock()
}
const levelNames:Record<string,string>={INFO:'提示',WARNING:'警告',CRITICAL:'严重'}
</script>

<template>
  <div ref="slot" class="alarm-card-slot" :style="slotStyle">
  <article ref="element" :style="style" class="alarm-panel-card" :class="[event.level.toLowerCase(),{dragging,floating:!!position,acknowledged:allAcknowledged}]" :aria-label="`告警：${notification.ruleName}`" @pointerdown="raise">
    <header><div class="alarm-card-drag" role="button" tabindex="0" :aria-label="`拖动告警卡片：${notification.ruleName}`" title="拖动标题移动；方向键微调，Home 恢复自动排列" @pointerdown.stop="start" @keydown="keyboard"><span class="drag-grip" aria-hidden="true">⠿</span><span class="level">{{levelNames[event.level]||event.level}}</span><b :title="notification.ruleName">{{notification.ruleName}}</b></div></header>
    <div v-if="audio" class="card-shortcuts" @pointerdown.stop>
      <span v-if="muted||allAcknowledged" class="card-sound-state" role="status">{{allAcknowledged?'已知晓 · 不发声':'已静音'}}</span>
      <button type="button" :class="{selected:muted}" :disabled="allAcknowledged" :aria-pressed="muted" :aria-label="allAcknowledged?'已知晓，无需静音':muted?'取消静音':'静音此告警'" :title="allAcknowledged?'此卡片全部告警已知晓，已停止发声，保留至恢复':muted?'取消静音：恢复此组声音，仍受声音总开关控制':'静音此告警：仅静音本次，不确认或恢复；其他未静音告警仍可发声'" @click.stop="toggleCardMute">
        <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M11 5 6 9H3v6h3l5 4z"/><path v-if="muted||allAcknowledged" d="m16 9 6 6m0-6-6 6"/><path v-else d="M15 8a6 6 0 0 1 0 8m3-11a10 10 0 0 1 0 14"/></svg>
      </button>
      <button type="button" :class="{selected:pinned}" :aria-pressed="pinned" :aria-label="pinned?'取消置顶':'置顶此告警'" :title="pinned?'取消置顶：恢复普通层级':'置顶此告警：优先排列并保持在上层'" @click="audio.togglePin(notification.groupId)">
        <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3h8l-1 7 4 4v2H5v-2l4-4z" :fill="pinned?'currentColor':'none'"/><path d="M12 16v6"/></svg>
      </button>
    </div>
    <div class="point"><span :title="`${event.deviceKey} / ${event.metric}`">{{event.pointName||event.metric}}</span><strong>{{alarmValueText(event)}}<small v-if="event.unit"> {{event.unit}}</small></strong></div>
    <div v-if="event.parentRuleId" class="limits" aria-label="已触发的限值"><span v-for="item in notification.events" :key="item.eventId" :class="item.level.toLowerCase()" :title="item.message">{{alarmLimitText(item)}}<small v-if="item.acknowledged"> · 已知晓</small></span></div>
    <div v-else class="brief" :title="event.message">{{event.message}}</div>
    <div class="meta"><span>{{allAcknowledged?'已知晓 · 等待恢复':acknowledgedCount?`部分已知晓（${acknowledgedCount}/${notification.events.length}）`:notification.events.length>1?`${notification.events.length} 项触发 · 待确认`:'待确认'}}</span><time>{{new Date(event.occurredAt).toLocaleString()}}</time></div>
    <div class="card-actions" @pointerdown.stop @pointerup.stop><details><summary>详细信息</summary><p class="source">{{event.deviceKey}} / {{event.metric}} · 质量 {{event.quality||'—'}}</p><p v-for="item in notification.events" :key="item.eventId"><b v-if="item.limitName">{{item.limitName}}：</b>{{item.message}}</p></details><button v-if="canAcknowledge" class="card-acknowledge" type="button" :disabled="acknowledging||allAcknowledged" :title="notification.events.length>1?'确认此卡片内当前全部告警项；不恢复仍在发生的告警':'仅确认知晓，不恢复仍在发生的告警'" @click.stop="acknowledge">{{acknowledging?'确认中…':allAcknowledged?'已知晓':'确认知晓'}}</button></div>
    <p v-if="acknowledgeError" class="card-action-error" role="alert">确认失败：{{acknowledgeError}}</p>
  </article>
  </div>
</template>

<style scoped>
.card-sound-state{align-self:center;margin-right:auto;font-size:11px;color:#526780}.card-shortcuts button:disabled{opacity:.5;cursor:default}
.card-actions{position:relative;z-index:3;display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-top:12px;pointer-events:auto}.card-actions details{flex:1;min-width:0}.card-acknowledge{position:relative;z-index:1;flex-shrink:0;border:1px solid #1677ff;border-radius:6px;padding:7px 12px;background:#1677ff;color:#fff;cursor:pointer;font-size:12px;touch-action:manipulation;user-select:none}.card-acknowledge:hover:not(:disabled){background:#0958d9}.card-acknowledge:disabled{opacity:.6;cursor:default}.card-action-error{margin:8px 0 0;color:#b4232c;font-size:12px;line-height:1.6;overflow-wrap:anywhere}
.card-shortcuts{display:flex;justify-content:flex-end;gap:8px;margin-top:10px}.card-shortcuts button{display:grid;place-items:center;width:30px;height:30px;padding:0;border:1px solid #dbe5f2;border-radius:6px;background:#fff;color:#64748b;cursor:pointer}.card-shortcuts button:hover{border-color:#93bdf1;background:#f4f8ff}.card-shortcuts button.selected{color:#1765c1;background:#edf4ff;border-color:#b9d3f7}@media(pointer:coarse){.card-shortcuts button{width:40px;height:40px}}
/* Pulse the border only: keep the background, text and geometry stable. */
.alarm-panel-card.critical:not(.acknowledged){border-width:2px 2px 2px 3px;animation:alarm-critical-pulse 2s steps(1,end) infinite}
@keyframes alarm-critical-pulse{0%,100%{border-color:var(--alarm-critical-pale,#f1b4b8);border-left-color:var(--alarm-critical-pale,#f1b4b8)}50%{border-color:var(--alarm-critical-color,#e5484d);border-left-color:var(--alarm-critical-color,#e5484d)}}
@media(prefers-reduced-motion:reduce){.alarm-panel-card.critical{animation:none;border-color:var(--alarm-critical-color,#e5484d)}}
.alarm-card-slot{min-width:0}.alarm-card-drag{display:flex;align-items:center;gap:8px;flex:1;min-width:0;cursor:grab;touch-action:none;user-select:none;border-radius:4px}.alarm-card-drag:focus-visible{outline:2px solid #1677ff;outline-offset:3px}.drag-grip{font-size:18px;color:#94a3b8;line-height:20px}.alarm-panel-card.floating{box-shadow:0 8px 26px #1e365022}.alarm-panel-card.dragging{box-shadow:0 14px 32px #1e365033}.dragging .alarm-card-drag{cursor:grabbing}.alarm-card-drag .level{flex-shrink:0}
.alarm-panel-card>header{position:static;height:auto;min-height:24px;padding:0;border:0;background:transparent;backdrop-filter:none;box-shadow:none;z-index:auto}
.alarm-panel-card{background:#fff;border:1px solid #e3e9f2;border-left:3px solid #d49a23;border-radius:8px;padding:10px 12px;color:#203047;flex-shrink:0}.alarm-panel-card.critical{border-left-color:#e5484d}.alarm-panel-card.info{border-left-color:#3383d5}header{display:flex;align-items:center;gap:8px}header b{flex:1;min-width:0;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}header button{border:0;background:transparent;color:#64748b;font-size:20px;width:24px;height:24px;padding:0;cursor:pointer}.level{font-size:11px;border-radius:4px;padding:2px 6px;color:#91620d;background:#fff4d8}.critical .level,.limits .critical{color:#b4232c;background:#fff0f1}.info .level,.limits .info{color:#1764a3;background:#eff7ff}.point{display:flex;align-items:baseline;gap:12px;justify-content:space-between;margin:8px 0}.point>span{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;color:#53647a}.point strong{font-size:19px;font-variant-numeric:tabular-nums;overflow-wrap:anywhere;max-width:65%}.point small{font-size:11px;font-weight:500}.limits{display:flex;gap:6px;flex-wrap:wrap}.limits span{font-size:11px;padding:3px 7px;border-radius:4px;background:#fff6e6;color:#8a6019;overflow-wrap:anywhere}.brief{font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;color:#53647a}.meta{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap;font-size:10px;color:#65758b;margin-top:8px}details{margin-top:6px;font-size:11px;color:#64748b}summary{cursor:pointer;width:fit-content}details p{margin:6px 0 0;line-height:1.6;overflow-wrap:anywhere}.source{color:#42526a}button:focus-visible,summary:focus-visible{outline:2px solid #1677ff;outline-offset:2px}
</style>

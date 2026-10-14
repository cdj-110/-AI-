import assert from 'node:assert/strict'
import {createRenderer,nextTick,ref} from 'vue'
import {useAlarmPresentation} from '../src/alarmPresentation.ts'
import {alarmNotificationKey} from '../src/alarmNotifications.ts'
const map=new Map(),listeners=new Set()
globalThis.localStorage={getItem:k=>map.get(k)||null,setItem:(k,v)=>map.set(k,v)}
globalThis.window={addEventListener:(n,f)=>{if(n==='storage')listeners.add(f)},removeEventListener:(n,f)=>listeners.delete(f)}
globalThis.document={documentElement:{style:{setProperty(){}}}}
const renderer=createRenderer({createElement:()=>({}),createText:()=>({}),createComment:()=>({}),insert(){},remove(){},setText(){},setElementText(){},parentNode:()=>null,nextSibling:()=>null,patchProp(){}})
function mount(){let audio;const user=ref('mute-test');const app=renderer.createApp({setup(){audio=useAlarmPresentation(()=>user.value);return ()=>null}});app.mount({});return {app,audio,user}}
const a=mount(),b=mount(),event=(id,rule=id)=>({eventId:id,ruleId:rule,deviceKey:'d1',metric:rule,level:'CRITICAL',state:'ACTIVE',acknowledged:false})
const first=event('one'),second=event('two'),fresh=event('new')
const broadcast=()=>{const key='gateway-alarm-presentation-v1:mute-test';for(const fn of listeners)fn({key,newValue:map.get(key)})}
try{
 a.audio.update([first,second],[]);b.audio.update([first,second],[])
 assert.equal(a.audio.hasActive.value,true);assert.equal(a.audio.allMuted.value,false)
 a.audio.toggleMuteAll();broadcast()
 assert.equal(a.audio.allMuted.value,true);assert.equal(b.audio.allMuted.value,true)
 assert.equal(b.audio.audible.value.length,0,'other tab becomes silent')
 assert.equal(a.audio.settings.value.enabled,true,'mute all never changes master switch')
 b.audio.toggleMute(alarmNotificationKey(first));broadcast()
 assert.equal(a.audio.allMuted.value,false);assert.equal(a.audio.audible.value.length,1,'single card can unmute')
 a.audio.toggleMuteAll();broadcast();a.audio.update([first,second,fresh],[fresh])
 assert.deepEqual(a.audio.audible.value.map(e=>e.eventId),['new'],'new events still notify')
 a.audio.toggleMuteAll();assert.equal(a.audio.allMuted.value,true)
 a.audio.toggleMuteAll();assert.equal(a.audio.audible.value.length,3,'second click unmutes current events')
 a.audio.toggleMuteAll();a.audio.update([],[])
 assert.equal(a.audio.allMuted.value,false);assert.equal(a.audio.hasActive.value,false)
 const ack={...event('ack','limits'),acknowledged:true,retainAfterAcknowledged:true}
 const high={...event('high','limits'),parentRuleId:'limits'},highHigh={...event('highHigh','limits'),parentRuleId:'limits'}
 a.audio.update([ack,high,highHigh,second],[])
 a.audio.toggleMuteEvents([ack,high,highHigh]);broadcast()
 assert.deepEqual(a.audio.audible.value.map(e=>e.eventId),['two'],'group mute affects all unacknowledged members, not another card')
 assert.equal(a.audio.isMutedEvents([ack,high,highHigh]),true)
 a.audio.toggleMuteEvents([ack]);assert.deepEqual(a.audio.audible.value.map(e=>e.eventId),['two'],'acknowledged card cannot change sound')
 a.audio.toggleMuteEvents([ack,high,highHigh])
 assert.equal(a.audio.audible.value.length,3,'group unmute restores only unacknowledged members')
 a.user.value='other-user';await nextTick();assert.equal(a.audio.hasActive.value,false)
 console.log('PASS: mute all cross-tab sync, single-card unmute, new alarms audible, unmute all, recovery cleanup, account isolation')
}finally{a.app.unmount();b.app.unmount()}

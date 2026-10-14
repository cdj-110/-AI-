import assert from 'node:assert/strict'
import {existsSync,readFileSync} from 'node:fs'
import {createRenderer,ref,nextTick} from 'vue'
import {resolveAlarmPanelMode} from '../src/alarmPanelMode.ts'
import {useAlarmPanelWindow} from '../src/composables/useAlarmPanelWindow.ts'
import {updateAlarmNotifications} from '../src/alarmNotifications.ts'
for(const value of [null,undefined,'','tab','invalid'])assert.equal(resolveAlarmPanelMode(value),'tab')
assert.equal(resolveAlarmPanelMode('page'),'page','preserve explicit in-page preference')
const map=new Map(),handlers=new Map();let opens=0,focuses=0,blocked=false
globalThis.localStorage={getItem:k=>map.get(k)||null,setItem:(k,v)=>map.set(k,v),removeItem:k=>map.delete(k)}
globalThis.window={location:{href:'http://gateway/?page=alarms'},addEventListener:(n,f)=>handlers.set(n,f),removeEventListener:n=>handlers.delete(n),setInterval:()=>1,clearInterval(){},open(){opens++;return blocked?null:{closed:false,location:{href:'about:blank'},focus(){focuses++}}}}
const renderer=createRenderer({createElement:()=>({}),createText:()=>({}),createComment:()=>({}),insert(){},remove(){},setText(){},setElementText(){},parentNode:()=>null,nextSibling:()=>null,patchProp(){}})
const user=ref('panel-new');let target
const app=renderer.createApp({setup(){target=useAlarmPanelWindow(()=>user.value);return ()=>null}});app.mount({})
try{
 assert.equal(target.mode.value,'tab')
 assert.equal(target.open(true),true);assert.equal(opens,1)
 target.open(true);assert.equal(opens,1);assert.equal(focuses,0,'polling never repeatedly focuses a tab')
 target.setMode('page');assert.equal(map.get('gateway-alarm-panel-mode-v1:panel-new'),'page')
 user.value='old-user';map.set('gateway-alarm-panel-mode-v1:old-user','page');await nextTick();assert.equal(target.mode.value,'page')
 handlers.get('storage')({key:'gateway-alarm-panel-mode-v1:old-user',newValue:null});assert.equal(target.mode.value,'tab')
}finally{app.unmount()}
const app2=renderer.createApp({setup(){target=useAlarmPanelWindow(()=> 'blocked-user');return ()=>null}});blocked=true;app2.mount({})
try{
 assert.equal(target.open(true),false);const attempts=opens
 target.open(true);assert.equal(opens,attempts,'blocked popup does not retry on every alarm')
 assert.match(target.notice.value,/打开告警标签页/)
}finally{app2.unmount()}
const event={eventId:'e1',ruleId:'r1',deviceKey:'d1',metric:'p1',state:'ACTIVE',acknowledged:false,level:'CRITICAL',popupEnabled:false}
assert.equal(updateAlarmNotifications([], [event],[event]).length,0,'disabled panel reminder stays out of the panel')
assert.equal(updateAlarmNotifications([], [{...event,popupEnabled:true}],[{...event,popupEnabled:true}]).length,1)
assert.equal(existsSync(new URL('../src/components/AlarmNotificationDialog.vue',import.meta.url)),false)
const view=readFileSync(new URL('../src/components/AlarmView.vue',import.meta.url),'utf8')
assert.ok(!view.includes('弹窗'));assert.ok(view.includes('面板提醒'));assert.ok(!view.includes('卡片显示时长（秒）'))
assert.ok(!view.includes('面板设置'),'obsolete panel settings tab removed')
assert.ok(!readFileSync(new URL('../src/styles.css',import.meta.url),'utf8').includes('alarm-toast'))
console.log('PASS: tab default, saved page preference, cross-tab preference, no repeated focus/open, blocked fallback, reminder filter, legacy component/CSS removed, obsolete settings removed')

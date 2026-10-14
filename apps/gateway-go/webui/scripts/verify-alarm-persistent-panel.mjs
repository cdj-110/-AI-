import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {updatePersistentAlarmNotifications,detectNewActiveAlarms} from '../src/alarmNotifications.ts'
const event=(id,extra={})=>({eventId:id,ruleId:id,ruleName:id,deviceKey:'d',metric:'p',state:'ACTIVE',acknowledged:false,level:'WARNING',occurredAt:'2026-09-12T00:00:00Z',...extra})
const a=event('a'),b=event('b'),high=event('high',{parentRuleId:'four',limitKey:'high'}),hh=event('hh',{parentRuleId:'four',limitKey:'highHigh',level:'CRITICAL'})
let cards=updatePersistentAlarmNotifications([], [a,b,high,hh])
assert.equal(cards.length,3)
const ids=cards.map(c=>c.groupId),revisions=cards.map(c=>c.revision)
for(let i=0;i<100;i++)cards=updatePersistentAlarmNotifications(cards,[hh,high,b,{...a,value:i}])
assert.deepEqual(cards.map(c=>c.groupId),ids,'poll order and live values cannot move cards')
assert.deepEqual(cards.map(c=>c.revision),revisions)
assert.equal(cards.find(c=>c.event.ruleId==='a').event.value,99)
assert.equal(updatePersistentAlarmNotifications([], [a,b,high,hh]).length,3,'reopening rebuilds all pending cards')
cards=updatePersistentAlarmNotifications(cards,[{...a,acknowledged:true},{...b,state:'RESOLVED'},high,{...hh,state:'MANUAL_RESOLVED'},event('off',{popupEnabled:false})])
assert.equal(cards.length,1);assert.equal(cards[0].events.length,1);assert.equal(cards[0].event.eventId,'high')
assert.deepEqual(updatePersistentAlarmNotifications(cards,[]),[])
const snapshot={active:[a],popupDurationSeconds:10}
let known=new Set(['a']),open=false
for(let i=0;i<100;i++){
 const detected=detectNewActiveAlarms(known,snapshot,true);known=detected.eventIds
 if(detected.events.length)open=true
 assert.equal(updatePersistentAlarmNotifications([],snapshot.active).length,1)
}
assert.equal(open,false,'closing the panel stays closed on unchanged polls')
assert.equal(detectNewActiveAlarms(known,{active:[a,b]},true).events.length,1)
const source=file=>readFileSync(new URL('../src/'+file,import.meta.url),'utf8')
for(const file of ['components/AlarmNotificationCard.vue','components/AlarmNotificationPanel.vue','components/AlarmWindow.vue','components/AlarmView.vue']){
 const text=source(file)
 assert.ok(!/popupDuration|:duration=|@dismiss=|面板设置|每组提示|setTimeout/.test(text),file)
}
assert.ok(!source('components/AlarmNotificationCard.vue').includes('关闭此条提示'))
assert.ok(source('App.vue').includes("permissionGranted('alarms.view')&&alarmPanelOpen"))
assert.ok(!source('App.vue').includes('alarmPanelOpen||alarmToasts.length'))
console.log('PASS: persistent cards, stable grouping/order, live refresh, acknowledgement/recovery/reminder filtering, reopening, no repeated auto-open, obsolete timer/settings removed')

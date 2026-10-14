import assert from 'node:assert/strict'
import {updateAlarmNotifications,alarmNotificationKey,alarmLimitText,detectNewActiveAlarms} from '../src/alarmNotifications.ts'
const high={eventId:'h',ruleId:'rule::high',parentRuleId:'rule',parentRuleName:'温度',ruleName:'温度 · 上限',deviceKey:'d1',metric:'t',limitKey:'high',limitName:'上限',threshold:100,unit:'℃',level:'WARNING',acknowledged:false,value:110,occurredAt:'2026-09-11T10:00:00Z',message:'温度过高'}
const hh={...high,eventId:'hh',ruleId:'rule::highHigh',ruleName:'温度 · 上上限',limitKey:'highHigh',limitName:'上上限',threshold:120,level:'CRITICAL',occurredAt:'2026-09-11T10:00:01Z'}
const other={...high,eventId:'other',parentRuleId:'another-rule'}
let cards=updateAlarmNotifications([], [high], [high])
assert.equal(cards.length,1);assert.equal(cards[0].event.level,'WARNING')
const key=cards[0].groupId
cards=updateAlarmNotifications(cards,[hh,high],[hh])
assert.equal(cards.length,1);assert.equal(cards[0].groupId,key);assert.equal(cards[0].event.level,'CRITICAL');assert.equal(cards[0].revision,2)
assert.deepEqual(cards[0].events.map(e=>e.limitKey),['highHigh','high'])
assert.equal(alarmLimitText(high),'上限 > 100 ℃')
assert.equal(updateAlarmNotifications([], [hh,high], [hh,high]).length,1,'same-cycle limits share one notification')
for(let i=0;i<20;i++)cards=updateAlarmNotifications(cards,[{...high,value:130},hh],[])
assert.equal(cards[0].revision,2,'polling cannot reset notification timer')
cards=updateAlarmNotifications(cards,[high],[])
assert.equal(cards.length,1);assert.equal(cards[0].event.level,'WARNING');assert.equal(cards[0].revision,2)
assert.equal(updateAlarmNotifications(cards,[],[]).length,0,'all recovered closes card')
assert.equal(updateAlarmNotifications(cards,[{...high,acknowledged:true}],[]).length,0)
assert.equal(updateAlarmNotifications(cards,[{...high,popupEnabled:false}],[]).length,0)
assert.equal(updateAlarmNotifications([], [hh,high], []).length,0,'closed card must not replay on poll')
assert.equal(updateAlarmNotifications([], [hh,high], [hh]).length,1,'new escalation reopens dismissed parent')
assert.equal(updateAlarmNotifications([], [hh,high,other], [hh,high,other]).length,2,'same name but different rule stays separate')
assert.notEqual(alarmNotificationKey(high),alarmNotificationKey({...high,deviceKey:'d2'}))
assert.notEqual(alarmNotificationKey({...high,parentRuleId:undefined,ruleId:'legacy::high'}),alarmNotificationKey({...hh,parentRuleId:undefined,ruleId:'legacy::highHigh'}),'do not infer groups from ID suffixes')
assert.equal(updateAlarmNotifications([], [hh,high],detectNewActiveAlarms(new Set(),{active:[hh,high]},false).events).length,0,'initial page load never replays old alarms')
const lower={...high,eventId:'ll',ruleId:'rule::lowLow',limitKey:'lowLow',limitName:'下下限',threshold:-20,level:'CRITICAL'}
assert.equal(alarmLimitText(lower),'下下限 < -20 ℃')
assert.equal(updateAlarmNotifications(cards,[lower],[lower])[0].groupId,key,'low and high sides share parent card')
const many=Array.from({length:6},(_,i)=>({...high,eventId:`many${i}`,parentRuleId:`rule${i}`}))
assert.equal(updateAlarmNotifications([],many,many).length,6,'panel must keep every eligible rule')
console.log('alarm notification grouping verification passed')

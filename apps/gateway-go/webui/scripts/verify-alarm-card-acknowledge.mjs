import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {acknowledgeCardEvents} from '../src/alarmCardAcknowledge.ts'
import {updatePersistentAlarmNotifications} from '../src/alarmNotifications.ts'
const event=(eventId,extra={})=>({eventId,ruleId:eventId,parentRuleId:'group',ruleName:eventId,deviceKey:'d',metric:'p',state:'ACTIVE',acknowledged:false,level:'CRITICAL',occurredAt:'2026-09-12T00:00:00Z',...extra})
const high=event('high'),hh=event('hh')
const calls=[]
let result=await acknowledgeCardEvents([high,hh,high,event('old',{acknowledged:true}),event('resolved',{state:'RESOLVED'})],async id=>calls.push(id))
assert.deepEqual(calls,['high','hh']);assert.deepEqual(result.acknowledged,calls);assert.equal(result.error,'')
result=await acknowledgeCardEvents([high,hh],async id=>{if(id==='hh')throw new Error('权限不足 (403)')})
assert.deepEqual(result.acknowledged,['high']);assert.match(result.error,/权限不足 \(403\)/)
const cards=updatePersistentAlarmNotifications([], [high,hh].map(e=>({...e,acknowledged:result.acknowledged.includes(e.eventId)})))
assert.equal(cards.length,1);assert.equal(cards[0].events[0].eventId,'hh','failed event remains visible')
const events=[high],sent=[]
await acknowledgeCardEvents(events,async id=>{sent.push(id);events.push(hh)})
assert.deepEqual(sent,['high'],'new triggers during request are not silently acknowledged')
const source=f=>readFileSync(new URL('../src/'+f,import.meta.url),'utf8')
const card=source('components/AlarmNotificationCard.vue')
assert.ok(card.includes('!props.canAcknowledge||acknowledging.value'))
assert.ok(card.includes('v-if="canAcknowledge"'));assert.ok(card.includes(':disabled="acknowledging||allAcknowledged"'))
assert.ok(card.includes('class="card-actions" @pointerdown.stop @pointerup.stop'));assert.ok(card.includes('@click.stop="acknowledge"'))
assert.ok(card.includes('role="alert"'));assert.ok(!card.includes('manual-recover'))
assert.ok(source('App.vue').includes(`:can-acknowledge="permissionGranted('alarms.manage')"`))
assert.ok(source('components/AlarmWindow.vue').includes("['*','alarms.manage','config.manage']"))
console.log('PASS: group acknowledgement, partial failure, original error, no implicit recovery, in-flight new event isolation, duplicate guard, both-page permissions')

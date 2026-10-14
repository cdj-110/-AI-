import assert from 'node:assert/strict'
import {readAlarmCardPosition,saveAlarmCardPosition,clearAlarmCardPositions,removeAlarmCardPosition} from '../src/alarmCardPositions.ts'
const data=new Map()
globalThis.localStorage={getItem:k=>data.get(k)||null,setItem:(k,v)=>data.set(k,v),removeItem:k=>data.delete(k),key:i=>[...data.keys()][i],get length(){return data.size}}
saveAlarmCardPosition('admin','rule/device/point',{x:100,y:250})
assert.deepEqual(readAlarmCardPosition('admin','rule/device/point'),{x:100,y:250})
assert.equal(readAlarmCardPosition('viewer','rule/device/point'),undefined)
saveAlarmCardPosition('viewer','rule/device/point',{x:10,y:20})
saveAlarmCardPosition('admin','inactive-rule',{x:20,y:30})
data.set('unrelated-setting','keep')
clearAlarmCardPositions('admin')
assert.equal(readAlarmCardPosition('admin','inactive-rule'),undefined)
assert.deepEqual(readAlarmCardPosition('viewer','rule/device/point'),{x:10,y:20})
assert.equal(data.get('unrelated-setting'),'keep')
removeAlarmCardPosition('viewer','rule/device/point')
assert.equal(readAlarmCardPosition('viewer','rule/device/point'),undefined)
const key='gateway-alarm-card-position-v1:admin:test'
for(const value of ['broken','null','{"x":-1,"y":2}','{"x":"10","y":2}','{"x":1e999,"y":2}']){data.set(key,value);assert.equal(readAlarmCardPosition('admin','test'),undefined)}
Object.defineProperty(globalThis,'localStorage',{get(){throw Error('blocked')}})
assert.equal(readAlarmCardPosition('admin','test'),undefined)
assert.doesNotThrow(()=>saveAlarmCardPosition('admin','test',{x:1,y:2}))
assert.doesNotThrow(()=>clearAlarmCardPositions('admin'))
console.log('Saved card positions passed: account isolation, reset inactive rules, invalid storage and unavailable storage')

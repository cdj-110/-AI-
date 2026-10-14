import assert from 'node:assert/strict'
import {AlarmAudioOwner} from '../src/alarmAudioOwner.ts'
const map=new Map(),storage={getItem:k=>map.get(k)||null,setItem:(k,v)=>map.set(k,v),removeItem:k=>map.delete(k)}
let now=100000
const main=new AlarmAudioOwner('main',storage,()=>now),tab=new AlarmAudioOwner('tab',storage,()=>now)
main.setScope('admin');tab.setScope('admin')
assert.deepEqual(main.check(),{owns:false,available:false})
main.setReady(true);assert.equal(main.check().owns,true)
assert.deepEqual(tab.check(),{owns:false,available:true},'locked new tab never steals audio')
tab.setReady(true);assert.equal(tab.check().owns,false,'even unlocked new tab retains incumbent')
for(let i=0;i<30;i++){now+=2000;assert.equal(main.check(true).owns,true);assert.equal(tab.check(true).owns,false)}
main.release();assert.equal(tab.check().owns,true,'unlocked tab takes over after source closes')
tab.setReady(false);assert.equal(tab.check().available,false,'suspended context relinquishes ownership')
assert.equal(main.check().owns,true)
now+=45001;assert.equal(tab.check().available,false,'stale owner expires, locked tab does not claim')
tab.setReady(true);assert.equal(tab.check().owns,true)
main.setScope('another-user');main.setReady(true);assert.equal(main.check().owns,true)
assert.equal(tab.check().owns,true,'accounts isolated')
main.release();tab.release()
console.log('PASS: locked/unlocked new tab retains source, single owner heartbeats, close/suspend failover, stale expiry, account isolation')

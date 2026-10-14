import assert from 'node:assert/strict'
import {effectScope,ref,nextTick} from 'vue'
import {useAlarmSoundDraft} from '../src/composables/useAlarmSoundDraft.ts'
import {normalizeAlarmSoundSettings} from '../src/alarmSound.ts'

const savedAudio={id:'audio-1234567890abcdef',name:'saved.wav',duration:2}
const source=ref(normalizeAlarmSoundSettings({levels:{CRITICAL:{customAudio:savedAudio,tone:'custom'}}}))
const owner=ref('test-user'),deleted=[],commits=[]
let rejectSave=false,finishUpload
const scope=effectScope()
const editor=scope.run(()=>useAlarmSoundDraft(()=>source.value,()=>owner.value,value=>{
  if(rejectSave)return false
  source.value=normalizeAlarmSoundSettings(value);commits.push(source.value);return true
},{store:async()=>await new Promise(resolve=>{finishUpload=resolve}),remove:async(user,id)=>{deleted.push([user,id])}}))
const editLevel=(key,value)=>editor.edit({...editor.draft.value,levels:{...editor.draft.value.levels,CRITICAL:{...editor.draft.value.levels.CRITICAL,[key]:value}}})
editLevel('color','#123456')
assert.equal(editor.dirty.value,true)
assert.notEqual(source.value.levels.CRITICAL.color,'#123456','draft must not apply live colors')
assert.equal(commits.length,0)
editor.reset()
assert.equal(editor.pending.value,false)
editLevel('volume',25)
rejectSave=true
assert.equal(editor.save(),false)
assert.equal(editor.dirty.value,true,'failed save retains draft')
assert.equal(source.value.levels.CRITICAL.volume,60)
rejectSave=false
assert.equal(editor.save(),true)
await nextTick()
assert.equal(editor.conflict.value,false,'own save must not create a conflict')
assert.equal(source.value.levels.CRITICAL.volume,25)
assert.equal(editor.dirty.value,false)

let task=editor.uploadAudio('CRITICAL',{})
assert.equal(editor.busy.value,true)
assert.equal(editor.pending.value,true)
assert.equal(editor.save(),false,'save cannot race audio validation')
const staged={id:'audio-fedcba0987654321',name:'draft.wav',duration:3}
finishUpload(staged);await task
assert.equal(source.value.levels.CRITICAL.customAudio.id,savedAudio.id)
assert.equal(editor.draft.value.levels.CRITICAL.customAudio.id,staged.id)
editor.reset();await nextTick()
assert(deleted.some(([,id])=>id===staged.id))
assert(!deleted.some(([,id])=>id===savedAudio.id),'discard must preserve saved audio')

task=editor.uploadAudio('CRITICAL',{})
editor.reset()
finishUpload({...staged,id:'audio-aaaaaaaaaaaaaaaa'});await task
assert(deleted.some(([,id])=>id==='audio-aaaaaaaaaaaaaaaa'),'discard while uploading cleans late completion')
assert.equal(editor.dirty.value,false)

task=editor.uploadAudio('CRITICAL',{})
finishUpload(staged);await task
assert(editor.save());await nextTick()
editor.dispose()
assert.equal(deleted.filter(([,id])=>id===staged.id).length,1,'saved upload must survive unmount')
scope.stop()

const scope2=effectScope()
const next=scope2.run(()=>useAlarmSoundDraft(()=>source.value,()=>owner.value,value=>{source.value=value;return true},{store:async()=>staged,remove:async()=>{}}))
next.edit({...next.draft.value,enabled:false})
source.value=normalizeAlarmSoundSettings({...source.value,volume:10})
assert.equal(next.save(),false,'detect cross-tab change even before Vue watcher flush')
await nextTick()
assert.equal(next.conflict.value,true)
assert.equal(next.draft.value.enabled,false,'external changes cannot silently erase the draft')
next.reset()
assert.equal(next.dirty.value,false)
assert.equal(next.draft.value.volume,10)
next.edit({...next.draft.value,enabled:false})
owner.value='other-user'
assert.equal(next.dirty.value,false,'account change discards prior account draft')
next.dispose();scope2.stop()
console.log('PASS: explicit save, failure, discard, staged uploads, pending upload cancellation, cross-tab conflict, account isolation')

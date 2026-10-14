import assert from 'node:assert/strict'
import {AlarmSoundPlayer,normalizeAlarmSoundSettings,alarmLevelSettings} from '../src/alarmSound.ts'
import {normalizeAlarmAudioFile} from '../src/alarmAudioFiles.ts'
const originalFetch=globalThis.fetch
const originalInterval=globalThis.setInterval,originalClear=globalThis.clearInterval
let intervals=new Set(),starts=[],stops=0
globalThis.setInterval=fn=>{intervals.add(fn);return fn};globalThis.clearInterval=fn=>intervals.delete(fn)
class Context{
 state='running';currentTime=0;destination={}
 resume(){this.state='running';return Promise.resolve()}
 close(){this.state='closed';return Promise.resolve()}
 decodeAudioData(){return Promise.resolve({duration:3})}
 createGain(){return {gain:{value:0,setValueAtTime(){},exponentialRampToValueAtTime(){}},connect(){},disconnect(){}}}
 createOscillator(){let frequency;const node={frequency:{setValueAtTime(v){frequency=v},linearRampToValueAtTime(){}},connect(){},disconnect(){},start(){starts.push({kind:'tone',frequency,type:node.type})},stop(){stops++;node.onended?.()}};return node}
 createBufferSource(){const node={connect(){},disconnect(){},start(){starts.push({kind:'file',loop:node.loop})},stop(){stops++;node.onended?.()}};return node}
}
globalThis.window={AudioContext:Context}
globalThis.fetch=async()=>({ok:true,arrayBuffer:async()=>new ArrayBuffer(8)})
const tick=()=>new Promise(resolve=>setTimeout(resolve,0))
const customAudio={id:'audio-0123456789abcdef0123456789abcdef',name:'alarm.wav',duration:3}
const settings={enabled:true,mode:'loop',tone:'custom',customAudio,volume:60,criticalOnly:false}
let player
try{
 assert.equal(normalizeAlarmSoundSettings({}).tone,'provided')
 assert.equal(normalizeAlarmSoundSettings({levels:{CRITICAL:{tone:'custom'}}}).levels.CRITICAL.tone,'file-alarm2')
 assert.equal(normalizeAlarmAudioFile({...customAudio,id:'https://bad.example/sound.mp3'}),undefined)
 assert.equal(normalizeAlarmAudioFile({...customAudio,duration:31}),undefined)
 const migrated=normalizeAlarmSoundSettings({tone:'bell',criticalOnly:true})
 assert.equal(alarmLevelSettings(migrated,'CRITICAL').tone,'file-alarm2');assert.equal(alarmLevelSettings(migrated,'INFO').enabled,false)
 const configured=normalizeAlarmSoundSettings({levels:{CRITICAL:{...alarmLevelSettings(migrated,'CRITICAL'),tone:'custom',customAudio}}})
 assert.deepEqual(normalizeAlarmSoundSettings(JSON.parse(JSON.stringify(configured))),configured)
 player=new AlarmSoundPlayer(async()=>new ArrayBuffer(8))
 player.sync('CRITICAL',settings,true);await tick()
 assert.deepEqual(starts,[{kind:'file',loop:true}])
 for(const fn of intervals)fn();await tick();assert.equal(starts.length,1,'a long custom loop is not overlapped every two seconds')
 player.sync(undefined,settings);assert.ok(stops>0,'mute immediately stops custom source')
 starts=[];player.sync('CRITICAL',{...settings,mode:'once'},true);await tick()
 assert.deepEqual(starts,[{kind:'file',loop:false}]);player.sync('CRITICAL',{...settings,mode:'once'});await tick();assert.equal(starts.length,1,'poll does not restart a once-only sound')
 player.close();starts=[]
 let release
 player=new AlarmSoundPlayer(()=>new Promise(resolve=>release=resolve))
 player.sync('CRITICAL',settings,true);player.stop();release(new ArrayBuffer(8));await tick()
 assert.equal(starts.length,0,'stop during asynchronous decode prevents late playback');player.close()
 let error='';player=new AlarmSoundPlayer(async()=>{throw Error('missing')},message=>error=message)
 starts=[];player.sync('CRITICAL',settings,true);await tick();assert.match(error,/本等级默认音频/);assert.deepEqual(starts,[{kind:'file',loop:true}]);player.close()
 player=new AlarmSoundPlayer(async()=>new ArrayBuffer(8));starts=[]
 await player.preview(settings);await tick();assert.deepEqual(starts,[{kind:'file',loop:false}]);player.stopPreview();player.close()
 console.log('PASS: legacy tone migration, file validation, full custom loops without overlap, once-only playback, immediate mute, decode cancellation, preview and failure fallback')
}finally{globalThis.fetch=originalFetch;player?.close();globalThis.setInterval=originalInterval;globalThis.clearInterval=originalClear}

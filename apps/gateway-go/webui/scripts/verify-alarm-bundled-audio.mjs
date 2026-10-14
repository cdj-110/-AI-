import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {bundledAlarmAudio,defaultAlarmFile} from '../src/alarmBundledAudio.ts'
import {AlarmSoundPlayer,normalizeAlarmSoundSettings,alarmSoundTones} from '../src/alarmSound.ts'
assert.deepEqual(alarmSoundTones.map(item=>item.key),['file-buzzer4','file-buzzer1','file-alarm2','file-siren1'],'only the four supplied audio choices remain')
for(const removed of ['standard','bell','buzzer','industrial','pulse','two-tone','siren']){
 const migrated=normalizeAlarmSoundSettings({enabled:false,volume:35,mode:'once',tone:removed,levels:{WARNING:{tone:removed,volume:75,color:'#ff9839',enabled:false}}})
 for(const level of ['INFO','WARNING','CRITICAL'])assert.equal(migrated.levels[level].tone,defaultAlarmFile(level))
 assert.equal(migrated.enabled,false);assert.equal(migrated.mode,'once');assert.equal(migrated.volume,35)
 assert.equal(migrated.levels.WARNING.enabled,false);assert.equal(migrated.levels.WARNING.volume,75);assert.equal(migrated.levels.WARNING.color,'#ff9839')
}
const expected={INFO:'Buzzer4.wav',WARNING:'Buzzer1.wav',CRITICAL:'Alarm2.WAV'}
for(const [level,name] of Object.entries(expected)){
 const tone=normalizeAlarmSoundSettings({}).levels[level].tone
 assert.equal(tone,defaultAlarmFile(level));assert.ok(bundledAlarmAudio[tone].url.endsWith(name))
}
assert.equal(normalizeAlarmSoundSettings({tone:'bell'}).levels.CRITICAL.tone,'file-alarm2','removed legacy tones migrate to severity defaults')
const durations=new Map()
for(const file of Object.values(bundledAlarmAudio)){
 const data=readFileSync(new URL(file.url));assert.equal(data.toString('ascii',0,4),'RIFF');assert.equal(data.toString('ascii',8,12),'WAVE')
 let bytes,rate
 for(let p=12;p+8<=data.length;){const size=data.readUInt32LE(p+4),tag=data.toString('ascii',p,p+4);assert.ok(p+8+size<=data.length)
  if(tag==='fmt '){assert.equal(data.readUInt16LE(p+8),1);rate=data.readUInt32LE(p+16)}
  if(tag==='data')bytes=size
  p+=8+size+(size%2)
 }
 const duration=bytes/rate;assert.ok(duration>0&&duration<=30);assert.ok(Math.abs(duration-file.duration)<.001)
 durations.set(data.length,duration)
}
const originalFetch=globalThis.fetch,originalWindow=globalThis.window
let starts=[],nodes=[],requests=[],release
class Context{
 state='running';currentTime=0;destination={}
 resume(){return Promise.resolve()}close(){return Promise.resolve()}
 decodeAudioData(bytes){return Promise.resolve({duration:durations.get(bytes.byteLength)})}
 createGain(){return {gain:{value:0},connect(){},disconnect(){}}}
 createBufferSource(){const source={connect(){},disconnect(){},start(){starts.push({loop:source.loop,duration:source.buffer.duration})},stop(){source.onended?.()}};nodes.push(source);return source}
}
globalThis.window={AudioContext:Context}
globalThis.fetch=async url=>{requests.push(url);const b=readFileSync(new URL(url));return {ok:true,arrayBuffer:async()=>b.buffer.slice(b.byteOffset,b.byteOffset+b.byteLength)}}
const tick=()=>new Promise(resolve=>setTimeout(resolve,0))
let player
try{
 for(const [tone,file] of Object.entries(bundledAlarmAudio)){
  starts=[];player=new AlarmSoundPlayer();const settings={enabled:true,mode:'loop',tone,volume:60,criticalOnly:false}
  player.sync('CRITICAL',settings,true);await tick()
  assert.equal(starts.length,1);assert.equal(starts[0].loop,true);assert.ok(Math.abs(starts[0].duration-file.duration)<.001)
  player.sync('CRITICAL',settings);await tick();assert.equal(starts.length,1)
  player.stop();player.close()
  starts=[];player=new AlarmSoundPlayer();await player.preview(settings);await tick()
  assert.equal(starts[0].loop,false);player.stopPreview();player.close()
 }
 globalThis.fetch=()=>new Promise(resolve=>release=resolve)
 starts=[];player=new AlarmSoundPlayer();player.sync('CRITICAL',{enabled:true,mode:'loop',tone:'file-alarm2',volume:60,criticalOnly:false},true);player.stop()
 release({ok:true,arrayBuffer:async()=>readFileSync(new URL(bundledAlarmAudio['file-alarm2'].url)).buffer});await tick()
 assert.equal(starts.length,0,'late asset loading must not restart a muted alarm')
 console.log('PASS: exactly four choices, removed tone migration with other preferences preserved, WAV format/duration, default mapping, loops/previews, no poll restart, cancellation')
}finally{player?.close();globalThis.fetch=originalFetch;globalThis.window=originalWindow}

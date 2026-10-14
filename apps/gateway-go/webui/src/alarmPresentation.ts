import {computed,onBeforeUnmount,ref,watch,type InjectionKey} from 'vue'
import {alarmNotificationKey,alarmSoundEligible,type AlarmEvent} from './alarmNotifications.ts'
import {AlarmSoundPlayer,highestAlarmLevel,type AlarmSoundSettings,normalizeAlarmSoundSettings,alarmLevelSettings} from './alarmSound.ts'
import {applyAlarmColors} from './alarmColors.ts'
import {loadAlarmAudio} from './alarmAudioFiles.ts'

export const alarmPresentationKey:InjectionKey<ReturnType<typeof useAlarmPresentation>>=Symbol('alarmPresentation')
export function useAlarmPresentation(scope:()=>string,canPlay:()=>boolean=()=>true,reportReady:(ready:boolean)=>void=()=>{},audioAvailable:()=>boolean=canPlay){
  const settings=ref<AlarmSoundSettings>(normalizeAlarmSoundSettings({}))
  const muted=ref<string[]>([]),pinned=ref<string[]>([]),blocked=ref(false),storageError=ref(false)
  const events=ref<AlarmEvent[]>([])
  const playbackError=ref('')
  const player=new AlarmSoundPlayer(id=>loadAlarmAudio(scope(),id),message=>{playbackError.value=message},reportReady)
  let attemptedUnlock=false,disposed=false
  const key=()=>`gateway-alarm-presentation-v1:${encodeURIComponent(scope())}`
  function persist(){try{localStorage.setItem(key(),JSON.stringify({settings:settings.value,pinned:pinned.value,muted:muted.value}));storageError.value=false}catch{storageError.value=true}}
  watch(scope,()=>{
    player.close();events.value=[];attemptedUnlock=false
    playbackError.value=''
    let saved:any={};try{saved=JSON.parse(localStorage.getItem(key())||'{}')||{}}catch{}
    settings.value=normalizeAlarmSoundSettings(saved.settings||{})
    const strings=(value:unknown)=>Array.isArray(value)?value.filter((v):v is string=>typeof v==='string'):[]
    pinned.value=strings(saved.pinned);muted.value=strings(saved.muted)
  },{immediate:true})
  const eligible=computed(()=>events.value.filter(e=>!e.acknowledged&&e.state==='ACTIVE'&&alarmSoundEligible(e)))
  watch(settings,applyAlarmColors,{immediate:true,deep:true})
  const audible=computed(()=>eligible.value.filter(e=>{const level=alarmLevelSettings(settings.value,e.level);return !muted.value.includes(e.eventId)&&level.enabled&&level.volume>0}))
  const allMuted=computed(()=>eligible.value.length>0&&eligible.value.every(e=>muted.value.includes(e.eventId)))
  const hasActive=computed(()=>eligible.value.length>0)
  function sync(force=false){const level=highestAlarmLevel(audible.value.map(e=>e.level)),resolved=alarmLevelSettings(settings.value,level||'INFO');const ok=player.sync(canPlay()?level:undefined,{enabled:settings.value.enabled,mode:resolved.mode,tone:resolved.tone,customAudio:resolved.customAudio,volume:resolved.volume,criticalOnly:false},force);blocked.value=!!level&&settings.value.enabled&&(!audioAvailable()||canPlay()&&!ok)}
  watch(canPlay,value=>sync(value),{flush:'sync'})
  watch(audioAvailable,()=>sync(),{flush:'sync'})
  function receiveStorage(event:StorageEvent){
    if(event.key!==key()||!event.newValue)return
    try{const saved=JSON.parse(event.newValue);settings.value=normalizeAlarmSoundSettings(saved.settings||{});pinned.value=Array.isArray(saved.pinned)?saved.pinned.filter((id:unknown)=>typeof id==='string'):[];muted.value=Array.isArray(saved.muted)?saved.muted.filter((id:unknown)=>typeof id==='string'):[];sync()}catch{}
  }
  window.addEventListener('storage',receiveStorage)
  function update(active:AlarmEvent[],fresh:AlarmEvent[]){
    events.value=active
    const ids=new Set(eligible.value.map(e=>e.eventId))
    const retained=muted.value.filter(id=>ids.has(id))
    if(retained.length!==muted.value.length){muted.value=retained;persist()}
    sync(fresh.some(e=>audible.value.some(a=>a.eventId===e.eventId)))
    // A blocked resume can remain pending until a gesture; never await it in polling.
    if(!attemptedUnlock&&scope()){attemptedUnlock=true;void unlock()}
  }
  function muteCandidates(items:readonly AlarmEvent[]){return items.filter(e=>e.state==='ACTIVE'&&!e.acknowledged)}
  function isMutedEvents(items:readonly AlarmEvent[]){const candidates=muteCandidates(items);return candidates.length>0&&candidates.every(e=>muted.value.includes(e.eventId))}
  // Use the card's concrete event IDs, not a second lookup of its display group.
  // This also keeps a click tied to the visible occurrence during a polling refresh.
  function toggleMuteEvents(items:readonly AlarmEvent[]){
    const ids=muteCandidates(items).map(e=>e.eventId),remove=isMutedEvents(items)
    if(!ids.length)return
    muted.value=remove?muted.value.filter(id=>!ids.includes(id)):[...new Set([...muted.value,...ids])]
    player.stopPreview();persist();sync(true)
  }
  function isMuted(group:string){return isMutedEvents(eligible.value.filter(e=>alarmNotificationKey(e)===group))}
  function toggleMute(group:string){toggleMuteEvents(eligible.value.filter(e=>alarmNotificationKey(e)===group))}
  function togglePin(group:string){pinned.value=pinned.value.includes(group)?pinned.value.filter(id=>id!==group):[...pinned.value,group];persist()}
  function toggleMuteAll(){
    const ids=eligible.value.map(e=>e.eventId)
    muted.value=allMuted.value?muted.value.filter(id=>!ids.includes(id)):[...new Set([...muted.value,...ids])]
    player.stopPreview();persist();sync(true)
  }
  function configure(value:AlarmSoundSettings){
    const next=normalizeAlarmSoundSettings(value)
    try{localStorage.setItem(key(),JSON.stringify({settings:next,pinned:pinned.value,muted:muted.value}));storageError.value=false}catch{storageError.value=true;return false}
    settings.value=next;playbackError.value='';sync();return true
  }
  async function unlock(){const user=scope(),ok=await player.unlock();if(!disposed&&user===scope())sync();return ok}
  async function preview(level='CRITICAL',value:AlarmSoundSettings=settings.value){if(!await unlock()){blocked.value=true;return}blocked.value=!await player.preview({...value,...alarmLevelSettings(value,level)},level)}
  function stopPreview(){player.stopPreview()}
  onBeforeUnmount(()=>{disposed=true;player.close();window.removeEventListener('storage',receiveStorage)})
  return {scope,settings,pinned,blocked,storageError,playbackError,audible,allMuted,hasActive,isMuted,isMutedEvents,toggleMute,toggleMuteEvents,toggleMuteAll,togglePin,configure,update,unlock,preview,stopPreview}
}

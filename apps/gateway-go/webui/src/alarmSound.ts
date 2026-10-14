import {bundledAlarmAudio,alarmBundledFile,defaultAlarmFile,type BundledAlarmTone} from './alarmBundledAudio.ts'
import {normalizeAlarmAudioFile,type AlarmAudioFile} from './alarmAudioFiles.ts'

const levelPriority: Record<string, number> = { INFO: 1, WARNING: 2, CRITICAL: 3 }

export function highestAlarmLevel(levels: string[]): string | undefined {
  return levels.reduce<string | undefined>((highest, level) => {
    const normalized = String(level || 'INFO').toUpperCase()
    return !highest || (levelPriority[normalized] || 0) > (levelPriority[highest] || 0) ? normalized : highest
  }, undefined)
}


export type AlarmSoundTone=BundledAlarmTone|'provided'|'custom'
export const alarmSoundTones=Object.entries(bundledAlarmAudio).map(([key,file])=>({key:key as BundledAlarmTone,name:file.name}))
const validTone=(value:unknown):value is AlarmSoundTone=>value==='provided'||value==='custom'||alarmSoundTones.some(item=>item.key===value)
export interface AlarmSoundSettings {
  enabled:boolean
  mode:'once'|'loop'
  tone:AlarmSoundTone
  customAudio?:AlarmAudioFile
  volume:number
  criticalOnly:boolean
  levels?:Record<string,AlarmLevelSettings>
}
export interface AlarmLevelSettings {enabled:boolean;mode:'once'|'loop';tone:AlarmSoundTone;volume:number;color:string;customAudio?:AlarmAudioFile}
export const alarmLevels=[{key:'INFO',name:'提示',color:'#3383d5'},{key:'WARNING',name:'警告',color:'#d49a23'},{key:'CRITICAL',name:'严重',color:'#e5484d'}] as const
export function alarmLevelSettings(settings:AlarmSoundSettings,level:string):AlarmLevelSettings{
 const definition=alarmLevels.find(item=>item.key===level)||alarmLevels[0],value=settings.levels?.[definition.key]
 return {enabled:value?.enabled??(!settings.criticalOnly||level==='CRITICAL'),mode:value?.mode??settings.mode,tone:value?.tone??(settings.tone==='provided'?defaultAlarmFile(level):settings.tone),volume:value?.volume??settings.volume,color:value?.color??definition.color,customAudio:value?.customAudio??settings.customAudio}
}
export function normalizeAlarmSoundSettings(value:Partial<AlarmSoundSettings>):AlarmSoundSettings{
  const base:AlarmSoundSettings={enabled:value.enabled!==false,mode:value.mode==='once'?'once':'loop',tone:validTone(value.tone)?value.tone:'provided',volume:typeof value.volume==='number'&&Number.isFinite(value.volume)?Math.max(0,Math.min(100,value.volume)):60,criticalOnly:value.criticalOnly===true}
  base.levels={}
  for(const level of alarmLevels){
    const raw=value.levels?.[level.key],fallback=alarmLevelSettings({...base,levels:undefined},level.key)
    const customAudio=normalizeAlarmAudioFile(raw?.customAudio)
    const selected=raw?.tone===undefined?fallback.tone:validTone(raw.tone)?raw.tone:defaultAlarmFile(level.key)
    const tone=selected==='provided'?defaultAlarmFile(level.key):selected
    base.levels[level.key]={enabled:typeof raw?.enabled==='boolean'?raw.enabled:fallback.enabled,mode:raw?.mode==='once'||raw?.mode==='loop'?raw.mode:fallback.mode,tone:tone==='custom'&&!customAudio?defaultAlarmFile(level.key):tone,customAudio,volume:typeof raw?.volume==='number'&&Number.isFinite(raw.volume)?Math.max(0,Math.min(100,raw.volume)):fallback.volume,color:typeof raw?.color==='string'&&/^#[0-9a-f]{6}$/i.test(raw.color)?raw.color.toLowerCase():level.color}
  }
  return base
}
export class AlarmSoundPlayer {
  private context?: AudioContext
  private level?:string
  private settings=normalizeAlarmSoundSettings({})
  private signature=''
  private timer?:ReturnType<typeof setInterval>
  private previewTimer?:ReturnType<typeof setTimeout>
  private nodes=new Set<OscillatorNode|AudioBufferSourceNode>()
  private generation=0
  private customBusy=false
  private buffers=new Map<string,AudioBuffer>()
  private loadAudio?:(id:string)=>Promise<ArrayBuffer>
  private reportError:(message:string)=>void
  private reportReady:(ready:boolean)=>void
  constructor(loadAudio?:(id:string)=>Promise<ArrayBuffer>,reportError:(message:string)=>void=()=>{},reportReady:(ready:boolean)=>void=()=>{}){this.loadAudio=loadAudio;this.reportError=reportError;this.reportReady=reportReady}
  private pending=false
  sync(level:string|undefined,settings:AlarmSoundSettings,force=false):boolean{
    const next=settings.enabled&&settings.volume>0?level:undefined
    const signature=JSON.stringify([next,settings])
    const changed=signature!==this.signature
    this.signature=signature;this.level=next;this.settings={...settings}
    if(!next){this.stop();return true}
    if(changed){this.stopPreview();this.cancelNodes();this.pending=false}
    if(force||(changed&&settings.mode==='loop')){this.stopPreview();this.cancelNodes();this.pending=true;this.playCurrent()}
    if(changed&&this.timer){clearInterval(this.timer);this.timer=undefined}
    if(settings.mode==='loop'&&!this.timer)this.timer=setInterval(()=>this.playCurrent(),2000)
    return this.context?.state==='running'||(!this.pending&&settings.mode==='once')
  }
  private playCurrent(){
    if(!this.level||this.previewTimer||this.customBusy)return
    const context=this.ensureContext()
    if(context?.state==='running'){this.pending=false;this.emit(context,this.level,this.settings)}else this.pending=true
  }
  private cancelNodes(){this.generation++;this.customBusy=false;for(const node of this.nodes){try{node.stop()}catch{}}this.nodes.clear()}
  stopPreview(){if(this.previewTimer){clearTimeout(this.previewTimer);this.previewTimer=undefined;this.cancelNodes()}}
  stop(){if(this.timer)clearInterval(this.timer);this.timer=undefined;this.level=undefined;this.pending=false;this.stopPreview();this.cancelNodes()}
  async preview(settings:AlarmSoundSettings,level='CRITICAL'):Promise<boolean>{
    this.stopPreview();this.cancelNodes()
    // A preview is explicitly requested and independent of the master enable switch.
    this.previewTimer=setTimeout(()=>{this.previewTimer=undefined;this.cancelNodes();if(this.settings.mode==='loop')this.playCurrent()},settings.tone==='custom'?(settings.customAudio?.duration??1)*1000+1000:alarmBundledFile(settings.tone,level)?alarmBundledFile(settings.tone,level)!.duration*1000+2000:1900)
    const ok=await this.unlock()
    if(ok&&this.context&&this.previewTimer)this.emit(this.context,level,{...settings,mode:'once'})
    return ok
  }

  async unlock(): Promise<boolean> {
    const context = this.ensureContext()
    if (!context) return false
    try {
      if (context.state === 'suspended') await context.resume()
    } catch {
      return false
    }
    if (context!==this.context||context.state !== 'running') return false
    this.reportReady(true)
    if(this.pending)this.playCurrent()
    return true
  }

  close(): void {
    this.stop()
    this.buffers.clear()
    const context = this.context
    this.context = undefined
    this.reportReady(false)
    if (context && context.state !== 'closed') void context.close().catch(() => undefined)
  }

  private ensureContext(): AudioContext | undefined {
    if (this.context) return this.context
    if (typeof window === 'undefined') return undefined
    const AudioContextConstructor = window.AudioContext
      || (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!AudioContextConstructor) return undefined
    try {
      this.context = new AudioContextConstructor()
      const context=this.context
      context.onstatechange=()=>{if(this.context===context)this.reportReady(context.state==='running')}
      this.reportReady(context.state==='running')
    } catch {
      return undefined
    }
    return this.context
  }

  private emit(context:AudioContext,level:string,settings:AlarmSoundSettings):void {
    const resolved=(settings.tone==='custom'&&settings.customAudio)||alarmBundledFile(settings.tone,level)?settings:{...settings,tone:defaultAlarmFile(level)}
    void this.emitCustom(context,level,resolved)
  }
  private async emitCustom(context:AudioContext,level:string,settings:AlarmSoundSettings,fallbackNotice=''){
    const file=alarmBundledFile(settings.tone,level)
    const ticket=this.generation,id=file?.url||settings.customAudio!.id
    this.customBusy=true
    try{
      let buffer=this.buffers.get(id)
      if(!buffer){
        let bytes:ArrayBuffer
        if(file){
          const response=await fetch(file.url)
          if(!response.ok)throw new Error('内置音频不可用')
          bytes=await response.arrayBuffer()
        }else{
          if(!this.loadAudio)throw new Error('自定义音频不可用')
          bytes=await this.loadAudio(id)
        }
        buffer=await context.decodeAudioData(bytes)
        if(buffer.duration<=0||buffer.duration>30)throw new Error('音频时长不符合要求')
        if(ticket!==this.generation)return
        if(this.buffers.size>=8)this.buffers.clear()
        this.buffers.set(id,buffer)
      }
      if(ticket!==this.generation)return
      const source=context.createBufferSource(),gain=context.createGain()
      source.buffer=buffer;source.loop=settings.mode==='loop';gain.gain.value=settings.volume/100*.5
      source.connect(gain);gain.connect(context.destination);this.nodes.add(source)
      source.onended=()=>{this.nodes.delete(source);source.disconnect();gain.disconnect();if(ticket===this.generation)this.customBusy=false}
      source.start();this.reportError(fallbackNotice)
    }catch{
      if(ticket!==this.generation)return
      this.customBusy=false
      if(file){this.reportError('内置告警音频无法播放，请刷新页面重试；告警记录和画面提示不受影响。')}
      else{void this.emitCustom(context,level,{...settings,tone:defaultAlarmFile(level)},'自定义音频无法播放，已改用本等级默认音频；请重新上传音频。')}
    }
  }
}

import {computed,onBeforeUnmount,ref,watch,type InjectionKey} from 'vue'
import {AlarmAudioOwner} from '../alarmAudioOwner.ts'
import {resolveAlarmPanelMode,type AlarmPanelMode} from '../alarmPanelMode.ts'

export type {AlarmPanelMode} from '../alarmPanelMode.ts'
type WindowState={id:string;at:number;closed:boolean}
export const alarmPanelWindowKey:InjectionKey<ReturnType<typeof useAlarmPanelWindow>>=Symbol('alarmPanelWindow')
export function useAlarmPanelWindow(scope:()=>string,standalone=false){
  const mode=ref<AlarmPanelMode>('tab'),notice=ref(''),online=ref(false),ownsAudio=ref(false),audioAvailable=ref(false)
  const id=`${Date.now()}-${Math.random().toString(36).slice(2)}`
  const audioOwner=new AlarmAudioOwner(id,{getItem:k=>localStorage.getItem(k),setItem:(k,v)=>localStorage.setItem(k,v),removeItem:k=>localStorage.removeItem(k)})
  function checkAudio(renew=false){const state=audioOwner.check(renew);ownsAudio.value=state.owns;audioAvailable.value=state.available}
  function setAudioReady(ready:boolean){audioOwner.setReady(ready);checkAudio(true)}
  let child:Window|null=null,attempted=false,timer=0,registeredScope=''
  const modeKey=()=>`gateway-alarm-panel-mode-v1:${encodeURIComponent(scope())}`
  const stateKey=(user=scope())=>`gateway-alarm-panel-window-v1:${encodeURIComponent(user)}`
  function state():WindowState|undefined{try{const v=JSON.parse(localStorage.getItem(stateKey())||'null');return v&&typeof v.id==='string'&&typeof v.at==='number'&&typeof v.closed==='boolean'?v:undefined}catch{return}}
  function write(value:WindowState,user=scope()){try{localStorage.setItem(stateKey(user),JSON.stringify(value))}catch{}}
  function live(value:WindowState|undefined){return !!value&&!value.closed&&Date.now()-value.at<45000}
  function check(){
    if(!scope())return
    let value=state()
    try{if(child&&!child.closed&&child.location.href!=='about:blank'&&new URL(child.location.href).searchParams.get('alarmPanel')!=='1'){child=null;attempted=true;write({id,at:Date.now(),closed:true});value=state()}}catch{child=null}
    if(standalone){
      if(!live(value)||value?.id===id){write({id,at:Date.now(),closed:false});registeredScope=scope();value=state()}
    }else{
      if(child?.closed){child=null;attempted=true;write({id,at:Date.now(),closed:true});value=state()}
      online.value=!!child&&!child.closed||live(value)
      if(online.value)notice.value=''
    }
  }
  function unregister(){audioOwner.release();if(registeredScope){try{const v=JSON.parse(localStorage.getItem(stateKey(registeredScope))||'null');if(v?.id===id)write({id,at:Date.now(),closed:true},registeredScope)}catch{}registeredScope=''}ownsAudio.value=false}
  function setMode(value:AlarmPanelMode){
    mode.value=value;notice.value='';attempted=false
    try{localStorage.setItem(modeKey(),value);if(value==='tab'&&state()?.closed)localStorage.removeItem(stateKey())}catch{notice.value='浏览器不允许保存设置，本次选择有效，刷新后可能丢失。'}
  }
  function open(automatic=false){
    check()
    if(standalone)return true
    if(automatic&&(online.value||attempted||state()?.closed))return online.value
    if(child&&!child.closed){if(!automatic)child.focus();return true}
    // Never navigate or focus an existing alarm tab from polling.
    if(automatic&&online.value)return true
    attempted=true
    const url=new URL(window.location.href);url.search='?alarmPanel=1';url.hash=''
    try{
      child=window.open('',`gateway-alarm-panel-${encodeURIComponent(scope())}`)
      if(child){
        let alreadyPanel=false
        try{const existing=new URL(child.location.href);alreadyPanel=existing.origin===url.origin&&existing.searchParams.get('alarmPanel')==='1'}catch{}
        if(!alreadyPanel)child.location.href=url.href
      }
    }catch{child=null}
    if(!child){notice.value='新标签页被浏览器拦截，请点击“打开告警标签页”；本页不会弹出遮挡操作。';return false}
    online.value=true;notice.value='';return true
  }
  function storage(event:StorageEvent){if(event.key===modeKey())mode.value=resolveAlarmPanelMode(event.newValue);if(event.key===stateKey())check();if(event.key===audioOwner.key())checkAudio()}
  watch(scope,()=>{unregister();audioOwner.setScope(scope());checkAudio();mode.value='tab';try{mode.value=resolveAlarmPanelMode(localStorage.getItem(modeKey()))}catch{}attempted=false;check()},{immediate:true})
  window.addEventListener('storage',storage);window.addEventListener('pagehide',unregister)
  timer=window.setInterval(()=>{check();checkAudio(true)},2000)
  onBeforeUnmount(()=>{unregister();window.clearInterval(timer);window.removeEventListener('storage',storage);window.removeEventListener('pagehide',unregister)})
  return {mode,notice,online,ownsAudio,audioAvailable,setAudioReady,setMode,open,standalone:computed(()=>standalone)}
}

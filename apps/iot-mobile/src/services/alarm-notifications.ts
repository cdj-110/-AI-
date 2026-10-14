import type { Alarm } from '../types'
import { serverURL, token } from './session'

function notifyNative(alarm:Alarm){
  if(typeof window==='undefined')return
  try{
    const title=`${alarm.level==='CRITICAL'?'严重':'设备'}告警 · ${alarm.deviceName||alarm.device?.name||'设备'}`
    window.WeikongNative?.showAlarmNotification?.(alarm.id,title,alarm.message)
  }catch{}
}

export function requestSystemNotificationPermission(){
  if(typeof window==='undefined')return
  try{window.WeikongNative?.requestNotificationPermission?.()}catch{}
}

export function subscribeAlarmNotifications(onAlarm?:(alarm:Alarm)=>void,options:{notifySystem?:boolean}={}){
  let stopped=false,controller:AbortController|undefined,retry:ReturnType<typeof setTimeout>|undefined
  async function connect(){
    if(stopped||!token())return
    controller?.abort();controller=new AbortController()
    try{
      const response=await fetch(`${serverURL()}/api/alarms/stream`,{headers:{Authorization:`Bearer ${token()}`},signal:controller.signal})
      if(!response.ok||!response.body)throw new Error('告警通道不可用')
      const reader=response.body.getReader(),decoder=new TextDecoder();let buffer=''
      while(!stopped){
        const {done,value}=await reader.read();if(done)break
        buffer+=decoder.decode(value,{stream:true});const messages=buffer.split('\n\n');buffer=messages.pop()||''
        for(const message of messages){
          const line=message.split('\n').find(item=>item.startsWith('data:'));if(!line)continue
          try{
            const envelope=JSON.parse(line.slice(5).trim());const alarm=(envelope.data||envelope) as Alarm
            onAlarm?.(alarm)
            if(alarm.status==='OPEN'&&options.notifySystem!==false)notifyNative(alarm)
          }catch{}
        }
      }
    }catch{if(controller?.signal.aborted)return}
    if(!stopped)retry=setTimeout(connect,3000)
  }
  void connect()
  return()=>{stopped=true;if(retry)clearTimeout(retry);controller?.abort()}
}

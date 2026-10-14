import { serverURL, token } from './session'

export type RealtimeState='CONNECTING'|'LIVE'|'FALLBACK'

export interface TelemetryUpdate{
  deviceId:string
  payload:{deviceKey?:string;time:string;metrics:Record<string,unknown>}
}

interface RealtimeHandlers{
  onState:(state:RealtimeState)=>void
  onTelemetry:(update:TelemetryUpdate)=>void
  onError?:(message:string)=>void
}

function realtimeURL(){
  return `${serverURL().replace(/^http:/i,'ws:').replace(/^https:/i,'wss:')}/api/realtime`
}

export function subscribeTelemetry(deviceId:string,handlers:RealtimeHandlers){
  let socket:UniApp.SocketTask|undefined
  let reconnectTimer:ReturnType<typeof setTimeout>|undefined
  let reconnectDelay=1500
  let stopped=false

  function send(payload:Record<string,unknown>){
    socket?.send({data:JSON.stringify(payload)})
  }
  function scheduleReconnect(){
    if(stopped||reconnectTimer!==undefined)return
    handlers.onState('FALLBACK')
    reconnectTimer=setTimeout(()=>{
      reconnectTimer=undefined
      connect()
    },reconnectDelay)
    reconnectDelay=Math.min(reconnectDelay*2,10000)
  }
  function connect(){
    if(stopped)return
    handlers.onState('CONNECTING')
    const task=uni.connectSocket({url:realtimeURL(),complete:()=>undefined})
    socket=task
    task.onOpen(()=>send({type:'auth',token:token()}))
    task.onMessage(({data})=>{
      let message:{type?:string;deviceId?:string;payload?:TelemetryUpdate['payload'];message?:string}
      try{message=JSON.parse(String(data))}catch{return}
      if(message.type==='auth.ok'){
        send({type:'telemetry.subscribe',deviceId})
        return
      }
      if(message.type==='telemetry.subscribed'&&message.deviceId===deviceId){
        reconnectDelay=1500
        handlers.onState('LIVE')
        return
      }
      if(message.type==='telemetry.update'&&message.deviceId===deviceId&&message.payload){
        handlers.onTelemetry({deviceId,payload:message.payload})
        return
      }
      if(message.type==='auth.error'||message.type==='subscription.error'){
        handlers.onError?.(message.message||'实时通道认证失败')
        task.close({code:4001,reason:'authentication failed'})
      }
    })
    task.onError(()=>scheduleReconnect())
    task.onClose(()=>{
      if(socket===task)socket=undefined
      scheduleReconnect()
    })
  }

  connect()
  return()=>{
    stopped=true
    if(reconnectTimer!==undefined)clearTimeout(reconnectTimer)
    reconnectTimer=undefined
    const active=socket
    socket=undefined
    active?.close({code:1000,reason:'page leave'})
  }
}

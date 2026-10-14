import {createApp,h,provide,ref} from 'vue'
import {createPinia} from 'pinia'
import SmartGatewayView from '../../src/components/SmartGatewayView.vue'
import GlobalConfirmDialog from '../../src/components/GlobalConfirmDialog.vue'
import {useConfigStore} from '../../src/stores/config.ts'
import {pageLeaveGuardKey} from '../../src/composables/usePageDraftGuard.ts'
import '../../src/styles.css'
import '../../src/legacy-interactions.css'
import '../../src/protocol-tools.css'

const protocol=new URLSearchParams(location.search).get('protocol')||'modbus-rtu'
const isForward=['modbus-tcp-slave','iec61850-mms-server'].includes(protocol)
const config={gatewayKey:'fixture',resources:[{resourceKey:'serial',name:'RS485-1',type:'serial',serial:{port:'/dev/ttyS1',baudRate:9600,dataBits:8,stopBits:1,parity:'none'}}],channels:[{channelKey:'channel',resourceKey:'serial',name:'通道1',role:isForward?'forward':'collect',protocol:isForward?'none':protocol,forwardProtocol:isForward?protocol:undefined}],devices:isForward?[{deviceKey:'source',channelKey:'source-channel',name:'来源',protocol:'modbus-rtu',points:[{metric:'source-value',name:'来源点位'}]}]:[{deviceKey:'device',channelKey:'channel',name:'采集设备1',protocol,address:'/dev/ttyS1',slaveId:2,points:[{metric:'P01',name:'点位1',function:3,register:0,dataType:'uint16',quantity:1,scale:0.1}]}],...(isForward?{forwardDevices:[{deviceKey:'device',channelKey:'channel',name:'转发设备1',protocol,points:[{metric:'custom-value',name:'自定义转发名称',sourceDeviceKey:'source',sourceMetric:'source-value',function:3,dataType:'uint16'}]}]}:{})}
const nativeFetch=window.fetch.bind(window)
const largeMode=new URLSearchParams(location.search).has('large')
const list=(isForward?config.forwardDevices:config.devices) as any[]
list.push({...structuredClone(list.find(item=>item.deviceKey==='device')),deviceKey:'device2',name:'另一台设备'})
const compact=()=>{const value=structuredClone(config);for(const item of [...value.devices,...(value.forwardDevices||[])])item.points=[];return value}
window.fetch=async(input,init)=>{
 const path=String(input)
 if(['/api/config','/api/config/compact'].includes(path)&&init?.method==='PUT'){
  ;(window as any).saveCount=((window as any).saveCount||0)+1
  if((window as any).failSave)return new Response('模拟保存失败',{status:500})
  const next=JSON.parse(String(init.body));for(const key of Object.keys(config))delete (config as any)[key];Object.assign(config,next)
  return Response.json({ok:true,restartRequired:false})
 }
 if(path.startsWith('/api/config/points?')){
  const query=new URLSearchParams(path.split('?')[1]),device=[...config.devices,...(config.forwardDevices||[])].find(item=>item.deviceKey===query.get('deviceKey'))
  const points=(device?.points||[]).filter((point:any)=>!query.has('function')||Number(point.function||3)===Number(query.get('function')))
  const offset=Number(query.get('offset')||0),limit=Number(query.get('limit')||160)
  return Response.json({total:points.length,offset,limit,points:points.slice(offset,offset+limit)})
 }
 if(path==='/api/config/point-mutations'){
  ;(window as any).mutationCount=((window as any).mutationCount||0)+1
  if((window as any).failSave)return new Response('模拟点位保存失败',{status:500})
  const body=JSON.parse(String(init?.body)),device=[...config.devices,...(config.forwardDevices||[])].find(item=>item.deviceKey===body.deviceKey)!
  if(body.action==='update'){const index=device.points.findIndex((point:any)=>point.metric===body.point.metric);device.points[index]=body.point}
  if(body.action==='rename'){const point=device.points.find((point:any)=>point.metric===body.oldMetric);if(point)point.metric=body.newMetric}
  return Response.json({ok:true,count:device.points.length})
 }
 if(path==='/api/point-status')return new Response('[]')
 if(path==='/api/network/interfaces')return new Response('{"interfaces":[]}')
 if(path==='/api/config/bootstrap')return new Response(JSON.stringify({content:JSON.stringify(largeMode?compact():config),largeMode,totalPoints:2,pointCounts:{device:1,device2:1}}))
 if(path==='/api/config')return new Response(JSON.stringify({content:JSON.stringify(config)}))
 return nativeFetch(input,init)
}
createApp({setup(){
 const store=useConfigStore();store.value=largeMode?compact():structuredClone(config);store.largeMode=largeMode;store.pointCounts={device:1,device2:1};if(largeMode)for(const item of [...store.value.devices,...(store.value.forwardDevices||[])])item._pointCount=1
 const mode=ref<'run'|'config'>('run'),selection=ref<any>({resourceKey:'serial'}),left=ref(0)
 const guards=new Set<()=>Promise<boolean>>()
 provide(pageLeaveGuardKey,(next:()=>Promise<boolean>)=>{guards.add(next);return()=>guards.delete(next)})
 async function guard(){for(const next of [...guards].reverse())if(!await next())return false;return true}
 ;(window as any).draftState=()=>JSON.parse(JSON.stringify(store.value))
 return()=>h('main',[
  h('button',{id:'edit',onClick:()=>mode.value='config'},'编辑'),
  h('button',{id:'run',onClick:()=>mode.value='run'},'运行'),
  h('button',{id:'resource',onClick:()=>selection.value={resourceKey:'serial',nonce:Date.now()}},'资源'),
  h('button',{id:'device',onClick:()=>selection.value={resourceKey:'serial',channelKey:'channel',deviceKey:'device',nonce:Date.now()}},'设备'),
  h('button',{id:'device2',onClick:()=>selection.value={resourceKey:'serial',channelKey:'channel',deviceKey:'device2',nonce:Date.now()}},'另一台设备'),
  h('button',{id:'channel',onClick:()=>selection.value={resourceKey:'serial',channelKey:'channel',nonce:Date.now()}},'通道'),
  h('button',{id:'leave',onClick:async()=>{if(await guard())left.value++}},'离开'),
  h('output',{id:'left'},String(left.value)),
  h(SmartGatewayView,{mode:mode.value,'onUpdate:mode':(value:'run'|'config')=>mode.value=value,selection:selection.value}),
  h(GlobalConfirmDialog),
 ])
}}).use(createPinia()).mount('#app')

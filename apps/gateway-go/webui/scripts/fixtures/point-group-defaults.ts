import {createApp,h,ref} from 'vue'
import {createPinia} from 'pinia'
import {useConfigStore} from '../../src/stores/config.ts'
import DevicePointWorkspace from '../../src/components/DevicePointWorkspace.vue'
import LargeDevicePointWorkspace from '../../src/components/LargeDevicePointWorkspace.vue'
import '../../src/styles.css'
import '../../src/legacy-interactions.css'
import '../../src/protocol-tools.css'
document.documentElement.dataset.gatewayDetail='device'
document.documentElement.dataset.configView='form'
const params=new URLSearchParams(location.search),protocol=params.get('protocol')||'modbus-rtu',scenario=params.get('scenario')||'only03'
const large=params.get('large')==='1'
const point=(fn=3,type='M_SP_NA_1',index=1)=>({metric:`P${index}`,name:`点位${index}`,function:fn,pointType:type,register:index,quantity:1,dataType:'uint16',scale:1,offset:0,decimals:0,unit:'',nodeId:'ns=2;s=probe',objectRef:'IEDLD1/GGIO1.AnIn1.mag.f',fc:'MX',itemId:'probe',gooseIndex:index,dlt645:{itemKey:'',dataIdentifier:'00000000',valueFormat:'bcd',dataLength:4,access:'read'}})
const points=scenario==='empty'||scenario==='late'?[]:scenario==='priority'?[...Array.from({length:200},(_,i)=>point(3,'M_SP_NA_1',i+1)),point(1,'M_ME_NC_1',201)]:scenario==='yk'?[point(3,'C_SC_NA_1')]:[point()]
const data:Record<string,any[]>={first:points,second:[point(1)]}
const requestLog:any[]=[]
;(window as any).pointRequests=requestLog
window.fetch=async(input)=>{
 const url=new URL(String(input),location.href)
 if(url.pathname==='/api/config/points'){
  const deviceKey=url.searchParams.get('deviceKey')||'first',fn=Number(url.searchParams.get('function')||0),limit=Number(url.searchParams.get('limit')||200),offset=Number(url.searchParams.get('offset')||0)
  const filtered=data[deviceKey].filter(p=>!fn||Number(p.function||3)===fn)
  requestLog.push({deviceKey,fn,limit})
  if(scenario==='slow'&&deviceKey==='first'&&limit===1)await new Promise(resolve=>setTimeout(resolve,300))
  return new Response(JSON.stringify({points:filtered.slice(offset,offset+limit),offset,limit,total:filtered.length}))
 }
 if(url.pathname==='/api/point-status')return new Response('[]')
 if(url.pathname==='/api/dlt645/dictionary')return new Response('{"items":[]}')
 return new Response('{}')
}
createApp({setup(){
 const store=useConfigStore();store.value={gatewayKey:'fixture'}
 const device=ref<any>({deviceKey:'first',name:'设备1',protocol,address:'/dev/ttyS1',slaveId:2,_pointCount:points.length,...(large?{}:{points})})
 ;(window as any).addLatePoints=()=>{device.value.points=[point()]}
 ;(window as any).switchProbeDevice=()=>{device.value={deviceKey:'second',name:'设备2',protocol,address:'/dev/ttyS1',slaveId:2,_pointCount:1,...(large?{}:{points:data.second})}}
 return()=>h('main',{class:'page'},[
  h('button',{id:'tick',onClick:()=>{if(!large)device.value.points.push(point(3,'M_SP_NA_1',999))}},'更新点位'),
  large?h(LargeDevicePointWorkspace,{device:device.value,readonly:true,isForward:protocol.includes('server')||protocol.includes('slave'),saving:false}):h(DevicePointWorkspace,{device:device.value,points:device.value.points,readonly:true,isForward:protocol.includes('server')||protocol.includes('slave'),saving:false,liveByMetric:new Map(),sourceDevices:[],sourceChannels:[],sourceResources:[]}),
 ])
}}).use(createPinia()).mount('#app')

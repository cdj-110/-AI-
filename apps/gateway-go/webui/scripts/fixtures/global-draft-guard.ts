import {createApp,h,provide,ref} from 'vue'
import {createPinia} from 'pinia'
import WiFiView from '../../src/components/WiFiView.vue'
import CellularView from '../../src/components/CellularView.vue'
import MaintenanceView from '../../src/components/MaintenanceView.vue'
import UserManagementView from '../../src/components/UserManagementView.vue'
import CloudView from '../../src/components/CloudView.vue'
import StorageView from '../../src/components/StorageView.vue'
import RemoteStorageView from '../../src/components/RemoteStorageView.vue'
import EdgeComputeView from '../../src/components/EdgeComputeView.vue'
import AlarmView from '../../src/components/AlarmView.vue'
import AiAssistantView from '../../src/components/AiAssistantView.vue'
import GlobalConfirmDialog from '../../src/components/GlobalConfirmDialog.vue'
import {useConfigStore} from '../../src/stores/config.ts'
import {pageLeaveGuardKey} from '../../src/composables/usePageDraftGuard.ts'
import '../../src/styles.css'
import '../../src/legacy-interactions.css'

const query=new URLSearchParams(location.search),name=query.get('view')||'wifi'
const views:any={wifi:WiFiView,cellular:CellularView,maintenance:MaintenanceView,users:UserManagementView,cloud:CloudView,storage:StorageView,remote:RemoteStorageView,edge:EdgeComputeView,alarms:AlarmView,ai:AiAssistantView}
let config:any={gatewayKey:'fixture',resources:[],channels:[],devices:[],forwardDevices:[],cloudConnectionEnabled:true,mqttChannels:[{name:'MQTT测试',enabled:true,broker:'tcp://fixture:1883',topic:'fixture',clientId:'fixture',username:'test',password:'',payloadFormat:'flat'}],remoteAccess:{enabled:false,endpoint:'',username:'',password:'',localUrl:'http://127.0.0.1:8088'},edgeComputing:{enabled:true,groups:[{groupKey:'group-1',name:'计算组1',enabled:true,points:[{metric:'edge-p1',name:'计算点1',dataType:'float64',expression:'1',inputs:[],enabled:true}]}]}}
const data:any={
 '/api/ai/settings':{settings:{enabled:false,deepseek:{baseUrl:'http://fixture',apiKey:'',model:'fixture-model'}},capabilities:{chat:false,pdf:false}},
 '/api/network/wifi':{config:{enabled:true,interface:'wlan0',ssid:'fixture',password:'',mode:'dhcp'},available:true,connected:true},
 '/api/network/cellular':{message:'ready'},
 '/api/maintenance/time':{config:{enabled:true,timezone:'Asia/Shanghai',primaryServer:'pool.ntp.org',secondaryServer:''},systemTime:new Date().toISOString(),supported:true,synchronized:true},
 '/api/maintenance/ssh':{supported:true,enabled:false,running:false,port:22},
 '/api/session':{username:'admin',roleKey:'admin',permissions:['*']},
 '/api/license/status':{license:{valid:true,restricted:false,features:[]},hardwareId:'fixture',chipUsid:'fixture'},
 '/api/license/details':{gatewayKey:'fixture',canReplace:false,license:{valid:true,restricted:false},authorization:{sn:'fixture',hardwareId:'fixture',chipUsid:'fixture',licenseVersion:2,keyId:'fixture',licenseSequence:1,issuedAt:'',expiresAt:'',features:[],signed:true}},
 '/api/security':{users:[{username:'admin',displayName:'管理员',enabled:true,roleKey:'admin'}],roles:[{roleKey:'admin',name:'管理员',description:'',permissions:['*'],builtIn:true}],permissions:[]},
 '/api/security/https':{enabled:false,certificateOK:true},'/api/security/mfa':{enabled:false,username:'admin',recoveryRemaining:0},
 '/api/history-storage':{configured:true,active:true,maxSizeMB:16,storagePath:'/userdata/weikong/data',devices:[],tasks:[{taskKey:'task1',name:'历史任务1',enabled:true,selectionMode:'all',points:[],saveMode:'interval',saveIntervalSeconds:1,changeDeadband:0}]},
 '/api/history-storage/exports':{jobs:[]},'/api/history-storage/maintenance':{database:{},backups:[],aggregates:{}},
 '/api/remote-storage':{config:{storagePath:'/userdata/weikong/data',storageUUID:'',maxSizeMB:16,targets:[]},revision:'fixture',statuses:[],devices:[],pointGroups:[]},
 '/api/remote-storage/status':{statuses:[]},
 '/api/activation':{content:'{}'},'/api/storage':{configured:false,active:false,storagePath:'/media/udisk1',maxSizeMB:16,devices:[]},
 '/api/alarm-rules':{rules:[{ruleId:'rule1',name:'规则1',enabled:true,deviceKey:'device',metric:'P1',type:'threshold',operator:'>',threshold:10,hysteresis:0,triggerDelaySeconds:0,recoveryDelaySeconds:0,level:'WARNING'}]},
 '/api/alarms':{active:[],history:[],activeCount:0,criticalCount:0,unackedCount:0},'/api/alarms/history':{items:[],page:1,total:0,totalPages:1},
 '/api/point-status':[],'/api/edge-compute/status':[],
}
;(window as any).saveRequests=[];(window as any).failSave=false
window.fetch=async(input,init)=>{
 const path=String(input),key=path.split('?')[0],method=init?.method||'GET'
 if(['PUT','POST'].includes(method)){
  ;(window as any).saveRequests.push({path:key,body:init?.body})
  if((window as any).failSave)return new Response('模拟保存失败',{status:500})
  const body=init?.body?JSON.parse(String(init.body)):{}
  if(key==='/api/config'){config=body;return Response.json({ok:true,restartRequired:false})}
  if(key==='/api/edge-compute/config'){config.edgeComputing=body.edgeComputing;return Response.json({ok:true})}
  if(key==='/api/network/wifi')data[key].config=body
  else if(key==='/api/ai/settings')data[key].settings=body
  else if(key==='/api/maintenance/time')data[key].config=body
  else if(key==='/api/history-storage')data[key]={...data[key],...body,configured:body.enabled}
  else if(key==='/api/remote-storage')data[key].config=body
  else if(key==='/api/security')data[key]={...data[key],...body}
  else if(key==='/api/storage')data[key]={...data[key],...body,configured:body.enabled}
  else if(key==='/api/alarm-rules')data[key]=body
  else if(key==='/api/network/cellular')config.cellular=body
 }
 if(key==='/api/config/bootstrap')return Response.json({content:JSON.stringify(config),largeMode:false,totalPoints:0})
 if(key==='/api/config')return Response.json({content:JSON.stringify(config)})
 if(key==='/api/network/wifi'&&method==='PUT')return Response.json({ok:true,restartRequired:false})
 return Response.json(data[key]||{})
}
createApp({setup(){
 const store=useConfigStore();store.value=structuredClone(config)
 const left=ref(0),mounted=ref(true),mode=ref<'config'|'run'>('config'),guards=new Set<()=>Promise<boolean>>()
 provide(pageLeaveGuardKey,(guard:()=>Promise<boolean>)=>{guards.add(guard);return()=>guards.delete(guard)})
 async function navigate(){for(const guard of [...guards].reverse())if(!await guard())return;left.value++;if(query.has('unmount'))mounted.value=false}
 return()=>h('main',[
  h('button',{id:'navigate',style:{position:'relative',zIndex:99999},onClick:navigate},'其他功能页'),h('output',{id:'left'},String(left.value)),
  mounted.value?h(views[name],{canManage:true,centerNonce:0,mode:mode.value,'onUpdate:mode':(value:any)=>mode.value=value}):h('p','其他页面'),h(GlobalConfirmDialog),
 ])
}}).use(createPinia()).mount('#app')

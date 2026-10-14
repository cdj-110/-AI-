import {channelRole} from './channelGroups.ts'
export interface DeviceHealth {deviceKey:string;total:number;healthy:number;failed:number;pending:number;stale:number}
export interface OverviewIssue {deviceKey:string;reason:string;detail:string;count:number;protocol:string;address:string}
export interface CollectionOverview {devices:DeviceHealth[];issues:OverviewIssue[];issueGroups:number;affectedPoints:number}
export type NodeState='healthy'|'partial'|'failed'|'disabled'|'pending'|'forward'
export type TopologySelection={resourceKey?:string;channelKey?:string;deviceKey?:string}
export const stateLabels:Record<NodeState,string>={healthy:'采集正常',partial:'部分异常',failed:'采集异常',disabled:'已停用',pending:'待采集',forward:'转发配置'}
export function deviceState(device:any,channel:any,resource:any,health?:DeviceHealth):NodeState{
  if(device.enabled===false||channel.enabled===false||resource.enabled===false||resource.serial?.enabled===false||resource.network?.enabled===false)return 'disabled'
  if(channelRole(channel)==='forward')return 'forward'
  if(!health||health.total===0)return 'pending'
  if(health.failed+health.stale>0)return health.healthy>0?'partial':'failed'
  return health.pending>0?'pending':'healthy'
}
export function buildTopology(config:any,health:DeviceHealth[]){
  const byDevice=new Map(health.map(d=>[d.deviceKey,d]))
  const channels=(Array.isArray(config.channels)?config.channels:[]).slice()
  const channelKeysByRole={collect:new Set(channels.filter((c:any)=>channelRole(c)==='collect').map((c:any)=>c.channelKey)),forward:new Set(channels.filter((c:any)=>channelRole(c)==='forward').map((c:any)=>c.channelKey))}
  for(const role of ['collect','forward'] as const){
    const list=role==='collect'?config.devices||[]:config.forwardDevices||[]
    if(list.some((d:any)=>!channelKeysByRole[role].has(d.channelKey)))channels.push({channelKey:'__unassigned-'+role,name:'未绑定通道的设备',resourceKey:'__unassigned',role,unassigned:true})
  }
  const resources=(Array.isArray(config.resources)?config.resources:[]).slice()
  const resourceKeys=new Set(resources.map((r:any)=>r.resourceKey))
  if(channels.some((c:any)=>!resourceKeys.has(c.resourceKey)))resources.push({resourceKey:'__unassigned',name:'未绑定资源',type:'unknown'})
  return resources.map((r:any)=>({...r,channels:channels.filter((c:any)=>r.resourceKey==='__unassigned'?!resourceKeys.has(c.resourceKey):c.resourceKey===r.resourceKey).map((c:any)=>{
    const role=channelRole(c),forward=role==='forward'
    return {...c,role,devices:(forward?config.forwardDevices||[]:config.devices||[]).filter((d:any)=>c.unassigned?!channelKeysByRole[role].has(d.channelKey):d.channelKey===c.channelKey).map((d:any)=>({...d,health:byDevice.get(d.deviceKey),state:c.unassigned||r.resourceKey==='__unassigned'?'pending':deviceState(d,c,r,byDevice.get(d.deviceKey))}))}
  }).sort((a:any,b:any)=>Number(a.role==='forward')-Number(b.role==='forward'))}))
}
export function deviceSelection(config:any,deviceKey:string):TopologySelection|undefined{
  const d=(config.devices||[]).find((d:any)=>d.deviceKey===deviceKey)
  const c=(config.channels||[]).find((c:any)=>c.channelKey===d?.channelKey)
  return d&&c?{resourceKey:c.resourceKey,channelKey:c.channelKey,deviceKey}:undefined
}
export function protocolLabel(value:string){return ({'modbus-rtu':'Modbus RTU',dlt645:'DL/T 645','modbus-tcp':'Modbus TCP',iec104:'IEC104','siemens-s7':'Siemens S7',opcua:'OPC UA',opcda:'OPC DA',iec61850:'IEC 61850 MMS','modbus-tcp-slave':'Modbus TCP 从站','iec104-server':'IEC104 服务端','iec61850-mms-server':'IEC61850 MMS 服务端'} as Record<string,string>)[value]||value||'未配置协议'}
export function channelProtocol(channel:any){return channelRole(channel)==='forward'&&channel.forwardProtocol&&channel.forwardProtocol!=='none'?channel.forwardProtocol:channel.protocol}

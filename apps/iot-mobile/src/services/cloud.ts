import type { Alarm, Device, DeviceCommandCapabilities, DeviceCommandTask, DeviceMetric, FactoryGateway, GatewayBindCredential, GatewayBindResult, GatewayBindVerification, GatewayNfcCredential, GatewayOption, ModelTemplate, TelemetryItem, Tenant, UserInfo } from '../types'
import { request } from './http'

export function login(username:string,password:string,portal:'tenant'|'admin'='tenant'){
  return request<{accessToken:string;user:UserInfo}>(`/api/auth/login/${portal}`,{method:'POST',data:{username,password}})
}
export function profile(){return request<UserInfo>('/api/auth/profile')}
export function devices(params:{page?:number;pageSize?:number;keyword?:string;deviceType?:string}={}){
  return request<{items:Device[];total:number;stats?:{total:number;online:number;offline:number;disabled:number}}>('/api/devices',{query:{page:params.page||1,pageSize:params.pageSize||100,keyword:params.keyword||'',deviceType:params.deviceType||''}})
}
export function device(id:string){return request<Device>(`/api/devices/${id}`)}
export function telemetry(id:string){return request<{deviceKey:string;items:TelemetryItem[]}>(`/api/devices/${id}/telemetry`)}
export function metrics(id:string){return request<DeviceMetric[]>(`/api/devices/${id}/metrics`)}
export function commandCapabilities(id:string){return request<DeviceCommandCapabilities>(`/api/devices/${id}/commands/capabilities`)}
export function deviceCommands(id:string){return request<DeviceCommandTask[]>(`/api/devices/${id}/commands`)}
export function sendDeviceCommand(id:string,data:{requestId:string;metric:string;value:unknown}){
  return request<DeviceCommandTask>(`/api/devices/${id}/commands`,{method:'POST',data})
}
export function alarms(params:{page?:number;pageSize?:number;status?:string}={}){
  return request<{items:Alarm[];total:number;unreadTotal:number}>('/api/alarms',{query:{page:params.page||1,pageSize:params.pageSize||50,status:params.status||''}})
}
export function markAlarmNotificationsRead(){return request<{readAt:string;unreadTotal:number}>('/api/alarms/notifications/read',{method:'PATCH'})}
export function gateways(){return request<GatewayOption[]>('/api/devices/gateways')}
export function templates(){return request<ModelTemplate[]>('/api/model-templates/options')}
export function createDevice(data:Record<string,unknown>){return request<Device>('/api/devices',{method:'POST',data})}
export function verifyGatewayBind(data:GatewayBindCredential){
  return request<GatewayBindVerification>('/api/factory-gateways/bind/verify',{method:'POST',data})
}
export function bindGateway(data:GatewayBindCredential&{name?:string;location?:string}){
  return request<GatewayBindResult>('/api/factory-gateways/bind',{method:'POST',data})
}
export function tenants(){return request<Tenant[]>('/api/tenants')}
export function factoryGateways(params:{page?:number;pageSize?:number;keyword?:string}={}){
  return request<{items:FactoryGateway[];total:number;stats?:{total:number;bound:number;unbound:number}}>('/api/factory-gateways',{query:{page:params.page||1,pageSize:params.pageSize||100,keyword:params.keyword||''}})
}
export function prepareGatewayNfcCredential(id:string){
  return request<GatewayNfcCredential>(`/api/factory-gateways/${encodeURIComponent(id)}/nfc-credential`,{method:'POST'})
}
export function resetGatewayNfcCredential(id:string){
  return request<GatewayNfcCredential>(`/api/factory-gateways/${encodeURIComponent(id)}/nfc-credential/reset`,{method:'POST'})
}
export function recordGatewayNfcWrite(id:string,data:{sn:string;success:boolean;message?:string}){
  return request<{ok:boolean}>(`/api/factory-gateways/${encodeURIComponent(id)}/nfc-write-result`,{method:'POST',data})
}
export function gatewayNfcWriteHistory(pageSize=50){
  return request<{items:Array<{id:string;username:string;action:string;targetName?:string;detail?:{sn?:string;success?:boolean;message?:string};createdAt:string}>;total:number}>('/api/factory-gateways/nfc-write-history',{query:{page:1,pageSize}})
}
export function removeFactoryGateway(id:string){
  return request<{message:string;id:string;sn:string}>(`/api/factory-gateways/${encodeURIComponent(id)}`,{method:'DELETE'})
}
export function reclaimFactoryGateway(id:string){
  return request<{message:string;id:string;sn:string}>(`/api/factory-gateways/${encodeURIComponent(id)}/reclaim`,{method:'POST'})
}
export type LabelPrintAgent={id:string;name:string;printerName?:string;online:boolean;lastSeenAt?:string;lastError?:string}
export function labelPrintAgents(){return request<LabelPrintAgent[]>('/api/label-print/agents')}
export function createLabelPrintJob(data:{agentId:string;factoryGatewayId:string;labelSize:string;copies:number}){
  return request<{id:string;status:string;sn:string}>('/api/label-print/jobs',{method:'POST',data})
}
export function labelPrintJobs(pageSize=50){
  return request<{items:Array<{id:string;sn:string;status:string;labelSize:string;copies:number;error?:string;createdAt:string;agent:{name:string}}> ;total:number}>('/api/label-print/jobs',{query:{page:1,pageSize}})
}

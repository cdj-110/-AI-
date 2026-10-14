export type RemoteDriver='mysql'|'sqlserver'|'postgresql'
export interface PointRef{deviceKey:string;metric:string}
export interface RemoteTarget{
  id:string;name:string;enabled:boolean;driver:RemoteDriver;host:string;port:number;database:string;schema:string;table:string;username:string;password:string;tlsMode:'verify-full'|'disabled';caFile:string;createTable:boolean;intervalSeconds:number;batchSize:number;timeoutSeconds:number;points:PointRef[]
}
export interface RemoteConfig{storagePath:string;storageUUID:string;maxSizeMB:number;targets:RemoteTarget[]}
export const driverNames:Record<RemoteDriver,string>={mysql:'MySQL',sqlserver:'SQL Server',postgresql:'PostgreSQL'}
export function changeRemoteDriver(t:RemoteTarget){
  t.port={mysql:3306,sqlserver:1433,postgresql:5432}[t.driver]
  t.schema={mysql:'',sqlserver:'dbo',postgresql:'public'}[t.driver]
}
export function newRemoteTarget():RemoteTarget{
  return {id:'db-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,7),name:'远程存储任务',enabled:false,driver:'mysql',host:'',port:3306,database:'',schema:'',table:'gateway_samples',username:'',password:'',tlsMode:'verify-full',caFile:'',createTable:false,intervalSeconds:5,batchSize:100,timeoutSeconds:10,points:[]}
}
export function pointKey(p:PointRef){return JSON.stringify([p.deviceKey,p.metric])}
export function validateRemoteTarget(t:RemoteTarget,requirePoints=true):string{
  if(!t.name.trim()||!t.host.trim()||!t.database.trim()||!t.username.trim())return '请填写任务名称、数据库主机、数据库名称和账号。'
  if(!Number.isInteger(t.port)||t.port<1||t.port>65535)return '端口范围为 1～65535。'
  if(!/^[A-Za-z_][A-Za-z0-9_]{0,47}$/.test(t.table)||(t.driver!=='mysql'&&!/^[A-Za-z_][A-Za-z0-9_]{0,47}$/.test(t.schema)))return '表名和模式名只允许字母、数字和下划线，不能以数字开头。'
  if(!Number.isInteger(t.intervalSeconds)||t.intervalSeconds<1||t.intervalSeconds>86400)return '存储周期范围为 1～86400 秒。'
  if(!Number.isInteger(t.batchSize)||t.batchSize<1||t.batchSize>500)return '批量大小范围为 1～500。'
  if(!Number.isInteger(t.timeoutSeconds)||t.timeoutSeconds<2||t.timeoutSeconds>60)return '数据库超时范围为 2～60 秒。'
  if(requirePoints&&(t.points.length<1||t.points.length>2000))return '每个任务请选择 1～2000 个点位。'
  if(new Set(t.points.map(pointKey)).size!==t.points.length)return '点位不能重复。'
  return ''
}

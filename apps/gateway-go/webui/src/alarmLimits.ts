export type AlarmLimitKey='lowLow'|'low'|'high'|'highHigh'
export type AlarmLimit={enabled:boolean;threshold?:number|null;level:'INFO'|'WARNING'|'CRITICAL'}
export type AlarmLimits=Partial<Record<AlarmLimitKey,AlarmLimit>>
export const alarmLimitItems=[
 {key:'lowLow' as const,name:'下下限',operator:'<',level:'CRITICAL' as const},
 {key:'low' as const,name:'下限',operator:'<',level:'WARNING' as const},
 {key:'high' as const,name:'上限',operator:'>',level:'WARNING' as const},
 {key:'highHigh' as const,name:'上上限',operator:'>',level:'CRITICAL' as const},
]
export function createAlarmLimits():AlarmLimits{return Object.fromEntries(alarmLimitItems.map(item=>[item.key,{enabled:item.key==='high',threshold:null,level:item.level}]))}
export function normalizeAlarmLimits(limits:AlarmLimits):AlarmLimits{return Object.fromEntries(alarmLimitItems.map(item=>[item.key,{enabled:limits[item.key]?limits[item.key]!.enabled!==false:false,threshold:limits[item.key]?.threshold??null,level:limits[item.key]?.level||item.level}]))}
export function alarmLimitsError(limits:AlarmLimits):string{
 let previous:{name:string;threshold:number}|undefined
 for(const item of alarmLimitItems){const limit=limits[item.key];if(!limit||limit.enabled===false)continue
  if(typeof limit.threshold!=='number'||!Number.isFinite(limit.threshold))return `请填写${item.name}的有效阈值`
  if(previous&&previous.threshold>=limit.threshold)return `${previous.name}必须小于${item.name}（下下限 < 下限 < 上限 < 上上限）`
  previous={name:item.name,threshold:limit.threshold}
 }
 return previous?'':'请至少启用一个限值'
}
export function alarmLimitsSummary(limits:AlarmLimits){return alarmLimitItems.filter(item=>limits[item.key]&&limits[item.key]!.enabled!==false).map(item=>`${item.name} ${item.operator} ${limits[item.key]!.threshold??'未设置'}`).join('；')}

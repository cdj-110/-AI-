export type SwitchPoint={value1:string;value2:string}
export function switchBoundPoint(request:any):SwitchPoint|null {
  if(request?.requestHttpType!=='实时数据')return null
  const raw=request?.requestParams?.Params
  const pairs=(Array.isArray(raw)?raw:Object.values(raw||{})).filter((p:any)=>p?.value1&&p?.value2) as SwitchPoint[]
  if(pairs.length!==1)return null
  return {value1:pairs[0].value1,value2:pairs[0].value2}
}
export function switchValueMapping(option:any,raw:unknown,declaredType?:string):{type:string;on:boolean|number|string;off:boolean|number|string}|null {
  if(option.valueMode==='manual') {
    const type=option.manualValueType||'NUMBER'
    const on=type==='BOOLEAN'?(option.controlOnBool??true):type==='NUMBER'?(option.controlOnNumber??1):(option.controlOnText??'ON')
    const off=type==='BOOLEAN'?(option.controlOffBool??false):type==='NUMBER'?(option.controlOffNumber??0):(option.controlOffText??'OFF')
    if(!['BOOLEAN','NUMBER','STRING'].includes(type)||on===off)return null
    if(type==='NUMBER'&&(!Number.isFinite(on)||!Number.isFinite(off)))return null
    if(type==='BOOLEAN'&&(typeof on!=='boolean'||typeof off!=='boolean'))return null
    if(type==='STRING'&&(typeof on!=='string'||typeof off!=='string'||on.length>2048||off.length>2048))return null
    return {type,on,off}
  }
  const type=String(declaredType||typeof raw).toUpperCase()
  if(['BOOLEAN','BOOL'].includes(type))return {type:'BOOLEAN',on:true,off:false}
  if(type==='NUMBER')return raw==null||raw===0||raw===1?{type:'NUMBER',on:1,off:0}:null
  if(type==='STRING'&&typeof raw==='string'){
    for(const [on,off] of [['ON','OFF'],['on','off'],['On','Off'],['true','false'],['TRUE','FALSE'],['True','False'],['1','0']])if(raw===on||raw===off)return {type:'STRING',on,off}
  }
  return null
}
export function switchWriteValue(on:boolean,dataType:string,option:any,raw?:unknown):boolean|number|string {
  const mapping=switchValueMapping(option,raw,dataType)
  if(!mapping)throw Error('无法识别开关值，请切换手动设置并配置不同的开关值')
  if(mapping.type!==dataType)throw Error('手动值类型与实际点位类型不一致，请调整配置')
  return on?mapping.on:mapping.off
}
export function commandApiData(result:any) {
  if(!result||![0,200,201].includes(result.code))throw Error(result?.message||result?.msg||'下发接口返回异常')
  return result.data
}

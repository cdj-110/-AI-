export function iec104PointGroup(point:any){
  const type=String(point?.pointType||'').toUpperCase(),data=String(point?.dataType||'').toLowerCase()
  if(type.startsWith('M_SP')||type.startsWith('M_DP')||['single','double'].includes(data))return 'yx'
  if(type.startsWith('C_SC')||type.startsWith('C_DC')||data.includes('command'))return 'yk'
  if(type.startsWith('C_SE')||data.includes('setpoint'))return 'yt'
  if(type.startsWith('M_IT')||data.includes('counter'))return 'ym'
  return 'yc'
}

export function firstPopulatedPointGroup(keys:readonly string[],counts:Record<string,number>,fallback='all'){
  return keys.find(key=>Number(counts[key]||0)>0)||keys[0]||fallback
}

export function defaultPointGroup(protocol:string,points:readonly any[]){
  const normalized=String(protocol||'').toLowerCase()
  const modbus=normalized.startsWith('modbus')
  const keys=modbus?['1','2','3','4']:['iec104','iec104-server'].includes(normalized)?['yc','yx','yk','yt','ym']:[]
  if(!keys.length)return 'all'
  const counts:Record<string,number>={}
  for(const point of points){
    const key=modbus?String(point.function||3):iec104PointGroup(point)
    counts[key]=(counts[key]||0)+1
  }
  return firstPopulatedPointGroup(keys,counts)
}

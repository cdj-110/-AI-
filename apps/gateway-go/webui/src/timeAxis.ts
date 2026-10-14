// Charts represent elapsed time, not the count of received samples.
export function timeAxisFractions(timestamps: string[]): number[] {
  const times=timestamps.map(value=>Date.parse(value))
  const valid=times.filter(Number.isFinite)
  if(!valid.length)return times.map(()=>NaN)
  const start=Math.min(...valid),end=Math.max(...valid)
  return times.map(value=>!Number.isFinite(value)?NaN:end===start?.5:(value-start)/(end-start))
}

export function contiguousSegments<T>(points:(T|undefined)[]):T[][] {
  const segments:T[][]=[]
  let current:T[]=[]
  for(const point of points){if(point===undefined){if(current.length)segments.push(current);current=[]}else current.push(point)}
  if(current.length)segments.push(current)
  return segments
}

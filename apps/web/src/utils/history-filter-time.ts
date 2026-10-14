export interface TimeBounds { min:number; max:number }
export function historyFilterBounds(start:Date,end:Date,earliest?:string,now=Date.now()):TimeBounds {
  return {min:Math.ceil(Math.max(start.getTime(),earliest?new Date(earliest).getTime():-Infinity)/1000)*1000,max:Math.floor(Math.min(end.getTime(),now)/1000)*1000};
}
export const validTimeBounds=(bounds:TimeBounds)=>Number.isFinite(bounds.min)&&Number.isFinite(bounds.max)&&bounds.min<bounds.max;
export function boundedFilterRange(value:[Date,Date]|null,bounds:TimeBounds):[Date,Date]|null {
  if(!validTimeBounds(bounds))return null;
  const fallback=()=>[new Date(bounds.min),new Date(bounds.max)] as [Date,Date];
  if(!value||value.some(d=>!Number.isFinite(d.getTime())))return fallback();
  const clamp=(d:Date)=>Math.min(bounds.max,Math.max(bounds.min,Math.floor(d.getTime()/1000)*1000));
  const start=clamp(value[0]),end=clamp(value[1]);
  return start<end?[new Date(start),new Date(end)]:fallback();
}
export function disabledFilterDate(date:Date,bounds:TimeBounds) {
  const start=new Date(date);start.setHours(0,0,0,0);const end=new Date(start);end.setDate(end.getDate()+1);
  return !validTimeBounds(bounds)||end.getTime()-1<bounds.min||start.getTime()>bounds.max;
}
export function disabledTimeUnits(date:Date,bounds:TimeBounds,unit:'hour'|'minute'|'second',hour=0,minute=0):number[] {
  const count=unit==='hour'?24:60,step=unit==='hour'?3600000:unit==='minute'?60000:1000;
  return Array.from({length:count},(_,i)=>i).filter(i=>{
    const start=new Date(date);start.setHours(unit==='hour'?i:hour,unit==='minute'?i:unit==='second'?minute:0,unit==='second'?i:0,0);
    return !validTimeBounds(bounds)||start.getTime()+step-1<bounds.min||start.getTime()>bounds.max;
  });
}

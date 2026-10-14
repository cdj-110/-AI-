import {alarmLevels,alarmLevelSettings,type AlarmSoundSettings} from './alarmSound.ts'
export function applyAlarmColors(settings:AlarmSoundSettings){
 for(const level of alarmLevels){
  const color=alarmLevelSettings(settings,level.key).color
  const rgb=[1,3,5].map(i=>parseInt(color.slice(i,i+2),16))
  const mix=(white:number)=>`rgb(${rgb.map(v=>Math.round(v*(1-white)+255*white)).join(', ')})`
  const text=`rgb(${rgb.map(v=>Math.round(v*.42)).join(', ')})`
  const prefix=`--alarm-${level.key.toLowerCase()}`
  for(const [suffix,value] of Object.entries({color,soft:mix(.92),pale:mix(.65),text}))document.documentElement.style.setProperty(`${prefix}-${suffix}`,value)
 }
}

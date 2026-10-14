// User-supplied WAV files, kept verbatim and bundled for offline gateway use.
export const bundledAlarmAudio={
 'file-buzzer4':{name:'提示声 · Buzzer4',duration:2.4,url:new URL('./assets/alarm-audio/Buzzer4.wav',import.meta.url).href},
 'file-buzzer1':{name:'警告声 · Buzzer1',duration:2.4,url:new URL('./assets/alarm-audio/Buzzer1.wav',import.meta.url).href},
 'file-alarm2':{name:'严重声 · Alarm2',duration:0.680090703,url:new URL('./assets/alarm-audio/Alarm2.WAV',import.meta.url).href},
 'file-siren1':{name:'备用警笛 · Siren1',duration:4.649795919,url:new URL('./assets/alarm-audio/Siren1.wav',import.meta.url).href},
} as const
export type BundledAlarmTone=keyof typeof bundledAlarmAudio
export function defaultAlarmFile(level:string):BundledAlarmTone{return level==='CRITICAL'?'file-alarm2':level==='WARNING'?'file-buzzer1':'file-buzzer4'}
export function alarmBundledFile(tone:string,level:string){const key=tone==='provided'?defaultAlarmFile(level):tone;return key in bundledAlarmAudio?bundledAlarmAudio[key as BundledAlarmTone]:undefined}

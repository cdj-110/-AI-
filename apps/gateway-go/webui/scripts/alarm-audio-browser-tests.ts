import {storeAlarmAudio,loadAlarmAudio,removeAlarmAudio} from '../src/alarmAudioFiles.ts'
const result=document.querySelector('#result')!,button=document.querySelector<HTMLButtonElement>('#run')!
const assert=(ok:boolean,message:string)=>{if(!ok)throw new Error(message);result.textContent+='PASS '+message+'\n'}
button.onclick=async()=>{
 button.disabled=true;result.textContent=''
 const scope='isolated-audio-tests-'+Date.now(),ids:string[]=[]
 try{
  const bytes=await (await fetch('/artifacts/alarm-audio/test-alarm.wav')).arrayBuffer()
  const file=new File([bytes],'tone.wav',{type:'audio/wav'})
  const saved=await storeAlarmAudio(scope,file);ids.push(saved.id)
  assert(Math.abs(saved.duration-2)<.01,'真实 WAV 解码与时长校验')
  assert((await loadAlarmAudio(scope,saved.id)).byteLength===bytes.byteLength,'IndexedDB 音频完整保存与读取')
  try{await loadAlarmAudio(scope+'-other',saved.id);throw new Error('unexpected access')}catch(e){assert(!(e instanceof Error&&e.message==='unexpected access'),'不同账号不能读取其他账号音频')}
  for(const [bad,expected] of [[new File([bytes],'file.txt'),/请选择/],[new File([],'empty.wav'),/不能为空/],[new File([new Uint8Array(2*1024*1024+1)],'big.wav'),/2 MB/],[new File(['corrupt audio'],'bad.mp3'),/无法解码/]] as const){
   try{await storeAlarmAudio(scope,bad);throw new Error('unexpected success')}catch(e){assert(e instanceof Error&&expected.test(e.message),'拒绝 '+bad.name)}
  }
  const longBytes=new Uint8Array(44+8000*31*2);longBytes.set(new Uint8Array(bytes).slice(0,44));const view=new DataView(longBytes.buffer);view.setUint32(4,longBytes.length-8,true);view.setUint32(40,longBytes.length-44,true)
  try{await storeAlarmAudio(scope,new File([longBytes],'long.wav'));throw new Error('unexpected success')}catch(e){assert(e instanceof Error&&e.message.includes('时长'),'拒绝超过 30 秒的音频')}
  await removeAlarmAudio(scope,saved.id)
  try{await loadAlarmAudio(scope,saved.id);throw new Error('unexpected access')}catch(e){assert(!(e instanceof Error&&e.message==='unexpected access'),'移除后音频不可读取')}
  result.textContent+='全部通过；仅使用本地模拟音频，无网关请求。'
 }catch(e){result.textContent+='FAILED '+String(e)}finally{await Promise.all(ids.map(id=>removeAlarmAudio(scope,id).catch(()=>undefined)));button.disabled=false}
}

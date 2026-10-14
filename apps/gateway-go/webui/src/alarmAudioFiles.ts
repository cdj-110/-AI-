export interface AlarmAudioFile {id:string;name:string;duration:number}
export const maxAlarmAudioBytes=2*1024*1024
export function normalizeAlarmAudioFile(value:unknown):AlarmAudioFile|undefined{
 const raw=value as Partial<AlarmAudioFile>|null
 if(!raw||typeof raw.id!=='string'||!/^audio-[a-z0-9-]{16,80}$/i.test(raw.id)||typeof raw.name!=='string'||typeof raw.duration!=='number'||!Number.isFinite(raw.duration)||raw.duration<=0||raw.duration>30)return
 return {id:raw.id,name:raw.name.slice(0,120),duration:raw.duration}
}
function database():Promise<IDBDatabase>{
 return new Promise((resolve,reject)=>{
  const request=indexedDB.open('gateway-alarm-audio-v1',1)
  request.onupgradeneeded=()=>request.result.createObjectStore('files')
  request.onerror=()=>reject(new Error('浏览器未允许保存音频，请检查存储权限或空间。'))
  request.onblocked=()=>reject(new Error('音频存储被其他页面占用，请关闭旧页面后重试。'))
  request.onsuccess=()=>resolve(request.result)
 })
}
async function access<T>(scope:string,id:string,mode:IDBTransactionMode,action:(store:IDBObjectStore,key:string)=>IDBRequest<T>):Promise<T>{
 const db=await database()
 return new Promise((resolve,reject)=>{
  const tx=db.transaction('files',mode),req=action(tx.objectStore('files'),JSON.stringify([scope,id]))
  tx.oncomplete=()=>{db.close();resolve(req.result)}
  tx.onabort=tx.onerror=()=>{db.close();reject(new Error('音频保存失败，请检查浏览器存储空间或权限。'))}
 })
}
export async function storeAlarmAudio(scope:string,file:File):Promise<AlarmAudioFile>{
 if(!/\.(mp3|wav|ogg)$/i.test(file.name))throw new Error('请选择 MP3、WAV 或 OGG 音频文件。')
 if(!file.size||file.size>maxAlarmAudioBytes)throw new Error('音频文件不能为空，且不能超过 2 MB。')
 const context=new AudioContext()
 let duration:number
 try{
  const decoded=await context.decodeAudioData(await file.arrayBuffer())
  duration=decoded.duration
  if(!Number.isFinite(duration)||duration<=0||duration>30)throw new Error('音频时长须大于 0 且不超过 30 秒。')
 }catch(error){
  if(error instanceof Error&&error.message.includes('时长'))throw error
  throw new Error('无法解码该音频，文件可能损坏或浏览器不支持，请转换为 MP3 或 WAV 后重试。')
 }finally{await context.close().catch(()=>undefined)}
 const random=crypto.getRandomValues(new Uint32Array(4))
 const metadata={id:`audio-${Array.from(random,n=>n.toString(16).padStart(8,'0')).join('')}`,name:file.name.slice(0,120),duration}
 await access(scope,metadata.id,'readwrite',(store,key)=>store.put(file,key))
 return metadata
}
export async function loadAlarmAudio(scope:string,id:string):Promise<ArrayBuffer>{
 const file=await access<Blob|undefined>(scope,id,'readonly',(store,key)=>store.get(key))
 if(!(file instanceof Blob)||!file.size||file.size>maxAlarmAudioBytes)throw new Error('自定义音频已丢失或不可用，请重新上传。')
 return file.arrayBuffer()
}
export async function removeAlarmAudio(scope:string,id:string){await access(scope,id,'readwrite',(store,key)=>store.delete(key))}

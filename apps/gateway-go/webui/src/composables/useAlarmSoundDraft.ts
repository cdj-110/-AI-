import {defaultAlarmFile} from '../alarmBundledAudio.ts'
import {computed,ref,watch} from 'vue'
import {alarmLevelSettings,normalizeAlarmSoundSettings,type AlarmSoundSettings} from '../alarmSound.ts'
import {storeAlarmAudio,removeAlarmAudio} from '../alarmAudioFiles.ts'

// Draft uploads must never replace or delete the live alarm audio before Save.
export function useAlarmSoundDraft(source:()=>AlarmSoundSettings,scope:()=>string,commit:(value:AlarmSoundSettings)=>boolean,
  files={store:storeAlarmAudio,remove:removeAlarmAudio}){
  const draft=ref(normalizeAlarmSoundSettings(source()))
  const baseline=ref(JSON.stringify(draft.value))
  const uploading=ref<Record<string,boolean>>({}),uploadError=ref<Record<string,string>>({})
  const notice=ref(''),conflict=ref(false)
  const versions:Record<string,number>={}
  const staged=new Map<string,string>()
  let disposed=false
  const busy=computed(()=>Object.values(uploading.value).some(Boolean))
  const dirty=computed(()=>JSON.stringify(draft.value)!==baseline.value)
  const pending=computed(()=>dirty.value||busy.value)
  function remove(owner:string,id:string){void files.remove(owner,id).catch(()=>undefined)}
  function reset(){
    for(const level of Object.keys(versions))versions[level]=(versions[level]||0)+1
    uploading.value={};uploadError.value={}
    for(const [id,owner] of staged)remove(owner,id)
    staged.clear()
    draft.value=normalizeAlarmSoundSettings(source());baseline.value=JSON.stringify(draft.value)
    conflict.value=false;notice.value=''
  }
  function edit(value:AlarmSoundSettings){draft.value=normalizeAlarmSoundSettings(value);notice.value=''}
  function save(){
    if(busy.value){notice.value='音频正在校验，请完成后再保存。';return false}
    if(JSON.stringify(normalizeAlarmSoundSettings(source()))!==baseline.value)conflict.value=true
    if(conflict.value){notice.value='其他标签页已修改设置，请先放弃本页修改以载入最新设置，再重新编辑。';return false}
    if(!commit(draft.value)){notice.value='保存失败，请检查浏览器存储权限或空间后重试。';return false}
    const used=new Set(Object.values(draft.value.levels||{}).map(level=>level.customAudio?.id))
    for(const [id,owner] of staged)if(!used.has(id))remove(owner,id)
    staged.clear()
    baseline.value=JSON.stringify(draft.value);notice.value='已保存，声音和颜色设置已生效。'
    return true
  }
  async function uploadAudio(level:string,file:File){
    const owner=scope(),version=(versions[level]||0)+1;versions[level]=version
    uploading.value[level]=true;uploadError.value[level]='';notice.value=''
    try{
      const stored=await files.store(owner,file)
      if(disposed||owner!==scope()||versions[level]!==version){remove(owner,stored.id);return}
      staged.set(stored.id,owner)
      edit({...draft.value,levels:{...draft.value.levels,[level]:{...alarmLevelSettings(draft.value,level),tone:'custom',customAudio:stored}}})
    }catch(error){if(!disposed&&owner===scope()&&versions[level]===version)uploadError.value[level]=error instanceof Error?error.message:'音频上传失败，请重试。'}
    finally{if(!disposed&&owner===scope()&&versions[level]===version)uploading.value[level]=false}
  }
  function clearAudio(level:string){
    versions[level]=(versions[level]||0)+1;uploading.value[level]=false;uploadError.value[level]=''
    const old=alarmLevelSettings(draft.value,level)
    edit({...draft.value,levels:{...draft.value.levels,[level]:{...old,customAudio:undefined,tone:old.tone==='custom'?defaultAlarmFile(level):old.tone}}})
  }
  const stopScope=watch(scope,reset,{flush:'sync'})
  const stopSource=watch(source,()=>{
    const latest=JSON.stringify(normalizeAlarmSoundSettings(source()))
    if(latest===baseline.value)return
    if(pending.value){conflict.value=true;return}
    reset()
  },{deep:true})
  function dispose(){disposed=true;stopScope();stopSource();reset()}
  return {draft,dirty,pending,busy,notice,conflict,uploading,uploadError,edit,save,reset,uploadAudio,clearAudio,dispose}
}

import {inject,onBeforeUnmount,onMounted} from 'vue'
import {confirmAction,confirmDraftAction} from '../confirm'

export const pageLeaveGuardKey='gateway-page-leave-guard'
export type RegisterPageLeaveGuard=(guard:()=>Promise<boolean>)=>()=>void

export function usePageDraftGuard(isDirty:()=>boolean,discard:()=>void,save?:()=>Promise<unknown>,navigationSelector=''){
  const register=inject<RegisterPageLeaveGuard|undefined>(pageLeaveGuardKey,undefined)
  let pending:Promise<boolean>|undefined,disposed=false,replaying=false
  async function check(){
    if(!isDirty())return true
    if(save){
      const choice=await confirmDraftAction()
      if(disposed||choice===false)return false
      if(choice==='discard'){discard();return true}
      try{if(await save()===false)return false;return !isDirty()}catch{return false}
    }
    const leave=await confirmAction({title:'尚有未保存的修改',message:'离开后将放弃本页未保存的修改。',detail:'如需保留，请选择继续编辑，再保存配置。',confirmText:'放弃修改并离开',cancelText:'继续编辑',danger:true})
    if(leave)discard()
    return leave
  }
  function requestLeave(){if(!pending)pending=check().finally(()=>{pending=undefined});return pending}
  const unregister=register?.(requestLeave)
  // Guard explicit in-page navigation boundaries before their Vue click handler.
  // Replay only once, after save/discard succeeds; never replay detached targets.
  function navigationClick(event:MouseEvent){
    if(replaying||!navigationSelector||!isDirty())return
    const target=(event.target as Element)?.closest<HTMLElement>(navigationSelector)
    if(!target||target.matches('.active,[aria-selected="true"],:disabled'))return
    event.preventDefault();event.stopImmediatePropagation()
    if(pending)return
    void requestLeave().then(allowed=>{if(allowed&&!disposed&&target.isConnected){replaying=true;try{target.click()}finally{replaying=false}}})
  }
  function beforeUnload(event:BeforeUnloadEvent){if(isDirty()){event.preventDefault();event.returnValue=''}}
  onMounted(()=>{window.addEventListener('beforeunload',beforeUnload);document.addEventListener('click',navigationClick,true)})
  onBeforeUnmount(()=>{disposed=true;unregister?.();window.removeEventListener('beforeunload',beforeUnload);document.removeEventListener('click',navigationClick,true)})
  return requestLeave
}

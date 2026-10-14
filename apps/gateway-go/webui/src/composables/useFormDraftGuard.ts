import {ref} from 'vue'
import {usePageDraftGuard} from './usePageDraftGuard.ts'

/** Baselines are established only after reads or successful saves, never by browsing. */
export function useFormDraftGuard(read:()=>unknown,restore:(value:any)=>void,save:()=>Promise<unknown>,navigationSelector=''){
  const baseline=ref('')
  const markSaved=()=>{baseline.value=JSON.stringify(read())}
  const isDirty=()=>!!baseline.value&&JSON.stringify(read())!==baseline.value
  const requestLeave=usePageDraftGuard(isDirty,()=>restore(JSON.parse(baseline.value)),save,navigationSelector)
  return {markSaved,isDirty,requestLeave}
}

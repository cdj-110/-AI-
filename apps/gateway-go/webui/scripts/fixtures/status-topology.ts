import {createApp,ref,h} from 'vue'
import {createPinia} from 'pinia'
import StatusWorkspace from '../../src/components/StatusWorkspace.vue'
import '../../src/styles.css'
import '../../src/modules.css'
import '../../src/button-system.css'
import '../../src/mobile-responsive.css'
createApp({setup(){
  const params=new URLSearchParams(location.search)
  const canStatus=params.get('permission')!=='topology-only',canTopology=params.get('permission')!=='status-only'
  const view=ref<'status'|'topology'>(!canStatus||(canTopology&&params.has('topology'))?'topology':'status'),selected=ref('')
  return()=>h('main',{class:'page'},[
    h(StatusWorkspace,{active:view.value,canStatus,canTopology,canConfig:canTopology,canIncidents:canStatus,
      onNavigate:key=>{if(key==='status'||key==='topology'){view.value=key;const url=new URL(location.href);url.searchParams.delete('topology');if(key==='topology')url.searchParams.set('topology','');history.replaceState(null,'',url)}selected.value=key},
      onOpenDevice:s=>selected.value=JSON.stringify(s)}),
    h('output',{id:'selected'},selected.value),
  ])
}}).use(createPinia()).mount('#app')

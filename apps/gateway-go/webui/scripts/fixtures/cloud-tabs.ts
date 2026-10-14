import { createApp, h, ref } from 'vue'
import { createPinia } from 'pinia'
import CloudView from '../../src/components/CloudView.vue'
import '../../src/styles.css'
import '../../src/modules.css'
import '../../src/button-system.css'
createApp({setup(){
  const dirty = ref(false)
  return ()=>h('main', {class:'page',style:'padding:20px'}, [h(CloudView,{onDirty:(value:boolean)=>dirty.value=value}),h('output',{id:'dirty'},String(dirty.value))])
}}).use(createPinia()).mount('#app')

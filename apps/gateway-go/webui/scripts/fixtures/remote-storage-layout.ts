import {createApp,h} from 'vue'
import RemoteStorageView from '../../src/components/RemoteStorageView.vue'
import '../../src/styles.css'
import '../../src/modules.css'
import '../../src/button-system.css'
createApp({render:()=>h('main',{class:'page'},[h(RemoteStorageView,{canManage:!location.search.includes('readonly')})])}).mount('#app')

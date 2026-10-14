import {createApp,defineComponent,h,ref} from 'vue'
import RemoteStorageView from '../../src/components/RemoteStorageView.vue'
import GlobalConfirmDialog from '../../src/components/GlobalConfirmDialog.vue'
import '../../src/styles.css'
import '../../src/modules.css'
import '../../src/button-system.css'
import '../../src/confirm.css'
createApp(defineComponent({setup(){const panel=ref<InstanceType<typeof RemoteStorageView>|null>(null),notice=ref('');return()=>h('main',{style:'max-width:1450px;margin:24px auto;padding:20px'},[h('h1','远程数据库 · 隔离测试页面'),h('p','所有接口均为本地模拟，不连接 EG100。'),h('button',{onClick:async()=>{if(await panel.value?.requestLeave())notice.value='允许离开'}},'测试切换页面'),h('span',notice.value),h(RemoteStorageView,{ref:panel,canManage:!location.search.includes('readonly')}),h(GlobalConfirmDialog)])}})).mount('#app')

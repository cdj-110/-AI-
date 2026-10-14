import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import AlarmWindow from './components/AlarmWindow.vue'
import {api} from './api'
import './styles.css'
import './tree.css'
import './modules.css'
import './legacy-interactions.css'
import './protocol-tools.css'
import './button-system.css'
import './ai-assistant.css'
import './confirm.css'
import './mobile-responsive.css'
import './live-value-pulse.css'
import './point-value-detail.css'
import './point-value-chart.css'
import './point-table-overflow.css'
import './dlt645.css'
import './alarm-level-colors.css'

async function mountGateway(){
 let root:typeof App|typeof AlarmWindow=App
 if(new URLSearchParams(window.location.search).get('alarmPanel')==='1'){
  try{const status=await api<{license:{valid:boolean;restricted:boolean}}>('/api/license/status');if(status.license.valid&&!status.license.restricted)root=AlarmWindow}catch{/* 授权未知时交给待激活入口，不展示业务面板 */}
 }
 createApp(root).use(createPinia()).mount('#app')
}
void mountGateway()

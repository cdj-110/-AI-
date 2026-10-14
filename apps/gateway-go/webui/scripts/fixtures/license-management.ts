import {createApp} from 'vue'
import {createPinia} from 'pinia'
import App from '../../src/App.vue'
import '../../src/styles.css'
import '../../src/modules.css'
import '../../src/button-system.css'
import '../../src/mobile-responsive.css'

// Isolated local preview: no credentials and no requests to a real gateway.
const license={valid:true,restricted:false}
const summary={gatewayKey:'WK20265AF2F5C6',canReplace:true,license,authorization:{sn:'WK20265AF2F5C6',hardwareId:'3AF104FF59D83AB08D4880783218BA75',chipUsid:'EE38111700000000',licenseVersion:2,keyId:'wk-production-20260916-01',licenseSequence:4,issuedAt:'2026-09-17T04:00:14Z',expiresAt:'2036-09-14T04:00:14Z',features:['collection','forward','history','remote-storage','edge','alarms','cloud','remote-access','remote-ssh'],signed:true}}
window.fetch=async(input,options)=>{
 const path=new URL(typeof input==='string'?input:input instanceof URL?input.href:input.url,location.origin).pathname
 const method=options?.method||'GET'
 if(path==='/api/license/status')return Response.json({authenticated:true,license,gatewayKey:summary.gatewayKey})
 if(path==='/api/session')return Response.json({username:'样式预览',permissions:['cloud.manage','maintenance.view']})
 if(path==='/api/license/details')return Response.json(summary)
 if(path.startsWith('/api/activation'))return Response.json({message:'样式预览不会导入授权，请在实际网关中操作。'},{status:400})
 if(method!=='GET')return Response.json({message:'样式预览不保存修改。'},{status:400})
 if(path==='/api/config')return Response.json({content:JSON.stringify({gateway:{key:summary.gatewayKey},devices:[],resources:[],channels:[]})})
 return Response.json({})
}
const url=new URL(location.href);url.searchParams.set('page','maintenance');history.replaceState(null,'',url)
createApp(App).use(createPinia()).mount('#app')

import {createApp,h,reactive,ref} from 'vue'
import Form from '../../src/components/DeviceConfigForm.vue'
import '../../src/styles.css'
import '../../src/button-system.css'
import '../../src/mobile-responsive.css'
import '../../src/dlt645.css'
const device=reactive({deviceKey:'device-006',name:'采集设备1',protocol:'dlt645',address:'/dev/ttyS2',enabled:true,dlt645:{version:'auto',meterAddress:'010000000000',responseTimeoutMilliseconds:1500,retries:1,wakeupBytes:4,writePassword:'***',operatorCode:'***'}})
createApp({setup(){
  const readonly=ref(true),tested=ref(0)
  return()=>h('div',{class:'page',style:'padding:20px;max-width:1600px;margin:auto;box-sizing:border-box'},[
    h('button',{id:'mode',onClick:()=>readonly.value=!readonly.value},readonly.value?'切换配置':'切换运行'),
    h('output',{id:'config',style:'display:none'},JSON.stringify(device)),
    h('output',{id:'tested'},String(tested.value)),
    h(Form,{device,readonly:readonly.value,isForward:false,onTest:()=>tested.value++}),
  ])
}}).mount('#app')

import {createApp,h,ref} from 'vue'
import ElementPlus,{ElSwitch} from 'element-plus'
import DeviceCommandsPanel from '../../src/components/DeviceCommandsPanel.vue'
import 'element-plus/dist/index.css'
createApp({setup(){
 const panel=ref<InstanceType<typeof DeviceCommandsPanel>>(),caps=ref<{points:{identifier:string;dataType:string}[],available:boolean}>({points:[],available:false}),boolLive=ref(false),pending=ref<string[]>([])
 ;(window as any).openCommand=(metric:string)=>panel.value?.open(metric)
 ;(window as any).setBoolLive=(value:boolean)=>boolLive.value=value
 ;(window as any).refreshCommands=()=>panel.value?.refresh()
 return()=>h('main',[
  h('article',{class:'metric-card'},[h('h2','目标值'),caps.value.points.some(point=>point.identifier==='setpoint')?h('button',{onClick:()=>panel.value?.open('setpoint')},'下发'):null]),
  h('article',{class:'metric-card'},[h('h2','只读输入')]),
  h('article',{class:'metric-card boolean-card'},[h('h2','设备开关'),caps.value.points.some(point=>point.identifier==='bool-point'&&point.dataType==='BOOLEAN')?h(ElSwitch,{modelValue:boolLive.value,disabled:!caps.value.available||pending.value.includes('bool-point'),inlinePrompt:true,activeText:'开',inactiveText:'关',onChange:(value:boolean)=>panel.value?.toggle('bool-point',value)}):null]),
  h(DeviceCommandsPanel,{ref:panel,deviceId:'test-child',deviceName:'下发测试设备',onCapabilities:(value:any)=>caps.value=value,onPending:(value:string[])=>pending.value=value})
 ])
}}).use(ElementPlus).mount('#app')

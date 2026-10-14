import {createApp, h, ref, reactive} from 'vue'
import {createPinia} from 'pinia'
import Workspace from '../../src/components/DevicePointWorkspace.vue'
import '../../src/styles.css'
import '../../src/tree.css'
import '../../src/modules.css'
import '../../src/legacy-interactions.css'
import '../../src/protocol-tools.css'
import '../../src/button-system.css'
import '../../src/ai-assistant.css'
import '../../src/confirm.css'
import '../../src/mobile-responsive.css'
import '../../src/live-value-pulse.css'
import '../../src/point-value-detail.css'
import '../../src/point-value-chart.css'
import '../../src/point-table-overflow.css'
import '../../src/dlt645.css'

document.documentElement.dataset.gatewayDetail='device'
const points=reactive(['float64','uint64','string','object',''].map((type,index)=>({
  metric:'dlt645-P0'+(index+1),name:'模拟点位'+(index+1),dataType:'int64',
  scale:1,offset:0,decimals:2,unit:index===0?'kWh':'',function:3,access:'read',requestedDataType:'auto',
  dlt645:{dataIdentifier:'0000000'+index,itemKey:'',valueFormat:'bcd',dataLength:4,access:'read'},
  expectedType:type,
})))
createApp({setup(){
  const readonly=ref(true),tick=ref(0),protocol=ref('dlt645')
  setInterval(()=>tick.value++,100)
  return()=>h('div',{class:'page',style:'padding:20px'},[
    h('button',{id:'mode',onClick:()=>readonly.value=!readonly.value},readonly.value?'切换配置':'切换运行'),
    h('span',{id:'tick'},String(tick.value)),
    h('select',{id:'protocol',value:protocol.value,onChange:(event:Event)=>protocol.value=(event.target as HTMLSelectElement).value},
      ['dlt645','modbus-rtu','siemens-s7','opcua','iec104','iec61850','iec61850-goose','opcda'].map(code=>h('option',{value:code},code))),
    h('output',{id:'types',style:'display:none'},JSON.stringify(points.map(point=>point.dataType))),
    h('output',{id:'point-configs',style:'display:none'},JSON.stringify(points)),
    h(Workspace,{device:{id:'test',deviceKey:protocol.value,name:'点位类型展示验证',protocol:protocol.value,dlt645:{version:'auto'},points},
      points,readonly:readonly.value,isForward:false,saving:false,sourceDevices:[],sourceChannels:[],sourceResources:[],
      liveByMetric:new Map(points.map((point,index)=>[point.metric,{
        value:881640+tick.value,actualDataType:point.expectedType,quality:'GOOD',
        diagnostic:index===4?'DL/T 645 读取失败：测试错误':'DL/T 645-2007',
      }]))}),
  ])
}}).use(createPinia()).mount('#app')

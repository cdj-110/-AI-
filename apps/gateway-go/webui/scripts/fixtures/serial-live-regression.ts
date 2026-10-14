import {createApp,h,ref,reactive} from 'vue'
import {createPinia} from 'pinia'
import ResourceWorkspace from '../../src/components/ResourceWorkspace.vue'
import DevicePointWorkspace from '../../src/components/DevicePointWorkspace.vue'
import '../../src/styles.css'
import '../../src/modules.css'
import '../../src/legacy-interactions.css'
import '../../src/protocol-tools.css'
import '../../src/button-system.css'
import '../../src/mobile-responsive.css'
import '../../src/live-value-pulse.css'
import '../../src/point-value-detail.css'
import '../../src/point-table-overflow.css'

document.documentElement.dataset.configView='form'
document.documentElement.dataset.gatewayDetail='resource'
const resource=reactive({resourceKey:'rs485-1',name:'RS485-1',type:'serial',enabled:true,serial:{port:'/dev/ttyS1',baudRate:9600,dataBits:8,stopBits:1,parity:'none'}})
const channel={channelKey:'rtu-1',resourceKey:'rs485-1',name:'RS485-1 modbus-rtu',protocol:'modbus-rtu',role:'collect',collectIntervalMilliseconds:1000}
const point=reactive({metric:'mbrtu-P01',name:'噪声值',protocol:'modbus-rtu',function:3,register:0,quantity:1,dataType:'uint16',scale:0.1,offset:0,decimals:0,unit:'dB'})
const device={deviceKey:'device-001',channelKey:channel.channelKey,name:'采集设备1',protocol:'modbus-rtu',address:'/dev/ttyS1',slaveId:2,points:[point]}
createApp({setup(){
 const value=ref(43.3)
 ;(window as any).updateProbeValue=(next:number)=>{value.value=next}
 return()=>h('main',{class:'page',style:'padding:24px'},[
  h(ResourceWorkspace,{resource,channels:[channel],collectDevices:[device],forwardDevices:[],readonly:true,saving:false}),
  h(DevicePointWorkspace,{device,points:[point],channelName:channel.name,interfaceName:'/dev/ttyS1',readonly:true,isForward:false,saving:false,sourceDevices:[],sourceChannels:[],sourceResources:[],liveByMetric:new Map([[point.metric,{value:value.value,quality:'GOOD'}]])}),
 ])
}}).use(createPinia()).mount('#app')

import {createApp,h} from 'vue'
import {createPinia} from 'pinia'
import {createRouter,createWebHashHistory,RouterView} from 'vue-router'
import ElementPlus from 'element-plus'
import * as echarts from 'echarts'
import DeviceDetail from '../../src/views/DeviceDetail.vue'
import 'element-plus/dist/index.css'
import '../../src/assets/main.css'

const router=createRouter({history:createWebHashHistory(),routes:[{path:'/devices/:id',component:DeviceDetail}]})
;(window as any).goDevice=(id:string)=>router.push('/devices/'+id)
;(window as any).showTrendTip=()=>{
 const host=document.querySelector<HTMLDivElement>('.trend-chart')
 if(host)echarts.getInstanceByDom(host)?.dispatchAction({type:'showTip',seriesIndex:0,dataIndex:0})
}
;(window as any).trendState=()=>{
 const host=document.querySelector<HTMLDivElement>('.trend-chart')
 const chart=host&&echarts.getInstanceByDom(host)
 return {hostWidth:host?.clientWidth,hostHeight:host?.clientHeight,chartWidth:chart?.getWidth(),chartHeight:chart?.getHeight(),chartId:(chart as any)?.id,series:(chart?.getOption().series as any[])?.[0]?.data}
}
await router.push('/devices/with-data')
createApp({render:()=>h('main',{id:'detail-frame',style:{width:'1100px',padding:'20px'}},[h(RouterView)])}).use(createPinia()).use(router).use(ElementPlus).mount('#app')

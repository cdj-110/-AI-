<script setup lang="ts">
import {timeAxisFractions,contiguousSegments} from '../timeAxis.ts'
import { dataTypeLabel } from '../dataTypeLabels.ts'
import { computed, ref } from 'vue'
import { pointHistory, pointStatusKey } from '../pointHistory'
import type { PointStatus } from '../types'

const props=defineProps<{point:any;status?:PointStatus}>()
const emit=defineEmits<{close:[]}>()
const colors=['#1677ff','#16a34a','#f59e0b','#e11d48','#7c3aed','#0891b2','#ea580c','#64748b']
type HoveredChartPoint={label:string;color:string;value:number;at:string;x:number;y:number}
const hoveredChartPoint=ref<HoveredChartPoint>()
const protocolKey=computed(()=>String(props.status?.protocol||props.point.protocol||'').toLowerCase())
const protocolLabel=computed(()=>({
  'modbus-rtu':'Modbus RTU',dlt645:'DL/T 645','modbus-tcp':'Modbus TCP','modbus-tcp-slave':'Modbus TCP 从站',
  opcda:'OPC DA',opcua:'OPC UA','siemens-s7':'Siemens S7',
  iec104:'IEC 60870-5-104','iec104-server':'IEC104 服务端',
  iec61850:'IEC 61850 MMS','iec61850-mms-server':'IEC61850 MMS 服务端',
  'iec61850-goose':'IEC61850 GOOSE','iec61850-goose-publisher':'IEC61850 GOOSE 发布',
}[protocolKey.value]||String(props.status?.protocol||props.point.protocol||'未知协议')))
const pointTitle=computed(()=>String(props.point.name||props.status?.name||props.point.metric||'-'))
const pointMetric=computed(()=>String(props.point.metric||props.status?.metric||'-'))
const protocolAddress=computed(()=>{
  const point=props.point,key=protocolKey.value
  if(key.startsWith('modbus'))return{label:'寄存器地址',value:String(point.register??'-')}
  if(key==='dlt645')return{label:'表地址 / DI',value:`${point.dlt645?.meterAddress||'-'} / ${point.dlt645?.dataIdentifier||point.dlt645?.itemKey||'-'}`}
  if(key==='opcda')return{label:'Item ID',value:String(point.itemId||'-')}
  if(key==='opcua')return{label:'Node ID',value:String(point.nodeId||'-')}
  if(key==='siemens-s7')return{label:'点位地址',value:String(point.address||(String(point.area||'DB').toUpperCase()==='DB'?`DB${point.dbNumber??0}.${point.register??0}`:`${point.area||''}${point.register??0}`))}
  if(key==='iec104'||key==='iec104-server')return{label:'IOA',value:String(point.ioa??'-')}
  if(key.startsWith('iec61850'))return{label:'对象引用',value:String(point.objectRef||'-')}
  return{label:'采集地址',value:String(point.address||props.status?.address||'-')}
})
const historyKey=computed(()=>props.status?pointStatusKey(props.status):`${props.point.deviceKey||''}::${props.point.metric||''}`)
const samples=computed(()=>pointHistory(historyKey.value))
const arrayItems=computed(()=>Array.isArray(props.status?.value)?props.status!.value.map((value,index)=>({index,value})):[])
const fullValue=computed(()=>{
  const value=props.status?.value
  if(value===undefined||value===null)return '-'
  if(props.status?.valueText&&!Array.isArray(value))return props.status.valueText
  if(typeof value==='string')return value
  try{return JSON.stringify(value,null,2)}catch{return String(value)}
})
function numberValue(value:unknown){
  if(typeof value==='boolean')return value?1:0
  return typeof value==='number'&&Number.isFinite(value)?value:undefined
}
const chart=computed(()=>{
  const source=samples.value
  if(!source.length)return undefined
  const arrayLength=Math.min(8,source.reduce((max,item)=>Math.max(max,Array.isArray(item.value)?item.value.length:0),0))
  const count=arrayLength||1
  const series=Array.from({length:count},(_,index)=>({
    label:arrayLength?`[${index}]`:'值',color:colors[index%colors.length],values:source.map(item=>item.quality&&item.quality!=='GOOD'?undefined:numberValue(Array.isArray(item.value)?item.value[index]:item.value)),
  }))
  const numeric=series.flatMap(item=>item.values.filter((value):value is number=>value!==undefined))
  if(!numeric.length)return undefined
  let min=Math.min(...numeric),max=Math.max(...numeric)
  if(min===max){const padding=Math.abs(min)*.05||1;min-=padding;max+=padding}
  const left=62,right=18,top=18,bottom=38,width=760,height=280,plotWidth=width-left-right,plotHeight=height-top-bottom
  const fractions=timeAxisFractions(source.map(item=>item.at))
  const mapped=series.map(item=>{
    const candidates=item.values.map((value,index)=>value===undefined||!Number.isFinite(fractions[index])?undefined:{x:left+fractions[index]*plotWidth,y:top+(max-value)/(max-min)*plotHeight,value,at:source[index]?.at||'',label:item.label,color:item.color})
    return {...item,points:candidates.filter((point):point is HoveredChartPoint=>!!point),segments:contiguousSegments(candidates)}
  })
  const ticks=Array.from({length:5},(_,index)=>{const ratio=index/4;return{y:top+ratio*plotHeight,value:max-ratio*(max-min)}})
  const baseline=height-bottom
  const areaPoints=mapped.length===1&&mapped[0].segments.length===1&&mapped[0].points.length>1?`${mapped[0].points[0].x},${baseline} ${mapped[0].points.map(point=>`${point.x},${point.y}`).join(' ')} ${mapped[0].points.at(-1)!.x},${baseline}`:''
  return{width,height,left,right,top,bottom,baseline,plotWidth,plotHeight,min,max,series:mapped,ticks,areaPoints,start:source[0]?.at,end:source[source.length-1]?.at}
})
function timeLabel(value?:string){if(!value)return'-';const date=new Date(value);return Number.isNaN(date.getTime())?value:date.toLocaleTimeString('zh-CN',{hour12:false,hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3})}
function fullTimeLabel(value?:string){if(!value)return'-';const date=new Date(value);return Number.isNaN(date.getTime())?value:new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3,hour12:false}).format(date)}
function chartValue(value:number){return Number(value.toPrecision(8)).toString()}
function updateChartHover(event:PointerEvent){
  const current=chart.value,svg=event.currentTarget as SVGSVGElement
  if(!current)return
  const bounds=svg.getBoundingClientRect(),x=(event.clientX-bounds.left)/Math.max(1,bounds.width)*current.width,y=(event.clientY-bounds.top)/Math.max(1,bounds.height)*current.height
  let nearest:HoveredChartPoint|undefined,distance=Number.POSITIVE_INFINITY
  for(const series of current.series)for(const point of series.points){const next=(point.x-x)**2+(point.y-y)**2*.35;if(next<distance){distance=next;nearest=point}}
  hoveredChartPoint.value=nearest
}
function clearChartHover(){hoveredChartPoint.value=undefined}
const chartTooltipStyle=computed(()=>{const current=chart.value,point=hoveredChartPoint.value;if(!current||!point)return{};return{left:`${point.x/current.width*100}%`,top:`${point.y/current.height*100}%`}})
const chartTooltipFlipped=computed(()=>!!chart.value&&!!hoveredChartPoint.value&&hoveredChartPoint.value.x>chart.value.width*.72)
const chartTooltipBelow=computed(()=>!!chart.value&&!!hoveredChartPoint.value&&hoveredChartPoint.value.y<chart.value.height*.42)
function detailValue(value:unknown){if(typeof value==='string')return value;try{return JSON.stringify(value)}catch{return String(value)}}
</script>

<template>
  <div class="point-value-detail-modal" role="dialog" aria-modal="true" aria-label="实时值详情" @click.self="emit('close')">
    <section class="point-value-detail-dialog">
      <header><div class="point-value-detail-heading"><div class="point-value-eyebrow"><small>实时值详情</small><span class="point-protocol-tag">{{protocolLabel}}</span></div><h3 :title="pointTitle">{{pointTitle}}</h3><div class="point-value-identity"><span>标识符</span><code :title="pointMetric">{{pointMetric}}</code></div></div><button type="button" aria-label="关闭" @click="emit('close')">×</button></header>
      <div class="point-value-facts">
        <div class="fact-current"><span>当前值</span><strong :class="{bad:!!status?.error}">{{Array.isArray(status?.value)?`数组 [${status?.value.length}]`:fullValue}}</strong></div>
        <div class="fact-quality" :class="String(status?.quality||'').toLowerCase()"><span>质量</span><strong>{{status?.quality||'-'}}</strong></div>
        <div class="fact-type"><span>实际类型</span><strong><code class="point-type-tag" :title="status?.actualDataType||point.dataType">{{dataTypeLabel(status?.actualDataType||point.dataType)}}</code></strong></div>
        <div class="fact-time"><span>源时间</span><strong>{{timeLabel(status?.sourceTimestamp)}}</strong></div>
        <div class="fact-time receive"><span>采集时间</span><strong>{{timeLabel(status?.receiveTimestamp||status?.updatedAt)}}</strong></div>
        <div class="fact-diagnostic" :class="{bad:!!status?.error}"><span>状态 / 诊断</span><strong :title="status?.error||status?.diagnostic||'正常'">{{status?.error||status?.diagnostic||'正常'}}</strong></div>
      </div>
      <div class="point-value-chart-panel">
        <div class="point-value-panel-head"><div><h4>变化趋势</h4><p>当前浏览器会话最近 {{samples.length}} / 120 次更新</p></div><div class="point-value-chart-meta"><span v-if="point.unit" class="point-value-unit-chip">{{point.unit}}</span><div v-if="chart" class="point-value-legend"><span v-for="item in chart.series" :key="item.label"><i :style="{background:item.color}"/>{{item.label}}</span></div></div></div>
        <div v-if="chart" class="point-value-chart-canvas">
          <svg class="point-value-chart" :viewBox="`0 0 ${chart.width} ${chart.height}`" preserveAspectRatio="none" role="img" :aria-label="`${point.name||point.metric} 实时变化趋势`" @pointermove="updateChartHover" @pointerleave="clearChartHover">
            <g class="point-value-chart-grid" v-for="tick in chart.ticks" :key="tick.y"><line :x1="chart.left" :x2="chart.width-chart.right" :y1="tick.y" :y2="tick.y"/><text :x="chart.left-8" :y="tick.y+4">{{chartValue(tick.value)}}</text></g>
            <text v-if="point.unit" :x="chart.left" :y="13" class="axis-unit">单位：{{point.unit}}</text>
            <polygon v-if="chart.areaPoints" class="point-value-chart-area" :points="chart.areaPoints"/>
            <g v-for="item in chart.series" :key="item.label"><polyline v-for="(segment,segmentIndex) in item.segments" :key="segmentIndex" class="point-value-chart-line-shadow" :points="segment.map(point=>`${point.x},${point.y}`).join(' ')" :stroke="item.color"/><polyline v-for="(segment,segmentIndex) in item.segments" :key="segmentIndex" class="point-value-chart-line" :points="segment.map(point=>`${point.x},${point.y}`).join(' ')" :stroke="item.color"/><circle v-for="(sample,index) in item.points" :key="index" class="point-value-chart-dot" :cx="sample.x" :cy="sample.y" r="2.5" :style="{stroke:item.color}"/></g>
            <g v-if="hoveredChartPoint" class="point-value-chart-focus"><line class="vertical" :x1="hoveredChartPoint.x" :x2="hoveredChartPoint.x" :y1="chart.top" :y2="chart.baseline"/><line class="horizontal" :x1="chart.left" :x2="chart.width-chart.right" :y1="hoveredChartPoint.y" :y2="hoveredChartPoint.y"/><circle class="halo" :cx="hoveredChartPoint.x" :cy="hoveredChartPoint.y" r="8" :style="{fill:hoveredChartPoint.color}"/><circle class="core" :cx="hoveredChartPoint.x" :cy="hoveredChartPoint.y" r="4" :style="{fill:hoveredChartPoint.color}"/></g>
            <text :x="chart.left" :y="chart.height-10" class="time">{{timeLabel(chart.start)}}</text><text :x="chart.width-chart.right" :y="chart.height-10" class="time end">{{timeLabel(chart.end)}}</text>
          </svg>
          <div v-if="hoveredChartPoint" class="point-value-chart-tooltip" :class="{flipped:chartTooltipFlipped,below:chartTooltipBelow}" :style="chartTooltipStyle">
            <div class="point-value-chart-tooltip-title"><strong>{{point.name||point.metric}}</strong><code>{{point.metric}}{{chart.series.length>1?` · ${hoveredChartPoint.label}`:''}}</code></div>
            <div class="point-value-chart-tooltip-value"><strong>{{chartValue(hoveredChartPoint.value)}}</strong><span>{{point.unit||'未配置单位'}}</span></div>
            <time>{{fullTimeLabel(hoveredChartPoint.at)}}</time>
          </div>
        </div>
        <div v-else class="point-value-chart-empty">当前数据不是可绘制的数值，或尚未积累趋势样本。</div>
      </div>
      <div class="point-value-raw-panel">
        <div class="point-value-panel-head"><div><h4>{{arrayItems.length?'数组明细':'完整值'}}</h4><p v-if="arrayItems.length">共 {{arrayItems.length}} 个元素</p></div></div>
        <div v-if="arrayItems.length" class="point-array-detail"><table><thead><tr><th>索引</th><th>值</th></tr></thead><tbody><tr v-for="item in arrayItems" :key="item.index"><td>{{item.index}}</td><td><code>{{detailValue(item.value)}}</code></td></tr></tbody></table></div>
        <pre v-else>{{fullValue}}</pre>
        <p v-if="status?.error" class="point-value-error">{{status.error}}</p>
      </div>
      <footer><span>{{protocolLabel}} · {{protocolAddress.label}}：<code :title="protocolAddress.value">{{protocolAddress.value}}</code></span><button type="button" @click="emit('close')">关闭</button></footer>
    </section>
  </div>
</template>

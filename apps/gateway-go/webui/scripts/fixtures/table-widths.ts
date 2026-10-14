import {createApp,defineComponent,h,onBeforeUnmount,ref} from 'vue'
import {usePointTableInteractions} from '../../src/composables/usePointTableInteractions.ts'
import {useResizableTableColumns} from '../../src/composables/useResizableTableColumns.ts'

const schemas:Record<string,string[]>={
  modbus:['寄存器','数量'],s7:['区域','DB号'],opcua:['NodeId','数据类型'],
  iec61850:['对象引用','FC'],iec104:['IOA','点类型'],goose:['数据集索引','数据类型'],dlt645:['DI','数据项 / 格式'],
}
createApp(defineComponent({setup(){
  const root=ref<HTMLElement>(),protocol=ref('modbus'),pulse=ref(0),visible=ref(16),checked=ref(false),metric=ref('P01')
  usePointTableInteractions(root)
  const opcda=useResizableTableColumns(visible)
  const timer=setInterval(()=>pulse.value++,50)
  onBeforeUnmount(()=>clearInterval(timer))
  const common=['','#','名称','标识符','实时值']
  function cells(labels:string[],header:boolean){return labels.map((label,index)=>h(header?'th':'td',{
    class:index===0?'point-sheet-select':index===1?'point-sheet-index':index===2?'point-name-cell':index===3?'point-metric-cell':index===4?'point-live-value':'',
  },header?label:index===0?h('input',{type:'checkbox',checked:checked.value,onChange:(e:Event)=>checked.value=(e.target as HTMLInputElement).checked}):index===2?'很长的点位名称用于验证双击自适应宽度且实时刷新不重置':index===3?h('input',{value:metric.value,onInput:(e:Event)=>metric.value=(e.target as HTMLInputElement).value}):index===4?String(pulse.value):`value-${index}`))}
  return()=>h('main',{ref:root},[
    h('style',`body{font:14px Arial;margin:20px} th,td{box-sizing:border-box;height:34px;border:1px solid #ddd;overflow:hidden;white-space:nowrap}table{border-collapse:collapse;table-layout:fixed;width:1400px} .host{overflow:auto;width:1700px;margin:12px 0} .interactive-point-table{table-layout:fixed} button,select{margin:8px}`),
    h('select',{id:'protocol',value:protocol.value,onChange:(e:Event)=>protocol.value=(e.target as HTMLSelectElement).value},Object.keys(schemas).map(p=>h('option',{value:p},p))),
    h('button',{id:'outside',onPointerdown:(e:PointerEvent)=>e.stopPropagation()},'表格外'),h('output',{id:'pulse'},String(pulse.value)),
    h('div',{class:'host'},[h('table',{id:'generic',class:'point-sheet-table'},[
      h('thead',[h('tr',cells([...common,...schemas[protocol.value]],true))]),
      h('tbody',[h('tr',{'data-row-key':`${protocol.value}::P01`},cells([...common,...schemas[protocol.value]],false))]),
    ])]),
    h('button',{id:'advanced',onClick:()=>visible.value=visible.value===16?24:16},'高级配置'),
    h('div',{class:'host'},[h('table',{
      id:'opcda',ref:opcda.tableRef,class:'point-sheet-table resizable-opcda-table',style:opcda.tableStyle.value,
      onPointerdown:opcda.onPointerDown,onPointermove:opcda.onPointerMove,onPointerleave:opcda.onPointerLeave,onDblclick:opcda.onDoubleClick,onClick:opcda.onClick,
    },[
      h('colgroup',opcda.columnStyles.value.map(style=>h('col',{style}))),
      h('thead',[h('tr',cells([...common,...Array.from({length:visible.value-5},(_,i)=>`字段${i}`)],true))]),
      h('tbody',[h('tr',{'data-row-key':'opcda::P01'},cells(Array(visible.value).fill(''),false))]),
    ])]),
  ])
}})).mount('#app')

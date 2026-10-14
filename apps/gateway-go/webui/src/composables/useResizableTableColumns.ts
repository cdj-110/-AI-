import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { pointTableResizeHeader } from '../tableResizeHitArea.ts'

type ResizeState={index:number;startX:number;startWidth:number}
type ResizableTableOptions={onLiveValueClick?:(metric:string)=>void}

export const opcdaBaseColumnWidths=[38,46,240,150,150,92,176,176,104,150,164,104,78,120,130,110]
export const opcdaAdvancedColumnWidths=[180,164,164,76,90,90,96,82]
export const opcdaMinimumColumnWidths=[34,42,180,135,110,82,150,150,88,120,156,88,64,90,110,96,120,156,156,64,72,72,72,68]

export function useResizableTableColumns(visibleCount:Ref<number>,options:ResizableTableOptions={}){
  const tableRef=ref<HTMLTableElement>()
  const widths=ref([...opcdaBaseColumnWidths,...opcdaAdvancedColumnWidths])
  const selectedColumn=ref(-1)
  let selectedRowKey='',hoveredColumn=-1,hoveredRow:HTMLTableRowElement|undefined,hotHeader:HTMLTableCellElement|undefined,resizeState:ResizeState|undefined,ignoreNextClick=false
  let measureCanvas:HTMLCanvasElement|undefined
  let containerObserver:ResizeObserver|undefined,manuallySized=false

  const visibleWidths=computed(()=>widths.value.slice(0,visibleCount.value))
  const tableWidth=computed(()=>visibleWidths.value.reduce((sum,width)=>sum+width,0))
  const columnStyles=computed(()=>visibleWidths.value.map(width=>({width:`${width}px`})))
  const tableStyle=computed(()=>({width:`${tableWidth.value}px`,minWidth:`${tableWidth.value}px`}))

  function eventCell(event:Event){
    const target=event.target
    if(!(target instanceof Element))return undefined
    const cell=target.closest<HTMLTableCellElement>('th,td')
    return cell&&tableRef.value?.contains(cell)?cell:undefined
  }
  function paintColumn(index:number,className:string,active:boolean){
    const table=tableRef.value
    if(!table||index<0)return
    for(const row of Array.from(table.rows))row.cells[index]?.classList.toggle(className,active)
  }
  function setHoveredColumn(index:number){
    if(index===hoveredColumn)return
    paintColumn(hoveredColumn,'is-column-hovered',false)
    hoveredColumn=index
    paintColumn(hoveredColumn,'is-column-hovered',true)
  }
  function dataRow(cell?:HTMLTableCellElement){
    const row=cell?.parentElement
    return row instanceof HTMLTableRowElement&&row.parentElement?.tagName==='TBODY'&&row.dataset.rowKey?row:undefined
  }
  function setHoveredRow(row?:HTMLTableRowElement){
    if(hoveredRow===row)return
    hoveredRow?.classList.remove('is-row-hovered')
    hoveredRow=row
    hoveredRow?.classList.add('is-row-hovered')
  }
  function clearSelectedRow(){
    tableRef.value?.querySelectorAll('tbody tr.is-row-selected').forEach(row=>row.classList.remove('is-row-selected'))
  }
  function clearSelection(){
    clearSelectedRow()
    selectedRowKey=''
    paintColumn(selectedColumn.value,'is-column-selected',false)
    selectedColumn.value=-1
  }
  function onDocumentPointerDown(event:PointerEvent){
    if(event.button!==0||resizeState)return
    const table=tableRef.value,target=event.target
    if(!table||!(target instanceof Node)||table.contains(target))return
    clearSelection()
    onPointerLeave()
  }
  function paintSelectedRow(){
    if(!selectedRowKey)return
    for(const row of Array.from(tableRef.value?.tBodies[0]?.rows||[])){
      if(row.dataset.rowKey===selectedRowKey)row.classList.add('is-row-selected')
    }
  }
  function setHotHeader(header?:HTMLTableCellElement){
    if(hotHeader===header)return
    hotHeader?.classList.remove('is-resize-hot')
    hotHeader=header
    hotHeader?.classList.add('is-resize-hot')
  }
  function onPointerMove(event:PointerEvent){
    const cell=eventCell(event)
    if(cell?.tagName==='TH'){
      setHoveredRow(undefined)
      setHoveredColumn(cell.cellIndex)
    }else{
      setHoveredColumn(-1)
      setHoveredRow(dataRow(cell))
    }
    if(!resizeState)setHotHeader(cell?pointTableResizeHeader(event,cell):undefined)
  }
  function onPointerLeave(){
    if(!resizeState){setHoveredColumn(-1);setHoveredRow(undefined);setHotHeader(undefined)}
  }
  function minimumWidth(index:number){
    return opcdaMinimumColumnWidths[index]??56
  }
  function resizeMove(event:PointerEvent){
    if(!resizeState)return
    const next=Math.min(640,Math.max(minimumWidth(resizeState.index),resizeState.startWidth+event.clientX-resizeState.startX))
    const updated=[...widths.value]
    updated[resizeState.index]=Math.round(next)
    widths.value=updated
  }
  function resizeEnd(){
    if(!resizeState)return
    resizeState=undefined
    ignoreNextClick=true
    document.body.classList.remove('is-resizing-table-column')
    window.removeEventListener('pointermove',resizeMove)
    window.removeEventListener('pointerup',resizeEnd)
    window.removeEventListener('pointercancel',resizeEnd)
    window.setTimeout(()=>{ignoreNextClick=false},0)
  }
  function onPointerDown(event:PointerEvent){
    if(event.button!==0)return
    const cell=eventCell(event)
    const header=cell?pointTableResizeHeader(event,cell):undefined
    if(!header)return
    event.preventDefault()
    event.stopPropagation()
    manuallySized=true
    resizeState={index:header.cellIndex,startX:event.clientX,startWidth:widths.value[header.cellIndex]||header.getBoundingClientRect().width}
    document.body.classList.add('is-resizing-table-column')
    window.addEventListener('pointermove',resizeMove)
    window.addEventListener('pointerup',resizeEnd)
    window.addEventListener('pointercancel',resizeEnd)
  }
  function measuredCellWidth(cell:HTMLTableCellElement){
    const control=cell.querySelector<HTMLInputElement|HTMLSelectElement>('input,select')
    let text=''
    if(control instanceof HTMLSelectElement)text=control.selectedOptions[0]?.text||control.value
    else if(control instanceof HTMLInputElement&&control.type!=='checkbox')text=control.value||control.placeholder
    else text=cell.innerText.replace(/[▶◀]/g,'').trim()
    const source=control||cell,style=getComputedStyle(source)
    measureCanvas??=document.createElement('canvas')
    const context=measureCanvas.getContext('2d')
    if(!context)return Math.max(56,text.length*8+28)
    context.font=style.font||`${style.fontSize} ${style.fontFamily}`
    return Math.ceil(context.measureText(text).width)+(control instanceof HTMLSelectElement?42:28)
  }
  function autoFit(index:number){
    const table=tableRef.value
    if(!table)return
    manuallySized=true
    let fitted=minimumWidth(index)
    for(const row of Array.from(table.rows)){
      const cell=row.cells[index]
      if(cell)fitted=Math.max(fitted,measuredCellWidth(cell))
    }
    const updated=[...widths.value]
    updated[index]=Math.min(640,Math.max(minimumWidth(index),fitted))
    widths.value=updated
  }
  function onDoubleClick(event:MouseEvent){
    const cell=eventCell(event)
    const header=cell?pointTableResizeHeader(event,cell):undefined
    if(!header)return
    event.preventDefault()
    event.stopPropagation()
    autoFit(header.cellIndex)
  }
  function onClick(event:MouseEvent){
    if(ignoreNextClick)return
    const cell=eventCell(event)
    if(!cell)return
    if(cell.matches('td.point-live-value')){
      const row=dataRow(cell),key=row?.dataset.rowKey||'',separator=key.indexOf('::'),metric=separator>=0?key.slice(separator+2):key
      if(metric&&options.onLiveValueClick){options.onLiveValueClick(metric);return}
    }
    if(cell.tagName==='TH'){
      clearSelectedRow()
      selectedRowKey=''
      paintColumn(selectedColumn.value,'is-column-selected',false)
      selectedColumn.value=selectedColumn.value===cell.cellIndex?-1:cell.cellIndex
      paintColumn(selectedColumn.value,'is-column-selected',true)
      return
    }
    const row=dataRow(cell)
    if(!row)return
    paintColumn(selectedColumn.value,'is-column-selected',false)
    selectedColumn.value=-1
    const rowKey=row.dataset.rowKey||''
    clearSelectedRow()
    selectedRowKey=selectedRowKey===rowKey?'':rowKey
    paintSelectedRow()
  }
  function refreshColumnEffects(){
    void nextTick(()=>{
      paintColumn(hoveredColumn,'is-column-hovered',true)
      paintColumn(selectedColumn.value,'is-column-selected',true)
      paintSelectedRow()
    })
  }

  function fitBaseColumnsToContainer(){
    if(manuallySized)return
    const available=Math.max(1180,Math.floor(tableRef.value?.parentElement?.clientWidth||0))
    if(!available)return
    const baseTotal=opcdaBaseColumnWidths.reduce((sum,width)=>sum+width,0)
    const next=[...widths.value]
    if(available<=baseTotal){
      opcdaBaseColumnWidths.forEach((width,index)=>next[index]=width)
      widths.value=next
      return
    }
    const fixed=opcdaBaseColumnWidths[0]+opcdaBaseColumnWidths[1]
    const flexible=opcdaBaseColumnWidths.slice(2).reduce((sum,width)=>sum+width,0)
    const ratio=(available-fixed)/flexible
    next[0]=opcdaBaseColumnWidths[0]
    next[1]=opcdaBaseColumnWidths[1]
    for(let index=2;index<opcdaBaseColumnWidths.length;index++)next[index]=Math.round(opcdaBaseColumnWidths[index]*ratio)
    const difference=available-next.slice(0,opcdaBaseColumnWidths.length).reduce((sum,width)=>sum+width,0)
    next[opcdaBaseColumnWidths.length-1]+=difference
    widths.value=next
  }

  function observeContainer(){
    containerObserver?.disconnect()
    fitBaseColumnsToContainer()
    const host=tableRef.value?.parentElement
    if(host&&typeof ResizeObserver!=='undefined'){
      containerObserver=new ResizeObserver(fitBaseColumnsToContainer)
      containerObserver.observe(host)
    }
  }
  watch(tableRef,()=>void nextTick(observeContainer),{flush:'post'})
  onMounted(()=>document.addEventListener('pointerdown',onDocumentPointerDown,true))

  onBeforeUnmount(()=>{
    resizeEnd()
    containerObserver?.disconnect()
    document.removeEventListener('pointerdown',onDocumentPointerDown,true)
    document.body.classList.remove('is-resizing-table-column')
  })

  return{tableRef,columnStyles,tableStyle,onPointerMove,onPointerLeave,onPointerDown,onDoubleClick,onClick,refreshColumnEffects}
}

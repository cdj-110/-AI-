import { nextTick, onBeforeUnmount, watch, type Ref } from 'vue'
import { normalizePointTableWidths, pointTableColumnMinimum, pointTableSchemaSignature, type PointTableColumnDescriptor } from '../pointTableLayout.ts'
import { pointTableResizeHeader } from '../tableResizeHitArea.ts'

type ResizeState={table:HTMLTableElement;index:number;startX:number;startWidth:number}
type WidthState={signature:string;widths:number[]}
type PointTableInteractionOptions={onLiveValueClick?:(metric:string)=>void}

export function usePointTableInteractions(rootRef:Ref<HTMLElement|undefined>,options:PointTableInteractionOptions={}){
  const widthsByTable=new WeakMap<HTMLTableElement,WidthState>()
  let root:HTMLElement|undefined,observer:MutationObserver|undefined,resizeState:ResizeState|undefined
  let hoveredRow:HTMLTableRowElement|undefined,hoveredTable:HTMLTableElement|undefined,hoveredColumn=-1
  let selectedTable:HTMLTableElement|undefined,selectedColumn=-1,selectedRowKey='',hotHeader:HTMLTableCellElement|undefined
  let ignoreNextClick=false,refreshFrame=0,pulseFrame=0,measureCanvas:HTMLCanvasElement|undefined
  const pendingPulseCells=new Set<HTMLTableCellElement>(),pulseTimers=new Set<number>()

  function eventCell(event:Event){
    const target=event.target
    if(!(target instanceof Element))return undefined
    const cell=target.closest<HTMLTableCellElement>('th,td'),table=cell?.closest<HTMLTableElement>('table.point-sheet-table')
    if(!cell||!table||table.classList.contains('resizable-opcda-table')||!root?.contains(table))return undefined
    return{cell,table}
  }
  function dataRow(cell:HTMLTableCellElement){
    const row=cell.parentElement
    return row instanceof HTMLTableRowElement&&row.parentElement?.tagName==='TBODY'?row:undefined
  }
  function rowKey(row:HTMLTableRowElement){
    const metric=row.dataset.rowKey||row.querySelector<HTMLInputElement>('.point-metric-cell input')?.value||''
    return metric||`row:${row.sectionRowIndex}`
  }
  function rowMetric(row:HTMLTableRowElement){
    const input=row.querySelector<HTMLInputElement>('.point-metric-cell input')?.value
    if(input)return input
    const key=row.dataset.rowKey||''
    const separator=key.indexOf('::')
    return separator>=0?key.slice(separator+2):key
  }
  function paintColumn(table:HTMLTableElement|undefined,index:number,className:string,active:boolean){
    if(!table||index<0)return
    for(const row of Array.from(table.rows))row.cells[index]?.classList.toggle(className,active)
  }
  function setHoveredColumn(table?:HTMLTableElement,index=-1){
    if(table===hoveredTable&&index===hoveredColumn)return
    paintColumn(hoveredTable,hoveredColumn,'is-column-hovered',false)
    hoveredTable=table;hoveredColumn=index
    paintColumn(hoveredTable,hoveredColumn,'is-column-hovered',true)
  }
  function setHoveredRow(row?:HTMLTableRowElement){
    if(row===hoveredRow)return
    hoveredRow?.classList.remove('is-row-hovered')
    hoveredRow=row
    hoveredRow?.classList.add('is-row-hovered')
  }
  function setHotHeader(header?:HTMLTableCellElement){
    if(header===hotHeader)return
    hotHeader?.classList.remove('is-resize-hot')
    hotHeader=header
    hotHeader?.classList.add('is-resize-hot')
  }
  function columns(table:HTMLTableElement):PointTableColumnDescriptor[]{
    return Array.from(table.tHead?.rows[0]?.cells||[]).map(cell=>({label:cell.textContent?.trim()||'',className:cell.className}))
  }
  function resetAppliedWidths(table:HTMLTableElement){
    table.querySelector(':scope > colgroup[data-interactive-columns]')?.remove()
    table.style.removeProperty('width')
    table.style.removeProperty('min-width')
    table.classList.remove('interactive-point-table')
  }
  function ensureWidths(table:HTMLTableElement){
    const headers=Array.from(table.tHead?.rows[0]?.cells||[])
    const descriptors=columns(table),signature=pointTableSchemaSignature(descriptors),known=widthsByTable.get(table)
    if(known&&known.signature===signature&&known.widths.length===headers.length){
      const widths=normalizePointTableWidths(known.widths,descriptors)
      widthsByTable.set(table,{signature,widths})
      table.classList.add('interactive-point-table')
      table.parentElement?.classList.add('interactive-point-table-wrap')
      applyWidths(table,widths)
      return widths
    }
    resetAppliedWidths(table)
    const widths=normalizePointTableWidths(headers.map(cell=>cell.getBoundingClientRect().width),descriptors)
    if(!widths.length)return widths
    widthsByTable.set(table,{signature,widths})
    table.classList.add('interactive-point-table')
    table.parentElement?.classList.add('interactive-point-table-wrap')
    applyWidths(table,widths)
    return widths
  }
  function applyWidths(table:HTMLTableElement,widths:number[]){
    let group=table.querySelector<HTMLTableColElement>(':scope > colgroup[data-interactive-columns]')
    if(!group){
      group=document.createElement('colgroup')
      group.dataset.interactiveColumns='true'
      table.insertBefore(group,table.firstChild)
    }
    while(group.children.length<widths.length)group.appendChild(document.createElement('col'))
    while(group.children.length>widths.length)group.lastElementChild?.remove()
    widths.forEach((width,index)=>(group.children[index] as HTMLElement).style.width=`${width}px`)
    const total=widths.reduce((sum,width)=>sum+width,0)
    table.style.setProperty('width',`${total}px`)
    table.style.setProperty('min-width',`${total}px`)
  }
  function minimumWidth(table:HTMLTableElement,index:number){return pointTableColumnMinimum(index,columns(table)[index]||{label:'',className:''})}
  function resizeMove(event:PointerEvent){
    if(!resizeState)return
    const state=widthsByTable.get(resizeState.table),widths=[...(state?.widths||[])]
    if(!widths.length)return
    widths[resizeState.index]=Math.round(Math.min(640,Math.max(minimumWidth(resizeState.table,resizeState.index),resizeState.startWidth+event.clientX-resizeState.startX)))
    widthsByTable.set(resizeState.table,{signature:state?.signature||pointTableSchemaSignature(columns(resizeState.table)),widths})
    applyWidths(resizeState.table,widths)
  }
  function resizeEnd(){
    if(!resizeState)return
    resizeState=undefined;ignoreNextClick=true
    document.body.classList.remove('is-resizing-table-column')
    window.removeEventListener('pointermove',resizeMove)
    window.removeEventListener('pointerup',resizeEnd)
    window.removeEventListener('pointercancel',resizeEnd)
    window.setTimeout(()=>{ignoreNextClick=false},0)
  }
  function onPointerDown(event:PointerEvent){
    if(event.button!==0)return
    const found=eventCell(event)
    if(!found)return
    const header=pointTableResizeHeader(event,found.cell)
    if(!header)return
    const widths=ensureWidths(found.table)
    if(!widths.length)return
    event.preventDefault();event.stopPropagation()
    resizeState={table:found.table,index:header.cellIndex,startX:event.clientX,startWidth:widths[header.cellIndex]||header.getBoundingClientRect().width}
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
    else text=cell.innerText.trim()
    const source=control||cell,style=getComputedStyle(source)
    measureCanvas??=document.createElement('canvas')
    const context=measureCanvas.getContext('2d')
    if(!context)return Math.max(56,text.length*8+28)
    context.font=style.font||`${style.fontSize} ${style.fontFamily}`
    return Math.ceil(context.measureText(text).width)+(control instanceof HTMLSelectElement?42:28)
  }
  function onDoubleClick(event:MouseEvent){
    const found=eventCell(event)
    if(!found)return
    const header=pointTableResizeHeader(event,found.cell)
    if(!header)return
    event.preventDefault();event.stopPropagation()
    const widths=[...ensureWidths(found.table)]
    let fitted=minimumWidth(found.table,header.cellIndex)
    for(const row of Array.from(found.table.rows)){
      const cell=row.cells[header.cellIndex]
      if(cell)fitted=Math.max(fitted,measuredCellWidth(cell))
    }
    widths[header.cellIndex]=Math.min(640,fitted)
    widthsByTable.set(found.table,{signature:pointTableSchemaSignature(columns(found.table)),widths})
    applyWidths(found.table,widths)
  }
  function onPointerMove(event:PointerEvent){
    const found=eventCell(event)
    if(found)ensureWidths(found.table)
    if(found?.cell.tagName==='TH'){
      setHoveredRow(undefined)
      setHoveredColumn(found.table,found.cell.cellIndex)
    }else{
      setHoveredColumn(undefined,-1)
      setHoveredRow(found?dataRow(found.cell):undefined)
    }
    if(!resizeState)setHotHeader(found?pointTableResizeHeader(event,found.cell):undefined)
  }
  function onPointerLeave(){
    if(resizeState)return
    setHoveredRow(undefined);setHoveredColumn(undefined,-1);setHotHeader(undefined)
  }
  function clearSelected(){
    paintColumn(selectedTable,selectedColumn,'is-column-selected',false)
    root?.querySelectorAll('.point-sheet-table tbody tr.is-row-selected').forEach(row=>row.classList.remove('is-row-selected'))
  }
  function clearSelection(){
    clearSelected()
    selectedTable=undefined;selectedColumn=-1;selectedRowKey=''
  }
  function onDocumentPointerDown(event:PointerEvent){
    if(event.button!==0||resizeState||!selectedTable)return
    const target=event.target
    if(!(target instanceof Node)||selectedTable.contains(target))return
    clearSelection()
    onPointerLeave()
  }
  function paintSelected(){
    paintColumn(selectedTable,selectedColumn,'is-column-selected',true)
    if(!selectedTable||!selectedRowKey)return
    for(const row of Array.from(selectedTable.tBodies[0]?.rows||[]))if(rowKey(row)===selectedRowKey)row.classList.add('is-row-selected')
  }
  function onClick(event:MouseEvent){
    if(ignoreNextClick)return
    const found=eventCell(event)
    if(!found)return
    ensureWidths(found.table)
    if(found.cell.matches('td.point-live-value')){
      const row=dataRow(found.cell),metric=row?rowMetric(row):''
      if(metric&&options.onLiveValueClick){options.onLiveValueClick(metric);return}
    }
    if(found.cell.tagName==='TH'){
      const same=selectedTable===found.table&&selectedColumn===found.cell.cellIndex
      clearSelected();selectedRowKey='';selectedTable=same?undefined:found.table;selectedColumn=same?-1:found.cell.cellIndex
      paintSelected();return
    }
    const row=dataRow(found.cell)
    if(!row)return
    const key=rowKey(row),same=selectedTable===found.table&&selectedRowKey===key
    clearSelected();selectedColumn=-1;selectedTable=same?undefined:found.table;selectedRowKey=same?'':key
    paintSelected()
  }
  function refresh(){
    if(refreshFrame)return
    refreshFrame=requestAnimationFrame(()=>{
      refreshFrame=0
      if(selectedTable&&!root?.contains(selectedTable)){selectedTable=undefined;selectedColumn=-1;selectedRowKey='';return}
      if(selectedTable){
        ensureWidths(selectedTable)
      }
      paintSelected()
    })
  }
  function liveCellFromNode(node:Node){
    const element=node instanceof Element?node:node.parentElement
    if(!element)return undefined
    const direct=element.closest<HTMLTableCellElement>('td.point-live-value')
    if(direct)return direct
    if(!element.closest('.opcda-collection-time-column'))return undefined
    return element.closest('tr')?.querySelector<HTMLTableCellElement>('td.point-live-value')||undefined
  }
  function queueLivePulses(mutations:MutationRecord[]){
    for(const mutation of mutations){
      const target=liveCellFromNode(mutation.target)
      if(target)pendingPulseCells.add(target)
      if(mutation.type==='childList')for(const node of Array.from(mutation.addedNodes)){
        const cell=liveCellFromNode(node)
        if(cell)pendingPulseCells.add(cell)
        if(node instanceof Element)node.querySelectorAll<HTMLTableCellElement>('td.point-live-value').forEach(item=>pendingPulseCells.add(item))
      }
    }
    if(!pendingPulseCells.size||pulseFrame)return
    pulseFrame=requestAnimationFrame(()=>{
      pulseFrame=0
      const cells=Array.from(pendingPulseCells).filter(cell=>root?.contains(cell))
      pendingPulseCells.clear()
      cells.forEach(cell=>cell.classList.remove('is-live-update'))
      if(cells.length)void cells[0].offsetWidth
      cells.forEach(cell=>cell.classList.add('is-live-update'))
      const timer=window.setTimeout(()=>{
        cells.forEach(cell=>cell.classList.remove('is-live-update'))
        pulseTimers.delete(timer)
      },760)
      pulseTimers.add(timer)
    })
  }
  function detach(){
    if(!root)return
    document.removeEventListener('pointerdown',onDocumentPointerDown,true)
    clearSelection()
    onPointerLeave()
    root.removeEventListener('pointermove',onPointerMove)
    root.removeEventListener('pointerleave',onPointerLeave)
    root.removeEventListener('pointerdown',onPointerDown)
    root.removeEventListener('dblclick',onDoubleClick)
    root.removeEventListener('click',onClick)
    observer?.disconnect();observer=undefined;root=undefined
    pendingPulseCells.clear()
    if(pulseFrame){cancelAnimationFrame(pulseFrame);pulseFrame=0}
    pulseTimers.forEach(timer=>window.clearTimeout(timer));pulseTimers.clear()
  }
  function attach(next?:HTMLElement){
    detach();root=next
    if(!root)return
    document.addEventListener('pointerdown',onDocumentPointerDown,true)
    root.addEventListener('pointermove',onPointerMove)
    root.addEventListener('pointerleave',onPointerLeave)
    root.addEventListener('pointerdown',onPointerDown)
    root.addEventListener('dblclick',onDoubleClick)
    root.addEventListener('click',onClick)
    observer=new MutationObserver(mutations=>{refresh();queueLivePulses(mutations)})
    observer.observe(root,{childList:true,subtree:true,characterData:true})
  }

  watch(rootRef,value=>void nextTick(()=>attach(value)),{flush:'post'})
  onBeforeUnmount(()=>{
    detach();resizeEnd()
    if(refreshFrame)cancelAnimationFrame(refreshFrame)
    document.body.classList.remove('is-resizing-table-column')
  })
}

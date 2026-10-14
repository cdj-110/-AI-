export type PointTableColumnDescriptor={label:string;className:string}

const interactionClasses=new Set(['is-column-hovered','is-column-selected','is-resize-hot','is-row-hovered','is-row-selected','is-live-update'])

export function pointTableSchemaSignature(columns:PointTableColumnDescriptor[]){
  // Hover/selection/resize feedback is not a schema change. Including it here
  // invalidates the saved widths as soon as the pointer leaves the divider.
  return columns.map(column=>{
    const structuralClasses=[...new Set(column.className.split(/\s+/).filter(name=>name&&!interactionClasses.has(name)))].sort().join(' ')
    return `${column.label}\u001f${structuralClasses}`
  }).join('\u001e')
}

export function pointTableColumnMinimum(index:number,column:PointTableColumnDescriptor){
  if(index===0||column.className.includes('point-sheet-select'))return 34
  if(index===1||column.className.includes('point-sheet-index'))return 42
  if(column.className.includes('point-name-cell')||column.label==='名称')return 240
  if(column.className.includes('point-metric-cell')||column.label==='标识符')return 150
  if(column.className.includes('point-live')||column.label==='实时值')return 110
  if(column.className.includes('dlt645-type-column'))return 156
  if(column.className.includes('dlt645-access-column'))return 104
  if(column.className.includes('dlt645-unit-column'))return 88
  if(column.className.includes('point-sheet-actions')||column.label==='操作')return 104
  if(column.className.includes('point-address')||['NodeId','对象引用'].includes(column.label))return 220
  if(['数据类型','实际类型','请求类型','输出类型'].includes(column.label))return 164
  if(['功能码','点类型'].includes(column.label))return 96
  if(['寄存器','字节地址','字节序','数据集索引'].includes(column.label))return 88
  if(['数量','区域','DB号','IOA','FC','倍率','偏移','单位','小数位'].includes(column.label))return 72
  return 72
}

export function normalizePointTableWidths(measured:number[],columns:PointTableColumnDescriptor[]){
  return columns.map((column,index)=>{
    const width=Number.isFinite(measured[index])?Math.round(measured[index]):0
    return Math.max(pointTableColumnMinimum(index,column),width)
  })
}

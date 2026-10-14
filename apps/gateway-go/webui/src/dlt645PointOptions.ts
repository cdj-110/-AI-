export type DLT645Item = {
  key: string; name: string; category: string; di2007?: string; di1997?: string
  valueFormat: string; dataLength: number; decimals: number; dataType: string; unit?: string; access: string
}

// These output types match decodeDLT645Value; changing a type must also change
// the wire decoder, rather than merely changing the dataType metadata.
export const dlt645Formats = [
  {key:'bcd', label:'BCD', type:'float64', length:4},
  {key:'signed-bcd', label:'带符号 BCD', type:'float64', length:4},
  {key:'binary', label:'二进制整数（小端）', type:'uint64', length:4},
  {key:'ascii', label:'ASCII', type:'string', length:4},
  {key:'date', label:'日期', type:'string', length:4},
  {key:'time', label:'时间', type:'string', length:3},
  {key:'datetime', label:'日期时间', type:'string', length:7},
  {key:'demand-time', label:'需量及发生时间', type:'object', length:8},
  {key:'raw-hex', label:'原始十六进制', type:'string', length:4},
]
export const dlt645ValueTypes = ['float64', 'uint64', 'string', 'object']
const canonicalFormat = (format:string) => format === 'raw_hex' ? 'raw-hex' : format
export function dlt645ConfiguredType(point:any, items:DLT645Item[]) {
  if (point.dlt645?.itemKey) return items.find(item=>item.key===point.dlt645.itemKey)?.dataType || point.dataType || ''
  return dlt645Formats.find(format=>format.key===canonicalFormat(point.dlt645?.valueFormat))?.type || ''
}
export function dlt645Selection(point:any) {
  return point.dlt645?.itemKey ? 'item:'+point.dlt645.itemKey : 'format:'+canonicalFormat(point.dlt645?.valueFormat || 'bcd')
}
export function selectDLT645Option(point:any, selection:string, items:DLT645Item[], version='auto') {
  const options = point.dlt645 || (point.dlt645={})
  if (selection.startsWith('item:')) {
    const item = items.find(item=>item.key===selection.slice(5))
    if (!item || (version==='2007'&&!item.di2007) || (version==='1997'&&!item.di1997)) return
    Object.assign(options,{itemKey:item.key,dataIdentifier:'',valueFormat:item.valueFormat,dataLength:item.dataLength,access:item.access})
    Object.assign(point,{dataType:item.dataType,decimals:item.decimals,unit:item.unit||'',access:item.access})
    return
  }
  const format = dlt645Formats.find(format=>'format:'+format.key===selection)
  if (!format) return
  const oldItem = items.find(item=>item.key===options.itemKey)
  if (oldItem && !options.dataIdentifier) options.dataIdentifier = version==='1997' ? oldItem.di1997 : oldItem.di2007 || oldItem.di1997
  const changed = canonicalFormat(options.valueFormat)!==format.key
  Object.assign(options,{itemKey:'',valueFormat:format.key})
  if (changed || !options.dataLength) options.dataLength=format.length
  if (changed) point.decimals=0
  point.dataType=format.type
}
export function selectDLT645Type(point:any, type:string, items:DLT645Item[]) {
  if (point.dlt645?.itemKey || !dlt645ValueTypes.includes(type)) return
  if (dlt645ConfiguredType(point,items)===type) { point.dataType=type; return }
  const format = dlt645Formats.find(format=>format.type===type)!
  selectDLT645Option(point,'format:'+format.key,items)
}

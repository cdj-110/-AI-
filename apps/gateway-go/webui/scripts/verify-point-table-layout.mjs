import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {normalizePointTableWidths,pointTableSchemaSignature} from '../src/pointTableLayout.ts'
import {opcdaBaseColumnWidths,opcdaMinimumColumnWidths} from '../src/composables/useResizableTableColumns.ts'

const modbus=[
  ['','point-sheet-select'],['#','point-sheet-index'],['名称',''],['标识符',''],['实时值',''],
  ['功能码',''],['寄存器',''],['数量',''],['字节序',''],['数据类型',''],['倍率',''],['偏移',''],['单位',''],['小数位',''],['操作','point-sheet-actions'],
].map(([label,className])=>({label,className}))
const s7=modbus.map(column=>({...column}))
s7.splice(5,4,{label:'区域',className:''},{label:'DB号',className:''},{label:'字节地址',className:''},{label:'字节序',className:''})
const base=modbus.slice(0,5)
const tail=modbus.slice(9)
const protocolSchemas={
  modbus,
  s7,
  iec104:[...base,{label:'IOA',className:''},{label:'点类型',className:''},...tail],
  opcua:[...base,{label:'NodeId',className:'point-address-cell'},...tail],
  iec61850:[...base,{label:'对象引用',className:'point-address-cell'},{label:'FC',className:''},...tail],
  goose:[...base,{label:'数据集索引',className:''},{label:'数据类型',className:''},{label:'单位',className:''},{label:'小数位',className:''},{label:'操作',className:'point-sheet-actions'}],
  dlt645:[...base,{label:'DI',className:''},{label:'数据项 / 格式',className:''},...tail],
}

const repaired=normalizePointTableWidths([38,49,318,191,127,0,34,93,76,91,117,74,74,76,74],modbus)
assert.equal(repaired.length,15)
assert.ok(repaired[5]>=96,'功能码不能折叠为 0')
assert.ok(repaired[6]>=88,'寄存器列不能低于可读宽度')
assert.ok(repaired[14]>=104,'操作列需要保留按钮空间')
assert.notEqual(pointTableSchemaSignature(modbus),pointTableSchemaSignature(s7),'相同列数的不同协议必须使用不同列宽缓存')
const decorated=modbus.map(column=>({...column,className:`is-resize-hot ${column.className} is-column-hovered is-column-selected`}))
assert.equal(pointTableSchemaSignature(modbus),pointTableSchemaSignature(decorated),'悬浮、选择与拖动状态不能使列宽缓存失效')
assert.equal(pointTableSchemaSignature([{label:'名称',className:'point-name-cell compact'}]),pointTableSchemaSignature([{label:'名称',className:'compact  point-name-cell compact'}]),'样式顺序和重复不改变列结构')
assert.notEqual(pointTableSchemaSignature([{label:'地址',className:'point-address'}]),pointTableSchemaSignature([{label:'地址',className:'point-name-cell'}]),'真实布局变化仍需重新计算')
for(const [protocol,columns] of Object.entries(protocolSchemas)){
  const widths=normalizePointTableWidths(Array(columns.length).fill(0),columns)
  assert.equal(widths.length,columns.length,`${protocol} 列数必须保持一致`)
  assert.ok(widths.every(width=>width>=34),`${protocol} 不允许出现零宽度列`)
}
const signatures=Object.values(protocolSchemas).map(pointTableSchemaSignature)
assert.equal(new Set(signatures).size,signatures.length,'不同协议表头必须拥有独立列宽缓存')

assert.equal(opcdaBaseColumnWidths.length,16,'OPC DA 基础表格列数必须保持一致')
assert.ok(opcdaBaseColumnWidths.every((width,index)=>width>=(opcdaMinimumColumnWidths[index]??0)),'OPC DA 默认列宽不能小于控件安全宽度')
assert.ok(opcdaBaseColumnWidths[2]>=240,'OPC DA 名称列需要保留可读宽度')
assert.ok(opcdaBaseColumnWidths[3]>=150,'OPC DA 标识符列需要保留可读宽度')
assert.ok(opcdaBaseColumnWidths[5]>=92,'OPC DA 质量列必须完整容纳 UNCERTAIN')
assert.ok(opcdaBaseColumnWidths.reduce((sum,width)=>sum+width,0)>1500,'窄屏应使用横向滚动，不能再次压缩全部基础列')

const css=await readFile(new URL('../src/styles.css',import.meta.url),'utf8')
assert.ok(!/:has\(thead th:nth-child\(15\):last-child\)[^{]*nth-child\(6\)[^{]*\{display:none\}/.test(css),'不能再按总列数隐藏第 6 列')

const overflowCss=await readFile(new URL('../src/point-table-overflow.css',import.meta.url),'utf8')
assert.match(overflowCss,/td\.point-name-cell[\s\S]*?overflow:\s*hidden/,'名称单元格必须裁剪越界控件')
assert.match(overflowCss,/td\.point-metric-cell[\s\S]*?overflow:\s*hidden/,'标识符单元格必须裁剪越界控件')
assert.match(overflowCss,/\.opcda-quality\s*\{[\s\S]*?text-overflow:\s*ellipsis/,'质量徽标必须限制在质量列内')

console.log('point table layout verification passed')

for(const file of ['DevicePointWorkspace.vue','LargeDevicePointWorkspace.vue']){
  const source=await readFile(new URL(`../src/components/${file}`,import.meta.url),'utf8')
  assert.match(source,/from ['"]\.\.\/composables\/usePointTableInteractions\.ts['"]/,'生产页面必须加载修复后的 TypeScript 源码')
  assert.match(source,/from ['"]\.\.\/composables\/useResizableTableColumns\.ts['"]/,'OPC DA 页面不能误用同名旧 JS')
}

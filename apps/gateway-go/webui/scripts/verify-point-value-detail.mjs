import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {pointHistory,recordPointStatuses} from '../src/pointHistory.ts'

const base={deviceKey:'verify-device',name:'test',metric:'verify-point',protocol:'opcda',address:'',quality:'GOOD'}
recordPointStatuses([{...base,value:1,receiveTimestamp:'2026-08-31T00:00:00.000Z'}])
recordPointStatuses([{...base,value:2,receiveTimestamp:'2026-08-31T00:00:01.000Z'}])
recordPointStatuses([{...base,value:99,receiveTimestamp:'2026-08-31T00:00:01.000Z'}])
assert.deepEqual(pointHistory('verify-device::verify-point').map(item=>item.value),[1,2],'相同接收时间不能重复记录')

const arrayBase={...base,metric:'verify-array'}
recordPointStatuses([{...arrayBase,value:[1,2,3],receiveTimestamp:'2026-08-31T00:00:00.000Z'}])
assert.deepEqual(pointHistory('verify-device::verify-array')[0].value,[1,2,3],'数组值必须完整保留')

const limitBase={...base,metric:'verify-limit'}
for(let index=0;index<125;index++)recordPointStatuses([{...limitBase,value:index,receiveTimestamp:new Date(1_800_000_000_000+index*1000).toISOString()}])
assert.equal(pointHistory('verify-device::verify-limit').length,120,'趋势历史必须限制为 120 个样本')

const component=await readFile(new URL('../src/components/PointValueDetailDialog.vue',import.meta.url),'utf8')
assert.match(component,/变化趋势/)
assert.match(component,/Math\.min\(8,/,'数值数组趋势最多绘制 8 条线')
assert.match(component,/数组明细/)
assert.match(component,/sourceTimestamp/)
assert.match(component,/receiveTimestamp/)
assert.match(component,/point-value-chart-tooltip/,'实时值趋势支持结构化悬浮提示')
assert.match(component,/point\.unit\|\|'未配置单位'/,'悬浮提示包含点位单位')
assert.match(component,/fullTimeLabel\(hoveredChartPoint\.at\)/,'悬浮提示包含毫秒时间戳')
assert.match(component,/point-value-chart-focus/,'实时值趋势提供十字线与聚焦点')
assert.match(component,/fact-current/,'当前值使用独立语义层级')
assert.match(component,/fact-quality/,'质量状态支持语义色')
assert.match(component,/point-type-tag/,'实际类型使用紧凑标签展示')
assert.match(component,/point-value-identity/,'名称与标识符使用独立布局区域')
assert.doesNotMatch(component,/<footer><span>Item ID/,'页脚不能硬编码为 OPC DA Item ID')
assert.doesNotMatch(component,/协议详情/,'实时值弹窗不应展示协议参数区')
assert.match(component,/protocolAddress/,'页脚需要根据协议显示对应地址')
assert.match(component,/寄存器地址/,'Modbus 页脚显示寄存器地址')
assert.match(component,/Item ID/,'OPC DA 页脚显示 Item ID')
assert.match(component,/Node ID/,'OPC UA 页脚显示 Node ID')
assert.match(component,/对象引用/,'IEC61850 页脚显示对象引用')

console.log('point value detail verification passed')

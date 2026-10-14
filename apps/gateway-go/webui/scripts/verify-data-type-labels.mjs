import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {dataTypeLabel} from '../src/dataTypeLabels.ts'
import {dlt645Format} from '../src/dlt645Presentation.ts'
for (const bits of [8,16,32,64]) {
  assert.equal(dataTypeLabel('int'+bits),bits+'位有符号整数')
  assert.equal(dataTypeLabel('uint'+bits),bits+'位无符号整数')
}
for(const [code,label] of Object.entries({auto:'自动识别',bool:'布尔值',float32:'32位浮点数',float64:'64位浮点数',string:'字符串',datetime:'日期时间',timestamp:'时间戳',object:'结构化数据',quality:'质量码',currency:'货币数值','float64[]':'64位浮点数数组','string[]':'字符串数组'})){
  assert.equal(dataTypeLabel(code),label)
}
assert.equal(dataTypeLabel(null),'—')
assert.equal(dataTypeLabel('custom_type'),'custom_type')
assert.equal(dlt645Format('signed-bcd',4),'带符号 BCD · 4字节')
assert.equal(dlt645Format('bcd',0),'BCD')
for(const file of ['DevicePointWorkspace','LargeDevicePointWorkspace','ProtocolPointTools']){
  const source=await readFile(new URL('../src/components/'+file+'.vue',import.meta.url),'utf8')
  const options=[...source.matchAll(/<option ([^>]+)>\{\{dataTypeLabel\(type\)\}\}<\/option>/g)]
  assert.ok(options.length>0)
  for(const [,attributes] of options) {
    assert.match(attributes,/:value="type"/,'translated option must retain the original type code')
    assert.match(attributes,/:title="type"/)
  }
}
console.log('Chinese type labels, arrays, unknown types, DLT wire formats and unchanged option values passed')

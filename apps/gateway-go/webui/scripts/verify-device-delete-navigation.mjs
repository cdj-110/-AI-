import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'

const source=await readFile(
  new URL('../src/components/SmartGatewayView.vue',import.meta.url),
  'utf8',
)
const removeDevice=source.match(
  /async function removeDevice\(\)\{([\s\S]*?)\n\}/,
)?.[1]||''

assert.ok(removeDevice,'必须保留设备删除处理函数')
assert.match(removeDevice,/const parentChannelKey=channelKey\.value/,'删除前必须保留所属通道')
assert.match(removeDevice,/selectChannel\(parentChannelKey\)/,'删除后必须返回所属通道配置页')
assert.doesNotMatch(removeDevice,/ensureSelection\(\)/,'删除后不能自动选中通道内的其他设备')

console.log('device delete navigation verification passed')

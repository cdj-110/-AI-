import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'

const interactions=await readFile(new URL('../src/composables/usePointTableInteractions.ts',import.meta.url),'utf8')
const css=await readFile(new URL('../src/styles.css',import.meta.url),'utf8')
const pulseCss=await readFile(new URL('../src/live-value-pulse.css',import.meta.url),'utf8')

assert.match(interactions,/characterData:true/,'实时值文本变化必须被监听')
assert.match(interactions,/opcda-collection-time-column/,'OPC DA 同值采样应由采集时间触发')
assert.match(interactions,/pendingPulseCells/,'同一批 DOM 更新必须合并后统一触发')
assert.match(css,/\.is-live-update\{animation:point-live-background-pulse \.72s ease-out 1\}/,'呼吸效果必须是数据驱动的单次动画')
assert.doesNotMatch(css,/point-live-background-breathe 2s ease-in-out infinite/,'不能继续使用与数据无关的无限循环动画')
assert.match(pulseCss,/\.is-live-update[\s\S]*animation:[^;]+!important/,'数据到达动画在悬浮和选中状态下也必须可见')
assert.match(pulseCss,/filter: brightness/,'行列高亮覆盖背景时仍需保留可见脉冲')

console.log('live value pulse verification passed')

import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'

const app=await readFile(new URL('../src/App.vue',import.meta.url),'utf8')
const styles=await readFile(new URL('../src/ai-assistant.css',import.meta.url),'utf8')

assert.doesNotMatch(app,/\{ key:'ai', label:'AI 助手'/,'AI 助手不能继续占用主导航')
assert.doesNotMatch(app,/active==='ai'/,'AI 助手不能继续作为整页导航状态')
assert.match(app,/class="ai-floating-button"/,'需要右下角 AI 悬浮按钮')
assert.match(app,/class="ai-assistant-modal"/,'点击悬浮按钮需要打开弹窗')
assert.match(app,/v-if="aiMounted" v-show="aiOpen"/,'关闭弹窗后应保留 AI 对话组件状态')
assert.match(app,/handleAIEscape/,'AI 弹窗需要支持 Esc 关闭')
assert.match(styles,/\.ai-floating-button\{position:fixed/,'AI 按钮需要固定悬浮')
assert.match(styles,/bottom:78px/,'移动端悬浮按钮需要避开底部导航')

console.log('AI floating dialog verification passed')

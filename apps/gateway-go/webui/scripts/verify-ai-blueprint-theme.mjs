import assert from 'node:assert/strict'
import {access, readFile} from 'node:fs/promises'

const component=await readFile(new URL('../src/components/AiAssistantView.vue',import.meta.url),'utf8')
const app=await readFile(new URL('../src/App.vue',import.meta.url),'utf8')
const styles=await readFile(new URL('../src/ai-assistant.css',import.meta.url),'utf8')

assert.match(component,/@tabler\/icons-vue/,'AI 助手交互图标应来自统一图标库')
assert.match(component,/class="ai-composer-field"/,'输入区需要显示技术化指令标签')
assert.match(component,/PROMPT · 输入指令/,'输入区需要明确的指令提示')
assert.match(component,/IconMessageChatbot/,'智能对话页签需要语义图标')
assert.match(component,/IconSend2/,'发送操作需要语义图标')
assert.match(app,/IconSparkles/,'悬浮入口需要使用统一图标库')
assert.match(app,/IconX/,'弹窗关闭操作需要使用统一图标库')
assert.match(styles,/ai-blueprint-surface\.png/,'AI 弹窗需要加载蓝图背景资源')
assert.match(styles,/\.ai-assistant-window \.ai-tabs\{display:grid/,'两个核心能力页签需要保持等宽布局')
assert.match(styles,/\.ai-composer-field/,'输入区需要有独立层级')
await access(new URL('../public/assets/ai-blueprint-surface.png',import.meta.url))

console.log('AI blueprint theme verification passed')

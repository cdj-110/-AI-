const fs = require('node:fs'), path = require('node:path'), vm = require('node:vm'), assert = require('node:assert/strict')
const ts = require('typescript'), vue = require('vue'), { parse, compileScript } = require('@vue/compiler-sfc')
const root = path.resolve(__dirname, '..')
const compile = source => ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
const helper = { exports: {}, require: () => vue }
vm.runInNewContext(compile(fs.readFileSync(path.join(root, 'src/utils/eventAttributePreview.ts'), 'utf8')), helper)
const api = helper.exports
const child = { id: 'line', isGroup: false, attr: { w: 300, h: 5 }, option: { borderColor: '#123456', chuxi: 5 }, styles: { opacity: 1 }, chartConfig: { title: '线-01' } }
const group = { id: 'group', isGroup: true, groupList: [child] }
const store = vue.reactive({ getComponentList: [group], getCurrentCanvasTitle: '画布1' })
const original = JSON.stringify(store.getComponentList)
const props = vue.reactive({ show: false, eventItem: { form: { targetComponent: 'line', saveAttrArr: {} } } })
const emitted = [], cleanup = []
const file = path.join(root, 'src/views/chart/ContentConfigurations/components/ChartEvent/AttrChangeDrawer.vue')
const descriptor = parse(fs.readFileSync(file, 'utf8')).descriptor
compileScript(descriptor, { id: 'attribute-preview-test' })
const context = { exports: {}, Symbol,
  window: { $message: { warning() {} } },
  defineProps: () => props,
  defineEmits: () => (name, value) => { emitted.push({ name, value }); if (name === 'update:show') props.show = value },
  require: name => {
    if (name === 'vue') return { ...vue, onBeforeUnmount: callback => cleanup.push(callback) }
    if (name.includes('eventAttributePreview')) return api
    if (name.includes('useTargetData')) return { useTargetData: () => ({ chartEditStore: store }) }
    if (name.includes('utils')) return { deepClone: value => structuredClone(vue.toRaw(value)) }
    return {}
  }
}
// 模拟 JSON 数据深拷贝，与实际配置类型保持一致。
context.require = ((read) => name => name.includes('utils/utils')
  ? { deepClone: value => JSON.parse(JSON.stringify(value)) } : read(name))(context.require)
vm.runInNewContext(compile(descriptor.scriptSetup.content + '\nexports.testing = { localData, onSave };'), context)
const drawer = context.exports.testing
async function open() { props.show = true; await vue.nextTick() }
async function main() {
  await open()
  const previewKey = api.eventPreviewRenderKey('line')
  drawer.localData.value.option.borderColor = '#ff0000'
  drawer.localData.value.option.chuxi = 12
  drawer.localData.value.styles.opacity = .4
  await vue.nextTick()
  const preview = api.eventPreviewComponents(store.getComponentList)[0].groupList[0]
  assert.equal(preview.option.borderColor, '#ff0000'); assert.equal(preview.option.chuxi, 12)
  assert.equal(preview.styles.opacity, .4); assert.equal(api.eventPreviewRenderKey('line'), previewKey)
  assert.equal(JSON.stringify(store.getComponentList), original, '实时预览不能污染持久化配置')
  props.show = false; await vue.nextTick()
  assert.equal(api.eventPreviewComponents(store.getComponentList)[0], store.getComponentList[0])
  assert.equal(api.eventPreviewRenderKey('line'), 'line')
  props.eventItem.form.saveAttrArr = { option: { borderColor: '#00ff00', chuxi: 8 } }
  await open()
  assert.equal(drawer.localData.value.option.borderColor, '#00ff00', '重新编辑应显示事件已保存的属性')
  drawer.localData.value.option.chuxi = 18
  drawer.onSave(); await vue.nextTick()
  const saved = emitted.find(item => item.name === 'save').value.snapshot
  assert.equal(saved.option.chuxi, 18)
  assert.equal(JSON.stringify(store.getComponentList), original)
  assert.equal(api.eventPreviewRenderKey('line'), 'line')
  await open(); store.getCurrentCanvasTitle = '画布2'; await vue.nextTick()
  assert.equal(props.show, false); assert.equal(api.eventPreviewRenderKey('line'), 'line')
  await open(); cleanup.forEach(callback => callback())
  assert.equal(api.eventPreviewRenderKey('line'), 'line')
  console.log('PASS: live nested-component preview, stable render references, source isolation, cancel, event snapshot save/reopen, canvas switch and unmount cleanup')
}
main().catch(error => { console.error(error); process.exitCode = 1 })

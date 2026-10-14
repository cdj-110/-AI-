const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
function load(file) {
  const result = { exports: {} }
  new Function('exports', 'module', ts.transpileModule(fs.readFileSync(path.resolve(__dirname, '../src/utils', file), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText)(result.exports, result)
  return result.exports
}
const { resizedComponentBounds: resize, componentResizePolicy: policy } = load('componentResize.ts')
const { resizedContentPosition: position } = load('contentResize.ts')
const { captureTextResize: text } = load('textResize.ts')
const { restoreComponentSnapshot: restore } = load('restoreComponentSnapshot.ts')
const original = { x: 100, y: 200, w: 200, h: 100, contentW: 200, contentH: 100 }
const keys = ['TextCommon', 'Number', 'Clock', 'CirclePoint', 'CircleFun', 'Rect', 'Image', 'BarCommon', 'TablesBasic', 'TablesAdvanced']
for (const key of keys) {
  const component = { key, chartConfig: { key, package: key.startsWith('Tables') ? 'Tables' : key === 'BarCommon' ? 'Charts' : 'BasicShapes' } }
  for (const point of ['lt', 'rt', 'lb', 'rb', 'l', 'r', 't', 'b']) {
    for (const mods of [{}, { shiftKey: true }, { ctrlKey: true }, { ctrlKey: true, shiftKey: true }, { metaKey: true }]) {
      const next = resize(original, point, 40, 30, mods, component)
      assert.equal(next.scaleContent, !(mods.ctrlKey || mods.metaKey))
      if (point.includes('l')) assert.equal(next.x + next.w, original.x + original.w)
      else assert.equal(next.x, original.x)
      if (point.includes('t')) assert.equal(next.y + next.h, original.y + original.h)
      else assert.equal(next.y, original.y)
      if (point.length === 2 && (mods.shiftKey || next.scaleContent && policy(component).circular)) assert.ok(Math.abs(next.w / next.h - 2) < 1e-10)
      const offset = position(original, next, next.scaleContent)
      if (!next.scaleContent) {
        assert.equal(next.x + offset.contentX, original.x)
        assert.equal(next.y + offset.contentY, original.y)
      }
      assert.ok(next.w > 0 && next.h > 0)
    }
  }
}
assert.equal(resize(original, 'r', 40, 0, {}).w, 240)
assert.equal(resize(original, 'r', 40, 0, {}).h, 100)
assert.equal(resize(original, 'b', 0, 30, { shiftKey: true }).w, 200)
assert.equal(resize(original, 'rb', -10000, -10000, {}, { chartConfig: { package: 'Tables', key: 'TablesBasic' } }).w, 160)
assert.equal(resize({ ...original, w: 20, h: 20, contentW: 20, contentH: 20 }, 'rb', -10, -10, {}, { chartConfig: { package: 'Tables' } }).w, 20)
const label = { key: 'TextCommon', option: { fontSize: 20, dataset: 'unchanged' } }, captured = text(label)
captured(2, 2, resize(original, 'rb', 200, 100, {}, label).scaleContent); assert.equal(label.option.fontSize, 40)
captured(2, 2, resize(original, 'rb', 200, 100, { ctrlKey: true }, label).scaleContent); assert.equal(label.option.fontSize, 20)
const chart = { key: 'BarCommon', option: { fontSize: 12 } }, table = { key: 'TablesAdvanced', option: { style: { fontSize: 16 } } }
text(chart)(2, 2, true); text(table)(2, 2, true)
assert.equal(chart.option.fontSize, 12); assert.equal(table.option.style.fontSize, 16)
const current = { attr: { w: 400, contentX: 0 }, option: { fontSize: 40, lineVector: { x: 300 }, style: { fontSize: 16 } } }
const attrRef = current.attr, optionRef = current.option, styleRef = current.option.style
restore(current, { attr: { w: 200 }, option: { fontSize: 20, style: { fontSize: 16 } } })
assert.equal(current.attr, attrRef); assert.equal(current.option, optionRef); assert.equal(current.option.style, styleRef)
assert.equal(current.option.fontSize, 20); assert.equal(current.option.lineVector, undefined); assert.equal(current.attr.contentX, undefined)
// 执行真实拖动处理器，验证松手坐标刷新、取消、历史与清理，而非只测几何函数。
const vm = require('node:vm'), listeners = new Map(), frames = new Map(), records = []
const store = { getEditCanvas: { scale: .5, lockScale: false }, setEditCanvas(key, value) { this.getEditCanvas[key] = value }, setMousePosition() {} }
const dependencies = {
  '@/utils/componentResize': load('componentResize.ts'), '@/utils/contentResize': load('contentResize.ts'), '@/utils/textResize': load('textResize.ts'),
  '@/store/modules/chartEditStore/chartEditStore': { useChartEditStore: () => store },
  '@/store/modules/chartHistoryStore/chartHistoryStore': { useChartHistoryStore: () => ({ createUpdateHistory: data => records.push(data) }) },
  '@/views/chart/hooks/useContextMenu.hook': { useContextMenu: () => ({}) },
  '@/store/modules/chartEditStore/chartEditStore.d': { EditCanvasTypeEnum: { IS_DRAG: 'isDrag' } },
  lodash: { cloneDeep: value => JSON.parse(JSON.stringify(value)) }
}
const handler = { exports: {}, require: name => dependencies[name] || {}, HTMLElement: class {},
  document: { activeElement: null, addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name) },
  requestAnimationFrame: fn => { frames.set(1, fn); return 1 }, cancelAnimationFrame: id => frames.delete(id) }
vm.runInNewContext(ts.transpileModule(fs.readFileSync(path.resolve(__dirname, '../src/views/chart/ContentEdit/hooks/useDrag.hook.ts'), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, handler)
const target = { id: 'test', key: 'TextCommon', status: { lock: false }, attr: { ...original }, option: { fontSize: 20 } }
const down = { button: 0, screenX: 100, screenY: 100, preventDefault() {}, stopPropagation() {} }
handler.exports.useMousePointHandle(down, 'rb', target.attr, target)
listeners.get('mousemove')({ screenX: 125, screenY: 110 })
listeners.get('mouseup')({ screenX: 150, screenY: 125 })
assert.equal(target.attr.w, 300); assert.equal(target.attr.contentW, 300); assert.equal(target.option.fontSize, 30)
assert.equal(records.length, 1); assert.equal(records[0][0].attr.w, 200); assert.equal(listeners.size, 0); assert.equal(frames.size, 0); assert.equal(store.getEditCanvas.lockScale, false)
const saved = JSON.stringify(target)
handler.exports.useMousePointHandle(down, 'lt', target.attr, target)
listeners.get('mousemove')({ screenX: 70, screenY: 70, ctrlKey: true }); frames.get(1)()
listeners.get('keydown')({ key: 'Escape' })
assert.equal(JSON.stringify(target), saved); assert.equal(records.length, 1); assert.equal(listeners.size, 0)
handler.exports.useMousePointHandle({ ...down, button: 2 }, 'rb', target.attr, target); assert.equal(listeners.size, 0)
console.log('PASS: 10 component types × 8 handles × 5 modifier modes; opposite-edge anchors, circle/Shift ratio, minimum sizes, legacy sizes, Ctrl world position, text scaling and chart/table font preservation')

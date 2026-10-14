const fs = require('node:fs'), path = require('node:path'), ts = require('typescript'), vm = require('node:vm'), assert = require('node:assert/strict')
const root = path.resolve(__dirname, '..')
const compile = source => ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
const helper = { exports: {} }
vm.runInNewContext(compile(fs.readFileSync(path.join(root, 'src/utils/figureLine.ts'), 'utf8')), helper)
const api = helper.exports, close = (a, b) => assert.ok(Math.abs(a - b) < 1e-7, `${a} != ${b}`)
const old = { attr: { x: 20, y: 30, w: 300, h: 80, contentW: 300, contentH: 80 }, option: { chuxi: 5, borderColor: '#123456' } }
const before = JSON.stringify(old), legacy = api.renderedLine(old.attr, old.option)
assert.equal(legacy.start.y, 2.5); assert.equal(legacy.end.x, 300); assert.equal(JSON.stringify(old), before)
for (const cap of ['butt', 'round']) for (const arrows of [false, true]) for (const delta of [{ x: 300, y: 0 }, { x: 0, y: 300 }, { x: -200, y: 125 }, { x: 100, y: -100 }]) {
  const start = { x: -20, y: 100 }, end = { x: start.x + delta.x, y: start.y + delta.y }
  const component = { attr: {}, option: { chuxi: 5, lineCap: cap, startArrow: arrows, endArrow: arrows } }
  api.setLineEndpoints(component, start, end)
  const result = api.lineWorldPoints(component.attr, component.option)
  close(result.start.x, start.x); close(result.start.y, start.y); close(result.end.x, end.x); close(result.end.y, end.y)
  assert.ok(component.attr.w >= 1 && component.attr.h >= 1)
  const local = api.renderedLine(component.attr, component.option)
  for (const point of [local.start, local.end]) assert.ok(point.x >= 0 && point.y >= 0 && point.x <= local.width && point.y <= local.height)
  const saved = JSON.parse(JSON.stringify(component))
  close(api.lineWorldPoints(saved.attr, saved.option).end.x, end.x)
}
for (const point of [{ x: 100, y: 3 }, { x: 4, y: 90 }, { x: -90, y: 80 }, { x: 30, y: -80 }]) {
  const snapped = api.constrainLineEnd({ x: 0, y: 0 }, point, true)
  const angle = Math.atan2(snapped.y, snapped.x) / (Math.PI / 4)
  close(angle, Math.round(angle)); close(Math.hypot(snapped.x, snapped.y), Math.hypot(point.x, point.y))
}
assert.equal(api.lineStroke({ chuxi: -3 }), .5); assert.equal(api.lineStroke({ chuxi: 500 }), 100)
const listeners = new Map(), frames = new Map(), records = [], cleanups = []
const store = { getEditCanvas: { scale: .5, lockScale: false, isDrag: false }, setTargetSelectChart() {}, setEditCanvas(key, value) { this.getEditCanvas[key] = value } }
const hook = { exports: {}, document: { activeElement: { blur() {} }, addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name) },
  requestAnimationFrame: fn => { frames.set(1, fn); return 1 }, cancelAnimationFrame: id => frames.delete(id),
  require: name => {
    if (name === 'vue') return { onBeforeUnmount: fn => cleanups.push(fn) }
    if (name.includes('cloneDeep')) return { default: value => JSON.parse(JSON.stringify(value)) }
    if (name.includes('figureLine')) return api
    if (name.includes('chartHistoryStore')) return { useChartHistoryStore: () => ({ createUpdateHistory: data => records.push(data) }) }
    if (name.endsWith('chartEditStore.d')) return { EditCanvasTypeEnum: { LOCK_SCALE: 'lockScale', IS_DRAG: 'isDrag' } }
    return { useChartEditStore: () => store }
  }
}
vm.runInNewContext(compile(fs.readFileSync(path.join(root, 'src/views/chart/ContentEdit/hooks/useFigureEndpoints.hook.ts'), 'utf8')), hook)
const drag = hook.exports.useFigureEndpoints(), target = JSON.parse(before)
const down = { button: 0, clientX: 100, clientY: 100, preventDefault() {}, stopPropagation() {} }
drag(down, 'end', target)
listeners.get('mousemove')({ clientX: 150, clientY: 125, shiftKey: false })
listeners.get('mouseup')({ clientX: 130, clientY: 120, shiftKey: false })
const moved = api.lineWorldPoints(target.attr, target.option)
close(moved.start.x, 20); close(moved.start.y, 32.5); close(moved.end.x, 380); close(moved.end.y, 72.5)
assert.equal(records.length, 1); assert.equal(listeners.size, 0); assert.equal(frames.size, 0)
assert.equal(store.getEditCanvas.lockScale, false)
const afterDrag = JSON.stringify(target)
drag(down, 'start', target); listeners.get('mousemove')({ clientX: 60, clientY: 50, shiftKey: true }); frames.get(1)()
listeners.get('keydown')({ key: 'Escape' })
assert.equal(JSON.stringify(target), afterDrag); assert.equal(records.length, 1)
drag(down, 'end', target); cleanups.forEach(fn => fn())
assert.equal(listeners.size, 0); assert.equal(store.getEditCanvas.isDrag, false)
const { parse, compileScript } = require('@vue/compiler-sfc')
for (const file of ['src/packages/components/BasicShapes/Graph/Figure/index.vue', 'src/packages/components/BasicShapes/Graph/Figure/config.vue', 'src/views/chart/ContentEdit/components/EditShapeBox/index.vue']) {
  const descriptor = parse(fs.readFileSync(path.join(root, file), 'utf8')).descriptor
  compileScript(descriptor, { id: 'figure-line-test' })
}
console.log('PASS: legacy geometry, endpoint preservation, directions, caps/arrows, Shift, serialization, mouseup flush, history, Escape/unmount cleanup, SFC compilation')

const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
const source = fs.readFileSync(path.resolve(__dirname, '../src/views/chart/ContentEdit/hooks/useDrag.hook.ts'), 'utf8')
const hook = source.slice(source.indexOf('export const useMouseHandle ='), source.indexOf('// * 移动锚点'))
const js = ts.transpileModule(hook, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
function drag(dx, dy, scale = 1, isGroup = false) {
  const item = { id: 'a', isGroup, attr: { x: 10, y: 20, w: 100, h: 100 }, status: { lock: false, hide: false } }
  const listeners = new Map(), history = []
  const store = { getEditCanvas: { scale }, getTargetChart: { selectId: ['a'] }, targetChart: { selectId: ['a'] },
    getComponentList: [item], setTargetSelectChart() {}, fetchTargetIndex: () => 0,
    setMousePosition() {}, setEditCanvas() {}, moveComponentList: items => history.push(...items) }
  const throttle = fn => { let pending; const queued = (...args) => { pending = args }; queued.flush = () => { if (pending) { fn(...pending); pending = null } }; queued.cancel = () => { pending = null }; return queued }
  const mod = { exports: {} }, document = { addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name) }
  new Function('exports', 'module', 'chartEditStore', 'window', 'document', 'toRaw', 'cloneDeep', 'throttle', 'onClickOutSide', 'MouseEventButton', 'EditCanvasTypeEnum', js)(mod.exports, mod, store, {}, document, x => x, x => JSON.parse(JSON.stringify(x)), throttle, () => {}, { LEFT: 1, RIGHT: 2 }, { IS_DRAG: 'drag' })
  mod.exports.useMouseHandle().mousedownHandle({ preventDefault() {}, stopPropagation() {}, buttons: 1, screenX: 0, screenY: 0 }, item)
  listeners.get('mousemove')({ screenX: dx, screenY: dy })
  listeners.get('mouseup')()
  assert.equal(item.attr.x, Math.round(10 + dx / scale))
  assert.equal(item.attr.y, Math.round(20 + dy / scale))
  assert.equal(history.length, 1, 'single-axis moves must be undoable')
  assert.equal(listeners.size, 0)
  const restored = JSON.parse(JSON.stringify(item))
  assert.equal(restored.attr.x, item.attr.x); assert.equal(restored.attr.y, item.attr.y)
  assert.equal(history[0].attr.x + history[0].attr.offsetX, item.attr.x)
  assert.equal(history[0].attr.y + history[0].attr.offsetY, item.attr.y)
}
for (const [dx, dy] of [[-1000, 0], [0, -1000], [5000, 0], [0, 5000], [-1000, -1000]]) drag(dx, dy)
drag(3000, -1000, .5, true)
const editor = fs.readFileSync(path.resolve(__dirname, '../src/views/chart/ContentEdit/index.vue'), 'utf8')
const range = fs.readFileSync(path.resolve(__dirname, '../src/views/chart/ContentEdit/components/EditRange/index.vue'), 'utf8')
assert.match(editor, /goId\('chart-edit-content'\) \{\s*overflow: visible/)
assert.match(editor, /& > :deep\(\.content\) \{\s*position: relative;\s*z-index: 0;\s*isolation: isolate;\s*overflow: hidden;/)
assert.match(editor, /& > :deep\(\.bottom\) \{\s*position: relative;\s*z-index: 1;/)
assert.match(range, /overflow: visible/)
console.log('PASS: drag fully outside all four edges, negative coordinates, zoom/group drag, final throttled position, undo offsets and position serialization')

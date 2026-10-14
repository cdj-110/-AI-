const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
const { parse, compileScript, compileTemplate } = require('@vue/compiler-sfc')
const root = path.resolve(__dirname, '..')
const transpile = source => ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
const helper = { exports: {} }
new Function('exports', transpile(fs.readFileSync(path.join(root, 'src/utils/alignment.ts'), 'utf8')))(helper.exports)
const { alignmentPoints, nearestAlignmentSnaps, visibleAlignmentSnaps } = helper.exports
const candidate = (rect, id = 'other', isCanvas = false) => ({ id, rect, isCanvas, points: alignmentPoints(rect) })
const selected = { x: 200, y: 101, w: 100, h: 60 }
const others = [candidate({ x: 0, y: 100, w: 100, h: 60 })]
const nearest = nearestAlignmentSnaps(selected, others, 5)
assert.equal(nearest.length, 1); assert.equal(nearest[0].targetValue, 100)
const moved = { ...selected, y: 100 }
assert.deepEqual(visibleAlignmentSnaps(moved, others).map(x => x.selectedEdge), ['start', 'end'])
const vertical = { x: 0, y: 300, w: 100, h: 60 }
assert.deepEqual(visibleAlignmentSnaps(vertical, others).map(x => x.selectedEdge), ['start', 'end'])
const center = visibleAlignmentSnaps({ x: 50, y: 300, w: 100, h: 60 }, [candidate({ x: 0, y: 0, w: 200, h: 60 })])
assert.equal(center.length, 1); assert.equal(center[0].selectedEdge, 'center')
assert.equal(visibleAlignmentSnaps(moved, [candidate({ x: 0, y: 100, w: 100, h: 80 })]).length, 1)
assert.equal(visibleAlignmentSnaps({ x: 100, y: 300, w: 50, h: 60 }, others)[0].targetEdge, 'end')
assert.equal(visibleAlignmentSnaps(moved, [candidate({ x: 0, y: 100.3, w: 100, h: 60 })]).length, 0, 'near alignment must not be drawn as exact')
assert.equal(nearestAlignmentSnaps({ ...selected, y: 109 }, others, 5).length, 0)
assert.equal(nearestAlignmentSnaps({ ...selected, y: 109 }, others, 5 / .5).length, 1, 'screen-space threshold at zoom')
const canvas = candidate(others[0].rect, 'canvas', true)
assert.equal(nearestAlignmentSnaps(selected, [canvas, ...others], 5)[0].target.id, 'other')
const file = 'src/views/chart/ContentEdit/components/EditAlignLine/index.vue'
const source = fs.readFileSync(path.join(root, file), 'utf8')
const { descriptor, errors } = parse(source, { filename: file }); assert.deepEqual(errors, [])
compileScript(descriptor, { id: 'align-test' })
assert.deepEqual(compileTemplate({ source: descriptor.template.content, filename: file, id: 'align-test' }).errors, [])
const store = { getEditCanvas: { scale: 1 }, getTargetChart: { selectId: ['selected'] }, getComponentList: [
  { id: 'selected', attr: moved, status: { hide: false } }, { id: 'other', attr: others[0].rect, status: { hide: false } },
  { id: 'hidden', attr: moved, status: { hide: true } }
], getEditCanvasConfig: { width: 1920, height: 1080 } }
const imports = name => {
  if (name === 'vue') return { reactive: x => x, computed: fn => ({ get value() { return fn() } }), watch() {}, onBeforeUnmount() {} }
  if (name.endsWith('/alignment')) return helper.exports
  if (name.includes('designStore')) return { useDesignStore: () => ({ getAppTheme: '#fff' }) }
  if (name.includes('chartEditStore')) return { useChartEditStore: () => store }
  if (name.includes('settingStore')) return { useSettingStore: () => ({ getChartAlignRange: 5 }) }
  if (name.includes('editCanvas')) return {}
  if (name.includes('chartEditStore.d')) return { EditCanvasTypeEnum: { IS_DRAG: 'drag' } }
  if (name === 'lodash/throttle') return { default: fn => Object.assign(fn, { cancel() {} }) }
  if (name === '@/utils') return { setComponentPosition() {} }
  return {}
}
const ui = new Function('exports', 'require', transpile(descriptor.scriptSetup.content) + '\nreturn { state, renderAlignment, getCandidates };')({}, imports)
ui.renderAlignment(moved, others, visibleAlignmentSnaps(moved, others))
assert.equal(ui.state.guideLines.length, 2)
assert.equal(ui.state.highlightLines.length, 4)
assert.equal(ui.state.measureLines.length, 1, 'dual edges must not duplicate gap labels')
assert.equal(ui.state.measureLines[0].distance, 100)
assert.ok(ui.getCandidates().every(item => item.id !== 'hidden'))
ui.renderAlignment(moved, [...others, candidate({ x: -200, y: 100, w: 100, h: 60 }, 'far')], visibleAlignmentSnaps(moved, [...others, candidate({ x: -200, y: 100, w: 100, h: 60 }, 'far')]))
assert.equal(ui.state.guideLines.length, 2, 'coincident guides must be merged')
assert.ok(ui.state.guideLines.every(line => line.x === -208 && line.w === 516))
const utility = fs.readFileSync(path.join(root, 'src/utils/utils.ts'), 'utf8')
const setter = utility.slice(utility.indexOf('export const setComponentPosition ='), utility.indexOf('/**', utility.indexOf('export const setComponentPosition =') + 1))
const result = {}; new Function('exports', transpile(setter))(result)
const component = { attr: { x: 10, y: 10 } }; result.setComponentPosition(component, 0, 0)
assert.deepEqual(component.attr, { x: 0, y: 0 })
console.log('PASS: two-edge alignment, center/cross-edge matching, nearest snap priority, zoom threshold, hidden exclusion, merged guides, single gap label, zero coordinates and SFC rendering state')

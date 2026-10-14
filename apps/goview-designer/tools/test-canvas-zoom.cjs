const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
const { parse, compileScript, compileTemplate } = require('@vue/compiler-sfc')
const root = path.resolve(__dirname, '..'), helper = { exports: {} }
new Function('exports', ts.transpileModule(fs.readFileSync(path.join(root, 'src/utils/canvasZoom.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText)(helper.exports)
const { captureZoomAnchor, zoomScrollCorrection, zoomTranslationRemainder, wheelZoomScale } = helper.exports
const close = (a, b) => assert.ok(Math.abs(a - b) < 1e-8, `${a} !== ${b}`)
for (const oldScale of [.1, .52, 1, 2]) for (const newScale of [.1, .5, 1, 2]) for (const pointer of [{ x: 250, y: 320 }, { x: 30, y: 90 }, { x: 1600, y: 900 }]) {
  const oldOrigin = { left: -180, top: -90 }, nextOrigin = { left: 120, top: 250 }
  const anchor = captureZoomAnchor(pointer, oldOrigin, oldScale)
  const delta = zoomScrollCorrection(anchor, nextOrigin, newScale)
  for (const actual of [delta, { x: 0, y: 0 }, { x: delta.x / 2, y: delta.y / 2 }]) {
    const shift = zoomTranslationRemainder(delta, actual)
    close(nextOrigin.left - actual.x + shift.x + anchor.canvasPoint.x * newScale, pointer.x)
    close(nextOrigin.top - actual.y + shift.y + anchor.canvasPoint.y * newScale, pointer.y)
  }
}
assert.equal(wheelZoomScale(.1, 10), .1); assert.equal(wheelZoomScale(2, -10), 2)
assert.equal(wheelZoomScale(1, 0), 1); assert.equal(wheelZoomScale(1, -10), 1.05)
assert.equal(wheelZoomScale(1, 10), .95)
const file = 'src/views/chart/ContentEdit/components/EditRule/index.vue'
const source = fs.readFileSync(path.join(root, file), 'utf8'), { descriptor, errors } = parse(source, { filename: file })
assert.deepEqual(errors, []); compileScript(descriptor, { id: 'zoom-test' })
assert.deepEqual(compileTemplate({ source: descriptor.template.content, filename: file, id: 'zoom-test', compilerOptions: { expressionPlugins: ['typescript'] } }).errors, [])
assert.match(source, /\$onAction/); assert.match(source, /rememberPointer\(e\)/)
assert.match(source, /await nextTick\(\)/); assert.match(source, /zoomTranslationRemainder/)
assert.doesNotMatch(source, /wheelDelta/)
assert.doesNotMatch(source, /}, 400\)/)
console.log('PASS: mouse-anchor invariance at all zoom levels, panned viewport, scroll-boundary compensation, standard wheel direction, zoom limits and SFC compilation')

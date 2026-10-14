const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
const source = fs.readFileSync(path.resolve(__dirname, '../src/utils/contentResize.ts'), 'utf8')
const moduleResult = { exports: {} }
new Function('exports', 'module', ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText)(moduleResult.exports, moduleResult)
const resize = moduleResult.exports.resizedContentPosition
const original = { x: 100, y: 200, w: 100, h: 100 }
for (const next of [
  { x: 80, y: 200, w: 120, h: 100 },
  { x: 100, y: 180, w: 100, h: 120 },
  { x: 80, y: 180, w: 120, h: 120 },
  { x: 120, y: 220, w: 80, h: 80 },
  { x: 100, y: 200, w: 120, h: 120 }
]) {
  const position = resize(original, next, false)
  assert.equal(next.x + position.contentX, original.x)
  assert.equal(next.y + position.contentY, original.y)
}
const offsetOriginal = { ...original, contentX: 20, contentY: 30 }
assert.deepEqual(resize(offsetOriginal, { x: 80, y: 180, w: 120, h: 120 }, false), { contentX: 40, contentY: 50 })
assert.deepEqual(resize(offsetOriginal, { x: 0, y: 100, w: 200, h: 200 }, true), { contentX: 40, contentY: 60 })
for (const file of ['src/views/chart/ContentEdit/hooks/useStyle.hook.ts', 'src/views/preview/utils/style.ts']) {
  const style = fs.readFileSync(path.resolve(__dirname, '..', file), 'utf8')
  assert.match(style, /attr.contentX/)
  assert.match(style, /attr.contentY/)
}
console.log('PASS: left/top/corner resize preserves content position; existing offsets, proportional scaling and editor/preview styles')

const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
const source = fs.readFileSync(path.resolve(__dirname, '../src/utils/textResize.ts'), 'utf8')
const result = { exports: {} }
new Function('exports', 'module', ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText)(result.exports, result)
const capture = result.exports.captureTextResize
for (const [key, field] of Object.entries({ TextFun: 'textSize1', TextCommon: 'fontSize', TextGradient: 'size', TextBarrage: 'fontSize', Number: 'numberSize', TimeCommon: 'timeSize', FlipperNumber: 'flipperWidth', CountDown: 'flipperHeight' })) {
  const component = { key, option: { [field]: 20, dataset: 100, fontWeight: 500, flipperSpeed: 450, lineHeight1: 1.3 } }
  const resize = capture(component)
  resize(2, 2, false); assert.equal(component.option[field], 20)
  resize(2, 1, true); assert.equal(component.option[field], 40)
  resize(3, 3, true); assert.equal(component.option[field], 60) // not 120
  resize(.5, .5, true); assert.equal(component.option[field], 10)
  resize(1, 2, true); assert.equal(component.option[field], 40)
  resize(2, 2, false); assert.equal(component.option[field], 20)
  assert.equal(component.option.dataset, 100)
  assert.equal(component.option.fontWeight, 500)
  assert.equal(component.option.flipperSpeed, 450)
  assert.equal(component.option.lineHeight1, 1.3)
}
const text = { chartConfig: { key: 'TextFun' }, option: { textSize1: 30, textSize2: 20, textSize3: 10, letter1: 0, marginLeft: -5 } }
capture(text)(2, 2, true)
assert.deepEqual(text.option, { textSize1: 60, textSize2: 40, textSize3: 20, letter1: 0, marginLeft: -10 })
const chart = { key: 'BarCommon', option: { fontSize: 12 } }
capture(chart)(2, 2, true); assert.equal(chart.option.fontSize, 12)
capture(undefined)(2, 2, true)
const drag = fs.readFileSync(path.resolve(__dirname, '../src/views/chart/ContentEdit/hooks/useDrag.hook.ts'), 'utf8')
assert.match(drag, /resizeText\(.*scaleContent\)/)
assert.match(drag, /resizedComponentBounds/)
console.log('PASS: text/number font scaling, ordinary resize, modifier release, repeated moves, prefix/suffix, non-pixel settings preserved')

const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
const { parse, compileScript, compileTemplate } = require('@vue/compiler-sfc')
const root = path.resolve(__dirname, '..')
const moduleResult = { exports: {} }
new Function('exports', 'module', ts.transpileModule(fs.readFileSync(path.join(root, 'src/utils/componentInitialBounds.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText)(moduleResult.exports, moduleResult)
const initialize = moduleResult.exports.initializeComponentBounds
const groups = [
  [['CircleFun', 'Clock', 'PieCircle', 'PieCommon', 'VChartPie', 'Dial', 'WaterPolo', 'Process', 'Polygon', 'FiveStars'], [200, 200]],
  [['FullScreen', 'CirclePoint'], [30, 30]],
  [['Number', 'TextFun', 'TextCommon', 'TextGradient', 'TimeCommon'], [300, 50]],
  [['TablesBasic', 'TablesAdvanced'], [900, 420]],
  [['MapBase', 'MapAmap'], [800, 600]]
]
for (const [keys, size] of groups) for (const key of keys) for (const previous of [[500, 300], [97, 97], [900, 420]]) {
  const component = initialize({ chartConfig: { key }, attr: { x: 50, y: 50, w: previous[0], h: previous[1] } })
  assert.deepEqual([component.attr.w, component.attr.h], size)
  assert.equal(component.attr.contentW, component.attr.w)
  assert.equal(component.attr.contentH, component.attr.h)
  assert.equal(component.attr.contentX, 0)
}
// 未分类图表的坐标轴/图例空间保留，不把所有图表硬改成方形。
const chart = initialize({ chartConfig: { key: 'BarCommon' }, attr: { w: 500, h: 300 } })
assert.deepEqual([chart.attr.w, chart.attr.h], [500, 300])
// 加载已保存项目的尺寸可覆盖新建默认值。
const saved = { w: 415, h: 270, contentW: 205, contentH: 180, contentX: 12, contentY: 20 }
Object.assign(chart.attr, saved)
for (const key of Object.keys(saved)) assert.equal(chart.attr[key], saved[key])
for (const key of ['FullScreen', 'CirclePoint']) {
  const component = initialize({ chartConfig: { key }, attr: {} })
  Object.assign(component.attr, saved)
  for (const field of Object.keys(saved)) assert.equal(component.attr[field], saved[field])
  for (const entry of ['BasicShapes', 'Decorates']) {
    const config = fs.readFileSync(path.join(root, `src/packages/components/${entry}/Mores/${key}/config.ts`), 'utf8')
    assert.match(config, /w: 30, h: 30/)
    if (key === 'CirclePoint') assert.match(config, /outCircle: 50/)
  }
}
for (const file of [
  'src/packages/components/BasicShapes/Mores/CirclePoint/index.vue',
  'src/packages/components/Decorates/Mores/CirclePoint/index.vue',
  'src/packages/components/Charts/Pies/PieCommon/index.vue'
]) {
  const filename = path.join(root, file)
  const source = fs.readFileSync(filename, 'utf8')
  const { descriptor, errors } = parse(source, { filename })
  assert.deepEqual(errors, [])
  compileScript(descriptor, { id: 'bounds' })
  const compiled = compileTemplate({ source: descriptor.template.content, filename, id: 'bounds', compilerOptions: { expressionPlugins: ['typescript'] } })
  assert.deepEqual(compiled.errors, [])
  if (file.includes('CirclePoint')) assert.match(source, /viewBox="0 0 110 110"/)
}
console.log('PASS: consistent shape/family defaults, duplicate entry points, matched content size, saved-size overrides and SFC compilation')

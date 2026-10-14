const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const assert = require('node:assert/strict')
const { parse, compileScript, compileTemplate } = require('@vue/compiler-sfc')
const cache = new Map()
function load(file) {
  file = path.resolve(__dirname, '..', file)
  if (cache.has(file)) return cache.get(file)
  const module = { exports: {} }
  cache.set(file, module.exports)
  const output = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
  new Function('exports', 'require', 'module', output)(module.exports, name => name.startsWith('.') ? load(path.relative(path.resolve(__dirname, '..'), path.resolve(path.dirname(file), name + '.ts'))) : require(name), module)
  return module.exports
}
async function main() {
  const helper = load('src/utils/platformHistoryChart.ts')
  assert.equal(helper.validateHistoryRange(null), '请选择开始和结束时间')
  assert.notEqual(helper.validateHistoryRange(['2026-10-09 10:00:00', '2026-10-09 09:00:00']), '')
  assert.notEqual(helper.validateHistoryRange(['2026-08-01 10:00:00', '2026-10-09 09:00:00']), '')
  assert.equal(helper.validateHistoryRange(['2026-10-09 09:00:00', '2026-10-09 10:00:00']), '')
  const component = { chartConfig: { key: 'BarCommon' }, request: { requestHttpType: '历史数据' } }
  assert.equal(helper.isHistoryQueryChart(component), true)
  component.request.requestHttpType = '实时数据'
  assert.equal(helper.isHistoryQueryChart(component), false)
  const dimensions = [{ key: 'timestamp' }, { key: 'deviceName' }, { key: 'metric_0', title: '设备A / 温度' }, { key: 'metric_1', title: '设备B / 温度' }]
  const rows = [
    { timestamp: '2026-10-09 10:00:02', metric_0: '42', metric_1: null },
    { timestamp: '2026-10-09 10:00:01', metric_0: 0, metric_1: null },
    { timestamp: '2026-10-09 10:00:01', metric_0: null, metric_1: 20 }
  ]
  const dataset = helper.historyChartDataset(dimensions, rows, {})
  assert.equal(dataset.source.length, 2)
  assert.equal(dataset.source[0].metric_0, 0)
  assert.equal(dataset.source[0].metric_1, 20)
  assert.equal(dataset.dimensions.length, 3)
  const option = { direction: 'horizontal' }
  assert.equal(helper.historyChartDataset(dimensions, rows, option, true).length, 3)
  assert.equal(option.xField, 'historyValue')
  assert.deepEqual(helper.historyChartDataset(dimensions, [], {}).source, [])
  let calls = 0
  const query = async params => {
    calls++
    return { dataset: { dimensions, source: Array.from({ length: params.page === 1 ? 100 : 1 }, () => rows[0]) }, total: 101, hasNext: params.page === 1 }
  }
  const result = await helper.fetchHistoryChart(query, { start: 'start', end: 'end' }, () => false)
  assert.equal(calls, 2)
  assert.equal(result.rows.length, 101)
  assert.equal(result.limited, false)
  calls = 0
  const previewResult = await helper.fetchHistoryChart(async params => {
    calls++
    return { dataset: { dimensions, source: Array.from({ length: 100 }, () => rows[0]) }, total: 6000, hasNext: true }
  }, {}, () => false, 500)
  assert.equal(calls, 5)
  assert.equal(previewResult.rows.length, 500)
  assert.equal(previewResult.limited, true)
  calls = 0
  const fullResult = await helper.fetchHistoryChart(async params => {
    calls++
    return { dataset: { dimensions, source: Array.from({ length: 100 }, () => rows[0]) }, total: 6000, hasNext: true }
  }, {}, () => false)
  assert.equal(calls, 50)
  assert.equal(fullResult.rows.length, 5000)
  assert.equal(await helper.fetchHistoryChart(query, {}, () => true), null)
  await assert.rejects(helper.fetchHistoryChart(async () => ({}), {}, () => false))
  for (const file of [
    'src/views/chart/ContentEdit/index.vue',
    'src/views/chart/ContentEdit/components/EditGroup/index.vue',
    'src/views/preview/components/HistoryQueryChart.vue',
    'src/views/preview/components/PreviewRenderList/index.vue',
    'src/views/preview/components/PreviewRenderGroup/index.vue',
    'src/views/chart/ContentConfigurations/components/ChartData/components/ChartDataRequest/components/RequestTargetConfig/index.vue'
  ]) {
    const { descriptor, errors } = parse(fs.readFileSync(path.resolve(__dirname, '..', file), 'utf8'), { filename: file })
    assert.deepEqual(errors, [])
    compileScript(descriptor, { id: 'test' })
    const compiled = compileTemplate({ source: descriptor.template.content, filename: file, id: 'test', compilerOptions: { expressionPlugins: ['typescript'] } })
    assert.deepEqual(compiled.errors, [])
  }
  const editor = fs.readFileSync(path.resolve(__dirname, '../src/views/chart/ContentEdit/index.vue'), 'utf8')
  const groupEditor = fs.readFileSync(path.resolve(__dirname, '../src/views/chart/ContentEdit/components/EditGroup/index.vue'), 'utf8')
  for (const source of [editor, groupEditor]) assert.match(source, /isHistoryQueryChart\(item\).*editing/)
  const historyRenderer = fs.readFileSync(path.resolve(__dirname, '../src/views/preview/components/HistoryQueryChart.vue'), 'utf8')
  assert.match(historyRenderer, /editing \? \{\} : useLifeHandler/)
  assert.match(historyRenderer, /props.editing && automatic \? 500 : 5000/)
  assert.match(historyRenderer, /@click="load\(\)"/)
  assert.match(historyRenderer, /cancelAnimationFrame\(automaticFrame\)/)
  console.log('PASS: time validation, realtime exclusion, multi-point alignment, empty data, VChart mapping, pagination, stale responses, SFC compilation')
}
main().catch(error => { console.error(error); process.exitCode = 1 })

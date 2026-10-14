import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { generatedPointName, nextGeneratedPointMetric } from '../src/pointNaming.ts'

// Execute the actual component handlers, including the manual-add allocator.
function functions(file, names) {
  const source = readFileSync(new URL(`../src/components/${file}`, import.meta.url), 'utf8')
  const script = source.match(/<script setup[^>]*>([\s\S]*?)<\/script>/)[1]
  const ast = ts.createSourceFile(file + '.ts', script, ts.ScriptTarget.Latest, true)
  const declarations = ast.statements.filter(node => ts.isFunctionDeclaration(node) && names.includes(node.name?.text))
  assert.equal(declarations.length, names.length)
  return ts.transpileModule(declarations.map(node => node.getText(ast)).join('\n'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText
}
const browserHandlers = functions('ProtocolPointTools.vue', [
  'appendPoint', 'confirmIEC', 'flattenIEC', 'normalizeIECNode', 'iecKey',
  'treeFromIECPoints', 'iecRef', 'iecParts',
])
const manualHandlers = functions('DevicePointWorkspace.vue', [
  'protocolMetricPrefix', 'uniqueMetric', 'reserveMetric', 'nextMetric', 'pointNamePrefix', 'pointName',
])
function createContext(useManual, protocol='iec61850', prefix=protocol) {
  const points = [{ metric: `${prefix}-P02`, objectRef: 'IED/LLN0.Mod.stVal', fc: 'ST' }]
  const props = {
    device: { protocol, deviceKey: 'ied-1', channelKey: 'ch-1', name: '保护设备', address: '192.0.2.1:102' },
    interfaceName: 'eth0', channelName: '采集通道', namePrefix: 'eth0_采集通道_保护设备', points,
    // Separate snapshot: recently appended points must also be checked for uniqueness.
    allPoints: [...points, { metric: `${prefix.toUpperCase()}-p09` }, { metric: 'unrelated-P99' }],
  }
  if (useManual) {
    const manual = vm.createContext({ props, metricIndex: new Set(props.allPoints.map(p => p.metric.toLowerCase())), metricSuffixCursor: new Map(), nextGeneratedMetricIndex: 10 })
    vm.runInContext(manualHandlers, manual)
    props.nextMetric = manual.nextMetric
    props.pointName = manual.pointName
  }
  const context = vm.createContext({ props, nextGeneratedPointMetric, generatedPointName, iecNodes: { value: [] }, iecSelected: { value: [] }, open: { value: true } })
  vm.runInContext(browserHandlers, context)
  return context
}
for (const useManual of [true, false]) {
  for (const importedFile of [true, false]) {
    const context = createContext(useManual)
    const candidates = [
      { objectRef: 'IED/LLN0.Mod.stVal', fc: 'ST', metric: 'old' }, // existing point
      { objectRef: 'IED/GGIO1.AnIn1.mag.f', fc: 'MX', metric: 'AnIn1', dataType: 'float32' },
      { objectRef: 'IED/GGIO1.AnIn2.mag.f', fc: 'MX', metric: 'AnIn2', dataType: 'float64' },
    ]
    context.iecNodes.value = importedFile ? context.treeFromIECPoints(candidates) : candidates.map(p => context.normalizeIECNode({ ...p, leaf: true }))
    // Repeated nodes within a response should not be inserted twice.
    context.iecNodes.value.push(context.normalizeIECNode({ ...candidates[1], leaf: true }))
    context.iecSelected.value = context.flattenIEC(context.iecNodes.value).map(p => p.key)
    context.confirmIEC()
    const points = context.props.points
    assert.deepEqual(points.map(p => p.metric), ['iec61850-P02', 'iec61850-P10', 'iec61850-P11'])
    assert.equal(points[1].name, 'eth0_采集通道_保护设备@iec61850-P10')
    assert.equal(points[1].objectRef, candidates[1].objectRef)
    assert.equal(points[1].fc, 'MX')
    assert.equal(points[1].dataType, 'float32')
    assert.equal(context.open.value, false)
    context.confirmIEC()
    assert.equal(points.length, 3, 're-import must not add existing points')
    if (useManual) assert.equal(context.props.nextMetric(), 'iec61850-P12', 'subsequent manual addition continues the same sequence')
  }
}
const empty = createContext(false)
empty.props.points = []
empty.props.allPoints = []
empty.appendPoint({ objectRef: 'IED/GGIO1.Ind.stVal', fc: 'ST' }, 'iec61850')
assert.equal(empty.props.points[0].metric, 'iec61850-P01')
console.log('IEC 61850 browse/CID import naming, manual allocator, uniqueness and reference preservation: ok')

for (const [protocol,prefix,address] of [
  ['opcua','opcua',{nodeId:'ns=2;s=Temperature'}],
  ['siemens-s7','s7',{area:'DB',dbNumber:3,register:12}],
  ['opcda','opcda',{itemId:'Random.Real8',access:'read'}],
]) {
  for (const useManual of [true,false]) {
    const context=createContext(useManual,protocol,prefix)
    for(let i=0;i<3;i++)context.appendPoint({...address,metric:'server_name',name:'Server display name',dataType:'float32'},prefix)
    assert.deepEqual(context.props.points.map(p=>p.metric),[`${prefix}-P02`,`${prefix}-P10`,`${prefix}-P11`,`${prefix}-P12`])
    assert.equal(context.props.points[1].name,`eth0_采集通道_保护设备@${prefix}-P10`)
    for(const [key,value] of Object.entries(address))assert.equal(context.props.points[1][key],value)
    if(useManual)assert.equal(context.props.nextMetric(),`${prefix}-P13`)
  }
}
// Other collection protocols already use the shared manual-add allocator.
for(const [protocol,prefix] of [['modbus-tcp','mbtcp'],['modbus-rtu','mbrtu'],['iec104','iec104'],['iec61850-goose','goose']]) {
  const context=createContext(true,protocol,prefix)
  assert.equal(context.props.nextMetric(),`${prefix}-P10`)
  assert.equal(context.props.nextMetric(),`${prefix}-P11`)
}
console.log('OPC UA / S7 / OPC DA batch naming and remaining collection protocol allocators: ok')

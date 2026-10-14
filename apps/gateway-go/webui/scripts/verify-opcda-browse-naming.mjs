import assert from 'node:assert/strict'
import { generatedPointName, nextGeneratedPointMetric } from '../src/pointNaming.ts'

const points = [
  { metric: 'opcda-P07' },
  { metric: 'opcda-P10' },
  { metric: 'array_of_real8' },
]

assert.equal(nextGeneratedPointMetric(points, 'opcda'), 'opcda-P11')
points.push({ metric: 'opcda-P11' })
assert.equal(nextGeneratedPointMetric(points, 'opcda'), 'opcda-P12')
assert.equal(
  generatedPointName('eth0_OPC DA 测试采集通道_Matrikon OPC Simulation', 'opcda-P11'),
  'eth0_OPC DA 测试采集通道_Matrikon OPC Simulation@opcda-P11',
)

console.log('OPC DA browse point naming: ok')

export type PlatformDataModel = '实时数据' | '历史数据'

// 按稳定组件标识分类，不依赖可修改的标题或当前选中图层。
const historyOnlyKeys = new Set(['TablesBasic', 'TablesAdvanced'])
const realtimeOnlyKeys = new Set([
  'Number', 'FlipperNumber', 'CirclePoint', 'SwitchFun',
  'Dial', 'Process', 'WaterPolo', 'PieCircle', 'CountDown',
  'TextCommon', 'TextGradient', 'TextBarrage'
])

export function componentKeyOf(component: any): string {
  const config = component?.chartConfig || component || {}
  const key = config.key || component?.key
  if (key) return key
  // 兼容旧项目只保存 chartKey / conKey 的情况。
  return String(config.conKey || config.chartKey || '').replace(/^VC?/, '')
}

export function platformDataModelsOf(component: any): PlatformDataModel[] {
  const key = componentKeyOf(component)
  if (historyOnlyKeys.has(key)) return ['历史数据']
  if (realtimeOnlyKeys.has(key)) return ['实时数据']
  // 单值组件消费最新值；历史接口返回的是时间序列，不能直接喂给单值组件。
  const dataset = component?.option?.dataset
  if (['number', 'string', 'boolean'].includes(typeof dataset)) return ['实时数据']
  return ['实时数据', '历史数据']
}

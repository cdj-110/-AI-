// 仅用于创建组件，加载已有项目时仍由项目保存的 attr 覆盖。
// 同名组件即使从不同组件库入口拖出，也使用相同尺寸。
const squareKeys = new Set([
  'CircleFun', 'Clock', 'PieCircle', 'PieCommon', 'VChartPie',
  'Dial', 'WaterPolo', 'Process', 'Polygon', 'FiveStars'
])
const sizes: Record<string, [number, number]> = {
  FullScreen: [30, 30], CirclePoint: [30, 30],
  Number: [300, 50], TextFun: [300, 50], TextCommon: [300, 50], TextGradient: [300, 50], TimeCommon: [300, 50],
  FlipperNumber: [300, 100], CountDown: [500, 100],
  Rect: [300, 200], ButtonFun: [150, 50], SwitchFun: [150, 60], InputFun: [300, 30],
  TablesBasic: [900, 420], TablesAdvanced: [900, 420],
  MapBase: [800, 600], MapAmap: [800, 600],
  PipelineH: [500, 15], PipelineV: [15, 500]
}

export function initializeComponentBounds(component: any) {
  const key = component.chartConfig?.key || component.key
  const size = squareKeys.has(key) ? [200, 200] : sizes[key]
  if (size) [component.attr.w, component.attr.h] = size
  component.attr.contentW = component.attr.w
  component.attr.contentH = component.attr.h
  component.attr.contentX = 0
  component.attr.contentY = 0
  return component
}

type Component = { key?: string; chartConfig?: { key?: string; package?: string }; request?: any; option?: any }
type Bounds = { x: number; y: number; w: number; h: number; contentW?: number; contentH?: number }
type Modifiers = { ctrlKey?: boolean; metaKey?: boolean; shiftKey?: boolean }

// 统一交互规则，内部仍使用各组件自己的重排/绘图逻辑，不做全局 CSS 拉伸。
export function componentResizePolicy(component?: Component) {
  const key = component?.chartConfig?.key || component?.key || ''
  const pkg = component?.chartConfig?.package || ''
  const circular = ['CircleFun', 'CirclePoint', 'Clock'].includes(key)
  if (pkg === 'Tables') return { minW: key === 'TablesAdvanced' ? 320 : 160, minH: key === 'TablesAdvanced' ? 160 : 100, circular }
  if (pkg === 'Charts' || pkg === 'VChart') {
    const query = component?.request?.historyQuery?.enabled
    return { minW: 120, minH: query ? 120 : 80, circular }
  }
  if (['ButtonFun', 'InputFun'].includes(key) || component?.chartConfig?.package === 'Informations' && /^Inputs/.test(key)) {
    return { minW: 40, minH: 24, circular }
  }
  return { minW: 8, minH: 8, circular }
}

export function resizedComponentBounds(original: Bounds, point: string, dx: number, dy: number, modifiers: Modifiers, component?: Component) {
  const left = point.includes('l'), right = point.includes('r')
  const top = point.includes('t'), bottom = point.includes('b')
  const corner = (left || right) && (top || bottom)
  const scaleContent = !(modifiers.ctrlKey || modifiers.metaKey)
  const policy = componentResizePolicy(component)
  // 已有小组件不会一拖就跳大；新增的缩小操作不能低于可用尺寸。
  const minW = scaleContent ? Math.min(original.w, policy.minW * original.w / Math.max(1, original.contentW ?? original.w)) : 1
  const minH = scaleContent ? Math.min(original.h, policy.minH * original.h / Math.max(1, original.contentH ?? original.h)) : 1
  let w = Math.max(minW, original.w + (left ? -dx : right ? dx : 0))
  let h = Math.max(minH, original.h + (top ? -dy : bottom ? dy : 0))
  if (corner && (modifiers.shiftKey || scaleContent && policy.circular)) {
    const wr = w / original.w, hr = h / original.h
    const ratio = Math.max(Math.abs(wr - 1) >= Math.abs(hr - 1) ? wr : hr, minW / original.w, minH / original.h)
    w = original.w * ratio
    h = original.h * ratio
  }
  return { x: original.x + (left ? original.w - w : 0), y: original.y + (top ? original.h - h : 0), w, h, scaleContent }
}

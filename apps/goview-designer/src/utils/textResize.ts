type TextComponent = { key?: string; chartConfig?: { key?: string }; option?: Record<string, any> }

// 仅缩放文字组件的像素属性，不触碰数据值、字体粗细、动画时长和无单位行高。
const pixelFields: Record<string, string[]> = {
  TextFun: ['textSize1', 'textSize2', 'textSize3', 'letter1', 'letter2', 'letter3', 'marginLeft', 'marginRight'],
  TextCommon: ['fontSize', 'letterSpacing', 'paddingX', 'paddingY', 'borderWidth', 'borderRadius'],
  TextGradient: ['size'],
  TextBarrage: ['fontSize', 'letterSpacing'],
  Number: ['numberSize'],
  TimeCommon: ['timeSize', 'timeLineHeight', 'timeTextIndent', 'hShadow', 'vShadow', 'blurShadow'],
  FlipperNumber: ['flipperWidth', 'flipperHeight', 'flipperRadius', 'flipperGap', 'flipperBorderWidth'],
  CountDown: ['flipperWidth', 'flipperHeight', 'flipperRadius', 'flipperGap']
}
const fontFields = new Set(['fontSize', 'size', 'numberSize', 'timeSize', 'textSize1', 'textSize2', 'textSize3', 'flipperWidth', 'flipperHeight'])

export function captureTextResize(component?: TextComponent) {
  const option = component?.option
  const fields = pixelFields[component?.chartConfig?.key || component?.key || ''] || []
  if (!option || !fields.length) return () => {}
  const initial = fields.filter(field => typeof option[field] === 'number' && Number.isFinite(option[field]))
    .map(field => [field, option[field]] as const)
  // 每帧都以拖动开始的值计算，避免累计放大；Ctrl 仅调整容器时恢复原字号。
  return (widthRatio = 1, heightRatio = 1, scaleContent = false) => {
    const ratio = scaleContent
      ? (Math.abs(widthRatio - 1) >= Math.abs(heightRatio - 1) ? widthRatio : heightRatio)
      : 1
    if (!Number.isFinite(ratio) || ratio <= 0) return
    for (const [field, value] of initial) {
      const scaled = Math.round(value * ratio * 100) / 100
      option[field] = fontFields.has(field) ? Math.max(1, scaled) : scaled
    }
  }
}

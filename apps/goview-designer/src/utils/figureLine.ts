export type LinePoint = { x: number; y: number }
const finite = (value: unknown, fallback: number) => Number.isFinite(Number(value)) ? Number(value) : fallback
export const lineStroke = (option: any) => Math.max(.5, Math.min(100, finite(option?.chuxi, 5)))
export const isFigureLine = (component: any) => (component?.chartConfig?.key || component?.key) === 'Figure'
export function lineGeometry(vector: LinePoint, option: any) {
  const dx = finite(vector.x, 300), dy = finite(vector.y, 0), length = Math.hypot(dx, dy)
  const ux = length ? dx / length : 1, uy = length ? dy / length : 0
  const stroke = lineStroke(option), padding = stroke * (option?.startArrow || option?.endArrow ? 1.5 : .5)
  const round = option?.lineCap === 'round' || option?.lineStyle === 'dotted'
  const px = Math.max(Math.abs(uy) * padding, round ? stroke / 2 : 0)
  const py = Math.max(Math.abs(ux) * padding, round ? stroke / 2 : 0)
  const start = { x: px + Math.max(0, -dx), y: py + Math.max(0, -dy) }
  return { start, end: { x: start.x + dx, y: start.y + dy }, width: Math.max(1, Math.abs(dx) + px * 2), height: Math.max(1, Math.abs(dy) + py * 2), length }
}
export function renderedLine(attr: any, option: any) {
  const width = Math.max(1, finite(attr?.contentW ?? attr?.w, 300)), height = Math.max(1, finite(attr?.contentH ?? attr?.h, lineStroke(option)))
  // 旧数据没有向量，保留原水平长度、顶部位置及粗细，不反向修改组件尺寸。
  if (!option?.lineVector) return { start: { x: 0, y: lineStroke(option) / 2 }, end: { x: width, y: lineStroke(option) / 2 }, width, height }
  const geometry = lineGeometry({ x: option.lineVector.dx, y: option.lineVector.dy }, option)
  const point = (p: LinePoint) => ({ x: p.x * width / geometry.width, y: p.y * height / geometry.height })
  return { start: point(geometry.start), end: point(geometry.end), width, height }
}
export function lineWorldPoints(attr: any, option: any) {
  const line = renderedLine(attr, option), x = finite(attr?.x, 0) + finite(attr?.contentX, 0), y = finite(attr?.y, 0) + finite(attr?.contentY, 0)
  return { start: { x: x + line.start.x, y: y + line.start.y }, end: { x: x + line.end.x, y: y + line.end.y } }
}
export function setLineEndpoints(component: any, start: LinePoint, end: LinePoint) {
  let dx = end.x - start.x, dy = end.y - start.y
  if (Math.hypot(dx, dy) < 1) { dx = 1; dy = 0 }
  component.option.lineVector = { dx, dy }
  const geometry = lineGeometry({ x: dx, y: dy }, component.option)
  Object.assign(component.attr, { x: start.x - geometry.start.x, y: start.y - geometry.start.y,
    w: geometry.width, h: geometry.height, contentW: geometry.width, contentH: geometry.height, contentX: 0, contentY: 0 })
}
export function constrainLineEnd(anchor: LinePoint, moving: LinePoint, shift: boolean): LinePoint {
  if (!shift) return moving
  const dx = moving.x - anchor.x, dy = moving.y - anchor.y, length = Math.hypot(dx, dy)
  const angle = Math.round(Math.atan2(dy, dx) / (Math.PI / 4)) * Math.PI / 4
  return { x: anchor.x + Math.cos(angle) * length, y: anchor.y + Math.sin(angle) * length }
}
export function arrowPoints(tip: LinePoint, other: LinePoint, stroke: number) {
  const length = Math.hypot(tip.x - other.x, tip.y - other.y)
  if (!length) return ''
  const ux = (tip.x - other.x) / length, uy = (tip.y - other.y) / length
  const depth = Math.min(stroke * 4, length / 3), half = Math.min(stroke * 1.5, depth / 2)
  const x = tip.x - ux * depth, y = tip.y - uy * depth
  return `${tip.x},${tip.y} ${x - uy * half},${y + ux * half} ${x + uy * half},${y - ux * half}`
}

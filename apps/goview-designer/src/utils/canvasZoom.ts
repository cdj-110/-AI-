type Point = { x: number; y: number }
type Origin = { left: number; top: number }
export type ZoomAnchor = { pointer: Point; canvasPoint: Point }

export function captureZoomAnchor(pointer: Point, origin: Origin, scale: number): ZoomAnchor {
  return { pointer, canvasPoint: { x: (pointer.x - origin.left) / scale, y: (pointer.y - origin.top) / scale } }
}
export function zoomScrollCorrection(anchor: ZoomAnchor, origin: Origin, scale: number): Point {
  return { x: origin.left + anchor.canvasPoint.x * scale - anchor.pointer.x,
    y: origin.top + anchor.canvasPoint.y * scale - anchor.pointer.y }
}
// 滚动到边界时，未能完成的滚动量通过画布平移补偿，保持锚点不跳动。
export function zoomTranslationRemainder(correction: Point, actualScroll: Point): Point {
  return { x: actualScroll.x - correction.x, y: actualScroll.y - correction.y }
}
export function wheelZoomScale(scale: number, deltaY: number) {
  if (!Number.isFinite(deltaY) || deltaY === 0) return scale
  return Math.max(0.1, Math.min(2, Math.round((scale + (deltaY < 0 ? 0.05 : -0.05)) * 100) / 100))
}

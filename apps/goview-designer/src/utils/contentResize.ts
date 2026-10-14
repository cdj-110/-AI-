type Bounds = { x: number; y: number; w: number; h: number; contentX?: number; contentY?: number }

// 内容位置相对容器保存，Ctrl 仅调整边界时补偿容器位移，保持画布绝对位置不变。
export function resizedContentPosition(original: Bounds, next: Bounds, scaleContent: boolean) {
  const x = original.contentX ?? 0, y = original.contentY ?? 0
  return scaleContent
    ? { contentX: original.w > 0 ? x * next.w / original.w : x,
        contentY: original.h > 0 ? y * next.h / original.h : y }
    : { contentX: x + original.x - next.x, contentY: y + original.y - next.y }
}

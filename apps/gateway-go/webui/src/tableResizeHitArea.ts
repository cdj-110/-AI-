/**
 * Keep the visible divider narrow while making it easy to grab from either
 * side. The returned index is the column whose right edge owns the divider.
 */
export const POINT_TABLE_RESIZE_HIT_RADIUS = 10

export function pointTableResizeColumn(
  clientX: number,
  left: number,
  right: number,
  cellIndex: number,
): number {
  if (Math.abs(right - clientX) <= POINT_TABLE_RESIZE_HIT_RADIUS) return cellIndex
  if (cellIndex > 0 && Math.abs(clientX - left) <= POINT_TABLE_RESIZE_HIT_RADIUS) return cellIndex - 1
  return -1
}

export function pointTableResizeHeader(
  event: PointerEvent | MouseEvent,
  cell: HTMLTableCellElement,
): HTMLTableCellElement | undefined {
  if (cell.tagName !== 'TH') return undefined
  const rect = cell.getBoundingClientRect()
  const index = pointTableResizeColumn(event.clientX, rect.left, rect.right, cell.cellIndex)
  if (index < 0) return undefined
  const header = cell.parentElement?.children[index]
  return header instanceof HTMLTableCellElement ? header : undefined
}

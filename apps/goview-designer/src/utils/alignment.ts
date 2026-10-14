export type AlignAxis = 'x' | 'y'
export type AlignEdge = 'start' | 'center' | 'end'
export type AlignRect = { x: number; y: number; w: number; h: number }
type Point = { axis: AlignAxis; edge: AlignEdge; value: number }
export type AlignCandidate = { id: string; rect: AlignRect; points: Point[]; isCanvas?: boolean }
export type AlignmentSnap = {
  axis: AlignAxis; selectedEdge: AlignEdge; targetEdge: AlignEdge
  targetValue: number; distance: number; target: AlignCandidate
}
export const ALIGN_EPSILON = 0.01
export const alignmentPoints = (rect: AlignRect): Point[] => [
  { axis: 'x', edge: 'start', value: rect.x },
  { axis: 'x', edge: 'center', value: rect.x + rect.w / 2 },
  { axis: 'x', edge: 'end', value: rect.x + rect.w },
  { axis: 'y', edge: 'start', value: rect.y },
  { axis: 'y', edge: 'center', value: rect.y + rect.h / 2 },
  { axis: 'y', edge: 'end', value: rect.y + rect.h }
]
const priority = (snap: AlignmentSnap) => (snap.selectedEdge === snap.targetEdge ? 0 : 2)
  + (snap.selectedEdge === 'center' ? 1 : 0) + (snap.target.isCanvas ? 4 : 0)
const crossDistance = (rect: AlignRect, snap: AlignmentSnap) => snap.axis === 'x'
  ? Math.abs(rect.y + rect.h / 2 - snap.target.rect.y - snap.target.rect.h / 2)
  : Math.abs(rect.x + rect.w / 2 - snap.target.rect.x - snap.target.rect.w / 2)

/** 每轴只选择一个位移，避免多条辅助线互相拉扯；同距离优先同边、邻近组件。 */
export function nearestAlignmentSnaps(rect: AlignRect, candidates: AlignCandidate[], range: number): AlignmentSnap[] {
  const best = new Map<AlignAxis, AlignmentSnap>()
  for (const selected of alignmentPoints(rect)) for (const candidate of candidates) for (const target of candidate.points) {
    if (selected.axis !== target.axis) continue
    const distance = Math.abs(selected.value - target.value)
    if (distance > range) continue
    const snap = { axis: selected.axis, selectedEdge: selected.edge, targetEdge: target.edge,
      targetValue: target.value, distance, target: candidate }
    const current = best.get(selected.axis)
    if (!current || distance < current.distance - ALIGN_EPSILON
      || (Math.abs(distance - current.distance) <= ALIGN_EPSILON && (priority(snap) < priority(current)
        || (priority(snap) === priority(current) && crossDistance(rect, snap) < crossDistance(rect, current))))) {
      best.set(selected.axis, snap)
    }
  }
  return [...best.values()]
}

/** 位移完成后重新检测所有真实对齐线；同尺寸双边对齐时省略冗余中心线。 */
export function visibleAlignmentSnaps(rect: AlignRect, candidates: AlignCandidate[]): AlignmentSnap[] {
  const result: AlignmentSnap[] = []
  for (const candidate of candidates) {
    const matches: AlignmentSnap[] = []
    for (const selected of alignmentPoints(rect)) for (const target of candidate.points) {
      const distance = Math.abs(selected.value - target.value)
      if (selected.axis === target.axis && distance <= ALIGN_EPSILON) matches.push({
        axis: selected.axis, selectedEdge: selected.edge, targetEdge: target.edge,
        targetValue: target.value, distance, target: candidate
      })
    }
    for (const axis of ['x', 'y'] as const) {
      const axisMatches = matches.filter(match => match.axis === axis)
      const dualEdges = axisMatches.some(match => match.selectedEdge === 'start' && match.targetEdge === 'start')
        && axisMatches.some(match => match.selectedEdge === 'end' && match.targetEdge === 'end')
      result.push(...axisMatches.filter(match => !dualEdges || match.selectedEdge !== 'center'))
    }
  }
  return result
}

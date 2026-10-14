<template>
  <div class="go-edit-align-line" :style="{ zIndex: overlayZIndex }">
    <!-- 画布级辅助线：主题色 / 青绿色 -->
    <div
      v-for="item in state.guideLines"
      :key="item.key"
      class="guide-line"
      :class="item.type"
      :style="getGuideLineStyle(item)"
    ></div>

    <!-- 参与对齐的组件边 / 中线高亮 -->
    <div
      v-for="item in state.highlightLines"
      :key="item.key"
      class="highlight-line"
      :class="item.type"
      :style="getHighlightLineStyle(item)"
    ></div>

    <!-- XD 风格测距线：绿色线 + 两端刻度 + 红色纯文字，文字不压在线上 -->
    <div
      v-for="item in state.measureLines"
      :key="item.key"
      class="measure-line"
      :class="item.type"
      :style="getMeasureLineStyle(item)"
    >
      <span class="measure-label">{{ item.distance }}px</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, watch, onBeforeUnmount } from 'vue'
import { useDesignStore } from '@/store/modules/designStore/designStore'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { EditCanvasTypeEnum } from '@/store/modules/chartEditStore/chartEditStore.d'
import { useSettingStore } from '@/store/modules/settingStore/settingStore'
import { CreateComponentType, CreateComponentGroupType } from '@/packages/index.d'
import { setComponentPosition } from '@/utils'
import throttle from 'lodash/throttle'
import { nearestAlignmentSnaps, visibleAlignmentSnaps, ALIGN_EPSILON } from '@/utils/alignment'
import { selectBoxIndex } from '@/settings/designSetting'

type Axis = 'x' | 'y'
type EdgeType = 'start' | 'center' | 'end'
type LineType = 'row' | 'col'

type Rect = {
  x: number
  y: number
  w: number
  h: number
}

type AlignPoint = {
  axis: Axis
  edge: EdgeType
  value: number
}

type Candidate = {
  id: string
  rect: Rect
  points: AlignPoint[]
  isCanvas?: boolean
}

type SnapResult = {
  axis: Axis
  selectedEdge: EdgeType
  targetEdge: EdgeType
  targetValue: number
  distance: number
  target: Candidate
}

type GuideLine = {
  key: string
  type: LineType
  x?: number
  y?: number
  w?: number
  h?: number
}

type RenderLine = {
  key: string
  type: LineType
  x: number
  y: number
  w?: number
  h?: number
}

type MeasureLine = RenderLine & {
  distance: number
  labelSide?: 'top' | 'bottom' | 'left' | 'right'
}

type AlignGroup = {
  axis: Axis
  value: number
  items: Array<{
    id: string
    rect: Rect
    edge: EdgeType
    isSelected?: boolean
  }>
}

const designStore = useDesignStore()
const chartEditStore = useChartEditStore()
const settingStore = useSettingStore()

const XD_MEASURE_COLOR = '#21c7b7'
const XD_TEXT_COLOR = '#ff2d2d'
const HIGHLIGHT_COLOR = '#ff2d2d'
const MIN_GAP_FOR_LABEL = 6
const SNAP_THROTTLE = 16

const state = reactive({
  guideLines: [] as GuideLine[],
  highlightLines: [] as RenderLine[],
  measureLines: [] as MeasureLine[]
})

const themeColor = computed(() => designStore.getAppTheme || XD_MEASURE_COLOR)
const canvasScale = computed(() => Math.max(0.1, Number(chartEditStore.getEditCanvas.scale) || 1))
const alignRange = computed(() => Number(settingStore.getChartAlignRange || 0) / canvasScale.value)
const isDragging = computed(() => !!chartEditStore.getEditCanvas[EditCanvasTypeEnum.IS_DRAG])
const selectedIds = computed<string[]>(() => chartEditStore.getTargetChart.selectId || [])
const allComponents = computed(() => chartEditStore.getComponentList as Array<CreateComponentType | CreateComponentGroupType>)
const overlayZIndex = computed(() => Math.max(selectBoxIndex + 1, allComponents.value.length + 1))

const round = (value: number) => Math.round(value * 100) / 100
const right = (rect: Rect) => rect.x + rect.w
const bottom = (rect: Rect) => rect.y + rect.h
const centerX = (rect: Rect) => rect.x + rect.w / 2
const centerY = (rect: Rect) => rect.y + rect.h / 2

const clearRenderState = () => {
  state.guideLines.splice(0)
  state.highlightLines.splice(0)
  state.measureLines.splice(0)
}

const getRect = (attr: any): Rect => {
  return {
    x: Number(attr?.x ?? 0),
    y: Number(attr?.y ?? 0),
    w: Number(attr?.w ?? 0),
    h: Number(attr?.h ?? 0)
  }
}

const getComponentById = (id: string) => allComponents.value.find(item => item.id === id)

const getEdgeValue = (rect: Rect, axis: Axis, edge: EdgeType) => {
  if (axis === 'x') {
    if (edge === 'start') return rect.x
    if (edge === 'center') return centerX(rect)
    return right(rect)
  }

  if (edge === 'start') return rect.y
  if (edge === 'center') return centerY(rect)
  return bottom(rect)
}

const getPoints = (rect: Rect): AlignPoint[] => [
  { axis: 'x', edge: 'start', value: rect.x },
  { axis: 'x', edge: 'center', value: centerX(rect) },
  { axis: 'x', edge: 'end', value: right(rect) },
  { axis: 'y', edge: 'start', value: rect.y },
  { axis: 'y', edge: 'center', value: centerY(rect) },
  { axis: 'y', edge: 'end', value: bottom(rect) }
]

const getSelectedRect = (): Rect | null => {
  const selected = selectedIds.value
    .map(id => getComponentById(id))
    .filter(Boolean) as Array<CreateComponentType | CreateComponentGroupType>

  if (!selected.length) return null

  const rects = selected.map(item => getRect(item.attr))
  const minX = Math.min(...rects.map(item => item.x))
  const minY = Math.min(...rects.map(item => item.y))
  const maxX = Math.max(...rects.map(item => right(item)))
  const maxY = Math.max(...rects.map(item => bottom(item)))

  return {
    x: minX,
    y: minY,
    w: maxX - minX,
    h: maxY - minY
  }
}

const getCanvasCandidate = (): Candidate => {
  const rect: Rect = {
    x: 0,
    y: 0,
    w: Number(chartEditStore.getEditCanvasConfig.width || 0),
    h: Number(chartEditStore.getEditCanvasConfig.height || 0)
  }

  return {
    id: 'canvas',
    rect,
    points: getPoints(rect),
    isCanvas: true
  }
}

const getCandidates = (): Candidate[] => {
  const selectedSet = new Set(selectedIds.value)
  const componentCandidates = allComponents.value
    .filter(item => !selectedSet.has(item.id) && !item.status.hide)
    .map(item => {
      const rect = getRect(item.attr)
      return {
        id: item.id,
        rect,
        points: getPoints(rect)
      }
    })

  return [...componentCandidates, getCanvasCandidate()]
}

const findSnapResults = (selectedRect: Rect, candidates: Candidate[]): SnapResult[] => {
  return nearestAlignmentSnaps(selectedRect, candidates, alignRange.value)
}

const getMoveOffset = (selectedRect: Rect, snapResults: SnapResult[]) => {
  let offsetX = 0
  let offsetY = 0

  snapResults.forEach(item => {
    const selectedValue = getEdgeValue(selectedRect, item.axis, item.selectedEdge)
    const offset = item.targetValue - selectedValue

    if (item.axis === 'x') offsetX = offset
    if (item.axis === 'y') offsetY = offset
  })

  return { offsetX, offsetY }
}

const moveSelected = (offsetX: number, offsetY: number) => {
  if (!offsetX && !offsetY) return

  selectedIds.value.forEach(id => {
    const component = getComponentById(id)
    if (!component) return

    const rect = getRect(component.attr)
    setComponentPosition(component, rect.x + offsetX, rect.y + offsetY)
  })
}

const moveRect = (rect: Rect, offsetX: number, offsetY: number): Rect => ({
  ...rect,
  x: rect.x + offsetX,
  y: rect.y + offsetY
})

const setGuideLines = (selectedRect: Rect, snaps: SnapResult[]) => {
  const guides = new Map<string, GuideLine>()
  const padding = 8 / canvasScale.value
  snaps.forEach(item => {
    const value = getEdgeValue(selectedRect, item.axis, item.selectedEdge)
    const key = `${item.axis}-${round(value)}`
    const previous = guides.get(key)
    if (item.axis === 'x') {
      const y = Math.min(Math.min(selectedRect.y, item.target.rect.y) - padding, previous?.y ?? Infinity)
      const end = Math.max(Math.max(bottom(selectedRect), bottom(item.target.rect)) + padding, previous ? previous.y! + previous.h! : -Infinity)
      guides.set(key, { key, type: 'col', x: value, y, h: end - y })
    } else {
      const x = Math.min(Math.min(selectedRect.x, item.target.rect.x) - padding, previous?.x ?? Infinity)
      const end = Math.max(Math.max(right(selectedRect), right(item.target.rect)) + padding, previous ? previous.x! + previous.w! : -Infinity)
      guides.set(key, { key, type: 'row', x, y: value, w: end - x })
    }
  })
  state.guideLines.splice(0, state.guideLines.length, ...guides.values())
}

const getAlignedGroups = (selectedRect: Rect, candidates: Candidate[], snaps: SnapResult[]) => {
  const groups = new Map<string, AlignGroup>()

  const addItem = (axis: Axis, value: number, id: string, rect: Rect, edge: EdgeType, isSelected = false) => {
    const key = `${axis}-${round(value)}`
    if (!groups.has(key)) groups.set(key, { axis, value, items: [] })

    const group = groups.get(key)!
    const exists = group.items.some(item => item.id === id && item.edge === edge)
    if (exists) return

    group.items.push({ id, rect, edge, isSelected })
  }

  snaps.forEach(snap => {
    const selectedValue = getEdgeValue(selectedRect, snap.axis, snap.selectedEdge)

    addItem(snap.axis, selectedValue, 'selected', selectedRect, snap.selectedEdge, true)

    candidates.forEach(candidate => {
      if (candidate.isCanvas) return

      candidate.points.forEach(point => {
        if (point.axis !== snap.axis) return
        if (Math.abs(point.value - selectedValue) > ALIGN_EPSILON) return
        addItem(snap.axis, selectedValue, candidate.id, candidate.rect, point.edge)
      })
    })
  })

  return Array.from(groups.values())
}

const uniqueLineKey = (item: RenderLine) => [
  item.type,
  Math.round(item.x),
  Math.round(item.y),
  Math.round(item.w ?? 0),
  Math.round(item.h ?? 0),
  item.key
].join('-')

const buildHighlightLines = (groups: AlignGroup[]) => {
  const lines: RenderLine[] = []
  const added = new Set<string>()

  const push = (line: RenderLine) => {
    const key = uniqueLineKey(line)
    if (added.has(key)) return
    added.add(key)
    lines.push(line)
  }

  groups.forEach(group => {
    group.items.forEach(item => {
      if (group.axis === 'y') {
        push({
          key: `hl-row-${item.id}-${item.edge}-${round(group.value)}`,
          type: 'row',
          x: item.rect.x,
          y: group.value,
          w: item.rect.w
        })
      } else {
        push({
          key: `hl-col-${item.id}-${item.edge}-${round(group.value)}`,
          type: 'col',
          x: group.value,
          y: item.rect.y,
          h: item.rect.h
        })
      }
    })
  })

  state.highlightLines.splice(0, state.highlightLines.length, ...lines)
}

const pushMeasure = (list: MeasureLine[], item: Omit<MeasureLine, 'distance'>) => {
  const distance = item.type === 'row'
    ? Math.round(item.w ?? 0)
    : Math.round(item.h ?? 0)

  if (distance < MIN_GAP_FOR_LABEL) return

  list.push({
    ...item,
    distance
  })
}

const buildGapMeasures = (groups: AlignGroup[]) => {
  const measures: MeasureLine[] = []
  const measuredPairs = new Set<string>()

  groups.forEach(group => {
    if (group.items.length < 2) return

    if (group.axis === 'y') {
      const ordered = group.items
        .slice()
        .sort((a, b) => a.rect.x - b.rect.x)

      for (let i = 0; i < ordered.length - 1; i++) {
        const leftItem = ordered[i]
        const rightItem = ordered[i + 1]
        const gapStart = right(leftItem.rect)
        const gapEnd = rightItem.rect.x
        const gap = gapEnd - gapStart

        if (gap <= 0) continue
        const pairKey = `row-${leftItem.id}-${rightItem.id}`
        if (measuredPairs.has(pairKey)) continue
        measuredPairs.add(pairKey)

        pushMeasure(measures, {
          key: `measure-row-${round(group.value)}-${leftItem.id}-${rightItem.id}`,
          type: 'row',
          x: gapStart,
          y: group.value,
          w: gap,
          labelSide: 'top'
        })
      }
    } else {
      const ordered = group.items
        .slice()
        .sort((a, b) => a.rect.y - b.rect.y)

      for (let i = 0; i < ordered.length - 1; i++) {
        const topItem = ordered[i]
        const bottomItem = ordered[i + 1]
        const gapStart = bottom(topItem.rect)
        const gapEnd = bottomItem.rect.y
        const gap = gapEnd - gapStart

        if (gap <= 0) continue
        const pairKey = `col-${topItem.id}-${bottomItem.id}`
        if (measuredPairs.has(pairKey)) continue
        measuredPairs.add(pairKey)

        pushMeasure(measures, {
          key: `measure-col-${round(group.value)}-${topItem.id}-${bottomItem.id}`,
          type: 'col',
          x: group.value,
          y: gapStart,
          h: gap,
          labelSide: 'right'
        })
      }
    }
  })

  state.measureLines.splice(0, state.measureLines.length, ...measures)
}

const renderAlignment = (selectedRect: Rect, candidates: Candidate[], snaps: SnapResult[]) => {
  setGuideLines(selectedRect, snaps)
  const groups = getAlignedGroups(selectedRect, candidates, snaps)
  buildHighlightLines(groups)
  buildGapMeasures(groups)
}

const getGuideLineStyle = (item: GuideLine) => {
  if (item.type === 'row') {
    return {
      top: `${item.y ?? 0}px`,
      left: `${item.x ?? 0}px`,
      width: `${item.w ?? 0}px`,
      borderColor: themeColor.value
    }
  }

  return {
    left: `${item.x ?? 0}px`,
    top: `${item.y ?? 0}px`,
    height: `${item.h ?? 0}px`,
    borderColor: themeColor.value
  }
}

const getHighlightLineStyle = (item: RenderLine) => {
  if (item.type === 'row') {
    return {
      left: `${item.x}px`,
      top: `${item.y}px`,
      width: `${item.w ?? 0}px`,
      backgroundColor: HIGHLIGHT_COLOR
    }
  }

  return {
    left: `${item.x}px`,
    top: `${item.y}px`,
    height: `${item.h ?? 0}px`,
    backgroundColor: HIGHLIGHT_COLOR
  }
}

const getMeasureLineStyle = (item: MeasureLine) => {
  if (item.type === 'row') {
    return {
      left: `${item.x}px`,
      top: `${item.y}px`,
      width: `${item.w ?? 0}px`,
      '--measure-color': XD_MEASURE_COLOR,
      '--measure-text-color': XD_TEXT_COLOR
    }
  }

  return {
    left: `${item.x}px`,
    top: `${item.y}px`,
    height: `${item.h ?? 0}px`,
    '--measure-color': XD_MEASURE_COLOR,
    '--measure-text-color': XD_TEXT_COLOR
  }
}

const updateAlignment = throttle(() => {
  try {
    clearRenderState()
    // 只在真正拖拽时计算，避免点击选中时被吸附算法改变位置
    if (!isDragging.value || !selectedIds.value.length) return
    const selectedRect = getSelectedRect()
    if (!selectedRect) return
    const candidates = getCandidates()
    const snaps = findSnapResults(selectedRect, candidates)
    if (!snaps.length) return
    const { offsetX, offsetY } = getMoveOffset(selectedRect, snaps)
    moveSelected(offsetX, offsetY)
    const movedSelectedRect = moveRect(selectedRect, offsetX, offsetY)
    renderAlignment(movedSelectedRect, candidates, visibleAlignmentSnaps(movedSelectedRect, candidates))
  } catch (error) {
    console.log(error)
  }
}, SNAP_THROTTLE)
watch(() => chartEditStore.getMousePosition, updateAlignment, { deep: true })
onBeforeUnmount(() => updateAlignment.cancel())

watch(
  () => isDragging.value,
  value => {
    if (!value) { updateAlignment.cancel(); clearRenderState() }
  }
)
</script>

<style lang="scss" scoped>
@include go(edit-align-line) {
  // 作为画布中的独立顶层覆盖层，红线、测距线和文字不会被组件边框遮挡。
  position: absolute;
  inset: 0;
  isolation: isolate;
  overflow: visible;
  pointer-events: none;
  user-select: none;

  .guide-line,
  .highlight-line,
  .measure-line {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: none;
    user-select: none;
  }

  .guide-line {
    z-index: 20;
    opacity: 0.65;
    border-style: solid;
    border-width: 1px;

    &.row {
      width: 100%;
      height: 0;
      border-left: none;
      border-right: none;
      border-bottom: none;
    }

    &.col {
      width: 0;
      height: 100%;
      border-top: none;
      border-right: none;
      border-bottom: none;
    }
  }

  .highlight-line {
    z-index: 210;
    opacity: 0.95;

    &.row {
      height: 2px;
      transform: translateY(-1px);
    }

    &.col {
      width: 2px;
      transform: translateX(-1px);
    }
  }

  .measure-line {
    z-index: 320;
    overflow: visible;
    background: transparent;
    --measure-color: #21c7b7;
    --measure-text-color: #ff2d2d;

    &::before,
    &::after {
      content: '';
      position: absolute;
      background-color: var(--measure-color);
    }

    &.row {
      height: 16px;
      transform: translateY(-8px);
      border-left: 1px solid var(--measure-color);
      border-right: 1px solid var(--measure-color);

      &::before {
        left: 0;
        right: 0;
        top: 50%;
        height: 1px;
        transform: translateY(-0.5px);
      }

      &::after {
        display: none;
      }

      .measure-label {
        left: 50%;
        top: -8px;
        transform: translate(-50%, -100%);
      }
    }

    &.col {
      width: 16px;
      transform: translateX(-8px);
      border-top: 1px solid var(--measure-color);
      border-bottom: 1px solid var(--measure-color);

      &::before {
        top: 0;
        bottom: 0;
        left: 50%;
        width: 1px;
        transform: translateX(-0.5px);
      }

      &::after {
        display: none;
      }

      .measure-label {
        left: 22px;
        top: 50%;
        transform: translateY(-50%);
      }
    }

    .measure-label {
      position: absolute;
      display: block;
      padding: 0;
      margin: 0;
      background: transparent;
      border: none;
      box-shadow: none;
      color: var(--measure-text-color);
      font-size: 12px;
      font-weight: 600;
      line-height: 14px;
      white-space: nowrap;
      pointer-events: none;
      user-select: none;
      text-shadow: none;
    }
  }
}
</style>

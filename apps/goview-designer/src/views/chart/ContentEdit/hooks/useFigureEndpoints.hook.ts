import { onBeforeUnmount } from 'vue'
import cloneDeep from 'lodash/cloneDeep'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartHistoryStore } from '@/store/modules/chartHistoryStore/chartHistoryStore'
import { EditCanvasTypeEnum } from '@/store/modules/chartEditStore/chartEditStore.d'
import { constrainLineEnd, lineWorldPoints, setLineEndpoints } from '@/utils/figureLine'

export function useFigureEndpoints() {
  const store = useChartEditStore(), history = useChartHistoryStore()
  let stop: (() => void) | undefined
  onBeforeUnmount(() => stop?.())
  return (event: MouseEvent, endpoint: 'start' | 'end', component: any) => {
    if (event.button !== 0 || component.status?.lock) return
    event.preventDefault(); event.stopPropagation(); stop?.()
    ;(document.activeElement as HTMLElement | null)?.blur()
    const before = cloneDeep(component), original = lineWorldPoints(component.attr, component.option)
    const moving = original[endpoint], fixed = original[endpoint === 'start' ? 'end' : 'start']
    const scale = Math.max(.1, store.getEditCanvas.scale), wasLocked = store.getEditCanvas.lockScale
    let frame = 0, latest: MouseEvent | null = null
    store.setTargetSelectChart(component.id)
    store.setEditCanvas(EditCanvasTypeEnum.LOCK_SCALE, true)
    store.setEditCanvas(EditCanvasTypeEnum.IS_DRAG, true)
    const apply = () => {
      frame = 0
      if (!latest) return
      const point = constrainLineEnd(fixed, { x: moving.x + (latest.clientX - event.clientX) / scale, y: moving.y + (latest.clientY - event.clientY) / scale }, latest.shiftKey)
      setLineEndpoints(component, endpoint === 'start' ? point : fixed, endpoint === 'end' ? point : fixed)
    }
    const move = (e: MouseEvent) => { latest = e; if (!frame) frame = requestAnimationFrame(apply) }
    const cleanup = () => {
      if (frame) cancelAnimationFrame(frame)
      document.removeEventListener('mousemove', move); document.removeEventListener('mouseup', up); document.removeEventListener('keydown', key)
      store.setEditCanvas(EditCanvasTypeEnum.IS_DRAG, false)
      store.setEditCanvas(EditCanvasTypeEnum.LOCK_SCALE, wasLocked)
      stop = undefined
    }
    const up = (e: MouseEvent) => {
      latest = e; if (frame) cancelAnimationFrame(frame); apply()
      if (JSON.stringify(component.attr) !== JSON.stringify(before.attr) || JSON.stringify(component.option) !== JSON.stringify(before.option)) history.createUpdateHistory([before])
      cleanup()
    }
    const key = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      component.attr = cloneDeep(before.attr); component.option = cloneDeep(before.option)
      cleanup()
    }
    stop = cleanup
    document.addEventListener('mousemove', move); document.addEventListener('mouseup', up); document.addEventListener('keydown', key)
  }
}

import { shallowReactive } from 'vue'

export type EventAttributeSnapshot = { option: any; attr: any; styles: any; chartConfig: any }
const previews = shallowReactive(new Map<string, { owner: symbol; snapshot: EventAttributeSnapshot; renderKey: symbol }>())

export function setEventAttributePreview(id: string, owner: symbol, snapshot: EventAttributeSnapshot) {
  const current = previews.get(id)
  if (current?.owner === owner && ['option', 'attr', 'styles', 'chartConfig'].every(key => current.snapshot[key as keyof EventAttributeSnapshot] === snapshot[key as keyof EventAttributeSnapshot])) return
  previews.set(id, { owner, snapshot, renderKey: Symbol('event-preview-render') })
}
// 部分组件在 setup 时使用 toRefs 固定配置对象。进入/退出预览只重建一次，
// 预览过程中保留本地响应式对象引用，使每次改值都能实时渲染。
export const eventPreviewRenderKey = (id: string): string | symbol => previews.get(id)?.renderKey || id
export function clearEventAttributePreview(id: string, owner: symbol) {
  if (previews.get(id)?.owner === owner) previews.delete(id)
}

// 仅编辑画布使用覆盖层，Store、撤销记录、项目保存及运行预览仍读取原始组件。
export function eventPreviewComponents<T extends { id: string; groupList?: any[] }>(components: T[]): T[] {
  return components.map(component => {
    const preview = previews.get(component.id)?.snapshot
    let result = preview ? { ...component, ...preview } : component
    if (Array.isArray(component.groupList)) {
      const children = eventPreviewComponents(component.groupList)
      if (children.some((child, index) => child !== component.groupList![index])) result = { ...result, groupList: children }
    }
    return result
  })
}

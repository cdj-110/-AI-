import { PickCreateComponentType } from '@/packages/index.d'
import { EditCanvasConfigType } from '@/store/modules/chartEditStore/chartEditStore.d'

type AttrType = PickCreateComponentType<'attr'>
type StatusType = PickCreateComponentType<'status'>
type PreviewConfig = PickCreateComponentType<'preview'>

// 设置位置
export const getComponentAttrStyle = (attr: AttrType, index: number) => {
  const componentStyle = {
    zIndex: index + 1,
    left: `${attr.x}px`,
    top: `${attr.y}px`
  }
  return componentStyle
}

// 设置大小
export const getSizeStyle = (attr: AttrType, scale?: number) => {
  return {
    width: `${scale ? scale * attr.w : attr.w}px`,
    height: `${scale ? scale * attr.h : attr.h}px`
  }
}

// 组件内容使用独立尺寸，容器仍使用 w/h。
export const getContentSizeStyle = (attr: AttrType, scale?: number) => {
  const width = typeof attr.contentW === 'number' ? attr.contentW : attr.w
  const height = typeof attr.contentH === 'number' ? attr.contentH : attr.h
  return {
    position: 'absolute' as const,
    left: `${(attr.contentX ?? 0) * (scale ?? 1)}px`,
    top: `${(attr.contentY ?? 0) * (scale ?? 1)}px`,
    width: `${scale ? scale * width : width}px`,
    height: `${scale ? scale * height : height}px`
  }
}

// 设置状态样式
export const getStatusStyle = (attr: StatusType) => {
  return {
    display: attr.hide ? 'none' : 'block'
  }
}

// 设置预览配置样式
export const getPreviewConfigStyle = (previewConfig: PreviewConfig) => {
  const previewStyle: Partial<CSSStyleDeclaration> = {}
  if (previewConfig) {
    if (previewConfig.overFlowHidden) {
      previewStyle.overflow = 'hidden'
    }
  }
  return previewStyle
}

// 全局样式
export const getEditCanvasConfigStyle = (canvas: EditCanvasConfigType) => {
  // 背景
  const computedBackground = canvas.selectColor
    ? { background: canvas.background }
    : {
        background: `url(${canvas.backgroundImage}) center center / cover no-repeat !important`
      }
  return {
    position: 'relative' as const,
    width: canvas.width ? `${canvas.width || 100}px` : '100%',
    height: canvas.height ? `${canvas.height}px` : '100%',
    ...computedBackground
  }
}

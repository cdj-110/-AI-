import { PickCreateComponentType } from '@/packages/index.d'
import { computed } from 'vue'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
type AttrType = PickCreateComponentType<'attr'>

export const useComponentStyle = (attr: AttrType, index: number) => {
  if(!attr) return {}
  const componentStyle = {
    zIndex: index + 1,
    left: `${attr.x}px`,
    top: `${attr.y}px`,
    width: `${attr.w}px`,
    height: `${attr.h}px`
  }
  return componentStyle
}

export const useSizeStyle = (attr: AttrType, scale?: number) => {
  if(!attr) return {}
  return {
    width: `${scale ? scale * attr.w : attr.w}px`,
    height: `${scale ? scale * attr.h : attr.h}px`
  }
}

export const usePointStyle = (point: string, index: number, attr: any, cursorResize: string[]) => {
    const { w: width, h: height } = attr
    const centerX = width / 2
    const centerY = height / 2

    let top = 0
    let left = 0

    switch (point) {
        case 'lt':
            top = 0
            left = 0
            break
        case 'l':
            top = centerY
            left = 0
            break
        case 'lb':
            top = height
            left = 0
            break
        case 'rt':
            top = 0
            left = width
            break
        case 'r':
            top = centerY
            left = width
            break
        case 'rb':
            top = height
            left = width
            break
        case 't':
            top = 0
            left = centerX
            break
        case 'b':
            top = height
            left = centerX
            break
    }

    return {
        top: `${top}px`,
        left: `${left}px`,
        cursor: `${cursorResize[index % cursorResize.length]}-resize`
    }
}

// 组件本体尺寸与选框（容器）尺寸分离。
export const useContentSizeStyle = (attr: AttrType, scale?: number) => {
  if (!attr) return {}
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

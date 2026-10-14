<template>
  <div class="go-sketch-rule">
    <sketch-rule
      v-if="sketchRuleReDraw"
      :thick="thick"
      :scale="scale"
      :width="canvasBox().width"
      :height="canvasBox().height"
      :startX="startX"
      :startY="startY"
      :lines="lines"
      :palette="paletteStyle"
    >
    </sketch-rule>
    <div ref="$app" class="edit-screens" @mousedown="dragCanvas" @scroll="handleScroll" @pointermove="rememberPointer">
      <div ref="$container" class="edit-screen-container" :style="{ width: containerWidth }">
        <div
          ref="refSketchRuleBox"
          class="canvas"
          @mousedown="dragCanvas"
          :style="{ marginLeft: '-' + (canvasBox().width / 2 - 25) + 'px', transform: `translate(${canvasShift.x}px, calc(-50% + ${canvasShift.y}px))` }"
        >
          <div :style="{ pointerEvents: isPressSpace ? 'none' : 'auto' }">
            <slot></slot>
          </div>
        </div>
      </div>
    </div>
    <!-- 修复右下角白点用的 -->
    <!-- <div v-if="designStore.getDarkTheme" class="fix-edit-screens-block"></div> -->
  </div>
</template>
<script setup lang="ts">
import { ref, reactive, onMounted, toRefs, watch, onUnmounted, computed, nextTick } from 'vue'
import { listen } from 'dom-helpers'
import { useDesignStore } from '@/store/modules/designStore/designStore'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartLayoutStore } from '@/store/modules/chartLayoutStore/chartLayoutStore'
import { ChartLayoutStoreEnum } from '@/store/modules/chartLayoutStore/chartLayoutStore.d'
import throttle from 'lodash/throttle'
import debounce from 'lodash/debounce'
import { captureZoomAnchor, zoomScrollCorrection, zoomTranslationRemainder, wheelZoomScale, ZoomAnchor } from '@/utils/canvasZoom'

const chartEditStore = useChartEditStore()
const chartLayoutStore = useChartLayoutStore()
const designStore = useDesignStore()

const thick = 20
let prevMoveXValue = [0, 0]
let prevMoveYValue = [0, 0]

const $app = ref()
const sketchRuleReDraw: any = ref(true)
const refSketchRuleBox: any = ref()
const $container = ref()
const isPressSpace = ref(false)
const cursorStyle = ref('auto')
const canvasShift = reactive({ x: 0, y: 0 })
let lastPointer: { x: number; y: number } | null = null
let pendingZoom: ZoomAnchor | null = null
let wheelFrame = 0
let queuedWheelScale: number | null = null
let unmounted = false
const { width, height } = toRefs(chartEditStore.getEditCanvasConfig)
const startX: any = ref(0)
const startY: any = ref(0)
const lines = reactive({ h: [], v: [] })

const scale = computed(() => {
  return chartEditStore.getEditCanvas.scale
})

// 滚动条拖动的宽度
const containerWidth = computed(() => {
  return `${window.innerWidth * 2}px`
})

// 滚动条拖动的高度
const containerHeight = computed(() => {
  return `${height.value * 2}px`
})

// 主题
const paletteStyle = computed(() => {
  const isDarkTheme = designStore.getDarkTheme
  return isDarkTheme
    ? {
        bgColor: '#18181c',
        longfgColor: '#4d4d4d',
        shortfgColor: '#4d4d4d',
        fontColor: '#4d4d4d',
        shadowColor: '#18181c',
        borderColor: '#18181c',
        cornerActiveColor: '#18181c'
      }
    : {}
})

// 颜色
const themeColor = computed(() => {
  return designStore.getAppTheme
})

// 处理鼠标拖动
const rememberPointer = (e: PointerEvent | WheelEvent) => {
  const viewport = $app.value?.getBoundingClientRect()
  if (viewport) lastPointer = { x: e.clientX - viewport.left, y: e.clientY - viewport.top }
}
const handleWheel = (e: WheelEvent) => {
  if (e.ctrlKey || e.metaKey) {
    e.preventDefault()
    rememberPointer(e)
    queuedWheelScale = wheelZoomScale(queuedWheelScale ?? scale.value, e.deltaY)
    if (!wheelFrame) wheelFrame = requestAnimationFrame(() => {
      wheelFrame = 0
      const nextScale = queuedWheelScale
      queuedWheelScale = null
      if (!unmounted && nextScale !== null && nextScale !== scale.value) chartEditStore.setScale(nextScale)
    })
  }
}

// 在 setScale 改写 DOM 尺寸前捕获锚点，覆盖滚轮、滑块和比例菜单。
const stopZoomSubscription = chartEditStore.$onAction(({ name, args, after }) => {
  if (name !== 'setScale' || unmounted || !$app.value || !chartEditStore.getEditCanvas.editContentDom) return
  if (chartEditStore.getEditCanvas.lockScale && !args[1]) return
  const requested = Number(args[0])
  if (!Number.isFinite(requested) || requested <= 0 || requested === scale.value) return
  const canvas = chartEditStore.getEditCanvas.editContentDom.querySelector('.go-edit-range') as HTMLElement | null
  if (!canvas) return
  const viewport = $app.value.getBoundingClientRect()
  const pointer = lastPointer || { x: viewport.width / 2, y: viewport.height / 2 }
  const anchor = pendingZoom || captureZoomAnchor(
    { x: viewport.left + pointer.x, y: viewport.top + pointer.y }, canvas.getBoundingClientRect(), scale.value
  )
  pendingZoom = anchor
  after(async () => {
    await nextTick()
    if (unmounted || pendingZoom !== anchor || !$app.value) return
    const correction = zoomScrollCorrection(anchor, canvas.getBoundingClientRect(), scale.value)
    const before = { x: $app.value.scrollLeft, y: $app.value.scrollTop }
    $app.value.scrollLeft += correction.x
    $app.value.scrollTop += correction.y
    const remainder = zoomTranslationRemainder(correction, {
      x: $app.value.scrollLeft - before.x, y: $app.value.scrollTop - before.y
    })
    canvasShift.x += remainder.x
    canvasShift.y += remainder.y
    pendingZoom = null
    await nextTick()
    if (!unmounted) { handleScroll(); redrawAfterZoom() }
  })
})

// 滚动条处理
const handleScroll = () => {
  if (!$app.value) return
  const screensRect = $app.value.getBoundingClientRect()
  const canvasRect = refSketchRuleBox.value.getBoundingClientRect()
  // 标尺开始的刻度
  startX.value = (screensRect.left + thick - canvasRect.left) / scale.value
  startY.value = (screensRect.top + thick - canvasRect.top) / scale.value
}

// 拖拽处理
const dragCanvas = (e: any) => {
  e.preventDefault()
  e.stopPropagation()
  
  if (e.which == 2) isPressSpace.value = true
  else if (!window.$KeyboardActive?.space) return

  isPressSpace.value = true

  // @ts-ignore
  document.activeElement?.blur()

  const startX = e.pageX
  const startY = e.pageY

  const listenMousemove = listen(window, 'mousemove', (e: any) => {
    const nx = e.pageX - startX
    const ny = e.pageY - startY

    const [prevMoveX1, prevMoveX2] = prevMoveXValue
    const [prevMoveY1, prevMoveY2] = prevMoveYValue

    prevMoveXValue = [prevMoveX2, nx]
    prevMoveYValue = [prevMoveY2, ny]

    $app.value.scrollLeft -=
      prevMoveX2 > prevMoveX1 ? Math.abs(prevMoveX2 - prevMoveX1) : -Math.abs(prevMoveX2 - prevMoveX1)
    $app.value.scrollTop -=
      prevMoveY2 > prevMoveY1 ? Math.abs(prevMoveY2 - prevMoveY1) : -Math.abs(prevMoveY2 - prevMoveY1)
  })

  const listenMouseup = listen(window, 'mouseup', () => {
    listenMousemove()
    listenMouseup()
    prevMoveXValue = [0, 0]
    prevMoveYValue = [0, 0]
    isPressSpace.value = false
  })
}

// 计算画布大小
const canvasBox = () => {
  const layoutDom = document.getElementById('go-chart-edit-layout')
  if (layoutDom) {
    // 此处减去滚动条的宽度和高度 
    const scrollW = 20
    return {
      height: layoutDom.clientHeight - scrollW,
      width: layoutDom.clientWidth - scrollW
    }
  }
  return {
    width: width.value,
    height: height.value
  }
}

// 重绘标尺
const reDraw = throttle(() => {
  sketchRuleReDraw.value = false
  setTimeout(() => {
    sketchRuleReDraw.value = true
  }, 10)
},20)
// 标尺会响应起始坐标变化；仅在一轮缩放停止后补一次完整重绘。
const redrawAfterZoom = debounce(() => reDraw(), 120)

let iii = 0;

// 滚动居中
const canvasPosCenter = () => {
  const { width: containerWidth, height: containerHeight } = $container.value.getBoundingClientRect()
  const { width, height } = canvasBox()
  if(iii == 0){
    $app.value.scrollLeft = containerWidth / 2 - width / 2;
    $app.value.scrollTop = containerHeight / 2 - height / 2;
    iii++;
  }

  // $container.value.addEventListener("mousemove",(e) => {
  //   if (e.ctrlKey) {
  //     console.log(e)
  //     $app.value.scrollLeft = containerWidth/ 2 - e.pageX
  //     $app.value.scrollTop = containerHeight / 2 - e.pageY
  //   }
  // })
}

// 处理主题变化
watch(
  () => designStore.getDarkTheme,
  () => {
    reDraw()
  }
)

// 处理标尺重制大小
watch(() => scale.value, async (newValue, oldValue) => {
  if (oldValue !== newValue && chartLayoutStore.getRePositionCanvas) {
    chartLayoutStore.setItemUnHandle(ChartLayoutStoreEnum.RE_POSITION_CANVAS, false)
  }
  if (pendingZoom) return
  await nextTick()
  if (!unmounted) { canvasPosCenter(); handleScroll(); redrawAfterZoom() }
})

// 处理鼠标样式
watch(
  () => isPressSpace.value,
  newValue => {
    cursorStyle.value = newValue ? 'grab' : 'auto'
  }
)

onMounted(() => {
  if ($app.value) {
    $app.value.addEventListener('wheel', handleWheel, { passive: false })
    canvasPosCenter()
  }
})

onUnmounted(() => {
  unmounted = true
  pendingZoom = null
  stopZoomSubscription()
  if (wheelFrame) cancelAnimationFrame(wheelFrame)
  queuedWheelScale = null
  redrawAfterZoom.cancel()
  reDraw.cancel()
  if ($app.value) {
    $app.value.removeEventListener('wheel', handleWheel)
  }
})

window.onKeySpacePressHold = (isHold: boolean) => {
  isPressSpace.value = isHold
}
</script>

<style>
/* 使用 SCSS 会报错，直接使用最基础的 CSS 进行修改，
此库有计划 Vue3 版本，但是开发的时候还没发布 */
#mb-ruler {
  top: 0;
  left: 0;
}

/* 横线 */
#mb-ruler .v-container .lines .line {
  /* 最大缩放 200% */
  width: 200vw !important;
  border-top: 1px dashed v-bind('themeColor') !important;
}

#mb-ruler .v-container .indicator {
  border-bottom: 1px dashed v-bind('themeColor') !important;
}

/* 竖线 */
#mb-ruler .h-container .lines .line {
  /* 最大缩放 200% */
  height: 200vh !important;
  border-left: 1px dashed v-bind('themeColor') !important;
}

#mb-ruler .h-container .indicator {
  border-left: 1px dashed v-bind('themeColor') !important;
}

/* 坐标数值背景颜色 */
#mb-ruler .indicator .value {
  background-color: rgba(0, 0, 0, 0);
}

/* 删除按钮 */
#mb-ruler .line .del {
  padding: 0;
  color: v-bind('themeColor');
  font-size: 26px;
  font-weight: bolder;
}

#mb-ruler .corner {
  border-width: 0 !important;
}
</style>

<style lang="scss" scoped>
@include go('sketch-rule') {
  overflow: hidden;
  width: 100%;
  height: 100%;

  .edit-screens {
    position: absolute;
    width: 100%;
    height: 100%;
    overflow: auto;
    overflow-anchor: none;
    scroll-behavior: auto;
    user-select: none;
    padding-bottom: 0px;

    /* 隐藏滚动条，但保留滚轮、触控板及空格拖动画布的滚动能力 */
    scrollbar-width: none;
    -ms-overflow-style: none;

    &::-webkit-scrollbar {
      display: none;
      width: 0;
      height: 0;
    }
    //处理鼠标在画布外面，拖动的时候鼠标样式的改变
    &:hover {
      cursor: v-bind('cursorStyle');
    }
    &:active {
      cursor: crosshair;
    }
  }

  .fix-edit-screens-block {
    position: absolute;
    bottom: 0;
    right: 0;
    width: 10px;
    height: 10px;
    background-color: $--color-dark-bg-1;
  }

  .edit-screen-container {
    position: absolute;
    height: v-bind('containerHeight');
    top: 0;
    left: 0;
  }

  .canvas {
    position: absolute;
    top:50%;
    left: 50%;
    transform-origin: 50% 0;
    transform: translateY(-50%);

    &:hover {
      cursor: v-bind('cursorStyle');
    }

    &:active {
      cursor: crosshair;
    }
  }
}
</style>

<template>
    <div class="go-shape-box" :class="{ lock, hide, 'line-shape': isLine, 'drag-aligning': isMoving }">
        <slot></slot>

        <!-- 锚点 -->
        <template v-if="!hiddenPoint && isLine && select">
            <div v-for="endpoint in lineEndpoints" :key="endpoint.name" class="shape-point line-endpoint"
                :data-endpoint="endpoint.name" :style="lineEndpointStyle(endpoint.point)"
                @mousedown="dragLineEndpoint($event, endpoint.name, item)"></div>
        </template>
        <template v-else-if="!hiddenPoint && !isLine">
            <div
                :class="`shape-point ${point} ${point === activePoint ? 'active' : ''}`"
                v-for="(point, index) in select ? pointList : []"
                :key="index"
                :style="usePointStyle(point, index, item.attr, cursorResize)"
                title="拖动：调整组件大小；Shift + 四角：等比例；Ctrl：仅调整容器；Ctrl + Shift + 四角：仅等比例调整容器"
                @mousedown="handlePointMouseDown($event, point, item.attr)"
            ></div>
        </template>

        <!-- 选中框 -->
        <div v-if="!isLine" class="shape-modal" :style="useSizeStyle(item.attr)">
            <div class="shape-modal-select" :class="{ active: select }"></div>
            <div class="shape-modal-change" :class="{ selectActive: select, hoverActive: hover }"></div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, PropType, ref, onMounted, onUnmounted } from 'vue'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { EditCanvasTypeEnum } from '@/store/modules/chartEditStore/chartEditStore.d'
import { useDesignStore } from '@/store/modules/designStore/designStore'
import { CreateComponentType, CreateComponentGroupType } from '@/packages/index.d'
import { useSizeStyle, usePointStyle } from '../../hooks/useStyle.hook'
import { useMousePointHandle } from '../../hooks/useDrag.hook'
import { useFigureEndpoints } from '../../hooks/useFigureEndpoints.hook'
import { isFigureLine, renderedLine } from '@/utils/figureLine'

const props = defineProps({
    item: {
        type: Object as PropType<CreateComponentType | CreateComponentGroupType>,
        required: true
    },
    hiddenPoint: {
        type: Boolean,
        required: false
    }
})

const designStore = useDesignStore()
const chartEditStore = useChartEditStore()
const isLine = computed(() => isFigureLine(props.item))
const isMoving = computed(() => !!chartEditStore.getEditCanvas[EditCanvasTypeEnum.IS_MOVING])
const dragLineEndpoint = useFigureEndpoints()
const lineEndpoints = computed(() => {
    const line = renderedLine(props.item.attr, props.item.option)
    return [{ name: 'start' as const, point: line.start }, { name: 'end' as const, point: line.end }]
})
const lineEndpointStyle = (point: { x: number; y: number }) => {
    const scale = Math.max(.1, chartEditStore.getEditCanvas.scale)
    return { left: `${point.x + (props.item.attr.contentX || 0)}px`, top: `${point.y + (props.item.attr.contentY || 0)}px`, width: `${9 / scale}px`, height: `${9 / scale}px`, borderWidth: `${1 / scale}px`, cursor: 'crosshair' }
}

// 锚点列表
const pointList = ['t', 'r', 'b', 'l', 'lt', 'rt', 'lb', 'rb']

// 光标方向
const cursorResize = ['n', 'e', 's', 'w', 'nw', 'ne', 'sw', 'se']

// 主题颜色
const themeColor = computed(() => designStore.getAppTheme)

// 是否悬停
const hover = computed(() => {
    const isDrag = chartEditStore.getEditCanvas[EditCanvasTypeEnum.IS_DRAG]
    if (isDrag) return false
    if (props.item.status.lock) return false
    return props.item.id === chartEditStore.getTargetChart.hoverId
})

// 是否选中
const select = computed(() => {
    const id = props.item.id
    if (props.item.status.lock) return false
    return chartEditStore.getTargetChart.selectId.includes(id)
})

// 是否锁定
const lock = computed(() => props.item.status.lock)

// 是否隐藏
const hide = computed(() => props.item.status.hide)

// 激活的锚点
const activePoint = ref<string | null>(null)

// 处理锚点点击
const originalMousePointHandle = useMousePointHandle
const handlePointMouseDown = (e: MouseEvent, point: string, attr: any) => {
    activePoint.value = point
    originalMousePointHandle(e, point, attr, props.item)
}

// 鼠标松开时清除激活状态
const handleDocumentMouseUp = () => {
    activePoint.value = null
}

onMounted(() => {
    document.addEventListener('mouseup', handleDocumentMouseUp)
})

onUnmounted(() => {
    document.removeEventListener('mouseup', handleDocumentMouseUp)
})
</script>

<style lang="scss" scoped>
@include go(shape-box) {
    position: absolute;
    cursor: move;
    &.line-shape { pointer-events: none; }
    .line-endpoint { pointer-events: auto; }
    // 只隐藏编辑器叠加层，不影响组件自身边框，也不遮挡上层对齐辅助线。
    &.drag-aligning {
        .shape-modal, .shape-point { visibility: hidden; pointer-events: none; }
    }

    &.lock {
        cursor: default !important;
    }

    &.hide {
        display: none;
    }

    /* 锚点样式 */
    .shape-point {
        z-index: 1;
        position: absolute;
        width: 7px;
        height: 7px;
        border: 1px solid v-bind('themeColor');
        border-radius: 50%;
        background-color: #fff;
        transform: translate(-50%, -50%);

        &.active {
            background-color: #51d6a9;
            border-color: #51d6a9;
        }

        &.t {
            top: 0;
            left: 50%;
        }

        &.b {
            bottom: 0;
            left: 50%;
        }

        &.l {
            left: 0;
            top: 50%;
        }

        &.r {
            right: 0;
            top: 50%;
        }

        &.lt {
            top: 0;
            left: 0;
        }

        &.rt {
            top: 0;
            right: 0;
        }

        &.lb {
            bottom: 0;
            left: 0;
        }

        &.rb {
            bottom: 0;
            right: 0;
        }
    }

    /* 选中框 */
    .shape-modal {
        position: absolute;
        top: 0;
        left: 0;

        .shape-modal-select,
        .shape-modal-change {
            position: absolute;
            width: 100%;
            height: 100%;
            border-radius: 4px;
        }

        .shape-modal-select {
            opacity: 0.1;
            top: 0;
            left: 0;

            &.active {
                background-color: v-bind('themeColor');
            }
        }

        .shape-modal-change {
            border: 1px solid rgba(0, 0, 0, 0);

            &.selectActive,
            &.hoverActive {
                border-color: v-bind('themeColor');
                border-width: 1px !important;
            }

            &.hoverActive {
                border-style: dotted;
            }

            &.selectActive {
                border-style: solid;
            }
        }
    }
}
</style>

<template>
    <div class="go-shape-box" :class="{ lock, hide }">
        <slot></slot>
        <!-- 锚点 -->
        <template v-if="!hiddenPoint">
            <!-- 添加active类判断 -->
            <div :class="`shape-point ${point} ${point === activePoint ? 'active' : ''}`"
                v-for="(point, index) in select ? pointList : []" :key="index"
                :style="usePointStyle(point, index, item.attr, cursorResize)"
                @mousedown="handlePointMouseDown($event, point, item.attr)"></div>
        </template>

        <!-- 选中 -->
        <div class="shape-modal" :style="useSizeStyle(item.attr)">
            <div class="shape-modal-select" :class="{ active: select }"></div>
            <div class="shape-modal-change" :class="{ selectActive: select, hoverActive: hover }"></div>
        </div>
    </div>
</template>

<script setup lang="ts">
// 改动3：导入ref、onMounted、onUnmounted
import { computed, PropType, ref, onMounted, onUnmounted } from 'vue'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { EditCanvasTypeEnum } from '@/store/modules/chartEditStore/chartEditStore.d'

import { useDesignStore } from '@/store/modules/designStore/designStore'
import { CreateComponentType, CreateComponentGroupType } from '@/packages/index.d'
import { useSizeStyle, usePointStyle } from '../../hooks/useStyle.hook'
import { useMousePointHandle } from '../../hooks/useDrag.hook'

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

// 锚点
const pointList = ['t', 'r', 'b', 'l', 'lt', 'rt', 'lb', 'rb']

// 光标朝向
const cursorResize = ['n', 'e', 's', 'w', 'nw', 'ne', 'sw', 'se']

// 颜色
const themeColor = computed(() => {
    return designStore.getAppTheme
})

// 计算当前选中目标
const hover = computed(() => {
    const isDrag = chartEditStore.getEditCanvas[EditCanvasTypeEnum.IS_DRAG]
    if (isDrag) return false

    if (props.item.status.lock) return false
    return props.item.id === chartEditStore.getTargetChart.hoverId
})

// 兼容多值场景
const select = computed(() => {
    const id = props.item.id
    if (props.item.status.lock) return false
    return chartEditStore.getTargetChart.selectId.find((e: string) => e === id)
})

// 锁定
const lock = computed(() => {
    return props.item.status.lock
})

// 隐藏
const hide = computed(() => {
    return props.item.status.hide
})

// 当前激活的锚点状态
const activePoint = ref<string | null>(null)

// 包装原有的拖拽处理函数，添加激活点设置
const originalMousePointHandle = useMousePointHandle
const handlePointMouseDown = (e: MouseEvent, point: string, attr: any) => {
    activePoint.value = point
    originalMousePointHandle(e, point, attr)
}
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

    &.lock {
        cursor: default !important;
    }

    &.hide {
        display: none;
    }

    /* 锚点 */
    /* 锚点 */
    .shape-point {
        z-index: 1;
        position: absolute;
        width: 7px;
        height: 7px;
        border: 1px solid v-bind('themeColor');
        border-radius: 50%;
        /* 圆形更稳定 */
        background-color: #fff;
        /* 所有锚点统一居中修正 */
        transform: translate(-50%, -50%);

        &.active {
            background-color: #51d6a9;
            border-color: #51d6a9;
        }

        /* 上边中点 */
        &.t {
            top: 0;
            left: 50%;
        }

        /* 下边中点 */
        &.b {
            bottom: 0;
            left: 50%;
        }

        /* 左边中点 */
        &.l {
            left: 0;
            top: 50%;
        }

        /* 右边中点 */
        &.r {
            right: 0;
            top: 50%;
        }

        /* 左上角 */
        &.lt {
            top: 0;
            left: 0;
        }

        /* 右上角 */
        &.rt {
            top: 0;
            right: 0;
        }

        /* 左下角 */
        &.lb {
            bottom: 0;
            left: 0;
        }

        /* 右下角 */
        &.rb {
            bottom: 0;
            right: 0;
        }
    }

    // .shape-point {
    //     z-index: 1;
    //     position: absolute;
    //     width: 7px;
    //     height: 7px;
    //     border: 1px solid v-bind('themeColor');
    //     border-radius: 5px;
    //     background-color: #fff;
    //     transform: translate(-40%, -30%);

    //     &.active {
    //         background-color: #51d6a9;
    //         border-color: #51d6a9;
    //     }

    //     &.t {
    //         //   width: 30px;
    //         transform: translate(-50%, -50%);
    //     }

    //     &.b {
    //         //   width: 30px;
    //         transform: translate(-50%, -30%);
    //     }

    //     &.l,
    //     &.r {
    //         //   height: 30px;
    //     }

    //     &.r {
    //         transform: translate(-20%, -50%);
    //     }

    //     &.l {
    //         transform: translate(-45%, -50%);
    //     }

    //     &.rt,
    //     &.rb {
    //         transform: translate(-30%, -30%);
    //     }
    // }

    /* 选中 */
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
            top: 2px;
            left: 2px;

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
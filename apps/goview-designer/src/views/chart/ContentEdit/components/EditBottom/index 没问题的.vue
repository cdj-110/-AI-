<template>
    <div class="go-edit-bottom">
        <n-space>
            <edit-history></edit-history>
            <div id="keyboard-dress-show" depth="3" style="width: 5.7vw;"></div>
        </n-space>

        <!-- 画布标签栏（加号固定在最右侧） -->
        <div class="target-bar">
            <!-- 左箭头：向前滚动 -->
            <n-button quaternary circle size="small" :disabled="scrollLeft <= 0" @click="handleScroll('left')"
                class="scroll-btn">
                <template #icon>
                    <n-icon size="16">
                        <ChevronBackOutline />
                    </n-icon>
                </template>
            </n-button>

            <!-- 滚动容器：只包含标签列表 -->
            <div class="tag-scroll-wrapper" ref="containerRef">
                <div class="tag-list" ref="tagListRef"
                    :style="{ transform: `translateX(-${scrollLeft}px)`, transition: 'transform 0.2s ease' }">
                    <!-- 画布标签 -->
                    <n-tag v-for="(item, index) in targetVal" :key="index" closable @close="targetclose(item)"
                        @click="targetChange(item)" :class="{ 'is-active': targetObj?.title === item.title }"
                        class="target-tag">
                        {{ item.title }}
                    </n-tag>
                </div>
            </div>

            <!-- 右箭头：向后滚动 -->
            <n-button quaternary circle size="small" :disabled="scrollLeft >= maxScrollWidth"
                @click="handleScroll('right')" class="scroll-btn">
                <template #icon>
                    <n-icon size="16">
                        <ChevronForward />
                    </n-icon>
                </template>
            </n-button>

            <!-- 加号按钮：固定在最右侧，不随标签滚动 -->
            <n-button quaternary circle size="small" @click="addCanvas" class="add-canvas-btn">
                <template #icon>
                    <n-icon size="16">
                        <AddIcon />
                    </n-icon>
                </template>
            </n-button>
        </div>

        <n-space class="bottom-ri">
            <edit-shortcut-key />
            <n-select ref="selectInstRef" class="scale-btn" v-model:value="filterValue" size="mini"
                :disabled="lockScale" :options="filterOptions" @update:value="selectHandle"></n-select>
            <n-tooltip trigger="hover">
                <template #trigger>
                    <n-button @click="lockHandle" text>
                        <n-icon class="lock-icon" :class="{ color: lockScale }" size="18" :depth="2">
                            <lock-closed-outline-icon v-if="lockScale"></lock-closed-outline-icon>
                            <lock-open-outline-icon v-else></lock-open-outline-icon>
                        </n-icon>
                    </n-button>
                </template>
                <span>{{ lockScale ? '解锁' : '锁定' }}当前比例</span>
            </n-tooltip>
            <n-slider class="scale-slider" v-model:value="sliderValue" :default-value="50" :min="10" :max="200"
                :step="5" :format-tooltip="sliderFormatTooltip" :disabled="lockScale" :marks="sliderMaks"
                @update:value="sliderHandle"></n-slider>
        </n-space>
    </div>
</template>

<script setup lang="ts">
import { AddIcon } from "naive-ui/lib/_internal/icons";
import { ChevronBackOutline, ChevronForward } from '@vicons/ionicons5';
import { SelectInst, useMessage } from 'naive-ui'
import { reactive, ref, toRefs, watchEffect, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { EditHistory } from '../EditHistory/index'
import EditShortcutKey from '../EditShortcutKey/index.vue'
import { useDesignStore } from '@/store/modules/designStore/designStore'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { EditCanvasTypeEnum } from '@/store/modules/chartEditStore/chartEditStore.d'
import { useChartLayoutStore } from '@/store/modules/chartLayoutStore/chartLayoutStore'
import { ChartLayoutStoreEnum } from '@/store/modules/chartLayoutStore/chartLayoutStore.d'
import { deepClone, goDialog, JSONParse, JSONStringify } from "@/utils";
import { useSync } from '@/views/chart/hooks/useSync.hook'

// 全局状态
const designStore = useDesignStore()
const themeColor = ref(designStore.getAppTheme)
const chartLayoutStore = useChartLayoutStore()
const chartEditStore = useChartEditStore()
const { lockScale, scale } = toRefs(chartEditStore.getEditCanvas)
const selectInstRef = ref<SelectInst | null>(null)
const { updateComponent } = useSync()

const message = useMessage();

// 初始化第一个页面，初始时list为null，等待数据加载后更新
const targetVal = ref([ { title: `画布${Date.now()}`, list: {} } ]);
const targetObj = ref<{ title: string,list: any } | {}>(targetVal.value[0]);

// 滚动核心变量
const scrollLeft = ref(0)
const maxScrollWidth = ref(0)
const containerRef = ref<HTMLElement | null>(null)
const tagListRef = ref<HTMLElement | null>(null)
const tagItemWidth = ref(0)

/**
 * 计算最大可滚动宽度
 */
const calcMaxScrollWidth = () => {
    nextTick(() => {
        if (!containerRef.value || !tagListRef.value) return

        const containerWidth = containerRef.value.clientWidth
        const listWidth = tagListRef.value.scrollWidth

        const firstTag = tagListRef.value.querySelector('.target-tag') as HTMLElement
        if (firstTag) {
            tagItemWidth.value = firstTag.offsetWidth + 8
        }

        maxScrollWidth.value = Math.max(0, listWidth - containerWidth)
        scrollLeft.value = Math.min(scrollLeft.value, maxScrollWidth.value)
    })
}

/**
 * 处理左右滚动
 */
const handleScroll = (direction: 'left' | 'right') => {
    if (tagItemWidth.value === 0) return

    if (direction === 'left') {
        scrollLeft.value = Math.max(0, scrollLeft.value - tagItemWidth.value)
    } else {
        scrollLeft.value = Math.min(maxScrollWidth.value, scrollLeft.value + tagItemWidth.value)
    }
}

// 页面操作工具类，统一管理画布操作
const unitConfig = {
    /** 清除当前画布所有内容 */
    clearClose: () => {
        chartEditStore.removeComponentList(
            chartEditStore.getStorageInfo()["componentList"].map(item => item.id),
            true
        )
    },

    /** 获取当前画布完整数据 */
    getViewVer: () => {
        return JSONParse(JSONStringify(chartEditStore.getStorageInfo()))
    },

    /** 创建一个空的画布结构 */
    createEmptyCanvas: () => {
        return {
            componentList: [],
            editCanvasConfig: JSONParse(JSONStringify(chartEditStore.getEditCanvasConfig)),
            requestGlobalConfig: JSONParse(JSONStringify(chartEditStore.getRequestGlobalConfig))
        }
    }
}

// 新增画布逻辑（先保存当前页面，再创建新页面）
const addCanvas = () => {
    const currentIndex = targetVal.value.findIndex(item => item.title === targetObj.value.title)
    if (currentIndex !== -1) {
        targetVal.value[currentIndex].list = unitConfig.getViewVer()
    }

    // let newPage = { title: `画布${targetVal.value.length + 1}`, list: unitConfig.createEmptyCanvas() };
    let newPage = { title: `画布${Date.now()}`, list: unitConfig.createEmptyCanvas() };
    targetVal.value.push(newPage);

    unitConfig.clearClose();
    targetObj.value = newPage;
}

// 删除画布逻辑
const targetclose = (e: any) => {
    goDialog({
        message: `是否确认删除【${e.title}】画布`,
        positiveText: '确认删除',
        negativeText: "取消删除",
        onPositiveCallback: () => {
            targetVal.value = targetVal.value.filter(item => item.title != e.title);
            const one = targetVal.value.findIndex(item => item.title === e.title);
            const two = targetVal.value.findIndex(item => item.title === targetObj.value.title)
            if(one == two){
                unitConfig.clearClose();
            }
        }
    })
}

// 切换画布逻辑
const targetChange = async (e: any) => {
    // 先保存当前页面的内容
    const currentIndex = targetVal.value.findIndex(item => item.title === targetObj.value.title)
    if (currentIndex !== -1 && targetObj.value.list !== null) {
        targetVal.value[currentIndex].list = unitConfig.getViewVer()
    }
    
    unitConfig.clearClose();
    targetObj.value = e;
    await updateComponent(e.list, true, false);
}

// 缩放相关逻辑
const filterOptions = [
    { label: '200%', value: 200 },
    { label: '150%', value: 150 },
    { label: '100%', value: 100 },
    { label: '50%', value: 50 },
    { label: '自适应', value: 0 }
]
const filterValue = ref('100%')
const selectHandle = (v: number) => {
    selectInstRef.value?.blur()
    if (v === 0) {
        chartLayoutStore.setItemUnHandle(ChartLayoutStoreEnum.RE_POSITION_CANVAS, true)
        chartEditStore.computedScale()
        return
    }
    chartEditStore.setScale(v / 100)
}

const lockHandle = () => {
    chartEditStore.setEditCanvas(EditCanvasTypeEnum.LOCK_SCALE, !lockScale.value)
}

const sliderValue = ref(100)
const sliderFormatTooltip = (v: string) => `${v}%`
const sliderHandle = (v: number) => {
    chartEditStore.setScale(v / 100)
}
const sliderMaks = reactive({ 100: '' })

// 合并重复的watchEffect
watchEffect(() => {
    // 监听画布数量变化，重新计算滚动宽度
    if (targetVal.value.length > 0) {
        calcMaxScrollWidth()
    }

    // 同步缩放比例
    const value = (scale.value * 100).toFixed(0)
    filterValue.value = `${value}%`
    sliderValue.value = parseInt(value)
})

// 初始化时不保存空数据，等待外部数据加载完成后更新
onMounted(() => {
    calcMaxScrollWidth()
    window.addEventListener('resize', calcMaxScrollWidth)
})

// 提供给外部调用的方法，用于更新指定画布的数据
const updateCanvasData = (canvasTitle: string, data: any) => {
    const index = targetVal.value.findIndex(item => item.title === canvasTitle)
    if (index !== -1) {
        targetVal.value[index].list = data
    }
}

// 导出方法供外部使用
defineExpose({
    updateCanvasData
})

onUnmounted(() => {
    window.removeEventListener('resize', calcMaxScrollWidth)
})
</script>

<style lang="scss" scoped>
$min-width: 500px;
$max-width: 100%;

@include go('edit-bottom') {
    width: 100%;
    min-width: $min-width;
    max-width: $max-width;
    padding: 0 10px 0 20px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;

    .bottom-ri {
        display: flex;
        align-items: center;
        gap: 8px;
        position: static;

        .lock-icon {
            padding-top: 4px;

            &.color {
                color: v-bind('themeColor');
            }
        }

        .scale-btn {
            width: 90px;
            font-size: 12px;

            @include deep() {
                .n-base-selection-label {
                    padding: 3px;
                }
            }
        }

        .scale-slider {
            position: relative;
            top: 10px;
            width: 100px;
        }
    }
}

/* 标签栏容器 */
.target-bar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    flex: 1;
    min-width: 200px;
}

/* 滚动箭头 */
.scroll-btn {
    color: #eaeaea !important;
    background: rgba(255, 255, 255, 0.08) !important;
    transition: all 0.2s ease;

    &:hover {
        background: rgba(255, 255, 255, 0.15) !important;
    }

    &:disabled {
        opacity: 0.3;
        cursor: not-allowed;
    }
}

/* 滚动容器：限制标签列表可视区域 */
.tag-scroll-wrapper {
    flex: 1;
    overflow: hidden;
    position: relative;
    height: 32px;
}

/* 标签列表：横向排列，通过translateX滚动 */
.tag-list {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 100%;
    white-space: nowrap;
}

/* 标签样式 */
.target-tag {
    margin-right: 0;
    border-radius: 8px !important;
    padding: 2px 10px;
    font-size: 12px;
    background: rgba(255, 255, 255, 0.08) !important;
    border: 1px solid rgba(255, 255, 255, 0.12) !important;
    color: #eaeaea !important;
    transition: all .2s ease;

    &:hover {
        background: rgba(99, 165, 255, 0.18) !important;
        border-color: rgba(99, 165, 255, 0.45) !important;
        transform: translateY(-1px);
    }

    &.is-active {
        background: linear-gradient(135deg, #5aa8ff, #6ed7ff) !important;
        border-color: transparent !important;
        color: #0c1b2a !important;
        box-shadow: 0 4px 12px rgba(90, 168, 255, 0.35);
    }

    .n-tag__close {
        color: rgba(255, 255, 255, 0.7) !important;
    }

    &.is-active .n-tag__close {
        color: rgba(0, 0, 0, 0.6) !important;
    }
}

/* 加号按钮：固定在最右侧，不随标签滚动 */
.add-canvas-btn {
    width: 24px;
    height: 24px;
    color: #eaeaea !important;
    background: rgba(255, 255, 255, 0.12) !important;
    transition: all 0.2s ease;

    &:hover {
        background: rgba(255, 255, 255, 0.2) !important;
    }
}
</style>
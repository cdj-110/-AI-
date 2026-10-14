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
                <draggable v-model="canvasPages" item-key="title" class="tag-list" ref="tagListRef"
                    :animation="180" ghost-class="canvas-drag-ghost" chosen-class="canvas-drag-chosen"
                    :style="{ transform: `translateX(-${scrollLeft}px)`, transition: draggingCanvas ? 'none' : 'transform 0.2s ease' }"
                    @start="handleCanvasDragStart" @end="handleCanvasDragEnd">
                    <template #item="{ element: item }">
                        <n-tag @click="targetChange(item)" :class="{ 'is-active': targetObj?.title === item.title }"
                            class="target-tag" :title="`${item.title}（拖动可排序）`">
                            <span class="target-tag-label" @dblclick.stop="openRenameCanvas(item)">
                                {{ item.title }}
                            </span>
                            <n-dropdown trigger="click" placement="top-end" :options="canvasMenuOptions"
                                @select="(key: string) => handleCanvasMenu(key, item)">
                                <button type="button" class="canvas-menu-trigger" title="画布操作" aria-label="画布操作"
                                    @click.stop>
                                    <n-icon size="15"><EllipsisVertical /></n-icon>
                                </button>
                            </n-dropdown>
                        </n-tag>
                    </template>
                </draggable>
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

        <n-modal v-model:show="renameVisible" preset="card" title="重命名画布" :style="{ width: '420px' }"
            :mask-closable="false" @after-enter="focusRenameInput">
            <n-input ref="renameInputRef" v-model:value="renameValue" maxlength="30" show-count
                placeholder="请输入画布名称" @keydown.enter.prevent="confirmRenameCanvas" />
            <template #footer>
                <div class="rename-modal-actions">
                    <n-button @click="renameVisible = false">取消</n-button>
                    <n-button type="primary" @click="confirmRenameCanvas">保存</n-button>
                </div>
            </template>
        </n-modal>
    </div>
</template>

<script setup lang="ts">
import { AddIcon } from "naive-ui/lib/_internal/icons";
import { ChevronBackOutline, ChevronForward, EllipsisVertical } from '@vicons/ionicons5';
import { InputInst, SelectInst, useMessage } from 'naive-ui'
import { computed, reactive, ref, toRefs, watchEffect, nextTick, onMounted, onUnmounted } from 'vue'
import Draggable from 'vuedraggable'
import { EditHistory } from '../EditHistory/index'
import EditShortcutKey from '../EditShortcutKey/index.vue'
import { useDesignStore } from '@/store/modules/designStore/designStore'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { EditCanvasConfigEnum, EditCanvasTypeEnum } from '@/store/modules/chartEditStore/chartEditStore.d'
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

const targetObj = ref<any>(chartEditStore.pageCallList[0]);
const renameVisible = ref(false)
const renameValue = ref('')
const renameSourceTitle = ref('')
const renameInputRef = ref<InputInst | null>(null)
const draggingCanvas = ref(false)
const canvasMenuOptions = [
    { label: '修改名称', key: 'rename' },
    { label: '删除画布', key: 'delete' }
]
const canvasPages = computed({
    get: () => chartEditStore.pageCallList,
    set: (pages: any[]) => chartEditStore.setPageCallList(pages)
})

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
        const tagListElement = ((tagListRef.value as any)?.$el || tagListRef.value) as HTMLElement
        const listWidth = tagListElement.scrollWidth

        const firstTag = tagListElement.querySelector('.target-tag') as HTMLElement
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
        chartEditStore.removeEditCanvasConfig();
    },

    /** 获取当前画布完整数据 */
    getViewVer: () => {
        return JSONParse(JSONStringify(chartEditStore.getStorageInfo()))
    },

    /** 创建一个空的画布结构 */
    createEmptyCanvas: () => {
        return {
            componentList: [],
            // editCanvasConfig: JSONParse(JSONStringify(chartEditStore.getDeepEditCanvasConfig("editCanvasConfig"))),
            // requestGlobalConfig: JSONParse(JSONStringify(chartEditStore.getDeepEditCanvasConfig("requestGlobalConfig")))
            editCanvasConfig: JSONParse(JSONStringify(chartEditStore.getEditCanvasConfig)),
            requestGlobalConfig: JSONParse(JSONStringify(chartEditStore.getRequestGlobalConfig))
        }
    }
}

// 新增画布逻辑（先保存当前页面，再创建新页面）
const addCanvas = () => {
    const currentIndex = chartEditStore.pageCallList.findIndex(item => item.title === targetObj.value.title)
    if (currentIndex !== -1) {
        chartEditStore.pageCallList[currentIndex].list = unitConfig.getViewVer()
    }

    const usedTitles = new Set(chartEditStore.pageCallList.map((item: any) => String(item.title || '')))
    let iii = 1
    while (usedTitles.has(`画布${iii}`)) iii += 1

    let newPage:any = { title: `画布${iii}`, list: unitConfig.createEmptyCanvas() };
    let arr = chartEditStore.pageCallList.concat([newPage]);
    chartEditStore.setPageCallList(arr);

    unitConfig.clearClose();
    targetObj.value = newPage;
    sessionStorage.setItem("chunyixiazhutai",JSONStringify(newPage));//将当前正在编辑的组态临时保存一下，点击保存的时候读取
    chartEditStore.setCurrentCanvasTitle(newPage.title);
}

const openRenameCanvas = (page: any) => {
    renameSourceTitle.value = String(page?.title || '')
    renameValue.value = renameSourceTitle.value
    renameVisible.value = true
}

const focusRenameInput = () => {
    nextTick(() => {
        renameInputRef.value?.focus()
        renameInputRef.value?.select()
    })
}

const updatePageJumpReferences = (value: any, oldTitle: string, newTitle: string, visited = new WeakSet<object>()) => {
    if (!value || typeof value !== 'object' || visited.has(value)) return
    visited.add(value)

    if (value.changeAttr === '页面跳转' && value.pageValue === oldTitle) value.pageValue = newTitle
    if (value.form?.changeAttr === '页面跳转' && value.form.pageValue === oldTitle) value.form.pageValue = newTitle

    Object.values(value).forEach(item => updatePageJumpReferences(item, oldTitle, newTitle, visited))
}

const updatePreviewCanvasCache = (oldTitle: string, newTitle: string) => {
    const raw = sessionStorage.getItem('GO_CHART_STORAGE_LIST')
    if (!raw) return
    try {
        const list = JSONParse(raw)
        if (!Array.isArray(list)) return
        let changed = false
        list.forEach((item: any) => {
            if (item?.canvasTitle === oldTitle) {
                item.canvasTitle = newTitle
                changed = true
            }
        })
        if (changed) sessionStorage.setItem('GO_CHART_STORAGE_LIST', JSONStringify(list))
    } catch {
        // 预览缓存异常不应阻止画布重命名，后续预览会重新生成缓存。
    }
}

const confirmRenameCanvas = () => {
    const oldTitle = renameSourceTitle.value
    const newTitle = renameValue.value.trim()
    if (!newTitle) {
        message.warning('画布名称不能为空')
        return
    }
    if (newTitle === oldTitle) {
        renameVisible.value = false
        return
    }
    if (chartEditStore.pageCallList.some((item: any) => item.title === newTitle)) {
        message.warning('画布名称不能重复')
        return
    }

    const pageIndex = chartEditStore.pageCallList.findIndex((item: any) => item.title === oldTitle)
    if (pageIndex === -1) {
        message.warning('未找到需要重命名的画布')
        return
    }

    const isCurrent = targetObj.value?.title === oldTitle
    if (isCurrent) {
        const storageInfo = unitConfig.getViewVer()
        chartEditStore.pageCallList[pageIndex].list = storageInfo
        Object.assign(chartEditStore.pageCallList[pageIndex], storageInfo)
    }

    chartEditStore.pageCallList.forEach((page: any) => updatePageJumpReferences(page, oldTitle, newTitle))
    const renamedPage = chartEditStore.pageCallList[pageIndex]
    renamedPage.title = newTitle
    chartEditStore.setPageCallList([...chartEditStore.pageCallList])
    updatePreviewCanvasCache(oldTitle, newTitle)

    if (isCurrent) {
        targetObj.value = renamedPage
        chartEditStore.setCurrentCanvasTitle(newTitle)
        sessionStorage.setItem('chunyixiazhutai', JSONStringify({ title: newTitle, ...unitConfig.getViewVer() }))
    }

    renameVisible.value = false
    message.success('画布名称已修改')
    calcMaxScrollWidth()
}

const handleCanvasMenu = (key: string, page: any) => {
    if (key === 'rename') openRenameCanvas(page)
    if (key === 'delete') targetclose(page)
}

const saveCurrentCanvas = () => {
    const currentIndex = chartEditStore.pageCallList.findIndex((item: any) => item.title === targetObj.value?.title)
    if (currentIndex === -1) return
    const storageInfo = unitConfig.getViewVer()
    chartEditStore.pageCallList[currentIndex].list = storageInfo
    chartEditStore.pageCallList[currentIndex] = { ...targetObj.value, ...storageInfo }
    targetObj.value = chartEditStore.pageCallList[currentIndex]
}

const handleCanvasDragStart = () => {
    saveCurrentCanvas()
    draggingCanvas.value = true
}

const handleCanvasDragEnd = () => {
    draggingCanvas.value = false
    chartEditStore.setPageCallList([...chartEditStore.pageCallList])
    const currentPage = chartEditStore.pageCallList.find((item: any) => item.title === targetObj.value?.title)
    if (currentPage) {
        targetObj.value = currentPage
        sessionStorage.setItem('chunyixiazhutai', JSONStringify(currentPage))
    }
    calcMaxScrollWidth()
}

// 删除画布逻辑
const targetclose = (e: any) => {
    if (chartEditStore.pageCallList.length <= 1) {
        message.warning('至少需要保留一个画布')
        return
    }
    goDialog({
        message: `是否确认删除【${e.title}】画布`,
        positiveText: '确认删除',
        negativeText: "取消删除",
        onPositiveCallback: () => {
            chartEditStore.setPageCallList(chartEditStore.pageCallList.filter((item: any) => item.title != e.title));
            targetChange(chartEditStore.pageCallList[chartEditStore.pageCallList.length - 1]);
            // const one = chartEditStore.pageCallList.findIndex((item: any) => item.title === e.title);
            // const two = chartEditStore.pageCallList.findIndex((item: any) => item.title === targetObj.value.title)
            // if(one == two){
            //     unitConfig.clearClose();
            // }
        }
    })
}

// 切换画布逻辑
const targetChange = async (e: any) => {
    if(!e.componentList){
        e = { ...e, ...e.list }
    }
    const currentIndex = chartEditStore.pageCallList.findIndex((item: any) => item.title === targetObj.value.title)

    if (currentIndex !== -1 && targetObj.value.list !== null) {
        chartEditStore.pageCallList[currentIndex].list = unitConfig.getViewVer()
        chartEditStore.pageCallList[currentIndex] = { ...targetObj.value, ...unitConfig.getViewVer() };
    }
     
    unitConfig.clearClose();
    targetObj.value = e;
    sessionStorage.setItem("chunyixiazhutai",JSONStringify(e));//将当前正在编辑的组态临时保存一下，点击保存的时候读取
    chartEditStore.setCurrentCanvasTitle(e.title);
    console.log("切换画布",e)
    await updateComponent(e, true, false);
    // if(e.componentList && Array.isArray(e.componentList) && e.componentList.length != 0){
    //     await updateComponent(e, true, false);
    // }
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
    if (chartEditStore.pageCallList.length > 0) {
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
    chartEditStore.getPageCallListVer();
    setTimeout(() => {
        sessionStorage.setItem("chunyixiazhutai",JSONStringify(chartEditStore.pageCallList[0]));//将当前正在编辑的组态临时保存一下，点击保存的时候读取
    },1000)
})

// 提供给外部调用的方法，用于更新指定画布的数据
const updateCanvasData = (canvasTitle: string, data: any) => {
    const index = chartEditStore.pageCallList.findIndex(item => item.title === canvasTitle)
    if (index !== -1) {
        chartEditStore.pageCallList[index].list = data
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
    position: relative;
    min-width: 78px;
    height: 26px;
    margin-right: 0;
    border-radius: 8px !important;
    padding: 2px 24px !important;
    font-size: 12px;
    background: rgba(255, 255, 255, 0.08) !important;
    border: 1px solid rgba(255, 255, 255, 0.12) !important;
    color: #eaeaea !important;
    transition: all .2s ease;

    @include deep() {
        .n-tag__content {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 100%;
            height: 100%;
            line-height: 1;
        }
    }

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

.target-tag-label {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
    text-align: center;
    line-height: 1;
}

.canvas-menu-trigger {
    position: absolute;
    top: 50%;
    right: 6px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    margin: 0;
    padding: 1px 0;
    border: 0;
    color: currentColor;
    background: transparent;
    cursor: pointer;
    opacity: .58;
    transform: translateY(-50%);

    &:hover {
        opacity: 1;
    }
}

.canvas-drag-ghost {
    opacity: .35;
}

.canvas-drag-chosen {
    cursor: grabbing !important;
}

.rename-modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
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

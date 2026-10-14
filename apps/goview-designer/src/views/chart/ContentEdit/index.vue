<template>
    <content-box id="go-chart-edit-layout" :flex="true" :showTop="false" :showBottom="true" :depth="1" :xScroll="true"
        :disabledScroll="true" @mousedown="mousedownHandleUnStop" @drop="dragHandle" @dragover="dragoverHandle"
        @dragenter="dragoverHandle">
        <edit-rule>
            <!-- 画布主体 -->
            <div id="go-chart-edit-content" @contextmenu="handleContextMenu">
                <!-- 展示 -->
                <edit-range>
                    <!-- 滤镜预览 -->
                    <div :style="{
                        ...getFilterStyle(chartEditStore.getEditCanvasConfig),
                        ...rangeStyle
                    }">
                        <!-- 图表 -->
                        <div v-for="(item, index) in previewComponentList" :key="item.id">
                            <!-- 分组 -->
                            <edit-group v-if="item.isGroup" :groupData="(item as CreateComponentGroupType)"
                                :groupIndex="index"></edit-group>

                            <!-- 单组件 -->
                            <edit-shape-box v-else :data-id="item.id" :index="index" :style="{
                                ...useComponentStyle(item.attr, index),
                                ...getBlendModeStyle(item.styles) as any
                            }" :item="item" @click="mouseClickHandle($event, item)"
                                @mousedown="mousedownHandle($event, item)" @mouseenter="mouseenterHandle($event, item)"
                                @mouseleave="mouseleaveHandle($event, item)"
                                @contextmenu="handleContextMenu($event, item, optionsHandle)">
                                <history-query-chart v-if="isHistoryQueryChart(item)" :key="eventPreviewRenderKey(item.id)" editing class="edit-content-chart"
                                    :class="animationsClass(item.styles.animations)" :chartConfig="item" :themeSetting="themeSetting"
                                    :themeColor="themeColor" :style="{
                                        ...useContentSizeStyle(item.attr), ...getFilterStyle(item.styles), ...getTransformStyle(item.styles)
                                    }" />
                                <component v-else :key="eventPreviewRenderKey(item.id)" class="edit-content-chart" :class="animationsClass(item.styles.animations)"
                                    :is="item.chartConfig.chartKey" :chartConfig="getContentChartConfig(item)" :themeSetting="themeSetting"
                                    v-bind="item.chartConfig.key === 'Figure' ? { editing: true, editorScale: chartEditStore.getEditCanvas.scale } : {}"
                                    :themeColor="themeColor" :style="{
                                        ...useContentSizeStyle(item.attr),
                                        ...getFilterStyle(item.styles),
                                        ...getTransformStyle(item.styles)
                                    }"></component>
                            </edit-shape-box>
                        </div>
                    </div>
                </edit-range>
            </div>
        </edit-rule>

        <!-- 工具栏 -->
        <template #aside>
            <edit-tools></edit-tools>
        </template>

        <!-- 底部控制 -->
        <template #bottom>
            <EditBottom ref="editBottomRef"></EditBottom>
        </template>
    </content-box>
</template>

<script lang="ts" setup>
import { onMounted, onBeforeUnmount, computed, provide, watch, ref, nextTick } from 'vue'
import { resetEditorNavigation, markEditorLoaded } from '@/utils/editorNavigation'
import { chartColors } from '@/settings/chartThemes/index'
import { MenuEnum } from '@/enums/editPageEnum'
import { CreateComponentType, CreateComponentGroupType } from '@/packages/index.d'
import { animationsClass, getFilterStyle, getTransformStyle, getBlendModeStyle, colorCustomMerge, getContentChartConfig, JSONStringify, JSONParse } from '@/utils'
import { useContextMenu } from '@/views/chart/hooks/useContextMenu.hook'
import { MenuOptionsItemType } from '@/views/chart/hooks/useContextMenu.hook.d'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { SCALE_KEY } from '@/views/preview/hooks/useScale.hook'
import { useLayout } from './hooks/useLayout.hook'
import { useAddKeyboard } from '../hooks/useKeyboard.hook'
import { dragHandle, dragoverHandle, mousedownHandleUnStop, useMouseHandle } from './hooks/useDrag.hook'
import { useComponentStyle, useSizeStyle, useContentSizeStyle } from './hooks/useStyle.hook'
import { useInitVChartsTheme } from '@/hooks'
import HistoryQueryChart from '@/views/preview/components/HistoryQueryChart.vue'
import { isHistoryQueryChart } from '@/utils/platformHistoryChart'
import { recoveryDraftKey, sessionRecovery } from '@/utils/sessionRecovery'
import { eventPreviewComponents, eventPreviewRenderKey } from '@/utils/eventAttributePreview'

import { ContentBox } from '../ContentBox/index'
import { EditGroup } from './components/EditGroup'
import { EditRange } from './components/EditRange'
import { EditRule } from './components/EditRule'
import { EditBottom } from './components/EditBottom'
import { EditShapeBox } from './components/EditShapeBox'
import { EditTools } from './components/EditTools'
import { apiV1configDatas_test, apiV1ConfigDatas, apiV1MediaImagesList } from '@/api/index'
import { useSync } from '@/views/chart/hooks/useSync.hook'
import { baseurlAddress } from "@/utils/request";

let userId = ref(sessionStorage.getItem('wk_UserId') as any);
let wk_Config_id = ref(sessionStorage.getItem('wk_Config_id') as any);
const { updateComponent } = useSync()

const chartEditStore = useChartEditStore()
const previewComponentList = computed(() => eventPreviewComponents(chartEditStore.getComponentList))
const { handleContextMenu } = useContextMenu()

// EditBottom组件引用
const editBottomRef = ref<InstanceType<typeof EditBottom> | null>(null)

// 编辑时注入scale变量，消除警告
provide(SCALE_KEY, null)

// 布局处理
useLayout(async () => { })

// 点击事件
const { mouseenterHandle, mouseleaveHandle, mousedownHandle, mouseClickHandle } = useMouseHandle()

// 右键事件
const optionsHandle = (
    targetList: MenuOptionsItemType[],
    allList: MenuOptionsItemType[],
    targetInstance: CreateComponentType
) => {
    // 多选处理
    if (chartEditStore.getTargetChart.selectId.length > 1) {
        return allList.filter(i => [MenuEnum.GROUP, MenuEnum.DELETE].includes(i.key as MenuEnum))
    }
    const statusMenuEnums: MenuEnum[] = []
    if (targetInstance.status.lock) {
        statusMenuEnums.push(MenuEnum.LOCK)
    } else {
        statusMenuEnums.push(MenuEnum.UNLOCK)
    }
    if (targetInstance.status.hide) {
        statusMenuEnums.push(MenuEnum.HIDE)
    } else {
        statusMenuEnums.push(MenuEnum.SHOW)
    }
    return targetList.filter(i => !statusMenuEnums.includes(i.key as MenuEnum))
}

// 主题色
const themeSetting = computed(() => {
    const chartThemeSetting = chartEditStore.getEditCanvasConfig.chartThemeSetting
    return chartThemeSetting
})

// 配置项
const themeColor = computed(() => {
    const colorCustomMergeData: any = colorCustomMerge(chartEditStore.getEditCanvasConfig.chartCustomThemeColorInfo)
    return colorCustomMergeData[chartEditStore.getEditCanvasConfig.chartThemeColor]
})

// 是否展示渲染
const filterShow = computed(() => {
    return chartEditStore.getEditCanvasConfig.filterShow
})

// 背景
const rangeStyle = computed(() => {
    // 设置背景色和图片背景
    const background = chartEditStore.getEditCanvasConfig.background
    const backgroundImage = chartEditStore.getEditCanvasConfig.backgroundImage
    const selectColor = chartEditStore.getEditCanvasConfig.selectColor
    const backgroundColor = background ? background : undefined

    const computedBackground = selectColor
        ? { background: backgroundColor }
        : { background: `url(${backgroundImage}) no-repeat center center / cover !important` }

    return {
        ...computedBackground,
        width: 'inherit',
        height: 'inherit'
    }
})

// 处理全局的 vChart 主题
useInitVChartsTheme(chartEditStore)

// 键盘事件
onMounted(() => {
    getList();
    useAddKeyboard()
})

/**
 * 规范化数据，处理从后端或 sessionStorage 加载的数据，确保组件数据完整
 */
const normalizeComponentData = (item: any): any => {
    if (!item.componentList) return item
    const normalizeList = (list: any[]) => {
        list.forEach((comp: any) => {
            // 处理 option 为 null 的情况
            if (comp.option === null || comp.option === undefined) {
                delete comp.option
            }
            // CirclePoint 组件迁移：chartFrame 从 static 升级为 common，添加 dataset
            if (comp.chartConfig && comp.chartConfig.key === 'CirclePoint') {
                if (comp.chartConfig.chartFrame === 'static') {
                    comp.chartConfig.chartFrame = 'common'
                }
                if (comp.option !== undefined && comp.option !== null && comp.option.dataset === undefined) {
                    comp.option.dataset = null
                }
            }
            // 处理分组
            if (comp.isGroup && comp.groupList) {
                normalizeList(comp.groupList)
            }
        })
    }
    normalizeList(item.componentList)
    return item
}

/**
 * 组态进来之后渲染组态界面到画板上
 */
let getList = async () => {
    resetEditorNavigation()
    const draftText = sessionStorage.getItem(recoveryDraftKey())
    if (draftText) {
        try {
            const draft = JSONParse(draftText)
            if (draft.projectId === wk_Config_id.value && Array.isArray(draft.pages) && draft.pages.length) {
                const page = draft.pages.find((item: any) => item.title === draft.title) || draft.pages[0]
                chartEditStore.setPageCallList(draft.pages)
                chartEditStore.setCurrentCanvasTitle(page.title)
                await updateComponent(normalizeComponentData({ ...page, ...page.list }), true, false)
                sessionRecovery.draftReady = true
                await nextTick()
                editBottomRef.value?.updateCanvasData(page.title, page)
                markEditorLoaded(chartEditStore, true)
                window['$message']?.info('已恢复登录过期前的本地草稿，请确认后保存')
                return
            }
        } catch { /* 本地草稿损坏时继续加载已保存的配置 */ }
    }
    if (sessionStorage.getItem("zhutaineirong")) {
        const cachedPages = JSONParse(sessionStorage.getItem("zhutaineirong") as any).main_data.item || [];
        chartEditStore.setPageCallList(cachedPages);
        chartEditStore.setCurrentCanvasTitle(cachedPages[0]?.title || '画布1');
        let res = cachedPages[0];
        if(!res) return;
        if(!res.componentList){
            res = { ...res, ...res.list };
        }
        res = normalizeComponentData(res);
        await updateComponent(res, true, false);
        sessionRecovery.draftReady = true;
        socketConnent(JSONParse(sessionStorage.getItem("zhutaineirong") as any));
    } else {
        try {
            // let res: any = await apiV1ConfigDatas({ user_id: userId.value, config_id: wk_Config_id.value });//带锁的接口
            let res: any = await apiV1configDatas_test({ user_id: userId.value, config_id: wk_Config_id.value });
            if (!res?.main_data?.item?.length) {
                res = { ...res, main_data: { item: [{ title: '画布1', ...chartEditStore.getStorageInfo() }] } };
            }
            if(!res.main_data.item[0].componentList){
                res.main_data.item[0] = { ...res.main_data.item[0], ...res.main_data.item[0].list };
            }
            console.log(res.main_data.item)
            chartEditStore.setPageCallList(res.main_data.item);
            chartEditStore.setCurrentCanvasTitle(res.main_data.item[0]?.title || '画布1');
            const normalizedItem = normalizeComponentData(res.main_data.item[0]);
            await updateComponent(normalizedItem, true, false);
            sessionRecovery.draftReady = true;
            sessionStorage.setItem("zhutaineirong", JSONStringify(res));
            socketConnent(res);
        } catch (err: any) {
            if(err.response){
                window['$message'].warning(err.response.data.detail);
            }
        }
    }
    
    // 数据加载完成后，使用实际名称更新首个画布，兼容用户重命名后的项目。
    await nextTick()
    if (editBottomRef.value && sessionStorage.getItem("zhutaineirong")) {
        const firstCanvas = JSONParse(sessionStorage.getItem("zhutaineirong") as any).main_data.item[0]
        editBottomRef.value.updateCanvasData(firstCanvas?.title || '画布1', firstCanvas)
    }
    if (sessionRecovery.draftReady) markEditorLoaded(chartEditStore)
}

onBeforeUnmount(() => {
    resetEditorNavigation()
    sessionRecovery.draftReady = false
})

/**
 * 连接websocket
 */
let socketConnent = (res: any) => {
    if (sessionStorage.getItem('wk_ApiBaseUrl')) return
    // 锁 WebSocket
    const lockSocketUrl = `${baseurlAddress.wss}/api/conf/ws/lock?user_id=${userId.value}&config_id=${wk_Config_id.value}&session_id=${res.session_id}`
    let socket = new WebSocket(lockSocketUrl)
    socket.onopen = () => {
        console.log('组态状态连接成功');
        window.addEventListener('beforeunload', (event: any) => {
            event.preventDefault();
            event.returnValue = '';
        });
    };
    socket.onclose = () => {
        console.log('组态状态连接关闭');
    };
}
</script>

<style lang="scss" scoped>
@include goId('chart-edit-layout') {
    position: relative;
    width: 100%;
    overflow: hidden;
    @extend .go-point-bg;
    @include background-image('background-point');

    // 画布外的组件只在编辑视口内显示，不参与底部工具栏的层叠。
    & > :deep(.content) {
        position: relative;
        z-index: 0;
        isolation: isolate;
        overflow: hidden;
    }

    & > :deep(.bottom) {
        position: relative;
        z-index: 1;
    }

    @include goId('chart-edit-content') {
        overflow: visible;
        transition: box-shadow 0.2s;
        @include fetch-theme('box-shadow');

        .edit-content-chart {
            position: absolute;
            overflow: hidden;
        }
    }
}
</style>

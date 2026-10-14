<template>
    <div class="chart-item" :id="item.id" v-for="(item, index) in listAtter"
        :class="animationsClass(item.styles.animations)" :key="item.id" :style="{
            ...getComponentAttrStyle(item.attr, index),
            ...getTransformStyle(item.styles),
            ...getStatusStyle(item.status),
            ...getPreviewConfigStyle(item.preview),
            ...getBlendModeStyle(item.styles) as any,
            ...getSizeStyle(item.attr)
        }">

        <!-- 分组 -->
        <preview-render-group v-if="item.isGroup" :groupData="(item as CreateComponentGroupType)" :groupIndex="index"
            :themeSetting="themeSetting" :themeColor="themeColor"></preview-render-group>
        <!-- 单组件 -->
        <history-query-chart v-else-if="isHistoryQueryChart(item)" :chartConfig="item" :themeSetting="themeSetting"
            :themeColor="themeColor" :style="{ ...getContentSizeStyle(item.attr), ...getFilterStyle(item.styles) }" />
        <component v-else :is="item.chartConfig.chartKey" :id="item.id" :chartConfig="getContentChartConfig(item)" :themeSetting="themeSetting"
            :themeColor="themeColor" :style="{
                ...getContentSizeStyle(item.attr),
                ...getFilterStyle(item.styles)
            }" v-on="useLifeHandler(item)"></component>
    </div>
</template>

<script setup lang="ts">
import { animationsClass, getFilterStyle, getTransformStyle, getBlendModeStyle, getContentChartConfig } from '@/utils'
import { getSizeStyle, getContentSizeStyle, getComponentAttrStyle, getStatusStyle, getPreviewConfigStyle } from '../../utils'
import { useLifeHandler } from '@/hooks'
import { PreviewRenderGroup } from '../PreviewRenderGroup/index'
import { CreateComponentGroupType } from '@/packages/index.d'
import previewRenderList from "./test";
import HistoryQueryChart from '../HistoryQueryChart.vue'
import { isHistoryQueryChart } from '@/utils/platformHistoryChart'

let {
    listAtter,

    themeSetting,
    themeColor
} = previewRenderList();
</script>

<style lang="scss" scoped>
.chart-item {
    position: absolute;
}
</style>

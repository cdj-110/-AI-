<template>
  <div
    :class="animationsClass(groupData.styles.animations)"
    :style="{
      ...getSizeStyle(groupData.attr),
      ...getFilterStyle(groupData.styles),
    }"
  >
    <div
      class="chart-item"
      v-for="item in groupData.groupList"
      :class="animationsClass(item.styles.animations)"
      :key="item.id"
      :style="{
      ...getComponentAttrStyle(item.attr, groupIndex),
      ...getStatusStyle(item.status),
      ...getPreviewConfigStyle(item.preview),
      ...getBlendModeStyle(item.styles) as any
    }"
    >
      <history-query-chart v-if="isHistoryQueryChart(item)" :chartConfig="item" :themeSetting="themeSetting"
        :themeColor="themeColor" :style="{ ...getContentSizeStyle(item.attr), ...getFilterStyle(item.styles), ...getTransformStyle(item.styles) }" />
      <component v-else
        :is="item.chartConfig.chartKey"
        :id="item.id"
        :chartConfig="getContentChartConfig(item)"
        :themeSetting="themeSetting"
        :themeColor="themeColor"
        :style="{
          ...getContentSizeStyle(item.attr),
          ...getFilterStyle(item.styles),
          ...getTransformStyle(item.styles)
        }"
        v-on="useLifeHandler(item)"
      ></component>
    </div>
  </div>
</template>

<script setup lang="ts">
import { PropType } from 'vue'
import { CreateComponentGroupType } from '@/packages/index.d'
import { animationsClass, getFilterStyle, getTransformStyle, getBlendModeStyle, getContentChartConfig } from '@/utils'
import { getSizeStyle, getContentSizeStyle, getComponentAttrStyle, getStatusStyle, getPreviewConfigStyle } from '../../utils'
import { useLifeHandler } from '@/hooks'
import HistoryQueryChart from '../HistoryQueryChart.vue'
import { isHistoryQueryChart } from '@/utils/platformHistoryChart'

const props = defineProps({
  groupData: {
    type: Object as PropType<CreateComponentGroupType>,
    required: true
  },
  themeSetting: {
    type: Object,
    required: true
  },
  themeColor: {
    type: Object,
    required: true
  },
  groupIndex: {
    type: Number,
    required: true
  }
})
</script>

<style lang="scss" scoped>
.chart-item {
  position: absolute;
}
</style>

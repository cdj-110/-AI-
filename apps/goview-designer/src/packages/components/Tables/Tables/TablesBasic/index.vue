<template>
  <div class="go-tables-basic">
    <div v-if="isServerPaged" role="status" style="font-size:12px;color:#e6b870">{{ historyNotice || historyWindowDescription() }}</div>
    <n-input
      v-model:value="inputData"
      placeholder="请输入信息"
      :style="`display: ${inputShow}`"
      style="margin-bottom: 5px; float: right; width: 240px"
    >
      <template #prefix>
        <n-icon :component="SearchIcon" />
      </template>
    </n-input>
    <n-data-table
      :style="`
      width: ${w}px;
      height: ${h}px;
      font-size: ${option.style.fontSize}px;
      border-width: ${option.style.border === 'on' ? option.style.borderWidth : 0}px;
      border-color: ${option.style.borderColor};
      border-style: ${option.style.borderStyle}`"
      :bordered="option.style.border === 'on'"
      :single-column="option.style.singleColumn === 'on'"
      :single-line="option.style.singleLine === 'on'"
      :bottom-bordered="option.style.bottomBordered === 'on'"
      :striped="option.style.striped === 'on'"
      :max-height="h"
      size="small"
      :columns="tableColumns"
      :data="filterData"
      :pagination="pagination"
      :remote="isServerPaged"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, PropType, toRefs, watch, reactive, ref } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { icon } from '@/plugins'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartDataFetch } from '@/hooks'
import { apiAnalysisQueryV3_2 } from '@/api'
import { refreshHistoryWindow, historyWindowDescription, normalizeHistoryRange } from '@/utils/historyRetentionWindow'
import { validateHistoryRange } from '@/utils/platformHistoryChart'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})

const { SearchIcon } = icon.ionicons5

//查询字段
const inputData = ref('')
const historyNotice = ref('')
//前台过滤
const filterData = computed(() => {
  const source = Array.isArray(option.dataset)
    ? option.dataset
    : Array.isArray(option?.dataset?.source) ? option.dataset.source : []
  return source.filter((item: any) => {
    return Object.values(item).some(val => {
      return String(val).toLowerCase().includes(inputData.value.toLowerCase())
    })
  })
})

const tableColumns = computed(() => {
  const source = Array.isArray(option.dataset)
    ? option.dataset
    : Array.isArray(option?.dataset?.source) ? option.dataset.source : []
  const dimensions = Array.isArray(option?.dataset?.dimensions)
    ? option.dataset.dimensions
    : source.length && source[0] && typeof source[0] === 'object'
      ? Object.keys(source[0]).filter(key => key !== 'key')
      : []
  return dimensions.map((column: any, index: number) => {
    if (typeof column === 'string') return { title: column, key: column, align: align.value }
    const key = column?.key || column?.title || `column_${index}`
    return { ...column, title: column?.title || key, key, align: align.value }
  })
})

const { align, pagination, inputShow } = toRefs(props.chartConfig.option)

const isServerPaged = computed(() => props.chartConfig.request?.requestHttpType === '历史数据')

const loadHistoryPage = async (page: number) => {
  await refreshHistoryWindow()
  const normalized = normalizeHistoryRange(props.chartConfig.request?.requestInterval as any)
  const range = normalized.range
  historyNotice.value = normalized.adjusted ? '原时间范围已失效，临时按最近1小时查询，请在数据绑定中重新确认' : ''
  const validation = validateHistoryRange(range)
  if (validation) { historyNotice.value = validation; return }
  const pairs = props.chartConfig.request?.requestParams?.Params
  if (!Array.isArray(range) || range.length < 2 || !Array.isArray(pairs) || !pairs.length) return
  const pageSize = Math.min(100, Math.max(1, Number(pagination.value.pageSize) || 5))
  const history: any = await apiAnalysisQueryV3_2({
    start: range[0],
    end: range[1],
    pairs,
    page,
    pageSize,
  })
  if (!history?.dataset) return
  props.chartConfig.option.dataset = history.dataset
  option.dataset = history.dataset
  pagination.value.page = history.page || page
  pagination.value.pageSize = history.pageSize || pageSize
  pagination.value.itemCount = Number(history.total) || 0
}

pagination.value.onChange = (page: number) => {
  pagination.value.page = page
  if (isServerPaged.value) void loadHistoryPage(page).catch(error => { historyNotice.value = error?.message || '历史查询失败，请重试' })
}

onMounted(() => {
  if (isServerPaged.value) void loadHistoryPage(Number(pagination.value.page) || 1).catch(error => { historyNotice.value = error?.message || '历史查询失败，请重试' })
})

const { w, h } = toRefs(props.chartConfig.attr)

const option = reactive({
  dataset: props.chartConfig.option.dataset,
  style: props.chartConfig.option.style
})

watch(
  () => props.chartConfig.option.dataset,
  (newData: any) => {
    option.dataset = newData
    const dimensions = Array.isArray(option?.dataset?.dimensions) ? option.dataset.dimensions : []
    dimensions.forEach((header: any) => {
      header.align = align.value
    })
  },
  {
    immediate: true,
    deep: true
  }
)

// setdata 数据监听与更改
useChartDataFetch(props.chartConfig, useChartEditStore, (newData: any) => {
  props.chartConfig.option.dataset = newData
})


</script>

<style lang="scss" scoped>
@include go('tables-basic') {
  display: flex;
  flex-direction: column;
  gap: 15px;
  align-items: flex-end;
}
</style>

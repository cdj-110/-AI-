<template>
  <div class="go-tables-advanced" :style="{ width: `${w}px`, height: `${h}px` }">
    <div class="query-bar">
      <n-input
        v-model:value="keyword"
        clearable
        :placeholder="option.query?.keywordPlaceholder || '请输入查询内容'"
        @keyup.enter="handleQuery"
      />
      <n-date-picker
        v-if="option.query?.showTimeRange"
        v-model:formatted-value="dateRange"
        value-format="yyyy-MM-dd HH:mm:ss"
        type="datetimerange"
        :is-date-disabled="historyDateDisabled"
        clearable
      />
      <n-button type="primary" :loading="loading" @click="handleQuery">查询</n-button>
      <n-button :disabled="loading" @click="handleReset">重置</n-button>
      <n-button :disabled="loading" @click="exportRows">导出CSV</n-button>
    </div>
    <div role="status" style="font-size:12px;color:#e6b870">{{ notice || historyWindowDescription() }}</div>
    <n-data-table
      class="advanced-table"
      :bordered="option.style.border === 'on'"
      :single-column="option.style.singleColumn === 'on'"
      :single-line="option.style.singleLine === 'on'"
      :bottom-bordered="option.style.bottomBordered === 'on'"
      :striped="option.style.striped === 'on'"
      :columns="tableColumns"
      :data="tableData"
      :pagination="pagination"
      :remote="true"
      :loading="loading"
      :style="{ fontSize: `${option.style.fontSize}px` }"
      size="small"
      flex-height
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, PropType, reactive, ref, toRefs, watch } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { apiAnalysisQueryV3_2 } from '@/api'
import { refreshHistoryWindow, historyDateDisabled, historyWindowDescription, normalizeHistoryRange } from '@/utils/historyRetentionWindow'
import { validateHistoryRange } from '@/utils/platformHistoryChart'
import { request } from '@/utils/request'

const props = defineProps({
  chartConfig: { type: Object as PropType<CreateComponentType>, required: true }
})

const { w, h } = toRefs(props.chartConfig.attr)
const option = reactive(props.chartConfig.option as any)
const pagination = option.pagination
const keyword = ref('')
const loading = ref(false)
const notice = ref('')
let generation = 0
const configuredRange = () => {
  const range = props.chartConfig.request?.requestInterval
  return Array.isArray(range) && range.length >= 2 ? [...range] : null
}
const dateRange = ref<string[] | null>(configuredRange())

const tableData = computed(() => {
  if (Array.isArray(option.dataset)) return option.dataset
  return Array.isArray(option.dataset?.source) ? option.dataset.source : []
})
const tableColumns = computed(() => {
  const dimensions = Array.isArray(option.dataset?.dimensions)
    ? option.dataset.dimensions
    : tableData.value.length ? Object.keys(tableData.value[0]).filter(key => key !== 'key') : []
  return dimensions.map((column: any, index: number) => {
    if (typeof column === 'string') return { title: column, key: column, align: option.align }
    const key = column?.key || column?.title || `column_${index}`
    return { ...column, title: column?.title || key, key, align: option.align }
  })
})

const loadPage = async (page = 1) => {
  const pairs = props.chartConfig.request?.requestParams?.Params
  if (!Array.isArray(pairs) || !pairs.length) return
  const current = ++generation
  loading.value = true
  try {
    await refreshHistoryWindow()
    if (generation !== current) return
    const normalized = normalizeHistoryRange(dateRange.value || configuredRange())
    dateRange.value = normalized.range
    const range = normalized.range
    const validation = validateHistoryRange(range)
    if (validation) { notice.value = validation; return }
    notice.value = normalized.adjusted ? '原时间范围已失效，已调整为有效范围内最近1小时' : ''
    const pageSize = Math.min(100, Math.max(1, Number(pagination.pageSize) || 10))
    const history: any = await apiAnalysisQueryV3_2({
      start: range[0],
      end: range[1],
      pairs,
      page,
      pageSize,
      keyword: keyword.value.trim()
    })
    if (generation !== current || !history?.dataset) return
    option.dataset = history.dataset
    props.chartConfig.option.dataset = history.dataset
    pagination.page = history.page || page
    pagination.pageSize = history.pageSize || pageSize
    pagination.itemCount = Number(history.total) || 0
    if (!history.total && !notice.value) notice.value = '所选时段未找到云端历史记录，请检查点位是否上报并保存；缺失值不会补0'
  } catch (error: any) {
    if (generation === current) notice.value = error?.message || '历史查询失败，请重试'
  } finally {
    if (generation === current) loading.value = false
  }
}

const handleQuery = () => { void loadPage(1) }
const exportRows = async () => {
  loading.value = true
  try {
    await refreshHistoryWindow()
    const normalized = normalizeHistoryRange(dateRange.value || configuredRange())
    dateRange.value = normalized.range
    const validation = validateHistoryRange(normalized.range)
    if (validation) { notice.value = validation; return }
    const result: any = await request({ url: '/conf/influx/analysis/export', method: 'POST', data: { pairs: props.chartConfig.request?.requestParams?.Params, start: normalized.range[0], end: normalized.range[1], keyword: keyword.value.trim() } })
    const url = URL.createObjectURL(new Blob([result.csv], { type: 'text/csv;charset=utf-8' }))
    const link = document.createElement('a'); link.href = url; link.download = '历史数据.csv'; link.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
    notice.value = result.limited ? `共${result.total}条，本次导出最新${result.exported}条（单次上限2000条），请缩小范围分批导出` : `已导出${result.exported}条历史记录`
  } catch (error: any) { notice.value = error?.message || '导出失败，请重试' }
  finally { loading.value = false }
}
const handleReset = () => {
  keyword.value = ''
  dateRange.value = configuredRange()
  void loadPage(1)
}
pagination.onChange = (page: number) => { void loadPage(page) }
pagination.onUpdatePageSize = (pageSize: number) => {
  pagination.pageSize = pageSize
  void loadPage(1)
}

watch(() => props.chartConfig.option.dataset, value => {
  if (value !== option.dataset) option.dataset = value
}, { deep: true })
onMounted(() => { void loadPage(Number(pagination.page) || 1) })
</script>

<style lang="scss" scoped>
@include go('tables-advanced') {
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: hidden;

  .query-bar {
    display: grid;
    grid-template-columns: minmax(180px, 1fr) minmax(300px, 1.4fr) auto auto auto;
    gap: 8px;
    align-items: center;
  }

  .advanced-table {
    flex: 1;
    min-height: 0;
  }
}
</style>

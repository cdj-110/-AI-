<template>
  <div class="history-chart">
    <div v-if="showQuery" class="history-query" @click.stop @mousedown.stop>
      <n-date-picker v-model:formatted-value="range" type="datetimerange" size="small"
        value-format="yyyy-MM-dd HH:mm:ss" :disabled="loading" :is-date-disabled="historyDateDisabled" />
      <div class="history-actions">
        <n-select v-model:value="quickRange" size="small" :options="quickOptions" :disabled="loading" @update:value="applyQuickRange" />
        <n-button size="small" type="primary" :loading="loading" @click="load()">查询</n-button>
        <n-button size="small" :disabled="loading" @click="reset">重置</n-button>
      </div>
    </div>
    <div class="history-notice" :title="historyWindowDescription()" role="status">{{ notice || historyWindowDescription() }}</div>
    <component :is="chartConfig.chartConfig.chartKey" :chartConfig="innerConfig"
      :themeSetting="themeSetting" :themeColor="themeColor" :style="chartStyle" v-on="editing ? {} : useLifeHandler(chartConfig)" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, PropType, ref, watch } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { getContentChartConfig } from '@/utils'
import { useLifeHandler } from '@/hooks'
import { apiAnalysisQueryV3_2 } from '@/api'
import { fetchHistoryChart, formatHistoryDate, historyChartDataset, validateHistoryRange } from '@/utils/platformHistoryChart'
import { refreshHistoryWindow, historyDateDisabled, historyHoursAllowed, historyWindowDescription, normalizeHistoryRange } from '@/utils/historyRetentionWindow'
const props = defineProps({
  editing: { type: Boolean, default: false },
  chartConfig: { type: Object as PropType<CreateComponentType>, required: true },
  themeSetting: { type: Object, required: true }, themeColor: { type: Object, required: true }
})
const showQuery = computed(() => props.chartConfig.request.historyQuery?.enabled !== false)
const configuredRange = (): string[] => {
  const saved = props.chartConfig.request.requestInterval as unknown
  if (Array.isArray(saved) && saved.length === 2) return [...saved]
  const end = new Date()
  return [formatHistoryDate(new Date(end.getTime() - 3600000)), formatHistoryDate(end)]
}
const range = ref<string[] | null>(configuredRange())
const quickRange = ref<number | null>(null)
const quickOptions = computed(() => [1, 6, 24, 168].map(hours => ({ label: hours === 168 ? '最近7天' : `最近${hours}小时`, value: hours, disabled: !historyHoursAllowed(hours) })))
const loading = ref(false), notice = ref('')
let generation = 0
let automaticFrame = 0
const content = computed(() => getContentChartConfig(props.chartConfig))
const innerHeight = computed(() => Math.max(1, content.value.attr.h - (showQuery.value ? 78 : 0) - 22))
const innerConfig = new Proxy(props.chartConfig, {
  get(target, key, receiver) {
    if (key === 'attr') return { ...content.value.attr, w: content.value.attr.w, h: innerHeight.value }
    return Reflect.get(target, key, receiver)
  }
})
const chartStyle = computed(() => ({ width: `${content.value.attr.w}px`, height: `${innerHeight.value}px` }))
const load = async (automatic = false) => {
  if (!automatic) cancelAnimationFrame(automaticFrame)
  const pairs = props.chartConfig.request.requestParams?.Params
  if (!Array.isArray(pairs) || !pairs.some((pair: any) => pair.value1 && pair.value2)) {
    notice.value = '请先绑定设备点位'; return
  }
  const current = ++generation
  loading.value = true; notice.value = ''
  try {
    await refreshHistoryWindow()
    if (current !== generation) return
    const normalized = normalizeHistoryRange(range.value)
    range.value = normalized.range
    const validation = validateHistoryRange(range.value)
    if (validation) { notice.value = validation; return }
    const maxRows = props.editing && automatic ? 500 : 5000
    const result = await fetchHistoryChart(apiAnalysisQueryV3_2, {
      start: range.value![0], end: range.value![1], pairs
    }, () => generation !== current, maxRows)
    if (!result) return
    const option = props.chartConfig.option
    option.dataset = historyChartDataset(result.dimensions, result.rows, option, props.chartConfig.chartConfig.key.startsWith('VChart'))
    notice.value = normalized.adjusted ? '原时间范围已失效，已调整为有效范围内最近1小时；请重新确认' : result.limited ? maxRows === 500
      ? `编辑预览：共${result.total}条，仅显示最新500条；点击查询可加载最多5000条`
      : `共${result.total}条，当前展示最新5000条，请缩小时间范围` : result.rows.length ? '' : '所选时间范围内暂无数据'
  } catch (error: any) {
    if (current === generation) notice.value = error?.message || '历史查询失败，请稍后重试'
  } finally { if (current === generation) loading.value = false }
}
const applyQuickRange = (hours: number) => {
  const end = new Date()
  range.value = [formatHistoryDate(new Date(end.getTime() - hours * 3600000)), formatHistoryDate(end)]
  void load()
}
const reset = () => { range.value = configuredRange(); quickRange.value = null; void load() }
watch(() => JSON.stringify([props.chartConfig.id, props.chartConfig.request.requestParams?.Params, props.chartConfig.request.requestInterval]), () => {
  generation++
  loading.value = false
  cancelAnimationFrame(automaticFrame)
  range.value = configuredRange(); quickRange.value = null
  // 先让画布完成一次绘制，再读取历史预览，避免初始化时争抢主线程。
  automaticFrame = requestAnimationFrame(() => {
    automaticFrame = requestAnimationFrame(() => { void load(true) })
  })
}, { immediate: true })
onBeforeUnmount(() => { generation++; cancelAnimationFrame(automaticFrame) })
</script>

<style scoped>
.history-chart { position: relative; }
.history-query { height: 78px; box-sizing: border-box; display: flex; flex-direction: column; gap: 6px; padding: 4px 6px 8px; }
.history-actions { display: flex; gap: 6px; }
.history-actions .n-select { flex: 1; min-width: 0; }
.history-notice { height: 22px; font-size: 12px; line-height: 22px; color: #e6b870; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>

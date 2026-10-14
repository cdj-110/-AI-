<template>
  <!-- 固定设计坐标，组件尺寸变化时由 SVG 缩放圆环，而非仅改变视口。 -->
  <svg :width="w" :height="h" viewBox="0 0 110 110" preserveAspectRatio="xMidYMid meet">
    <defs>
      <filter :id="blurFilterId" x="-20%" y="-20%" width="140%" height="140%">
        <feGaussianBlur in="SourceGraphic" stdDeviation="1" />
      </filter>
    </defs>
    <circle
      cx="55"
      cy="55"
      :r="inCircle"
      :fill="matchedState.inCircleColor"
      :filter="`url(#${blurFilterId})`"
      :style="flashStyle"
    />
    <circle
      cx="55"
      cy="55"
      :r="outCircle"
      fill="none"
      :stroke="matchedState.outCircleColor"
      :stroke-width="outCircleWidth"
      :style="flashStyle"
    />
  </svg>
</template>

<script setup lang="ts">
import { PropType, computed, toRefs } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartDataFetch } from '@/hooks'
import type { StateItem } from './config'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true,
  },
})
const { w, h } = toRefs(props.chartConfig.attr)
const { outCircle, inCircle, outCircleWidth } = toRefs(props.chartConfig.option)
const blurFilterId = computed(() => `circle-point-blur-${props.chartConfig.id}`)

const hasValidMatchValues = (state: StateItem): boolean => {
  return state.matchValues && state.matchValues.length > 0 && state.matchValues.some(mv => mv !== '' && mv !== null && mv !== undefined)
}

const getRealtimeValue = (data: any): any => {
  if (data === null || data === undefined) return data
  if (Array.isArray(data)) {
    if (data.length === 0) return null
    const last = data[data.length - 1]
    if (typeof last === 'object' && last !== null) {
      const valKeys = ['value', 'val', 'status', 'data', 'result', 'current', 'realtime']
      for (const key of valKeys) {
        if (key in last && last[key] !== null && last[key] !== undefined) {
          return last[key]
        }
      }
      const firstVal = Object.values(last).find(v => v !== null && v !== undefined && typeof v !== 'object')
      return firstVal !== undefined ? firstVal : last
    }
    return last
  }
  if (typeof data === 'object' && data !== null) {
    const valKeys = ['value', 'val', 'status', 'data', 'result', 'current', 'realtime']
    for (const key of valKeys) {
      if (key in data && data[key] !== null && data[key] !== undefined) {
        return data[key]
      }
    }
    const firstVal = Object.values(data).find(v => v !== null && v !== undefined && typeof v !== 'object')
    return firstVal !== undefined ? firstVal : data
  }
  return data
}

const matchedState = computed<StateItem>(() => {
  const states: StateItem[] = props.chartConfig.option.states || []
  const dataset = props.chartConfig.option.dataset
  if (states.length === 0) {
    return { value: 0, matchValues: [], outCircleColor: '#3f5261', inCircleColor: '#ffffff', flash: false, frequency: 100 }
  }
  const dataVal = getRealtimeValue(dataset)
  if (dataVal === null || dataVal === undefined) {
    const previewIndex = props.chartConfig.option.previewState ?? 0
    return states[Math.min(previewIndex, states.length - 1)] || states[0]
  }
  const dataStr = String(dataVal)
  for (let i = states.length - 1; i >= 0; i--) {
    const state = states[i]
    if (!hasValidMatchValues(state)) continue
    if (state.matchValues.some((mv) => String(mv) === dataStr)) {
      return state
    }
  }
  return states[0]
})

const flashStyle = computed(() => {
  if (matchedState.value.flash) {
    const duration = (matchedState.value.frequency || 100) / 1000
    return {
      animation: `circleFlash ${duration}s ease-in-out infinite`
    }
  }
  return {}
})

useChartDataFetch(props.chartConfig, useChartEditStore, (_newData: any) => {
})
</script>

<style>
@keyframes circleFlash {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.2; }
}
</style>

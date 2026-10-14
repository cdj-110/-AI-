<template>
  <CollapseItem name="线段" :expanded="true">
    <SettingItemBox name="颜色">
      <SettingItem name="线条颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.borderColor"></n-color-picker>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="线条粗细">
      <n-input-number :value="optionData.chuxi" :min=".5" :max="100" :step=".5" size="small" placeholder="线条粗细" @update:value="value => updateOption('chuxi', value)" />
    </SettingItemBox>
    <SettingItemBox v-if="chartAttr" name="长度 / 角度">
      <n-input-number :value="length" :min="1" :max="100000" size="small" placeholder="长度" @update:value="setLength" />
      <n-input-number :value="angle" :min="-180" :max="180" size="small" placeholder="角度" @update:value="setAngle" />
    </SettingItemBox>
    <SettingItemBox name="线型"><n-select :value="optionData.lineStyle || 'solid'" :options="lineStyles" size="small" @update:value="value => updateOption('lineStyle', value)" /></SettingItemBox>
    <SettingItemBox name="端点样式"><n-select :value="optionData.lineCap || 'butt'" :options="caps" size="small" @update:value="value => updateOption('lineCap', value)" /></SettingItemBox>
    <SettingItemBox name="箭头">
      <n-checkbox :checked="!!optionData.startArrow" @update:checked="value => updateOption('startArrow', value)">起点</n-checkbox>
      <n-checkbox :checked="!!optionData.endArrow" @update:checked="value => updateOption('endArrow', value)">终点</n-checkbox>
    </SettingItemBox>
    <n-text depth="3" style="font-size: 12px">拖动两端改变线段，Shift 限制为水平、垂直或 45°。</n-text>
  </CollapseItem>
</template>

<script setup lang="ts">
import { computed, PropType } from 'vue'
import {
  CollapseItem,
  SettingItemBox,
  SettingItem
} from '@/components/Pages/ChartItemSetting'
import { option } from './config'
import { lineWorldPoints, setLineEndpoints } from '@/utils/figureLine'

const props = defineProps({
  optionData: {
    type: Object as PropType<typeof option>,
    required: true
  },
  chartAttr: { type: Object, required: false }
})
const lineStyles = [{ label: '实线', value: 'solid' }, { label: '虚线', value: 'dashed' }, { label: '点线', value: 'dotted' }]
const caps = [{ label: '平头', value: 'butt' }, { label: '圆头', value: 'round' }]
const points = computed(() => lineWorldPoints(props.chartAttr || { w: 300, h: props.optionData.chuxi }, props.optionData))
const length = computed(() => Math.round(Math.hypot(points.value.end.x - points.value.start.x, points.value.end.y - points.value.start.y) * 100) / 100)
const angle = computed(() => Math.round(Math.atan2(points.value.end.y - points.value.start.y, points.value.end.x - points.value.start.x) * 18000 / Math.PI) / 100)
const commit = (start: any, end: any) => { if (props.chartAttr) setLineEndpoints({ attr: props.chartAttr, option: props.optionData }, start, end) }
function updateOption(key: string, value: any) {
  if (value === null || value === undefined) return
  const current = points.value
  ;(props.optionData as any)[key] = value
  if (['chuxi', 'lineCap', 'lineStyle', 'startArrow', 'endArrow'].includes(key)) commit(current.start, current.end)
}
function setVector(newLength: number, newAngle: number) {
  const start = points.value.start, radians = newAngle * Math.PI / 180
  commit(start, { x: start.x + Math.cos(radians) * newLength, y: start.y + Math.sin(radians) * newLength })
}
function setLength(value: number | null) { if (value !== null && value >= 1) setVector(value, angle.value) }
function setAngle(value: number | null) { if (value !== null) setVector(length.value, value) }
</script>

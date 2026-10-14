<template>
  <svg class="go-Figure" :viewBox="`0 0 ${line.width} ${line.height}`" preserveAspectRatio="none" style="overflow: visible">
    <line v-if="editing" class="line-hit-area" :x1="line.start.x" :y1="line.start.y" :x2="line.end.x" :y2="line.end.y"
      stroke="transparent" :stroke-width="Math.max(stroke, 12 / Math.max(.1, editorScale))" stroke-linecap="round" pointer-events="stroke" />
    <line class="line-stroke" :x1="line.start.x" :y1="line.start.y" :x2="line.end.x" :y2="line.end.y"
      :stroke="color" :stroke-width="stroke" :stroke-linecap="option.lineCap === 'round' || option.lineStyle === 'dotted' ? 'round' : 'butt'" :stroke-dasharray="dash" />
    <polygon v-if="option.startArrow" :points="arrowPoints(line.start, line.end, stroke)" :fill="color" pointer-events="visiblePainted" />
    <polygon v-if="option.endArrow" :points="arrowPoints(line.end, line.start, stroke)" :fill="color" pointer-events="visiblePainted" />
  </svg>
</template>

<script setup lang="ts">
import { computed, PropType } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { arrowPoints, lineStroke, renderedLine } from '@/utils/figureLine'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  },
  editing: { type: Boolean, default: false },
  editorScale: { type: Number, default: 1 }
})

const option = computed(() => props.chartConfig.option || {})
const line = computed(() => renderedLine(props.chartConfig.attr, option.value))
const stroke = computed(() => lineStroke(option.value))
const color = computed(() => option.value.borderColor || '#3774B6ff')
const dash = computed(() => option.value.lineStyle === 'dashed' ? `${stroke.value * 4} ${stroke.value * 2}`
  : option.value.lineStyle === 'dotted' ? `${stroke.value * .2} ${stroke.value * 2}` : undefined)
</script>

<style lang="scss" scoped>
@include go('Figure') { display: block; pointer-events: none; }
.line-stroke { pointer-events: stroke; }
</style>

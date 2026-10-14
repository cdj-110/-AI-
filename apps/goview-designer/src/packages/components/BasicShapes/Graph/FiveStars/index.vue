<template>
  <div class="go-FiveStars">
    <div :style="{
      width: '100%',
      height: '100%',
      clipPath: `polygon(50% 0%, 61% 35%, 100% 35%, 68% 57%, 79% 100%, 50% 70%, 21% 100%, 32% 57%, 0% 35%, 39% 35%)`,
      background: `linear-gradient(to bottom right, ${fillColor[0]}, ${fillColor[1]} ${fillColorNumber}%)`
    }"></div>
  </div>
</template>

<script setup lang="ts">
import { computed, PropType, toRefs, watch } from 'vue'
import { CreateComponentType } from '@/packages/index.d'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})

const { w, h } = toRefs(props.chartConfig.attr)
const { lineWidth, lineColor, fillColor, fillColorNumber } = toRefs(props.chartConfig.option)
</script>

<style lang="scss" scoped>
@include go('FiveStars') {}
</style>

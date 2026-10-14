<template>
  <div class="go-rect">
    <div :style="rectStyle"
    ></div>
  </div>
</template>

<script setup lang="ts">
import { PropType, toRefs, computed } from 'vue'
import { CreateComponentType } from '@/packages/index.d'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})

const { w, h } = toRefs(props.chartConfig.attr)
const { 
  borderColor, 
  borderNumber, 
  borderType, 
  backgroundLinearColor, 
  backgroundLinearNumber, 
  radius,
  showBorder // 引入填充开关
} = toRefs(props.chartConfig.option)

// 计算最终样式
const rectStyle = computed(() => {
  // ✅ 填充开关关闭时，背景完全透明
  if (!showBorder.value) {
    return {
      width: `${w.value}px`,
      height: `${h.value}px`,
      border: `${borderNumber.value}px ${borderType.value} ${borderColor.value}`,
      background: 'transparent',
      borderRadius: `${radius.value[0]}px ${radius.value[1]}px ${radius.value[2]}px ${radius.value[3]}px`,
      boxSizing: 'border-box'
    }
  }

  // 开关打开时，正常显示纯色/渐变
  const isSolid = backgroundLinearColor.value[0] === backgroundLinearColor.value[1]
  return {
    width: `${w.value}px`,
    height: `${h.value}px`,
    border: `${borderNumber.value}px ${borderType.value} ${borderColor.value}`,
    background: isSolid 
      ? backgroundLinearColor.value[0] 
      : `linear-gradient(to bottom right, ${backgroundLinearColor.value[0]}, ${backgroundLinearColor.value[1]} ${backgroundLinearNumber.value}%)`,
    borderRadius: `${radius.value[0]}px ${radius.value[1]}px ${radius.value[2]}px ${radius.value[3]}px`,
    boxSizing: 'border-box'
  }
})
</script>

<style lang="scss" scoped>
@include go('rect') {
  width: 100%;
  height: 100%;
}
</style>
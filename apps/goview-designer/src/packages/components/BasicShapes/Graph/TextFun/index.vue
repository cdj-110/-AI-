<template>
  <div class="go-TextFun">
    <n-statistic>
      <template #default>
        <span :style="`
          font-size: ${textSize1}px;
          font-weight: ${fontWeight1};
          color: ${color1};
          letter-spacing: ${letter1}px;
          line-height: ${lineHeight1};
          writing-mode: ${writing1};
          background: ${bgColor1};
          height: 100%;
          text-decoration: ${decoration1};
          font-style: ${fontStyle1};
          font-family: ${fontFamily1};
          `">
          {{ text }}
        </span>
      </template>
      <template #prefix>
        <span :style="`
          font-size: ${textSize2}px;
          font-weight: ${fontWeight2};
          color: ${color2};
          letter-spacing: ${letter2}px;
          line-height: ${lineHeight2};
          writing-mode: ${writing2};
          background: ${bgColor2};
          height: 100%;
          text-decoration: ${decoration2};
          font-style: ${fontStyle2};
          margin-right: ${ marginRight }px;
          font-family: ${fontFamily2};
          `">
          {{ prefixText }}
        </span>
      </template>
      <template #suffix>
        <span :style="`
          font-size: ${textSize3}px;
          font-weight: ${fontWeight3};
          color: ${color3};
          letter-spacing: ${letter3}px;
          line-height: ${lineHeight3};
          writing-mode: ${writing3};
          background: ${bgColor3};
          height: 100%;
          text-decoration: ${decoration3};
          font-style: ${fontStyle3};
          margin-left: ${ marginLeft }px;
          font-family: ${fontFamily3};
          `">
          {{ suffixText }}
        </span>
      </template>
    </n-statistic>
  </div>
</template>

<script setup lang="ts">
import { PropType, toRefs } from 'vue'
import { CreateComponentType } from '@/packages/index.d'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})

const { w, h } = toRefs(props.chartConfig.attr)
const {
  text, textSize1, color1, bgColor1, fontWeight1, letter1, lineHeight1, writing1, decoration1, fontStyle1, fontFamily1,
  prefixText, textSize2, color2, bgColor2, fontWeight2, letter2, lineHeight2, writing2, decoration2, fontStyle2, marginRight, fontFamily2,
  suffixText, textSize3, color3, bgColor3, fontWeight3, letter3, lineHeight3, writing3, decoration3, fontStyle3, marginLeft, fontFamily3
} = toRefs(props.chartConfig.option)
</script>

<style lang="scss" scoped>
@include go('TextFun') {

}
</style>

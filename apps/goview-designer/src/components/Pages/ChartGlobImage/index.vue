<!--
 * @Author: 何彩彩 "18210946822@163.com"
 * @Date: 2025-12-26 13:31:58
 * @LastEditors: 何彩彩 "18210946822@163.com"
 * @LastEditTime: 2026-01-23 10:30:21
 * @FilePath: \web\go-view-master\src\components\Pages\ChartGlobImage\index.vue
 * @Description: 这是默认设置,请设置 customMade，打开koroFileHeader查看配置 进行设置：https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
<template>
  <img class="list-img" v-lazy="imageInfo" alt="图表图片" />
</template>

<script setup lang="ts">
import { ref, PropType, watch } from 'vue'
import { fetchImages } from '@/packages'
import { ConfigType } from '@/packages/index.d'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<ConfigType>,
    required: true
  }
})

const imageInfo = ref('')

// 获取图片
const fetchImageUrl = async () => {
  imageInfo.value = await fetchImages(props.chartConfig)
}

watch(
  () => props.chartConfig.key,
  () => fetchImageUrl(),
  {
    immediate: true
  }
)
</script>

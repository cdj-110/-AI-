<template>
    <div class="go-ButtonFun" :style="`width: ${w}px; height: ${h}px; border: 1px solid ${borderColor};`">
        <!-- 按钮填满容器+文字居中 -->
        <n-button type="default"
            :style="{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: fontSize + 'px', color: fontColor,backgroundColor:bgColor,borderStyle: borderStyle,borderWidth: borderWidth + 'px',borderColor: borderColor,borderRadius: borderRadius + 'px' }">
            {{ fontText }}
        </n-button>
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
const { borderColor, fontSize, fontColor,bgColor,borderStyle,borderWidth,borderRadius,fontText } = toRefs(props.chartConfig.option)
</script>

<!-- 修复SCSS报错，改用原生CSS -->
<style scoped>
.go-ButtonFun {
    position: relative;
    display: inline-block;
    box-sizing: border-box;
}
</style>
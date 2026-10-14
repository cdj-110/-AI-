<!--
 * @Author: 何彩彩 "18210946822@163.com"
 * @Date: 2026-01-12 16:11:36
 * @LastEditors: 何彩彩 "18210946822@163.com"
 * @LastEditTime: 2026-01-23 10:10:32
 * @FilePath: \web\go-view-master\src\packages\components\BasicShapes\Graph\InputFun\index.vue
 * @Description: 这是默认设置,请设置 customMade，打开koroFileHeader查看配置 进行设置：https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
<!--
 * @Author: 何彩彩 "18210946822@163.com"
 * @Date: 2026-01-12 16:11:36
 * @LastEditors: 何彩彩 "18210946822@163.com"
 * @LastEditTime: 2026-01-23 09:36:42
 * @FilePath: \web\go-view-master\src\packages\components\BasicShapes\Graph\InputFun\index.vue
 * @Description: 这是默认设置,请设置 customMade，打开koroFileHeader查看配置 进行设置：https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
<template>
    <div class="go-InputFun">
        <!-- :style="`border: 1px solid ${borderColor};`" -->
        <n-input v-model:value="value" :type="type" :placeholder="placeholder" :size="size" :minlength="minLength"
            :maxlength="maxLength" style="width: 100%" :show-count="showCount"
            :style="enableRadius ? { borderRadius: '20px' } : {}">
            <template #prefix>
                <span v-if="prefix">{{ prefix }}</span>
            </template>
            <template #suffix>
                <span v-if="suffix">{{ suffix }}</span>
            </template>
        </n-input>
    </div>
</template>

<script setup lang="ts">
import { PropType, toRefs } from 'vue';
import { CreateComponentType } from '@/packages/index.d';
// 接收父组件传入的配置
const props = defineProps({
    chartConfig: {
        type: Object as PropType<CreateComponentType>,
        required: true
    },
});
const { w, h } = toRefs(props.chartConfig.attr)
// 解构配置属性（响应式）
const {
    value, placeholder, type,
    prefix, suffix, size,
    minLength, maxLength, enableRadius, showCount
} = toRefs(props.chartConfig.option);

// 实现v-model双向绑定
// const value = defineModel<string>('value');
</script>

<style lang="scss" scoped>
@include go(InputFun) {}
</style>

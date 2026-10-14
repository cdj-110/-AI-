<!--
 * @Author: 何彩彩 "18210946822@163.com"
 * @Date: 2026-01-12 16:11:36
 * @LastEditors: 何彩彩 "18210946822@163.com"
 * @LastEditTime: 2026-01-23 10:25:35
 * @FilePath: \web\go-view-master\src\packages\components\BasicShapes\Graph\InputFun\config.vue
 * @Description: 这是默认设置,请设置 customMade，打开koroFileHeader查看配置 进行设置：https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
<template>
    <!-- 边框配置 -->
    <!-- <CollapseItem name="边框" :expanded="true">
        <SettingItemBox name="样式">
            <SettingItem name="边框颜色">
                <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.borderColor" />
            </SettingItem>
        </SettingItemBox>
    </CollapseItem> -->

    <!-- 内容配置 -->
    <CollapseItem name="内容" :expanded="true">
         <SettingItemBox name="内容" class="conten">
            <!-- :maxlength="optionData.maxLength" :minLength="optionData.minLength" -->
             <n-input v-model:value="optionData.value"  :show-count="optionData.showCount" clearable/>
        </SettingItemBox>
        <SettingItemBox name="基础">
            <SettingItem name="类型">
                <n-select v-model:value="optionData.type" :options="[
                    { label: '文本', value: 'text' },
                    { label: '密码', value: 'password' },
                    { label: '多行文本', value: 'textarea' }
                ]" />
            </SettingItem>
            <SettingItem name="占位文字">
                <n-input v-model:value="optionData.placeholder" clearable/>
            </SettingItem>
            <SettingItem name="前缀">
                <n-input v-model:value="optionData.prefix" />
            </SettingItem>
            <SettingItem name="后缀">
                <n-input v-model:value="optionData.suffix" />
            </SettingItem>
        </SettingItemBox>
    </CollapseItem>

    <!-- 样式配置 -->
    <CollapseItem name="样式" :expanded="true">
        <SettingItemBox name="尺寸">
            <SettingItem name="大小">
                <n-select v-model:value="optionData.size" :options="[
                    { label: '小', value: 'small' },
                    { label: '中', value: 'medium' },
                    { label: '大', value: 'large' }
                ]" />
            </SettingItem>
        </SettingItemBox>
    </CollapseItem>

    <!-- 限制配置 -->
    <CollapseItem name="限制" :expanded="true">
        <SettingItemBox name="长度">
            <SettingItem name="最小输入长度">
                <n-input-number v-model:value="optionData.minLength" :min="0" controls-position="right" />
            </SettingItem>
            <SettingItem name="最大输入长度">
                <n-input-number v-model:value="optionData.maxLength" :min="0" controls-position="right" />
            </SettingItem>
        </SettingItemBox>
    </CollapseItem>
    <SettingItemBox name="其他">
  <SettingItem name="启用圆角">
    <n-switch v-model:value="optionData.enableRadius" />
  </SettingItem>
  <!-- 新增：显示数量开关 -->
  <SettingItem name="显示数量">
    <n-switch v-model:value="optionData.showCount" />
  </SettingItem>
</SettingItemBox>
</template>

<script setup lang="ts">
import { PropType } from 'vue';
import {
    CollapseItem,
    SettingItemBox,
    SettingItem
} from '@/components/Pages/ChartItemSetting';
import { CreateComponentType } from '@/packages/index.d';

// 接收外部传入的配置数据
const props = defineProps({
    optionData: {
        type: Object as PropType<CreateComponentType['option']>,
        required: true
    }
});
</script>
<style lang="scss" scoped>
.conten{
    display: none;
}
</style>
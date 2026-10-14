<template>
    <CollapseItem name="颜色" :expanded="true">
        <SettingItemBox name="纯色背景">
            <SettingItem name="勾选后颜色同步">
                <n-checkbox v-model:checked="optionData.colorIsLine"></n-checkbox>
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="渐变背景">
            <SettingItem name="颜色1">
                <n-color-picker v-model:value="props.optionData.gradientColor[0]" size="small"
                    :modes="['hex']"></n-color-picker>
            </SettingItem>
            <SettingItem name="颜色2">
                <n-color-picker v-model:value="props.optionData.gradientColor[1]" size="small"
                    :modes="['hex']"></n-color-picker>
            </SettingItem>
            <SettingItem name="渐变角度">
                <n-input-number v-model:value="optionData.backgroundLinearNumber" size="small"
                    :step="10"></n-input-number>
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="填充透明度">
            <SettingItem name="填充透明度">
                <n-input-number v-model:value="optionData.fillOpacity" size="small" :step="1" :min="0" :max="100"
                    :precision="0" placeholder="0-100">
                    <template #suffix>%</template>
                </n-input-number>
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="描边透明度">
            <SettingItem name="描边透明度">
                <n-input-number v-model:value="optionData.strokeOpacity" size="small" :step="1" :min="0" :max="100"
                    :precision="0" placeholder="0-100">
                    <template #suffix>%</template>
                </n-input-number>
            </SettingItem>
        </SettingItemBox>
    </CollapseItem>
    <CollapseItem name="边框" :expanded="true">
        <SettingItemBox name="边框颜色">
            <SettingItem name="边框颜色">
                <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.borderColor"></n-color-picker>
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="边框样式">
            <SettingItem name="边框类型">
                <n-select v-model:value="optionData.borderType" size="small" :options="[
                    { label: '实线', value: 'solid' },
                    { label: '虚线', value: 'dashed' }
                ]"></n-select>
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="描边宽度">
            <n-input-number v-model:value="optionData.borderWidth" size="small" placeholder="描边宽度" />
        </SettingItemBox>
        <SettingItemBox name="线段长度" v-if="optionData.borderType === 'dashed'">
            <SettingItem name="线段长度">
                <n-input-number v-model:value="optionData.dashLength" :min="1" :max="20" size="small"
                    placeholder="线段长度" />
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="间隙长度" v-if="optionData.borderType === 'dashed'">
            <SettingItem name="间隙长度">
                <n-input-number v-model:value="optionData.gapLength" :min="1" :max="20" size="small"
                    placeholder="间隙长度" />
            </SettingItem>
        </SettingItemBox>
    </CollapseItem>
</template>

<script setup lang="ts">
import { PropType, ref, watch } from 'vue'
import {
    CollapseItem,
    SettingItemBox,
    SettingItem
} from '@/components/Pages/ChartItemSetting'
import { option } from './config'

const props = defineProps({
    optionData: {
        type: Object as PropType<typeof option>,
        required: true
    }
})
//监听传值后判断是否勾选颜色同步
watch(props, (newval) => {
  if(newval.optionData.colorIsLine) {
    newval.optionData.gradientColor[1] = newval.optionData.gradientColor[0];
  }
},{immediate: true,deep:true});
</script>

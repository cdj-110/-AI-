<template>
  <CollapseItem name="配置" :expanded="true">
    <SettingItemBox name="纯色背景">
      <SettingItem name="勾选后颜色同步">
        <n-checkbox v-model:checked="optionData.colorIsLine"></n-checkbox>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="渐变背景">
      <SettingItem name="颜色1">
        <n-color-picker v-model:value="props.optionData.fillColor[0]" size="small" :modes="['hex']"></n-color-picker>
      </SettingItem>
      <SettingItem name="颜色2">
        <n-color-picker v-model:value="props.optionData.fillColor[1]" size="small" :modes="['hex']"></n-color-picker>
      </SettingItem>
      <SettingItem name="渐变角度">
        <n-input-number v-model:value="optionData.fillColorNumber" size="small" :step="10"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <!-- <SettingItemBox name="宽度">
      <SettingItem name="边框宽度">
        <n-input-number v-model:value="optionData.lineWidth" size="small"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="填充">
      <SettingItem name="填充颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.fillColor"></n-color-picker>
      </SettingItem>
    </SettingItemBox> -->
  </CollapseItem>
</template>

<script setup lang="ts">
import { PropType, watch } from 'vue'
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
    newval.optionData.fillColor[1] = newval.optionData.fillColor[0];
  }
},{immediate: true,deep:true});
</script>

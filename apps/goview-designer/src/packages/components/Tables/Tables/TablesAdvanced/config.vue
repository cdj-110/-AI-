<template>
  <collapse-item name="高级表格设置" :expanded="true">
    <n-tag type="info">查询条件会在预览页面调用服务端历史数据</n-tag>
    <setting-item-box :alone="true" name="关键词提示">
      <setting-item :alone="true">
        <n-input v-model:value="optionData.query.keywordPlaceholder" size="small" />
      </setting-item>
    </setting-item-box>
    <setting-item-box :alone="false" name="查询设置">
      <setting-item name="时间范围" :alone="true">
        <n-switch v-model:value="optionData.query.showTimeRange" />
      </setting-item>
      <setting-item name="每页条数" :alone="true">
        <n-select v-model:value="optionData.pagination.pageSize" size="small" :options="pageSizes" />
      </setting-item>
    </setting-item-box>
    <setting-item-box :alone="true" name="对齐方式">
      <setting-item :alone="true">
        <n-select v-model:value="optionData.align" size="small" :options="alignOptions" />
      </setting-item>
    </setting-item-box>
    <setting-item-box :alone="false" name="表格样式">
      <setting-item name="显示边框" :alone="true">
        <n-switch :value="optionData.style.border === 'on'" @update:value="value => optionData.style.border = value ? 'on' : 'off'" />
      </setting-item>
      <setting-item name="斑马条纹" :alone="true">
        <n-switch :value="optionData.style.striped === 'on'" @update:value="value => optionData.style.striped = value ? 'on' : 'off'" />
      </setting-item>
      <setting-item name="字体大小" :alone="true">
        <n-input-number v-model:value="optionData.style.fontSize" :min="12" :max="30" size="small" />
      </setting-item>
    </setting-item-box>
  </collapse-item>
</template>

<script setup lang="ts">
import { PropType } from 'vue'
import { option } from './config'
import { CollapseItem, SettingItemBox, SettingItem } from '@/components/Pages/ChartItemSetting'

defineProps({ optionData: { type: Object as PropType<typeof option>, required: true } })
const pageSizes = [5, 10, 15, 30, 50, 100].map(value => ({ label: String(value), value }))
const alignOptions = [
  { label: '靠左', value: 'left' },
  { label: '居中', value: 'center' },
  { label: '靠右', value: 'right' }
]
</script>

<template>
  <CollapseItem name="文本属性" :expanded="true">
    <SettingItemBox name="文本内容">
      <SettingItem name="主体文本">
        <n-input v-model:value="optionData.text" type="text" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文本补充">
      <SettingItem name="前缀文字">
        <n-input v-model:value="optionData.prefixText" type="text" />
      </SettingItem>
      <SettingItem name="后缀文字">
        <n-input v-model:value="optionData.suffixText" type="text" />
      </SettingItem>
    </SettingItemBox>
  </CollapseItem>
  <CollapseItem name="文本字体属性" :expanded="false">
    <SettingItemBox name="字体大小">
      <SettingItem name="改变字体大小">
        <n-input-number v-model:value="optionData.textSize1" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体粗细">
      <SettingItem name="改变字体粗细">
        <n-select v-model:value="optionData.fontWeight1" :options="[{value:500,label:'细'},{value:700,label:'粗'}]" />
        <!-- <n-input-number v-model:value="optionData.fontWeight1" size="small" :step="100"></n-input-number> -->
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体颜色">
      <SettingItem name="改变字体颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.color1"></n-color-picker>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="背景颜色">
      <SettingItem name="文字背景颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.bgColor1"></n-color-picker>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体间距">
      <SettingItem name="改变字体间距">
        <n-input-number v-model:value="optionData.letter1" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体行高">
      <SettingItem name="改变字体行间距">
        <n-input-number v-model:value="optionData.lineHeight1" size="small" :step="0.1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字方向">
      <SettingItem name="改变文字方向">
        <n-select v-model:value="optionData.writing1" :options="[{value:'horizontal-tb',label:'水平'},{value:'vertical-lr',label:'垂直'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字线段">
      <SettingItem name="改变文字标注">
        <n-select v-model:value="optionData.decoration1" :options="[{value:'',label:'无'},{value:'line-through',label:'删除线'},{value:'underline',label:'下划线'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字样式">
      <SettingItem name="改变文字样式">
        <n-select v-model:value="optionData.fontStyle1" :options="[{value:'',label:'正常'},{value:'italic',label:'斜体'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字字体">
      <SettingItem name="改变文字字体">
        <n-select v-model:value="optionData.fontFamily1" :options="[{value:'cursive',label:'cursive'},{value:'fangsong',label:'fangsong'},{value:'monospace',label:'monospace'},{value:'serif',label:'serif'},{value:'system-ui',label:'system-ui'},{value:'黑体',label:'黑体'},{value:'宋体',label:'宋体'},{value:'微软雅黑',label:'微软雅黑'}]" />
      </SettingItem>
    </SettingItemBox>
  </CollapseItem>
  <CollapseItem name="前缀字体属性" :expanded="false">
    <SettingItemBox name="字体大小">
      <SettingItem name="改变字体大小">
        <n-input-number v-model:value="optionData.textSize2" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体粗细">
      <SettingItem name="改变字体粗细">
        <n-select v-model:value="optionData.fontWeight2" :options="[{value:500,label:'细'},{value:700,label:'粗'}]" />
        <!-- <n-input-number v-model:value="optionData.fontWeight2" size="small" :step="100"></n-input-number> -->
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体颜色">
      <SettingItem name="改变字体颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.color2"></n-color-picker>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="背景颜色">
      <SettingItem name="文字背景颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.bgColor2"></n-color-picker>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体间距">
      <SettingItem name="改变字体间距">
        <n-input-number v-model:value="optionData.letter2" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体行高">
      <SettingItem name="改变字体行间距">
        <n-input-number v-model:value="optionData.lineHeight2" size="small" :step="0.1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字方向">
      <SettingItem name="改变文字方向">
        <n-select v-model:value="optionData.writing2" :options="[{value:'horizontal-tb',label:'水平'},{value:'vertical-lr',label:'垂直'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字线段">
      <SettingItem name="改变文字标注">
        <n-select v-model:value="optionData.decoration2" :options="[{value:'',label:'无'},{value:'line-through',label:'删除线'},{value:'underline',label:'下划线'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字样式">
      <SettingItem name="改变文字样式">
        <n-select v-model:value="optionData.fontStyle2" :options="[{value:'',label:'正常'},{value:'italic',label:'斜体'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="右边距">
      <SettingItem name="改变前缀右边距">
        <n-input-number v-model:value="optionData.marginRight" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字字体">
      <SettingItem name="改变文字字体">
        <n-select v-model:value="optionData.fontFamily2" :options="[{value:'cursive',label:'cursive'},{value:'fangsong',label:'fangsong'},{value:'monospace',label:'monospace'},{value:'serif',label:'serif'},{value:'system-ui',label:'system-ui'},{value:'黑体',label:'黑体'},{value:'宋体',label:'宋体'},{value:'微软雅黑',label:'微软雅黑'}]" />
      </SettingItem>
    </SettingItemBox>
  </CollapseItem>
  <CollapseItem name="后缀字体属性" :expanded="false">
    <SettingItemBox name="字体大小">
      <SettingItem name="改变字体大小">
        <n-input-number v-model:value="optionData.textSize3" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体粗细">
      <SettingItem name="改变字体粗细">
        <n-select v-model:value="optionData.fontWeight3" :options="[{value:500,label:'细'},{value:700,label:'粗'}]" />
        <!-- <n-input-number v-model:value="optionData.fontWeight3" size="small" :step="100"></n-input-number> -->
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体颜色">
      <SettingItem name="改变字体颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.color3"></n-color-picker>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="背景颜色">
      <SettingItem name="文字背景颜色">
        <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.bgColor3"></n-color-picker>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体间距">
      <SettingItem name="改变字体间距">
        <n-input-number v-model:value="optionData.letter3" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="字体行高">
      <SettingItem name="改变字体行间距">
        <n-input-number v-model:value="optionData.lineHeight3" size="small" :step="0.1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字方向">
      <SettingItem name="改变文字方向">
        <n-select v-model:value="optionData.writing3" :options="[{value:'horizontal-tb',label:'水平'},{value:'vertical-lr',label:'垂直'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字线段">
      <SettingItem name="改变文字标注">
        <n-select v-model:value="optionData.decoration3" :options="[{value:'',label:'无'},{value:'line-through',label:'删除线'},{value:'underline',label:'下划线'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字样式">
      <SettingItem name="改变文字样式">
        <n-select v-model:value="optionData.fontStyle3" :options="[{value:'',label:'正常'},{value:'italic',label:'斜体'}]" />
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="左边距">
      <SettingItem name="改变前缀左边距">
        <n-input-number v-model:value="optionData.marginLeft" size="small" :step="1"></n-input-number>
      </SettingItem>
    </SettingItemBox>
    <SettingItemBox name="文字字体">
      <SettingItem name="改变文字字体">
        <n-select v-model:value="optionData.fontFamily3" :options="[{value:'cursive',label:'cursive'},{value:'fangsong',label:'fangsong'},{value:'monospace',label:'monospace'},{value:'serif',label:'serif'},{value:'system-ui',label:'system-ui'},{value:'黑体',label:'黑体'},{value:'宋体',label:'宋体'},{value:'微软雅黑',label:'微软雅黑'}]" />
      </SettingItem>
    </SettingItemBox>
  </CollapseItem>
</template>

<script setup lang="ts">
import { PropType } from 'vue'
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
</script>

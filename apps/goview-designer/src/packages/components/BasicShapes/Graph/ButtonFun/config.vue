<template>
    <CollapseItem name="颜色" :expanded="true">
        <SettingItemBox name="文字颜色">
            <SettingItem name="按钮文字颜色">
                <n-color-picker v-model:value="optionData.fontColor" :modes="['hex']" :show-alpha="false" />
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="背景颜色">
            <SettingItem name="按钮背景颜色">
                <n-color-picker v-model:value="optionData.bgColor" :modes="['hex']" :show-alpha="false" />
            </SettingItem>
        </SettingItemBox>
    </CollapseItem>
    <CollapseItem name="边框" :expanded="true">
        <collapsingItem name="边框配置" :expanded="true">
            <!-- 边框开关 -->
            <SettingItemBox name="显示边框">
                <SettingItem>
                    <n-switch v-model:value="optionData.showBorder" />
                </SettingItem>
            </SettingItemBox>

            <!-- 边框宽度 -->
            <SettingItemBox v-if="optionData.showBorder" name="边框宽度">
                <SettingItem name="边框宽度">
                    <n-input-number v-model:value="optionData.borderWidth" :min="1" :max="20" :step="1" suffix="px" />
                </SettingItem>
            </SettingItemBox>

            <!-- 边框颜色 -->
            <SettingItemBox v-if="optionData.showBorder" name="边框颜色">
                <SettingItem name="边框颜色">
                    <n-color-picker v-model:value="optionData.borderColor" />
                </SettingItem>
            </SettingItemBox>

            <!-- 圆角设置 -->
            <SettingItemBox name="圆角设置">
                <SettingItem name="圆角半径">
                    <n-input-number v-model:value="optionData.borderRadius" :min="0" :max="50" :step="1" suffix="px" />
                </SettingItem>
            </SettingItemBox>
        </collapsingItem>
    </CollapseItem>
    <CollapseItem name="文字配置" :expanded="true">
        <SettingItemBox name="文字内容">
            <SettingItem name="改变文字内容">
                <n-input v-model:value="optionData.fontText" clearable />
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="文字大小">
            <SettingItem name="改变文字大小">
                <n-input-number v-model:value="optionData.fontSize" clearable />
            </SettingItem>
        </SettingItemBox>
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
    if (newval.optionData.showBorder == false) {
        newval.optionData.borderWidth = 0;
    }
}, { immediate: true, deep: true });
</script>

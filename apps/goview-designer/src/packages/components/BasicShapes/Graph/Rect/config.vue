<template>
    <CollapseItem name="边框" :expanded="true">
        <SettingItemBox name="颜色">
            <SettingItem name="边框颜色">
                <n-color-picker size="small" :modes="['hex']" v-model:value="optionData.borderColor" />
            </SettingItem>
            <SettingItem>
                <n-button size="small" @click="optionData.borderColor = option.borderColor">恢复默认</n-button>
            </SettingItem>
        </SettingItemBox>

        <SettingItemBox name="粗细">
            <SettingItem name="边框粗细">
                <n-input-number v-model:value="optionData.borderNumber" size="small" :step="1" :min="0" />
            </SettingItem>
        </SettingItemBox>

        <SettingItemBox name="样式">
            <SettingItem name="边框样式">
                <n-select v-model:value="optionData.borderType" :options="borderTypeOption" />
            </SettingItem>
        </SettingItemBox>

        <!-- 圆角模块 -->
        <SettingItemBox name="圆角">
            <div class="radius-container">
                <div class="radius-switch">
                    <n-button size="tiny" :type="isUnified ? 'primary' : 'default'" @click="toggleUnified(true)">
                        <n-icon>
                            <square />
                        </n-icon>
                    </n-button>
                    <n-button size="tiny" :type="!isUnified ? 'primary' : 'default'" @click="toggleUnified(false)">
                        <n-icon>
                            <circle />
                        </n-icon>
                    </n-button>
                </div>
            </div>
        </SettingItemBox>
        <SettingItemBox>
            <SettingItem v-if="isUnified" class="radius-input">
                <n-input-number v-model:value="unifiedRadius" size="small" :step="1" :min="0" />
            </SettingItem>
            <div v-else class="radius-grid">
                <SettingItem>
                    <n-input-number v-model:value="optionData.radius[0]" size="small" :step="1" :min="0" />
                </SettingItem>
                <SettingItem>
                    <n-input-number v-model:value="optionData.radius[1]" size="small" :step="1" :min="0" />
                </SettingItem>
                <SettingItem>
                    <n-input-number v-model:value="optionData.radius[2]" size="small" :step="1" :min="0" />
                </SettingItem>
                <SettingItem>
                    <n-input-number v-model:value="optionData.radius[3]" size="small" :step="1" :min="0" />
                </SettingItem>
            </div>
        </SettingItemBox>
    </CollapseItem>

    <CollapseItem name="填充" :expanded="true">
        <!-- 填充开关 -->
        <SettingItemBox name="填充开关">
            <SettingItem>
                <n-switch v-model:value="optionData.showBorder" />
            </SettingItem>
        </SettingItemBox>

        <!-- ✅ 开关打开时才显示下面的设置 -->
        <template v-if="optionData.showBorder">
            <SettingItemBox name="纯色背景">
                <SettingItem name="勾选后颜色同步">
                    <n-checkbox v-model:checked="isColorSync" @update:checked="handleColorSyncChange">
                    </n-checkbox>
                </SettingItem>
            </SettingItemBox>

            <!-- 渐变背景 -->
            <SettingItemBox name="渐变背景">
                <div class="gradient-row">
                    <SettingItem name="颜色1">
                        <n-color-picker v-model:value="optionData.backgroundLinearColor[0]" size="small" :modes="['hex']" />
                    </SettingItem>
                    <SettingItem name="颜色2">
                        <n-color-picker v-model:value="optionData.backgroundLinearColor[1]" size="small" :modes="['hex']"
                            :disabled="isColorSync" />
                    </SettingItem>
                </div>
                <SettingItem name="渐变角度">
                    <n-input-number v-model:value="optionData.backgroundLinearNumber" size="small" :step="10" />
                </SettingItem>
            </SettingItemBox>
        </template>
    </CollapseItem>
</template>

<script setup lang="ts">
import { PropType, watch, ref, computed } from 'vue'
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

// 控制切换状态
const isUnified = ref<boolean>(true)

// 统一圆角值（临时变量，初始值从配置读取）
const unifiedRadius = ref<number>(props.optionData.radius[0])

// 切换按钮事件
const toggleUnified = (value: boolean) => {
    isUnified.value = value
    // 切换到统一设置时，同步当前值
    if (value) {
        unifiedRadius.value = props.optionData.radius[0]
    }
}

// 监听统一圆角的变化，同步到四个角
watch(unifiedRadius, (newVal) => {
    if (newVal !== null && newVal !== undefined) {
        props.optionData.radius = [newVal, newVal, newVal, newVal]
    }
})

// 监听独立圆角的变化，如果它们相等，则更新统一圆角
watch(() => props.optionData.radius, (newVal) => {
    if (newVal[0] === newVal[1] && newVal[1] === newVal[2] && newVal[2] === newVal[3]) {
        unifiedRadius.value = newVal[0]
        isUnified.value = true
    } else {
        isUnified.value = false
    }
}, { deep: true })

// 颜色同步计算属性（数字转布尔）
const isColorSync = computed({
    get: () => props.optionData.colorIsLine === 1,
    set: (val) => {
        props.optionData.colorIsLine = val ? 1 : 0
    }
})

// 处理颜色同步开关变化
const handleColorSyncChange = (checked: boolean) => {
    if (checked) {
        // 勾选时，立即同步颜色1到颜色2
        props.optionData.backgroundLinearColor[1] = props.optionData.backgroundLinearColor[0]
    }
}

// 监听颜色1的变化，同步时自动更新颜色2
watch(
    () => props.optionData.backgroundLinearColor[0],
    (newColor) => {
        if (isColorSync.value) {
            props.optionData.backgroundLinearColor[1] = newColor
        }
    }
)

// 初始加载时，如果已勾选，确保颜色同步
watch(
    () => props.optionData.colorIsLine,
    (newVal) => {
        if (newVal === 1) {
            props.optionData.backgroundLinearColor[1] = props.optionData.backgroundLinearColor[0]
        }
    },
    { immediate: true }
)

/**
 * 边框样式
 */
const borderTypeOption = [
    { label: "实线", value: "solid" },
    { label: "粗虚线", value: "dashed" },
    { label: "细虚线", value: "dotted" },
]
</script>

<style scoped>
.radius-switch {
    display: flex;
    gap: 8px;
}

.gradient-row {
    gap: 16px;
}
</style>
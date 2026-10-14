<template>
    <CollapseItem name="样式" :expanded="true">
        <SettingItemBox name="样式">
            <SettingItem name="外圆环半径">
                <n-input-number size="small" v-model:value="optionData.outCircle"></n-input-number>
            </SettingItem>
            <SettingItem name="内部圆形半径">
                <n-input-number size="small" v-model:value="optionData.inCircle"></n-input-number>
            </SettingItem>
            <SettingItem name="外圆环粗细">
                <n-input-number size="small" v-model:value="optionData.outCircleWidth"></n-input-number>
            </SettingItem>
        </SettingItemBox>
    </CollapseItem>

    <CollapseItem name="状态" :expanded="true">
        <div>
            <SettingItem name="预览状态">
                <n-select size="small" :options="stateOptions" v-model:value="optionData.previewState" />
            </SettingItem>
            <div v-for="(state, index) in optionData.states" :key="index" @click="setPreviewState(index)">
                <SettingItemBox :name="`状态${index + 1}`">
                    <SettingItem name="匹配值">
                        <n-input v-for="(mv, mvIdx) in state.matchValues" :key="mvIdx" size="small"
                            v-model:value="state.matchValues[mvIdx]" class="match-value-input" placeholder="" />
                    </SettingItem>
                    <n-icon v-if="index <= 1" > </n-icon>
                    <n-icon v-if="index >= 2" size="22" class="state-remove-btn" @click.stop="removeState(index)">
                        <RemoveIcon />
                    </n-icon>
                    <SettingItem name="外圆环颜色">
                        <n-color-picker size="small" :modes="['hex']"
                            v-model:value="state.outCircleColor"></n-color-picker>
                    </SettingItem>
                    <SettingItem name="内部圆形颜色">
                        <n-color-picker size="small" :modes="['hex']"
                            v-model:value="state.inCircleColor"></n-color-picker>
                    </SettingItem>
                    <SettingItem name="闪烁">
                        <n-switch v-model:value="state.flash" style="display: flex;justify-content: left;" />
                    </SettingItem>
                    <SettingItem name="频率">
                        <n-input-number size="small" v-model:value="state.frequency">
                        </n-input-number>
                    </SettingItem>
                </SettingItemBox>
            </div>

            <div class="add-state-btn" @click="addState">
                <n-icon size="18">
                    <AddIcon />
                </n-icon>
                <span>添加状态</span>
            </div>
        </div>
    </CollapseItem>
</template>

<script setup lang="ts">
import { computed, PropType } from 'vue'
import {
    CollapseItem,
    SettingItemBox,
    SettingItem
} from '@/components/Pages/ChartItemSetting'
import { option, StateItem } from './config'
import { icon } from '@/plugins'

const { RemoveIcon, AddIcon } = icon.ionicons5

const props = defineProps({
    optionData: {
        type: Object as PropType<typeof option>,
        required: true
    }
})

const stateOptions = computed(() => {
    return props.optionData.states.map((_, index) => ({
        label: `状态${index + 1}`,
        value: index
    }))
})

const setPreviewState = (index: number) => {
    props.optionData.previewState = index
}

const addState = () => {
    const lastIndex = props.optionData.states.length - 1
    const lastState = props.optionData.states[lastIndex]
    const newState: StateItem = {
        value: lastIndex + 2,
        matchValues: [''],
        outCircleColor: lastState?.outCircleColor || '#3f5261',
        inCircleColor: lastState?.inCircleColor || '#ffffff',
        flash: false,
        frequency: lastState?.frequency || 100
    }
    props.optionData.states.push(newState)
}

const removeState = (index: number) => {
    if (index >= 2) {
        props.optionData.states.splice(index, 1)
        if (props.optionData.previewState >= index) {
            props.optionData.previewState = Math.max(0, props.optionData.previewState - 1)
        }
    }
}
</script>

<style lang="scss" scoped>
.state-remove-btn {
    color: #ff4d4f;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background-color: rgba(255, 77, 79, 0.15);
    transition: background-color 0.2s;
    margin-left: auto;
    flex-shrink: 0;

    &:hover {
        background-color: rgba(255, 77, 79, 0.3);
    }
}

.add-state-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.3rem;
    padding: 0.5rem;
    border: 1px dashed rgba(255, 255, 255, 0.25);
    border-radius: 6px;
    cursor: pointer;
    color: rgba(255, 255, 255, 0.7);
    font-size: 0.85rem;
    transition: all 0.2s;

    &:hover {
        border-color: #0a7ae2;
        color: #0a7ae2;
        background-color: rgba(10, 122, 226, 0.08);
    }
}
</style>

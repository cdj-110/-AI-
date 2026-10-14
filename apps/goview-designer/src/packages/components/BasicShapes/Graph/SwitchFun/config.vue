<template>
    <CollapseItem name="开关配置" :expanded="true">
        <SettingItemBox name="背景颜色">
            <SettingItem name="打开背景颜色">
                <n-color-picker v-model:value="optionData.kaiColor" :modes="['hex']" :show-alpha="false" />
            </SettingItem>
            <SettingItem name="关闭背景颜色">
                <n-color-picker v-model:value="optionData.guanColor" :modes="['hex']" :show-alpha="false" />
            </SettingItem>
        </SettingItemBox>
        <SettingItemBox name="点位控制" :alone="true">
            <SettingItem name="允许下发"><div class="setting-with-help"><n-switch v-model:value="optionData.controlEnabled" /><n-tooltip trigger="hover" placement="left" :style="{maxWidth:'280px'}"><template #trigger><button class="help-trigger" aria-label="查看点位控制说明"><n-icon :size="17"><HelpOutlineIcon /></n-icon></button></template>编辑时不下发。开启后仅在预览/运行中控制一个可写点位，需管理员权限；布尔点位使用 true/false。</n-tooltip></div></SettingItem>
        </SettingItemBox>
        <SettingItemBox name="值识别方式" :alone="true">
            <SettingItem><div class="setting-with-help"><n-select v-model:value="valueMode" :options="[{value:'auto',label:'自动识别'},{value:'manual',label:'手动设置'}]" /><n-tooltip trigger="hover" placement="left" :style="{maxWidth:'280px'}"><template #trigger><button class="help-trigger" aria-label="查看值识别说明"><n-icon :size="17"><HelpOutlineIcon /></n-icon></button></template>自动模式根据绑定点位类型和实际上报值识别 true/false、1/0、ON/OFF 等常见格式。特殊数值或文本请切换手动设置；手动类型须与实际点位一致，打开和关闭值不能相同。</n-tooltip></div></SettingItem>
        </SettingItemBox>
        <SettingItemBox v-if="valueMode==='manual'" name="值类型">
            <SettingItem><n-select v-model:value="manualValueType" :options="[{value:'BOOLEAN',label:'布尔值'},{value:'NUMBER',label:'数值'},{value:'STRING',label:'文本'}]" /></SettingItem>
        </SettingItemBox>
        <SettingItemBox v-if="valueMode==='manual'&&manualValueType==='BOOLEAN'" name="布尔开关值">
            <SettingItem name="打开"><n-switch :value="optionData.controlOnBool??true" @update:value="value=>optionData.controlOnBool=value"><template #checked>true</template><template #unchecked>false</template></n-switch></SettingItem>
            <SettingItem name="关闭"><n-switch :value="optionData.controlOffBool??false" @update:value="value=>optionData.controlOffBool=value"><template #checked>true</template><template #unchecked>false</template></n-switch></SettingItem>
        </SettingItemBox>
        <SettingItemBox v-if="valueMode==='manual'&&manualValueType==='NUMBER'" name="数值开关值">
            <SettingItem name="打开"><n-input-number v-model:value="optionData.controlOnNumber" /></SettingItem>
            <SettingItem name="关闭"><n-input-number v-model:value="optionData.controlOffNumber" /></SettingItem>
        </SettingItemBox>
        <SettingItemBox v-if="valueMode==='manual'&&manualValueType==='STRING'" name="文本开关值">
            <SettingItem name="打开"><n-input v-model:value="optionData.controlOnText" maxlength="2048" /></SettingItem>
            <SettingItem name="关闭"><n-input v-model:value="optionData.controlOffText" maxlength="2048" /></SettingItem>
        </SettingItemBox>
    </CollapseItem>
</template>

<script setup lang="ts">
import { PropType, computed } from 'vue'
import {
    CollapseItem,
    SettingItemBox,
    SettingItem
} from '@/components/Pages/ChartItemSetting'
import { option } from './config'
import { icon } from '@/plugins'
const { HelpOutlineIcon }=icon.ionicons5

const props = defineProps({
    optionData: {
        type: Object as PropType<typeof option>,
        required: true
    }
})
const valueMode=computed({get:()=>props.optionData.valueMode||'auto',set:(value:string)=>{props.optionData.valueMode=value}})
const manualValueType=computed({get:()=>props.optionData.manualValueType||'NUMBER',set:(value:string)=>{props.optionData.manualValueType=value}})
</script>

<style lang="scss" scoped>
.setting-with-help{display:flex;align-items:center;gap:8px;width:100%;min-width:0}
.setting-with-help :deep(.n-select){flex:1;min-width:0}
.help-trigger{display:inline-flex;align-items:center;justify-content:center;flex-shrink:0;width:24px;height:24px;padding:0;border:0;background:transparent;color:#92959b;cursor:help;border-radius:4px}
.help-trigger:hover,.help-trigger:focus-visible{color:#63d6c0;outline:1px solid currentColor}
:deep(.n-switch){
    justify-content: left;
}
</style>

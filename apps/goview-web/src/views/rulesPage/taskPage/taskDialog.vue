<template>
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="40%" align-center @close="emit('updateVisible', false)">
        <el-form :model="form" label-width="auto">
            <el-form-item :label="t('taskPage.taskName')">
                <el-input v-model="form.ruleTitle" :placeholder="t('taskPage.pleaseEnterTaskName')" />
            </el-form-item>
            <el-form-item :label="t('taskPage.deviceSourceType')">
                <el-radio-group v-model="form.facilityType">
                    <el-radio :label="t('taskPage.device')" value="1" />
                    <el-radio :label="t('taskPage.deviceModel')" value="2" />
                </el-radio-group>
            </el-form-item>
            <el-form-item :label="t('taskPage.deviceSource')">
                <el-select v-model="form.facilityFountain" :placeholder="t('taskPage.pleaseSelectDeviceSource')">
                    <el-option v-for="item in facilityList" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
            </el-form-item>
            <div style="font-size: 1.1rem;color: #303133;margin-bottom: 1rem;">{{ t('taskPage.actionConfig') }}</div>
            <el-form-item :label="t('taskPage.taskType')">
                <el-radio-group v-model="form.taskType">
                    <el-radio :label="t('taskPage.repeatExecution')" :value="1" />
                    <el-radio :label="t('taskPage.executeOnce')" :value="2" />
                    <el-radio :label="t('taskPage.advancedConfig')" :value="3" />
                </el-radio-group>
            </el-form-item>
            <el-form-item :label="t('taskPage.executionAction')">
                <el-row :gutter="5" v-for="(item, index) in triggerList" :key="index" style="width: 100%;">
                    <el-col :span="9" style="width: 100%;">
                        <el-select v-model="item.trigger2" placeholder="" style="width: 100%;">
                            <el-option label="temperature" value="temperature" />
                        </el-select>
                    </el-col>
                    <el-col :span="5" style="width: 100%;">
                        <el-select v-model="item.trigger3" placeholder="" style="width: 100%;">
                            <el-option :label="t('taskPage.greaterThan')" value="greaterThan" />
                            <el-option :label="t('taskPage.lessThan')" value="lessThan" />
                            <el-option :label="t('taskPage.equalTo')" value="equalTo" />
                        </el-select>
                    </el-col>
                    <el-col :span="9" style="width: 100%;">
                        <el-select v-model="item.trigger1" placeholder="" style="width: 100%;">
                            <el-option :label="t('taskPage.and')" value="AND" />
                            <el-option :label="t('taskPage.or')" value="OR" />
                        </el-select>
                    </el-col>
                    <el-col :span="1" style="width: 100%;">
                        <el-icon color="#f57c7c" size="20" @click="triggerClick(0, index)" style="margin-top: 0.5rem;cursor: pointer;">
                            <DeleteFilled />
                        </el-icon>
                    </el-col>
                </el-row>
                <el-button type="primary" plain @click="triggerClick(1, 0)" style="margin-top: 1rem;">{{ t('taskPage.addAction') }}</el-button>
            </el-form-item>
            <el-form-item :label="t('taskPage.planWeek')" v-if="form.taskType === 1">
                <el-checkbox-group v-model="form.week">
                    <el-checkbox :label="t('taskPage.monday')" :value="t('taskPage.monday')" />
                    <el-checkbox :label="t('taskPage.tuesday')" :value="t('taskPage.tuesday')" />
                    <el-checkbox :label="t('taskPage.wednesday')" :value="t('taskPage.wednesday')" />
                    <el-checkbox :label="t('taskPage.thursday')" :value="t('taskPage.thursday')" />
                    <el-checkbox :label="t('taskPage.friday')" :value="t('taskPage.friday')" />
                    <el-checkbox :label="t('taskPage.saturday')" :value="t('taskPage.saturday')" />
                    <el-checkbox :label="t('taskPage.sunday')" :value="t('taskPage.sunday')" />
                </el-checkbox-group>
            </el-form-item>
            <el-form-item :label="t('taskPage.planDate')" v-if="form.taskType === 2">
                <el-date-picker v-model="form.yearValue" type="date" :placeholder="t('taskPage.pleaseSelectPlanDate')" />
            </el-form-item>
            <el-form-item :label="t('taskPage.time')" v-if="form.taskType === 1 || form.taskType === 2">
                <el-time-picker v-model="form.weekTime" :placeholder="t('taskPage.pleaseSelectTime')" />
            </el-form-item>
            <el-form-item :label="t('taskPage.cronExpression')" v-if="form.taskType === 3">
                <el-input v-model="form.cron" :placeholder="t('taskPage.pleaseEnterCron')" @click="openCronGenerator" />
            </el-form-item>
            <el-alert type="warning" :closable="false">
                <template #title>
                    <div style="color: #303133;font-weight: 500;">
                        <div>{{ t('taskPage.note') }}</div>
                        <div>{{ t('taskPage.cronFormat') }}</div>
                        <div>{{ t('taskPage.cronSixFields') }}</div>
                        <div style="display: flex;align-items: center;">
                            <p>{{ t('taskPage.cronExample') }}</p>
                            <p @click="openCronGenerator" style="position: absolute;right: 1rem;color: #1C5CFF;cursor: pointer;">{{ t('taskPage.cronGenerator') }}</p>
                        </div>
                    </div>
                </template>
            </el-alert>
        </el-form>
        <template #footer>
            <el-button type="info" @click="handleClose">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" @click="handleSubmit">{{ t('taskPage.create') }}</el-button>
        </template>
    </el-dialog>
    <cronGenerator v-model="cronVisible" @updateVisible="updateVisible" />
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { DeleteFilled } from '@element-plus/icons-vue'
import cronGenerator from './cronGenerator.vue'

const { t } = useI18n()

const props = defineProps<{
    visible: boolean
    type: number
    row?: any
}>()
let dialogVisible = ref(true)
const emit = defineEmits<{
    (e: 'updateVisible', value: boolean): void
    (e: 'submit', data: any): void
}>()

const facilityList = ref([{ value: "设备源1", label: "设备源1" }, { value: "设备源2", label: "设备源2" }])

const form = ref({ 
    ruleTitle: "", 
    facilityType: "", 
    taskType: 1, 
    week: [], 
    weekTime: "", 
    facilityFountain: "", 
    yearValue: "", 
    cron: "" 
})

const triggerList = ref([{ trigger1: "", trigger2: "", trigger3: "" }])
const cronVisible = ref(false)

const dialogTitle = computed(() => {
    return props.type === 0 ? t('taskPage.addTask') : t('taskPage.editTask')
})

const triggerClick = (i: number, index: number) => {
    if (i === 0) {
        triggerList.value.splice(index, 1)
    } else {
        triggerList.value.push({ trigger1: "", trigger2: "", trigger3: "" })
    }
}
// Cron表达式生成器打开弹窗
const openCronGenerator = () => {
    cronVisible.value = true
}
// Cron表达式生成器取消弹窗
let updateVisible = () => {
    cronVisible.value = false
}

const handleClose = () => {
    emit('updateVisible', false)
}

const handleSubmit = () => {
    emit('submit', { ...form.value, triggerList: triggerList.value })
    emit('updateVisible', false)
}
</script>
<style lang="scss" scoped>
:deep(.el-form-item__label) {
    white-space: nowrap;
}
</style>

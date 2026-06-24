<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="t('warning.warningRule.allocateGroup')" width="550" @close="emit('onlineAllocationGroupClose', false)">
            <div style="padding:0.5rem 1rem;">
                <el-form ref="ruleFormRef" :model="ruleForm" :rules="rules" label-width="7rem">
                    <el-form-item :label="t('warning.warningRule.notificationGroup')" prop="region">
                        <el-select v-model="ruleForm.region" :placeholder="t('warning.warningRule.pleaseSelect')">
                            <el-option label="Zone one" value="shanghai" />
                            <el-option label="Zone two" value="beijing" />
                        </el-select>
                    </el-form-item>
                    <div class="cont_text">
                        {{ t('warning.warningRule.notificationMethod') }}
                        <el-tooltip class="box-item" effect="dark" placement="top">
                            <template #content>
                                <span style="width: 22rem; display: inline-block;">
                                    {{ t('warning.warningRule.notificationMethodTip') }}
                                </span>
                            </template>
                            <el-icon style="color: #909399; cursor: help;">
                                <QuestionFilled />
                            </el-icon>
                        </el-tooltip>
                    </div>
                    <div style="padding: 1rem 0 1rem 2.6rem;">
                        <div class="flex_end"> <span class="size">{{ t('warning.warningRule.wechat') }}</span>
                            <span><el-switch v-model="ruleForm.value1" active-value="1" inactive-value="2" /></span>
                        </div>
                        <div class="flex_end"> <span class="size">{{ t('warning.warningRule.sms') }}</span>
                            <span><el-switch v-model="ruleForm.value2" active-value="1" inactive-value="2" /></span>
                        </div>
                        <div class="flex_end"> <span class="size">{{ t('warning.warningRule.phone') }}</span>
                            <span><el-switch v-model="ruleForm.value3" active-value="1" inactive-value="2" /></span>
                        </div>
                        <div class="flex_end"><span class="size">{{ t('warning.warningRule.email') }}</span>
                            <span><el-switch v-model="ruleForm.value4" active-value="1" inactive-value="2" /></span>
                        </div>
                        <div class="flex_end"> <span class="size">{{ t('warning.warningRule.app') }}</span>
                            <span><el-switch v-model="ruleForm.value5" active-value="1" inactive-value="2" /></span>
                        </div>
                        <div class="flex_end" style="margin-top: 2rem;">
                            <span class="size">
                                {{ t('warning.warningRule.dailyLimit') }}
                                <el-tooltip class="box-item" effect="dark" placement="top">
                                    <template #content>
                                        <span style="width: 22rem; display: inline-block;">
                                            {{ t('warning.warningRule.dailyLimitTip') }}
                                        </span>
                                    </template>
                                    <el-icon style="color: #909399; cursor: help;">
                                        <QuestionFilled />
                                    </el-icon>
                                </el-tooltip>
                            </span>
                            <span>
                                <el-input-number v-model="ruleForm.num" :min="1" :max="10" controls-position="right" />
                            </span>
                        </div>
                        <div class="flex_end">
                            <span class="size">
                                {{ t('warning.warningRule.deviceDailyLimit') }}
                                <el-tooltip class="box-item" effect="dark" placement="bottom">
                                    <template #content>
                                        <span style="width: 22rem; display: inline-block;">
                                            {{ t('warning.warningRule.deviceDailyLimitTip') }}
                                        </span>
                                    </template>
                                    <el-icon style="color: #909399; cursor: help;">
                                        <QuestionFilled />
                                    </el-icon>
                                </el-tooltip>
                            </span>
                            <span>
                                <el-input-number v-model="ruleForm.num1" :min="1" :max="10" controls-position="right" />
                            </span>
                        </div>
                    </div>
                </el-form>

            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('onlineAllocationGroupClose', false)">{{ t('warning.warningRule.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ t('warning.warningRule.submit') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { QuestionFilled } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

let emit = defineEmits(["onlineAllocationGroupClose"]);
let ruleFormRef = ref<FormInstance>()
import type { FormInstance, FormRules } from 'element-plus'
let dialogVisible = ref(true)
let ruleForm = ref({
    region: '', value1: '2', value2: '2', value3: '2', value4: '2', value5: '2', num: '0', num1: '0' })

let rules = ref({
    region: [{ required: true, message: t('warning.warningRule.selectNotificationGroup'), trigger: 'change' }],
})


let submitForm = async (formEl: FormInstance | undefined) => {
    if (!formEl) return
    await formEl.validate((valid, fields) => {
        if (valid) {
            console.log('submit!')
        } else {
            console.log('error submit!', fields)
        }
    })
}
</script>
<style lang="scss" scoped>
:deep(.el-select__placeholder) {
    font-size: 0.75rem;
}

:deep(.el-form-item__label) {
    font-size: 0.82rem;
}

:deep(.el-button>span) {
    font-size: 0.8rem;
}

:deep(.el-dialog__title) {
    font-weight: 700;
}

.size {
    font-size: 0.82rem;
}

.cont_text {
    margin-top: 3rem;
    font-weight: 700;
    font-size: 1rem;
}

.flex_end {
    display: flex;
    justify-content: space-between;
    margin: 0.7rem 0;
}
</style>

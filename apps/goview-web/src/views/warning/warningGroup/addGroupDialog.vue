<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="550" @close="emit('handleClose', false)">
            <div style="padding:0.5rem 1rem;">
                <el-form ref="ruleFormRef" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="t('warning.warningGroup.notificationGroupName')" prop="region">
                        <el-input v-model="ruleForm.region" :placeholder="t('warning.warningGroup.enterGroupName')" />
                    </el-form-item>
                    <el-form-item :label="t('warning.warningGroup.members')" prop="desc">
                        <el-input v-model="ruleForm.desc" type="textarea" :placeholder="t('warning.warningGroup.selectMembers')" />
                        <el-button plain style="margin-top: 0.5rem;" @click="adduser">{{ t('warning.warningGroup.addMember') }}</el-button>
                    </el-form-item>
                    <div class="cont_text">
                        {{ t('warning.warningGroup.nonSubAccount') }}
                        <el-tooltip class="box-item" effect="dark" placement="top">
                            <template #content>
                                <span style="width: 22rem; display: inline-block;">
                                    {{ t('warning.warningGroup.nonSubAccountTip') }}
                                </span>
                            </template>
                            <el-icon style="color: #909399; cursor: help;">
                                <QuestionFilled />
                            </el-icon>
                        </el-tooltip>
                    </div>
                    <el-form-item>
                        <template #label>
                            <span style="padding:0 0.2rem 0 0;">{{ t('warning.warningGroup.smsVoice') }}</span>
                            <el-tooltip placement="top" style="width: 5rem;">
                                <template #content>
                                    <span style="width: 20rem; display: inline-block;">
                                        {{ t('warning.warningGroup.smsVoiceTip') }}
                                    </span>
                                </template>
                                <el-icon style="color: #909399; cursor: help;">
                                    <QuestionFilled />
                                </el-icon>
                            </el-tooltip>
                        </template>
                        <el-input v-model="ruleForm.value1" type="textarea" :placeholder="t('warning.warningGroup.enterSmsVoice')" />
                    </el-form-item>
                    <el-form-item>
                        <template #label>
                            <span style="padding:0 0.2rem 0 0;">{{ t('warning.warningGroup.email') }}</span>
                            <el-tooltip placement="top" style="width: 5rem;">
                                <template #content>
                                    <span style="width: 20rem; display: inline-block;">
                                        {{ t('warning.warningGroup.emailTip') }}
                                    </span>
                                </template>
                                <el-icon style="color: #909399; cursor: help;">
                                    <QuestionFilled />
                                </el-icon>
                            </el-tooltip>
                        </template>
                        <el-input v-model="ruleForm.value2" type="textarea" :placeholder="t('warning.warningGroup.enterEmail')" />
                    </el-form-item>
                    <el-form-item :label="t('warning.warningGroup.groupRemark')">
                        <el-input v-model="ruleForm.value3" type="textarea" :placeholder="t('warning.warningGroup.enterGroupRemark')" />
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleClose', false)">{{ t('warning.warningGroup.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ t('warning.warningGroup.submit') }}</el-button>
                </div>
            </template>
        </el-dialog>
        <addUserDialog v-if="isaddUserDialog" :title="title" @handleUserClose="handleUserClose"> </addUserDialog>
    </div>
</template>
<script lang="ts" setup>
import addUserDialog from './addUserDialog.vue'
import { QuestionFilled } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
    title: {
        type: String,
        required: false
    }
});
let emit = defineEmits(["handleClose"]);
let ruleFormRef = ref<FormInstance>()
import type { FormInstance, FormRules } from 'element-plus'
let dialogVisible = ref(true)
let ruleForm = ref({ region: '', desc: '', value1: '', value2: '', value3: '' })

let title = ref("")
let isaddUserDialog = ref(false)

let rules = ref({
    region: [{ required: true, message: t('warning.warningGroup.selectGroup'), trigger: 'change' }],
    desc: [{ required: true, message: t('warning.warningGroup.selectMembers'), trigger: 'change' }],
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

let adduser = () => {
    title.value = t('warning.warningGroup.addMember')
    isaddUserDialog.value = true
}

let handleUserClose = (e: any) => { isaddUserDialog.value = false }
</script>
<style lang="scss" scoped>
:deep(.el-textarea__inner),
:deep(.el-input__inner) {
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

.cont_text {
    margin: 3rem 0 1rem 0;
    font-weight: 700;
    font-size: 1rem;
}

.flex_end {
    display: flex;
    justify-content: space-between;
    margin: 0.7rem 0;
}

:deep(.el-button.is-plain) {
    border: 1px #409eff solid;
    color: #409eff;
}

:deep(.el-form-item--label-right .el-form-item__label) {
    align-items: center;
}
</style>

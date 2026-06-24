<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="550" @close="emit('handleClose', false)">
            <div style="padding:0.5rem 1rem;">
                <el-form ref="ruleFormRef" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="$t('dataCenter.reportName')" prop="region">
                        <el-input v-model="ruleForm.region" :placeholder="$t('dataCenter.pleaseEnterReportName')" />
                    </el-form-item>
                    <el-form-item :label="$t('dataCenter.deviceType')" prop="type">
                        <el-radio :label="$t('dataCenter.device')" value="1" />
                        <el-radio :label="$t('dataCenter.deviceModel')" value="2" />
                    </el-form-item>
                    <el-form-item :label="$t('dataCenter.deviceSource')" prop="desc">
                        <el-select v-model="ruleForm.desc" :placeholder="$t('dataCenter.pleaseSelect')">
                            <el-option :label="$t('dataCenter.zoneOne')" value="shanghai" />
                            <el-option :label="$t('dataCenter.zoneTwo')" value="beijing" />
                        </el-select>
                    </el-form-item>
                    <el-form-item>
                        <template #label>
                            <span style="padding:0 0.2rem 0 0;">{{ $t('dataCenter.property') }}</span>
                            <el-tooltip placement="top" style="width: 5rem;">
                                <template #content>
                                    <span style="width: 9rem; display: inline-block;">
                                        {{ $t('dataCenter.propertyTip') }}
                                    </span>
                                </template>
                                <el-icon style="color: #909399; cursor: help;">
                                    <QuestionFilled />
                                </el-icon>
                            </el-tooltip>
                        </template>
                         <el-select v-model="ruleForm.value2" :placeholder="$t('dataCenter.pleaseSelect')">
                            <el-option :label="$t('dataCenter.zoneOne')" value="shanghai" />
                            <el-option :label="$t('dataCenter.zoneTwo')" value="beijing" />
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="$t('dataCenter.timeCycle')" prop="value1">
                        <el-date-picker v-model="ruleForm.value1" type="datetimerange" :start-placeholder="$t('dataCenter.startTime')"
                            :end-placeholder="$t('dataCenter.endTime')" />
                    </el-form-item>
                </el-form>

            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleClose', false)">{{ $t('common.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ $t('dataCenter.create') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
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
let ruleForm = ref({ region: '', desc: '', value1: '', type: '', value2:'' })

let rules = ref<FormRules>({
    region: [{ required: true, message: t('dataCenter.pleaseEnterReportName'), trigger: 'change' }],
    desc: [{ required: true, message: t('dataCenter.pleaseSelectDeviceSource'), trigger: 'change' }],
    value1: [{ required: true, message: t('dataCenter.pleaseSelectTime'), trigger: 'change' }],
})


// 提交
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
:deep(.el-select__placeholder.is-transparent),
:deep(.el-input__inner),
:deep(.el-date-editor .el-range-input) {
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
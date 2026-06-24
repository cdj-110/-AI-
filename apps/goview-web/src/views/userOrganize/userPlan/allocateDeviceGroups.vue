<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="$t('userOrganize.assignDeviceGroups')" width="500" @close="emit('handleCloseGroups', false)">
            <div style="padding: 0.5rem 1rem;">
                <el-form ref="ruleFormRef" style="max-width: 600px" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="$t('userOrganize.organization')" prop="a">
                        <el-select v-model="ruleForm.a" :placeholder="$t('userOrganize.pleaseSelect') + $t('userOrganize.organization')">
                            <el-option label="Zone one" value="1" />
                            <el-option label="Zone two" value="2" />
                        </el-select>
                    </el-form-item>
                    <el-space fill>
                        <el-alert type="warning" show-icon :closable="false" class="custom-alert">
                            <p style="color: #2B2E1D;font-size: 0.8rem;padding: 0.5rem 0;">{{ $t('userOrganize.assignDeviceGroupsTip') }}</p>
                        </el-alert>
                    </el-space>
                </el-form>
            </div>
            <template #footer>
                <div style="display: flex;justify-content: center;">
                    <el-button @click="emit('handleCloseGroups', false)">{{ $t('common.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ $t('common.confirm') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

let emit = defineEmits(["handleCloseGroups"]);
import type { FormInstance, FormRules } from 'element-plus'
let ruleFormRef = ref<FormInstance>()
let dialogVisible = ref(true)
let ruleForm = ref({ a: '' })
let rules = ref<FormRules>({
    a: [{ required: true, message: t('userOrganize.pleaseSelect') + t('userOrganize.organization'), trigger: 'change' }]
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
    font-size: 12px;
}

:deep(.el-button>span) {
    font-size: 13px;
}
.custom-alert{
  background-color:#FCF1B5;  
  margin-top: 1rem;
}
:deep(.el-alert .el-alert__icon.is-big){
     font-size: 1rem;
     color: #2B2E1D;
}
</style>
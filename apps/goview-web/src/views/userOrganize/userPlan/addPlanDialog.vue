<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="500" @close="emit('handleClose', false)">
            <div style="padding: 0.5rem 1rem;">
                <el-form ref="ruleFormRef" style="max-width: 600px" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="$t('userOrganize.organization')" prop="a">
                        <el-select v-model="ruleForm.a" :placeholder="$t('userOrganize.notJoinParent')">
                            <el-option label="Zone one" value="1" />
                            <el-option label="Zone two" value="2" />
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="$t('userOrganize.organizationName')" prop="b">
                        <el-input v-model="ruleForm.b" :placeholder="$t('userOrganize.pleaseEnter') + $t('userOrganize.organizationName')" />
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div style="display: flex;justify-content: center;">
                    <el-button @click="emit('handleClose', false)">{{ $t('common.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ $t('common.confirm') }}</el-button>
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
import type { FormInstance, FormRules } from 'element-plus'
let ruleFormRef = ref<FormInstance>()
let dialogVisible = ref(true)
let ruleForm = ref({ a: '', b: '' })
let rules = ref<FormRules>({
    b: [{ required: true, message: t('userOrganize.pleaseEnter') + t('userOrganize.organizationName'), trigger: 'change' }]
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
</style>
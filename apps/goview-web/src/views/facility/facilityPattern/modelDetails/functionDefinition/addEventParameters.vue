<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.addEventTitle" width="600"
            @close="emit('addEventParameters', false)">
            <el-form ref="ruleFormRef" style="margin: 0 2rem;margin-top: 1rem;" :model="ruleForm" :rules="rules"
                label-width="auto">
                <el-form-item :label="t('facilityPattern.paramName')" prop="param_name">
                    <el-input v-model="ruleForm.param_name" />
                </el-form-item>
                <el-form-item :label="t('facilityPattern.paramIdentifier')" prop="param_identifier">
                    <el-input v-model="ruleForm.param_identifier" />
                </el-form-item>
                <el-form-item :label="t('facilityPattern.dataType')" prop="data_type">
                    <el-select v-model="ruleForm.data_type" :placeholder="t('facilityPattern.pleaseSelectDataType')">
                        <div>
                            <el-option v-for="(item, index) in dataType" :key="index" :label="item.value"
                                :value="item.value" />
                        </div>
                    </el-select>
                </el-form-item>
                <el-form-item :label="t('facilityPattern.eventDescription')">
                    <el-input v-model="ruleForm.event_description" type="textarea" maxlength="50" show-word-limit />
                </el-form-item>
            </el-form>

            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('addEventParameters', false)">{{ t('facilityPattern.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)"> {{ t('facilityPattern.ok') }} </el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { get_parameter_info } from '@/api/facilityPattern/index'
let emit = defineEmits(["addEventParameters", "objText"]);
import type { FormInstance, FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
import { ref, onMounted, computed, watch } from 'vue';
let ruleFormRef = ref<FormInstance>()
const props = defineProps({
    addEventTitle: {
        type: String,
        required: false
    },
    obj: {
        type: Object,
        required: false
    },
});
let dialogVisible = ref(true)
let ruleForm = ref({ param_name: '', param_identifier: '', data_type: 'bool', event_description: '' })
let dataType = ref([]) // 数据类型
// 标识符校验
let validateIdentifier = (rule: any, value: any, callback: any) => {
    const reg = /^[a-zA-Z_][a-zA-Z0-9_]*$/;
    if (!value) {
        callback(new Error(t('facilityPattern.pleaseEnterIdentifier')));
    } else if (!reg.test(value)) {
        callback(new Error(t('facilityPattern.identifierRule')));
    } else {
        callback();
    }
}
let rules = ref({
    param_name: [{ required: true, message: t('facilityPattern.pleaseEnterPropertyName'), trigger: 'change' }],
    param_identifier: [{ required: true, message: t('facilityPattern.pleaseEnterIdentifier'), trigger: 'change' }, { validator: validateIdentifier }],
    data_type: [{ required: true, message: t('facilityPattern.pleaseSelectDataType'), trigger: 'change' }]
})
// 监听 obj 的变化，当 obj 有值时初始化表格数据
watch(() => props, (val: any) => {
    if (val.addEventTitle == '修改事件参数') {
        ruleForm.value = props.obj
    } else {
        ruleForm.value = { param_name: '', param_identifier: '',data_type: 'bool', event_description: '' }
    }
}, { immediate: true })

onMounted(() => {
    list()
})
let list = () => {
    get_parameter_info().then((res: any) => {
        for (const key in res.data.data_type) {
            dataType.value.push({ label: key, value: res.data.data_type[key] })
        }
    })
}

let submitForm = async (formEl: FormInstance | undefined) => {
    if (!formEl) return
    await formEl.validate((valid, fields) => {
        if (valid) {
            emit('objText', ruleForm.value)
            dialogVisible.value = false
        }
    })
}
</script>
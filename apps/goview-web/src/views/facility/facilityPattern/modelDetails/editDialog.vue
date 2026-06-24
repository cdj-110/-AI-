<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="t('facilityPattern.modifyModelInfo')" width="550" @close="emit('handleClose', false)">
            <div style="margin: 0 2rem;">
                <el-form ref="ruleFormRef" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="t('facilityPattern.modelName')" prop="model_name">
                        <el-input v-model="ruleForm.model_name" />
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.deviceType')" prop="device_type">
                        <el-radio-group v-model="ruleForm.device_type">
                            <el-radio v-for="(item, index) in deviceTypeList" :key="index" :value="item.value"
                                disabled>{{ item.label }}</el-radio>
                        </el-radio-group>
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.communication')" prop="communication">
                        <el-select v-model="ruleForm.communication" :placeholder="t('facilityPattern.pleaseSelectCommunication')"
                            :disabled="props.model_type != 1">
                            <div>
                                <el-option v-for="(item, index) in communicationList" :key="index" :label="item.label"
                                    :value="item.value" />
                            </div>
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.accessProtocol')" prop="protocol">
                        <el-select v-model="ruleForm.protocol" :placeholder="t('facilityPattern.pleaseSelectProtocol')"
                            :disabled="props.model_type != 1">
                            <el-option v-for="(item, index) in protocolList" :key="index" :label="item.label"
                                :value="item.value" />
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.modelDescription')" prop="model_description">
                        <el-input v-model="ruleForm.model_description" type="textarea" :placeholder="t('facilityPattern.pleaseEnterDescription')"
                            maxlength="50" show-word-limit />
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleClose', false)">{{ t('facilityPattern.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">
                        {{ t('facilityPattern.confirm') }}
                    </el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { modelDatas } from '@/api/facilityList/index'
import { update_device_model } from '@/api/facilityPattern/index'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({
    detailObj: {
        type: Object,
        required: false
    },
    model_id: {
        type: String,
        required: false
    },
    model_type: {
        type: Number,
        required: false
    }
});
let emit = defineEmits(["handleClose"]);
import type { FormInstance, FormRules } from 'element-plus'
import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let ruleFormRef = ref<FormInstance>()
let dialogVisible = ref(true)
let ruleForm = ref({ model_name: '', device_type: '', communication: '', protocol: '', model_description: '' })
let deviceTypeList: any = ref([]);//设备类型
let communicationList: any = ref([]);//通信方式
let protocolList: any = ref([]);//接入协议
let rules = ref({
    model_name: [{ required: true, message: t('facilityPattern.pleaseEnterModelName'), trigger: 'change' }],
    communication: [{ required: true, message: t('facilityPattern.pleaseSelectCommunication'), trigger: 'change' }],
    protocol: [{ required: true, message: t('facilityPattern.pleaseSelectProtocol'), trigger: 'change' }],
    name4: [{ required: true, message: '', trigger: 'change' }]
})
onMounted(() => {
    console.log(props.detailObj, ' ruleForm.value')
    ruleForm.value = { ...props.detailObj }
    list()
})
let list = () => {
    modelDatas().then((res: any) => {
        for (const key in res.data.communication) {
            communicationList.value.push({ label: key, value: res.data.communication[key] })
        }
        for (const key in res.data.protocol) {
            protocolList.value.push({ label: key, value: res.data.protocol[key] })
        }
        for (const key in res.data.device_type) {
            deviceTypeList.value.push({ label: key, value: res.data.device_type[key] })
        }
    })
}
let submitForm = async (formEl: FormInstance | undefined) => {
    if (!formEl) return
    await formEl.validate((valid, fields) => {
        if (valid) {
            let params = {
                user_id: user_id.value,
                model_id: props.model_id,
                model_name: ruleForm.value.model_name,
                communication: ruleForm.value.communication,
                protocol: ruleForm.value.protocol,
                model_description: ruleForm.value.model_description
            }
            update_device_model(params).then((res: any) => {
                if (res.code == 200) {
                    ElMessage.success(res.msg);
                    emit('handleClose', false)
                } else {
                    ElMessage.error(res.msg);
                }
            })
        }
    })
}
</script>
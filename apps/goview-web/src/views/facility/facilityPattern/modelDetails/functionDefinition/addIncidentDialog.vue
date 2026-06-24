<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="600" @close="emit('handleIncidentClose', false)">
            <div style="margin: 0 2rem;">
                <el-form ref="ruleFormRef" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="t('facilityPattern.eventName')" prop="event_name">
                        <el-input v-model="ruleForm.event_name" />
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.identifier')" prop="identifier">
                        <el-input v-model="ruleForm.identifier" />
                    </el-form-item>

                    <!-- 事件参数 -->
                    <div>
                        <p style="margin: 1rem 0;font-size: 1rem;font-weight: 700;">{{ t('facilityPattern.eventParameters') }}</p>
                        <div>
                            <el-table :data="tableData" style="width: 100%">
                                <el-table-column prop="param_name" :label="t('facilityPattern.paramName')" align="center" />
                                <el-table-column prop="param_identifier" :label="t('facilityPattern.paramIdentifier')" align="center" />
                                <el-table-column prop="data_type" :label="t('facilityPattern.dataType')" align="center" />
                                <el-table-column :label="t('facilityPattern.operation')" align="center">
                                    <template #default="scope">
                                        <el-icon color="#5271AD">
                                            <Edit @click="editClick(scope.row, scope.$index)" />
                                        </el-icon>
                                        <el-icon color="red" style="margin-left: 1rem;" @click="deleteRow(scope.row, scope.$index)">
                                            <Delete />
                                        </el-icon>
                                    </template>
                                </el-table-column>
                            </el-table>
                            <div style="display: flex;justify-content: flex-end; margin: 1rem 0;">
                                <el-button plain @click="add" :disabled="tableData.length >= 10">{{ t('facilityPattern.addEventParam') }}</el-button>
                            </div>
                        </div>
                    </div>
                    <el-form-item :label="t('facilityPattern.eventDescription')">
                        <el-input v-model="ruleForm.event_description" type="textarea" maxlength="50" show-word-limit />
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleIncidentClose', false)">{{ t('facilityPattern.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">
                        {{ t('facilityPattern.ok') }}
                    </el-button>
                </div>
            </template>
            <!-- 添加事件参数 -->
            <addEvent v-if="isaddEvent" :addEventTitle="addEventTitle" @addEventParameters="addEventParameters"
                :obj="obj" @objText="objText" :editingIndex="editingIndex">
            </addEvent>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
let emit = defineEmits(["handleIncidentClose"]);
import addEvent from './addEventParameters.vue'
import { add_event ,edit_model_event} from '@/api/facilityPattern/index'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
const props = defineProps({
    title: {
        type: String,
        required: false
    },
    action: {
        type: String,
        required: false
    },
    model_id: {
        type: String,
        required: false
    },
    model_name: {
        type: String,
        required: false
    },
    IncidenObj: {
        type: Object,
        required: false,
        default: () => ({})
    }
});
import { valueEquals, type FormInstance, type FormRules } from 'element-plus'
let ruleFormRef = ref<FormInstance>()
let dialogVisible = ref(true)
let ruleForm = ref({ event_name: '', identifier: '', event_description: '' })
let tableData = ref([])
let isaddEvent = ref(false) // 添加事件弹窗
let addEventTitle = ref('') // 添加事件弹窗title
let obj = ref({}) // 添加事件参数
const editingIndex = ref(-1) // 当前编辑的索引，-1表示新增
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
    event_name: [{ required: true, message: t('facilityPattern.pleaseEnterEventName'), trigger: 'change' }],
    identifier: [{ required: true, message: t('facilityPattern.pleaseEnterIdentifier'), trigger: 'change' }, { validator: validateIdentifier }]
})

watch(props, (val: any) => {
    if (val.action == 'edit') {
        ruleForm.value = val.IncidenObj
        tableData.value = val.IncidenObj.data_type
    }
}, { immediate: true, deep: true })




let submitForm = async (formEl: FormInstance | undefined) => {
    if (!formEl) return
    await formEl.validate((valid, fields) => {
        if (valid) {
            let params = {
                user_id: user_id.value,
                model_id: props.model_id,
                event_name: ruleForm.value.event_name,
                model_name: props.model_name,
                identifier: ruleForm.value.identifier,
                event_description: ruleForm.value.event_description,
                parameters: tableData.value
            }
            // event_id
            if (props.action == 'add') {
                add_event(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg);
                        emit('handleIncidentClose', false)
                    } else {
                        ElMessage.success(res.msg);
                    }
                })
            } else {
                params.event_id = ruleForm.value.id
                edit_model_event(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg);
                        emit('handleIncidentClose', false)
                    } else {
                        ElMessage.success(res.msg);
                    }
                })
            }
        }
    })
}
// 添加事件参数
let add = () => {
    editingIndex.value = -1
    isaddEvent.value = true
    addEventTitle.value = t('facilityPattern.addEventParam')
}
// 修改事件弹窗
let editClick = (e: any, index: any) => {
    isaddEvent.value = true
    addEventTitle.value = t('facilityPattern.editEventParam')
    obj.value = e
    editingIndex.value = index
}
// 取消事件弹窗
let addEventParameters = (e: any) => { isaddEvent.value = false, obj.value = {} }
// 添加事件 修改事件参数
let objText = (e: any) => {
    if (editingIndex.value === -1) {
        tableData.value.push({ ...e })
    } else {
        tableData.value[editingIndex.value] = { ...e }
    }
}
// 删除参数事件
const deleteRow = (e: any, index: any) => {
  tableData.value.splice(index, 1)
}
</script>
<style lang="scss" scoped>
:deep(.el-button>span),
:deep(.el-input__inner) {
    font-size: 13px;
}
</style>
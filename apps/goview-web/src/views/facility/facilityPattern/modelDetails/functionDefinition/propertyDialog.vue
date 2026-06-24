<template>
    <div>
        <el-dialog v-model="dialogTableVisible" :title="props.title" width="600" @close="emit('handleClose', false)">
            <div style="margin: 0 2rem;">
                <el-form ref="ruleFormRef" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="t('facilityPattern.propertyName')" prop="property_name">
                        <el-input v-model="ruleForm.property_name" maxlength="50" show-word-limit />
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.identifier')" prop="identifier">
                        <el-input v-model="ruleForm.identifier" maxlength="50" show-word-limit />
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.readWriteType')" prop="read_write_type">
                        <el-radio-group v-model="ruleForm.read_write_type">
                            <el-radio v-for="(item, index) in readWriteType" :key="index" :value="item.value">{{
                                item.label }}</el-radio>
                        </el-radio-group>
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.dataType')" prop="data_type">
                        <el-select v-model="ruleForm.data_type" :placeholder="t('facilityPattern.pleaseSelectDataType')">
                            <div>
                                <el-option v-for="(item, index) in dataType" :key="index" :label="item.value"
                                    :value="item.value" />
                            </div>
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="t('facilityPattern.boolean')" v-if="ruleForm.data_type == 'bool'">
                        <el-col :span="11">
                            <el-form-item>
                                1- &nbsp;
                                <el-input v-model="ruleForm.a" style="width: 100px;" :placeholder="t('facilityPattern.on')" />
                            </el-form-item>
                        </el-col>
                        <el-col :span="11">
                            <el-form-item>
                                0- &nbsp;<el-input v-model="ruleForm.b" style="width: 100px;" :placeholder="t('facilityPattern.off')" />
                            </el-form-item>
                        </el-col>
                    </el-form-item>
                    <div v-if="ruleForm.data_type == 'number'">
                        <el-form-item :label="t('facilityPattern.valueRange')">
                            <div style="display: flex;justify-content: space-between;">
                                <el-form-item prop="min">
                                    <el-input v-model="ruleForm.min" type="number" :placeholder="t('facilityPattern.pleaseEnterMin')"
                                        @change="handleMinMaxChange" />
                                </el-form-item>
                                <div style="margin: 0 1rem;">~</div>
                                <el-form-item prop="max">
                                    <el-input v-model="ruleForm.max" type="number" :placeholder="t('facilityPattern.pleaseEnterMax')"
                                        @change="handleMinMaxChange" />
                                </el-form-item>
                            </div>
                        </el-form-item>
                        <el-form-item :label="t('facilityPattern.precision')">
                            <el-select v-model="ruleForm.precision" :placeholder="t('facilityPattern.pleaseSelectDataType')">
                                <el-option label="1" value="1" />
                                <el-option label="0.1" value="0.1" />
                                <el-option label="0.01" value="0.01" />
                                <el-option label="0.001" value="0.001" />
                                <el-option label="0.0001" value="0.0001" />
                            </el-select>
                        </el-form-item>
                        <el-form-item :label="t('facilityPattern.step')">
                            <div style="display: flex;">
                                <div> <el-input v-model="ruleForm.step" :placeholder="t('facilityPattern.step')" type="number" /></div>
                                <div style="font-size: 12px;padding: 0 0 0 0.5rem;"> {{ t('facilityPattern.stepDesc') }}</div>
                            </div>
                        </el-form-item>
                        <el-form-item :label="t('facilityPattern.unit')" prop="unit">
                            <el-input v-model="ruleForm.unit" :placeholder="t('facilityPattern.pleaseEnterUnit')" />
                        </el-form-item>
                        <el-form-item prop="mapParams">
                            <template #label>
                                <div>
                                    <span>{{ t('facilityPattern.uplinkMapping') }}</span>
                                    <el-tooltip placement="top" style="width: 5rem;">
                                        <template #content>
                                            <span style="width: 20rem; display: inline-block;">
                                                {{ t('facilityPattern.uplinkMappingDesc') }}
                                            </span>
                                        </template>
                                        <el-icon style="color: #909399; cursor: help;">
                                            <QuestionFilled />
                                        </el-icon>
                                    </el-tooltip>
                                </div>
                            </template>
                            <div style="display: flex; align-items: center; gap: 0.5rem;">
                                <el-form-item prop="x1" style="margin-bottom: 0;">
                                    <el-input v-model="ruleForm.x1" placeholder="x1" />
                                </el-form-item>
                                <el-form-item prop="x2" style="margin-bottom: 0;">
                                    <el-input v-model="ruleForm.x2" placeholder="x2" />
                                </el-form-item>
                                <span>=>></span>
                                <el-form-item prop="y1" style="margin-bottom: 0;">
                                    <el-input v-model="ruleForm.y1" placeholder="y1" />
                                </el-form-item>
                                <el-form-item prop="y2" style="margin-bottom: 0;">
                                    <el-input v-model="ruleForm.y2" placeholder="y2" />
                                </el-form-item>
                            </div>
                        </el-form-item>
                    </div>
                    <el-form-item :label="t('facilityPattern.collectionFrequency')" prop="collection_frequency">
                        <el-select v-model="ruleForm.collection_frequency" :placeholder="t('facilityPattern.pleaseSelectFrequency')">
                            <div>
                                <el-option v-for="(item, index) in samplingFrequency" :key="index" :label="item.label"
                                    :value="item.value" />
                            </div>
                        </el-select>
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleClose', false)">{{ t('facilityPattern.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)"> {{ t('facilityPattern.ok') }} </el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { reactive, ref } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { get_parameter_info, add_property, edit_property } from '@/api/facilityPattern/index'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
let emit = defineEmits(["handleClose"]);
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
    propertyObj: {
        type: Object,
        required: false,
        default: () => ({})
    }
});
let ruleFormRef = ref<FormInstance>()
import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let dialogTableVisible = ref(true)
let ruleForm = ref({
    property_name: '', identifier: '', read_write_type: 'r', data_type: 'bool', max: '', min: '', collection_frequency: '15s', precision: '1', a: '', b: '',
    step: '', unit: '', x1: null, x2: null, y1: null, y2: null, id: ''
})
let readWriteType = ref([]) // 读写类型
let dataType = ref([]) // 数据类型
let samplingFrequency = ref([]) // 采集频率
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
// 单位校验规则
const checkUnit = (rule: any, value: any, callback: any) => {
    if (/\d/.test(value)) {
        callback(new Error(t('facilityPattern.unitCannotContainNumber')))
    }
    else {
        callback()
    }
}
// 上行映射输入框仅允许数字（整数/小数/负数）
const validateNumber = (rule: any, value: any, callback: any) => {
    const numReg = /^-?\d+(\.\d+)?$/;
    if (!value) {
        callback();
    } else if (!numReg.test(value)) {
        callback(new Error(t('facilityPattern.onlyNumber')));
    } else {
        callback();
    }
};
// 上行映射联合校验：四个输入框“全填或全空”
const validateMapComplete = (rule: any, value: any, callback: any) => {
    let { x1, x2, y1, y2 } = ruleForm.value;
    let hasValues = [x1, x2, y1, y2].map(item => (item || '').trim() !== '');
    let isAllEmpty = hasValues.every(v => !v);
    let isAllFilled = hasValues.every(v => v);

    if (!isAllEmpty && !isAllFilled) {
        callback(new Error(t('facilityPattern.mappingIncomplete')));
    } else {
        callback();
    }
};
// 联动触发校验：改min/max时，同时更新两个字段的校验状态
const handleMinMaxChange = () => {
    ruleFormRef.value?.validateField(['min', 'max'])
}
// 校验最小值
const checkMin = (rule: any, value: any, callback: any) => {
    const minVal = Number(value)
    const maxVal = Number(ruleForm.value.max)

    if (isNaN(minVal)) return callback()
    if (isNaN(maxVal)) return callback()
    if (minVal > maxVal) return callback(new Error(t('facilityPattern.minCannotBeGreaterThanMax')))

    callback()
}

// 校验最大值
const checkMax = (rule: any, value: any, callback: any) => {
    const maxVal = Number(value)
    const minVal = Number(ruleForm.value.min)

    if (isNaN(maxVal)) return callback()
    if (isNaN(minVal)) return callback()
    if (maxVal < minVal) return callback(new Error(t('facilityPattern.maxCannotBeLessThanMin')))

    callback()
}
let rules = ref({
    property_name: [{ required: true, message: t('facilityPattern.pleaseEnterPropertyName'), trigger: 'change' }],
    identifier: [{ required: true, message: t('facilityPattern.pleaseEnterIdentifier'), trigger: 'change' }, { validator: validateIdentifier }],
    read_write_type: [{ required: true, message: t('facilityPattern.pleaseSelectReadWriteType'), trigger: 'change' }],
    data_type: [{ required: true, message: t('facilityPattern.pleaseSelectDataType'), trigger: 'change' }],
    collection_frequency: [{ required: true, message: t('facilityPattern.pleaseSelectFrequency'), trigger: 'change' }],
    x1: [{ validator: validateNumber, trigger: ['change', 'blur'] }],
    x2: [{ validator: validateNumber, trigger: ['change', 'blur'] }],
    y1: [{ validator: validateNumber, trigger: ['change', 'blur'] }],
    y2: [{ validator: validateNumber, trigger: ['change', 'blur'] }],
    mapParams: [{ validator: validateMapComplete, trigger: ['change', 'blur'] }],
    unit: [{ validator: checkUnit, trigger: ['change', 'blur'] }],
    max: [{ validator: checkMax, trigger: ['change', 'blur'] }],
    min: [{ validator: checkMin, trigger: ['change', 'blur'] }]

})
watch(props, (val: any) => {
    if (val.action !== 'edit') return;

    // 修复后的正则：支持负数、小数、任意空格
    const formulaRegex = /\(x\s*-\s*(-?\d+\.?\d*)\)\s*\*\s*\(\(\s*(-?\d+\.?\d*)\s*-\s*(-?\d+\.?\d*)\s*\)\s*\/\s*\(\s*(-?\d+\.?\d*)\s*-\s*\1\s*\)\)\s*\+\s*(-?\d+\.?\d*)/;

    const extendedInfo = val.propertyObj?.extended_info || {};
    const formula = extendedInfo.formula || '';

    const match = formula.match(formulaRegex) || [];
    // 赋值到表单（现在能正确捕获负数和小数）
    if (match.length >= 6) {
        ruleForm.value.x1 = match[1];
        ruleForm.value.y2 = match[2];
        ruleForm.value.y1 = match[3];
        ruleForm.value.x2 = match[4];
    }
    // 其他赋值逻辑
    ruleForm.value = {
        ...ruleForm.value,
        ...val.propertyObj,
        ...(extendedInfo ? {
            max: extendedInfo.max || '',
            min: extendedInfo.min || '',
            a: extendedInfo[1] || "",
            b: extendedInfo[0] || "",
            step: extendedInfo.step || '',
            unit: extendedInfo.unit || '',
            mapParams: formula,
            precision: extendedInfo.precision || '1'
        } : {})
    };
}, { immediate: true, deep: true })
onMounted(() => {
    list()
})
let list = () => {
    get_parameter_info().then((res: any) => {
        const labelMap = { '读': '只读', '写': '只写', '读写': '读写' };
        for (const key in res.data.readwritetype) {
            // readWriteType.value.push({ label: key, value: res.data.readwritetype[key] })
            const mappedLabel = labelMap[key] || key; // 如果没有映射，则用原值
            readWriteType.value.push({ label: mappedLabel, value: res.data.readwritetype[key] })
        }
        for (const key in res.data.data_type) {
            dataType.value.push({ label: key, value: res.data.data_type[key] })
        }
        for (const key in res.data.collectionfrequency) {
            samplingFrequency.value.push({ label: key, value: res.data.collectionfrequency[key] })
        }
    })
}
let submitForm = async (formEl: FormInstance | undefined) => {
    if (!formEl) return
    await formEl.validate((valid, fields) => {
        if (valid) {
            //  解构表单值，减少重复访问
            const { x1, x2, y1, y2, a, b, max, min, step, unit, precision, property_name, identifier, read_write_type, data_type, collection_frequency } = ruleForm.value;
            // 拼接公式字符串
            const hasMapParams = x1 !== null && x2 !== null && y1 !== null && y2 !== null;
            const formulaStr = hasMapParams ? `(x - ${x1}) * ((${y2} - ${y1}) / (${x2} - ${x1})) + ${y1}` : "";
            // 构造参数（对象展开+条件合并，替代后续赋值）
            const params = {
                user_id: user_id.value,
                model_id: props.model_id,
                model_name: props.model_name,
                property_name,
                identifier,
                read_write_type,
                data_type,
                collection_frequency: Number(collection_frequency) || 15,
                extended_info: {
                    ...(data_type === 'bool' ? { 1: a || '开', 0: b || '关' } : {}),
                    ...(data_type === 'number' ? { max: max || '', min: min || '' } : {}),
                    ...(data_type === 'number' && precision !== null ? { precision } : {}),
                    ...(data_type === 'number' && step !== null ? { step } : {}),
                    ...(data_type === 'number' && unit !== null ? { unit } : {}),
                    ...(data_type === 'number' && formulaStr !== null ? { formula: formulaStr } : {}),
                }
            };
            if (props.action == 'add') {
                add_property(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg);
                        emit('handleClose', false)
                    } else {
                        ElMessage.success(res.msg);
                    }
                })
            } else {
                params.property_id = ruleForm.value.id
                edit_property(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg);
                        emit('handleClose', false)
                    } else {
                        ElMessage.success(res.msg);
                    }
                })
            }
        }
    })
}
</script>
<style lang="scss" scoped>
:deep(.el-button>span) {
    font-size: 13px;
}

:deep(.el-select__placeholder.is-transparent),
:deep(.el-input__inner) {
    font-size: 0.8rem;
}
</style>
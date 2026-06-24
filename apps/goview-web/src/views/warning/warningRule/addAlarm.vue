<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="40%" @close="emit('handleClose', false)">
            <div style="padding:0.5rem 3rem;">
                <el-form ref="ruleFormRef" style="max-width: 700px" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="t('warning.warningRule.ruleName')" prop="a">
                        <el-input v-model="ruleForm.a" :placeholder="t('warning.warningRule.sampleRule')" />
                    </el-form-item>
                    <el-form-item :label="t('warning.warningRule.deviceType')">
                        <el-radio-group v-model="ruleForm.b">
                            <el-radio value="1">{{ t('warning.warningRule.device') }}</el-radio>
                            <el-radio value="2">{{ t('warning.warningRule.deviceType') }}</el-radio>
                        </el-radio-group>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningRule.orgName')" prop="b">
                        <el-input v-model="ruleForm.b" />
                    </el-form-item>
                    <el-form-item :label="t('warning.warningRule.deviceSource')" prop="c">
                        <el-select v-model="ruleForm.c" multiple filterable allow-create default-first-option
                            :reserve-keyword="false" :placeholder="t('warning.warningRule.pleaseSelect')">
                            <el-option label="Zone one" value="shanghai" />
                            <el-option label="Zone two" value="beijing" />
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningRule.alertCondition')" prop="d">
                        <div class="conditions-container">
                            <div v-for="(condition, index) in ruleForm.d" :key="index" class="condition-row"
                                v-if="ruleForm.d.length >= 1">
                                <span>
                                    <el-select v-model="condition.logic" :placeholder="t('warning.warningRule.and')" clearable
                                        style="width: 8rem;margin: 0 1rem 0 0;">
                                        <el-option label="AND" value="and" />
                                        <el-option label="OR" value="or" />
                                    </el-select>
                                    <el-select v-model="condition.property" :placeholder="t('warning.warningRule.selectProperty')" clearable>
                                        <el-option :label="t('warning.warningRule.cpuUsage')" value="cpu" />
                                        <el-option :label="t('warning.warningRule.memoryUsage')" value="memory" />
                                        <el-option label="磁盘空间" value="111111" />
                                        <el-option :label="t('warning.warningRule.networkTraffic')" value="network" />
                                        <el-option :label="t('warning.warningRule.temperature')" value="temperature" />
                                        <el-option :label="t('warning.warningRule.humidity')" value="humidity" />
                                    </el-select>
                                </span>
                                <span v-if="condition.logic">
                                    <el-select v-model="condition.operator" style="margin: 0 1rem 0 0;" clearable>
                                        <el-option :label="t('warning.warningRule.equals')" value="equals" />
                                        <el-option :label="t('warning.warningRule.greater')" value="greater" />
                                        <el-option :label="t('warning.warningRule.less')" value="less" />
                                        <el-option :label="t('warning.warningRule.greaterEqual')" value="greater_equal" />
                                        <el-option :label="t('warning.warningRule.lessEqual')" value="less_equal" />
                                    </el-select>
                                    <el-select v-model="condition.value" style="width: 8rem;" clearable>
                                        <el-option label="ON" value="on" />
                                        <el-option label="OFF" value="off" />
                                    </el-select>
                                </span>
                                <el-button v-if="ruleForm.d.length >= 1" type="danger" :icon="Delete" circle
                                    @click="removeCondition(index)" />
                            </div>
                            <div>
                                <el-button plain @click="addCondition">
                                    {{ t('warning.warningRule.addCondition') }}
                                </el-button>
                            </div>
                        </div>
                    </el-form-item>
                    <div style="display: flex;justify-content: space-between;">
                        <el-form-item>
                            <template #label>
                                <span>{{ t('warning.warningRule.repeatTimes') }}</span>
                                <el-tooltip placement="top" style="width: 5rem;">
                                    <template #content>
                                        <span style="width: 20rem; display: inline-block;">
                                            {{ t('warning.warningRule.repeatTimesTip') }}
                                        </span>
                                    </template>
                                    <el-icon style="color: #909399; cursor: help;">
                                        <QuestionFilled />
                                    </el-icon>
                                </el-tooltip>
                            </template>
                            <el-select v-model="ruleForm.e" :placeholder="t('warning.warningRule.none')" style="width: 10rem" clearable>
                                <el-option :label="t('warning.warningRule.once')" value="1" />
                                <el-option :label="t('warning.warningRule.twice')" value="2" />
                            </el-select>
                        </el-form-item>
                        <el-form-item>
                            <template #label>
                                <span>{{ t('warning.warningRule.duration') }}</span>
                                <el-tooltip placement="top" style="width: 5rem;">
                                    <template #content>
                                        <span style="width: 20rem; display: inline-block;">
                                            {{ t('warning.warningRule.durationTip') }}
                                        </span>
                                    </template>
                                    <el-icon style="color: #909399; cursor: help;">
                                        <QuestionFilled />
                                    </el-icon>
                                </el-tooltip>
                            </template>
                            <el-select v-model="ruleForm.f" :placeholder="t('warning.warningRule.none')" style="width: 10rem" clearable>
                                <el-option :label="t('warning.warningRule.oneDay')" value="1" />
                                <el-option :label="t('warning.warningRule.twoDays')" value="2" />
                            </el-select>
                        </el-form-item>
                    </div>
                    <el-form-item :label="t('warning.warningRule.alertLevel')">
                        <el-radio-group v-model="ruleForm.radio">
                            <el-radio :value="1">{{ t('warning.warningRule.normalAlert') }}</el-radio>
                            <el-radio :value="2">{{ t('warning.warningRule.importantAlert') }}</el-radio>
                            <el-radio :value="3">{{ t('warning.warningRule.criticalAlert') }}</el-radio>
                        </el-radio-group>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningRule.alertRemark')">
                        <el-input v-model="ruleForm.desc" type="textarea" :placeholder="t('warning.warningRule.enterAlertConfig')" />
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div style="padding:1rem 0 2rem 0;">
                    <el-button @click="emit('handleClose', false)">{{ t('warning.warningRule.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ t('warning.warningRule.submit') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { Delete, Plus, QuestionFilled } from '@element-plus/icons-vue'
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
let ruleForm = ref({
    a: '', b: '', c: '', d: [], e: '', f: '', desc: '',radio:1
})
let rules = ref({
    a: [{ required: true, message: t('warning.warningRule.enterAlertRule'), trigger: 'change' }],
    c: [{ required: true, message: t('warning.warningRule.enterDeviceSource'), trigger: 'change' }],
    d: [{ required: true, message: t('warning.warningRule.enterAlertCondition'), trigger: 'change' }]
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

let addCondition = () => {
    ruleForm.value.d.push({
        logic: '',
        property: '',
        operator: '',
        value: ''
    })
}

const removeCondition = (index: any) => {
    if (ruleForm.value.d.length > 0) {
        ruleForm.value.d.splice(index, 1)
    }
}
</script>
<style lang="scss" scoped>
:deep(.el-select__placeholder),
:deep(.el-textarea__inner),
:deep(.el-input__inner) {
    font-size: 0.75rem;
}

:deep(.el-button>span) {
    font-size: 0.82rem;
}

.conditions-container {
    margin-bottom: 0.95rem;
}

.condition-row {
    display: flex;
    align-items: center;
    margin-bottom: 10px;
    border-radius: 4px;

    span {
        display: flex;
        justify-content: space-between;
        margin: 0 1rem 0 0;
    }
}

:deep(.el-form-item--label-right .el-form-item__label) {
    align-items: center;
}
</style>

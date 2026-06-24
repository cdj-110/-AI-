<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="t('warning.warningChronicle.detailTitle')" width="500" @close="emit('handleClose', false)">
            <div style="padding: 0.5rem 3rem;">
                <el-form ref="ruleFormRef" style="max-width: 600px" :model="ruleForm" label-width="auto">
                    <el-form-item :label="t('warning.warningChronicle.ruleName')">
                        <span class="text">{{ t('warning.warningChronicle.sampleRule') }}</span>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningChronicle.device')">
                        <span class="text">{{ props.objValue.device }}</span>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningChronicle.alertStatus')">
                        <!-- 图标部分 -->
                        <el-icon v-if="props.objValue.level === '1'" class="status-danger">
                            <Warning color="#f56c6c"/>
                        </el-icon>
                        <el-icon v-else-if="props.objValue.level === '2'" class="status-warning">
                            <InfoFilled color="#e6a23c"/>
                        </el-icon>
                        <el-icon v-else-if="props.objValue.level === '3'" class="status-warning">
                            <Bell color="#909399"/>
                        </el-icon>
                        <el-icon v-else-if="props.objValue.level === '4'" class="status-warning">
                            <CircleCheck color="#67c23a"/>
                        </el-icon>
                        <span class="text" :class="'status-' + props.objValue.level" style="padding-left:0.3rem ;">
                            {{ statusMap[props.objValue.level] }}
                        </span>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningChronicle.alertInfo')">
                        <span class="text"> <span>{{ props.objValue.co2 }}{{ t('warning.warningChronicle.ppm') }}</span></span>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningChronicle.alertRemark')">
                        <span class="text">{{ props.objValue.description }}</span>
                    </el-form-item>
                    <el-form-item :label="t('warning.warningChronicle.alertReason')">
                        <span></span>
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div>
                    <el-button @click="emit('handleClose', false)">{{ t('warning.warningChronicle.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm()">{{ t('warning.warningChronicle.confirm') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { Warning, InfoFilled, Bell, CircleCheck } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
    objValue: {
        type: Object,
        required: false
    }
});
let emit = defineEmits(["handleClose"]);
let dialogVisible = ref(true)
let ruleForm = ref({ a: '', b: '' })
let statusMap = { 
    1: t('warning.warningChronicle.alert.critical') + t('warning.warningChronicle.alertSuffix'), 
    2: t('warning.warningChronicle.alert.important') + t('warning.warningChronicle.alertSuffix'), 
    3: t('warning.warningChronicle.alert.normal') + t('warning.warningChronicle.alertSuffix'), 
    4: t('warning.warningChronicle.alert.resolved') + t('warning.warningChronicle.alertSuffix') 
}
let submitForm = async () => {

}
</script>
<style lang="scss" scoped>
:deep(.el-select__placeholder) {
    font-size: 12px;
}

:deep(.el-button>span) {
    font-size: 13px;
}

:deep(.el-form-item__label) {
    color: #3A2E30;
}

.text {
    color: #837F7E;
}

.status-1 {
    color: #f56c6c;
}

/* danger */
.status-2 {
    color: #e6a23c;
}

/* warning */
.status-3 {
    color: #909399;
}

/* info */
.status-4 {
    color: #67c23a;
}

/* success */
</style>

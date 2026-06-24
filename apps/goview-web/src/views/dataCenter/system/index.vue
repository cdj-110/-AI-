<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="$t('dataCenter.systemNotification')" width="600" @close="emit('alarmHistoryClose', false)">
            <div style="display: flex;justify-content: center;align-items: center;padding: 4rem 0;">
                <span>{{ $t('dataCenter.reportGenerated') }}</span> 
                <span style="text-align: right;color: #3C93E7;cursor: pointer; margin-left: 1rem;" @click="download">{{ $t('dataCenter.goDownload') }}</span>
            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button type="primary" @click="emit('alarmHistoryClose', false)">{{ $t('dataCenter.gotIt') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

let emit = defineEmits(["alarmHistoryClose"]);
import { useRouter } from 'vue-router'
const router = useRouter()
let dialogVisible = ref(true)

//  下载跳转
let download = () => {
    router.push({
        path: '/dataReport', // 相对路径，自动拼接到当前路径后
    })
    emit('alarmHistoryClose', false)
}
</script>
<style lang="scss" scoped>
:deep(.el-upload-dragger) {
    border: none;
}
</style>
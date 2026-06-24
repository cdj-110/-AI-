<template>
    <el-dialog v-model="dialogVisible" :title="t('taskPage.executionHistory')" width="50%" align-center @close="emit('update:visible', false)">
        <div style="display: flex;align-items: center;justify-content: space-between;margin-bottom: 1.3rem;">
            <div>{{ t('taskPage.ruleName') }}：{{ ruleName }}</div>
            <div style="text-align: right;">
                <el-date-picker v-model="dateRange" type="datetimerange" 
                    :start-placeholder="t('taskPage.startTime')" 
                    :end-placeholder="t('taskPage.endTime')" 
                    format="YYYY-MM-DD HH:mm:ss" 
                    style="width: 50%;margin-right: 1rem;" />
                <el-button type="primary" icon="RefreshRight" @click="handleRefresh">{{ t('taskPage.refresh') }}</el-button>
            </div>
        </div>
        <el-table :data="historyList">
            <el-table-column prop="a" :label="t('taskPage.timeField')" align="center" />
            <el-table-column prop="a" :label="t('taskPage.result')" align="center" />
            <el-table-column prop="a" :label="t('taskPage.details')" align="center" />
        </el-table>
        <template #footer>
            <el-button type="primary" @click="handleClose">{{ t('common.confirm') }}</el-button>
        </template>
    </el-dialog>
</template>

<script lang="ts" setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
    ruleName?: string
}>()
let dialogVisible = ref(true)
let emit = defineEmits(["updateClick"]);

const historyList = ref([{ a: 111 }])
const dateRange = ref([])

const handleClose = () => {
    emit('updateClick', false)
}

const handleRefresh = () => {
}
</script>

<template>
    <div style="background: #fff;height: 100%;">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="openAddDialog">{{ t('taskPage.addTask') }}</el-button>
            </template>
            <template #content>
                <el-table :data="rulesList">
                    <el-table-column prop="a" :label="t('taskPage.taskName')" align="center" />
                    <el-table-column prop="a" :label="t('taskPage.status')" align="center">
                        <template #default="scope">
                            <el-switch v-model="scope.row.a" />
                        </template>
                    </el-table-column>
                    <el-table-column prop="a" :label="t('taskPage.executionMethod')" align="center" />
                    <el-table-column prop="a" :label="t('taskPage.executionTime')" align="center" />
                    <el-table-column prop="a" :label="t('taskPage.createTime')" align="center" />
                    <el-table-column fixed="right" :label="t('taskPage.operation')" align="center">
                        <template #default="scope">
                            <el-button link type="primary" @click="openHistoryDialog">{{ t('taskPage.executionHistory') }}</el-button>
                            <el-button link type="primary" @click="openEditDialog(scope.row)">{{ t('common.edit') }}</el-button>
                            <el-button link type="danger">{{ t('common.delete') }}</el-button>
                        </template>
                    </el-table-column>
                </el-table>
            </template>
        </facilityHeader>
        <taskDialog v-if="taskDialogVisible" :type="taskDialogType" :row="editRow" @updateVisible="handleTaskSubmit"/>
        <historyDialog v-if="historyDialogVisible" :ruleName="'qweqwweqwwe'"  @updateClick="handleHistoryRefresh" />
    </div>
</template>

<script lang="ts" setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import taskDialog from './taskDialog.vue'
import historyDialog from './historyDialog.vue'

const { t } = useI18n()

const facilityConfig = computed(() => ({
    title: t('taskPage.taskPage'),
    search: [
        {
            fields: "c",
            type: "input",
            placeholder: t('taskPage.taskName')
        },
        {
            fields: "",
            type: "button",
            label: t('common.search'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                console.log(e)
            }
        }
    ]
}))

const rulesList = ref([{ a: 111 }])

const taskDialogVisible = ref(false)
const taskDialogType = ref(0)
const editRow = ref<any>({})

const historyDialogVisible = ref(false)
// 新建定时任务弹窗
const openAddDialog = () => {
    taskDialogType.value = 0
    editRow.value = {}
    taskDialogVisible.value = true
}

const openEditDialog = (row: any) => {
    taskDialogType.value = 1
    editRow.value = row
    taskDialogVisible.value = true
}
// 执行历史弹窗
const openHistoryDialog = () => {
    historyDialogVisible.value = true
     console.log(historyDialogVisible.value)
}
// 新建定时任务取消弹窗
const handleTaskSubmit = (data: any) => {
    taskDialogVisible.value = false
}
//执行历史取消弹窗
const handleHistoryRefresh = (e:any) => {
    historyDialogVisible.value = e
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>

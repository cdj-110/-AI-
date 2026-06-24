<template>
    <div class="userGroup">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="addGroup">{{ $t('dataCenter.addGroup') }}</el-button>
            </template>
            <template #content>
                <el-table :data="tableData" style="width: 100%;margin-top: 0.5rem;">
                    <el-table-column prop="ID" :label="$t('dataCenter.taskID')" align="center" />
                    <el-table-column prop="name" :label="$t('dataCenter.reportName')" align="center" />
                    <el-table-column prop="action_type" :label="$t('dataCenter.timeCycle')" align="center" />
                    <el-table-column prop="creation_time" :label="$t('dataCenter.createTime')" align="center" />
                    <el-table-column prop="name" :label="$t('dataCenter.status')" align="center" />
                    <el-table-column prop="message" :label="$t('dataCenter.operation')" align="center">
                        <template #default="scope">
                            <el-button link type="primary" size="small" :disabled="scope.row.value == '1'">{{ $t('dataCenter.exportReport') }}</el-button>
                            <el-button link type="danger" size="small"  @click="dele(scope.row)" :disabled="scope.row.value == '1'">
                                {{ $t('dataCenter.deleteTask') }}
                            </el-button>
                        </template>
                    </el-table-column>
                </el-table>
                <div style="display: flex;justify-content: flex-end;margin: 0.7rem 0 3rem 0;">
                    <el-pagination v-model:current-page="page" v-model:page-size="size"
                        :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next"
                        :total="total" @size-change="handleSizeChange" @current-change="handleCurrentChange" />
                </div>
            </template>
        </facilityHeader>
        <!-- 新建报表 -->
        <addGroupDialog v-if="isOk" :title="title" @handleClose="handleClose"></addGroupDialog>
    </div>
</template>
<script lang="ts" setup>
import { useI18n } from 'vue-i18n'
import facilityHeader from "@/components/facilityHeader.vue";
import addGroupDialog from './addGroupDialog.vue'

const { t } = useI18n()

let facilityConfig = () => ({
    title: t('dataCenter.dataReport'),
    search: [
        {
            fields: "b",
            type: "select",
            placeholder: t('dataCenter.status'),
            options: [
                { label: t('dataCenter.offline'), value: 0 },
                { label: t('dataCenter.online'), value: 1 },
                { label: t('dataCenter.deleted'), value: 2 }
            ]
        },
        {
            fields: "c",
            type: "input",
            placeholder: t('dataCenter.reportName')
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
})
let tableData = ref([{value:'1'}])
let isOk = ref(false)
let title = ref('')
let page = ref(1)
let size = ref(10)
let total = ref(20)

let addGroup = () => { isOk.value = true, title.value = t('dataCenter.addGroup') }
let handleClose = (e: any) => { isOk.value = false }

let dele = (e: any) => {
    ElMessageBox.confirm(t('dataCenter.areYouSureDelete', { name: e.name || '' }), t('common.prompt'), {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
    }).then(() => {
        ElMessage.success(t('common.deleteSuccess'));
    })
}
// 分页 条
let handleSizeChange = (val: number) => {
    console.log(`${val} items per page`)
}
// 分页 页
let handleCurrentChange = (val: number) => {
    console.log(`current page: ${val}`)
}
</script>
<style lang="scss" scoped>
.userGroup {
    background: #fff;
    height: 100%;
    border-radius: 0.7rem;

    :deep(.el-table th.el-table__cell) {
        background-color: #f5f7fa;
    }
}
</style>
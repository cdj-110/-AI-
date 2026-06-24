<template>
    <div class="userGroup">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="addGroup">{{ t('warning.warningGroup.addGroup') }}</el-button>
            </template>
            <template #content>
                <el-table :data="tableData" style="width: 100%;margin-top: 0.5rem;">
                    <el-table-column prop="name" :label="t('warning.warningGroup.notificationGroup')" align="center" />
                    <el-table-column prop="action_type" :label="t('warning.warningGroup.members')" align="center" />
                    <el-table-column prop="creation_time" :label="t('warning.warningGroup.createTime')" align="center" />
                    <el-table-column prop="message" :label="t('warning.warningGroup.operation')" align="center">
                        <template #default="scope">
                            <el-button link type="primary" size="small" @click="editGroup(scope.row)">{{ t('warning.warningGroup.edit') }}</el-button>
                            <el-button link type="primary" size="small" style="color: red;" @click="dele(scope.row)"> {{ t('warning.warningGroup.delete') }}
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
        <!-- 新建通知组 -->
        <addGroupDialog v-if="isOk" :title="title" @handleClose="handleClose"></addGroupDialog>
    </div>
</template>
<script lang="ts" setup>
import facilityHeader from "@/components/facilityHeader.vue";
import addGroupDialog from './addGroupDialog.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

// 查询条件
let facilityConfig = () => ({
    title: t('warning.warningGroup.pageTitle'),
    search: [
        {
            fields: "c",
            type: "input",
            placeholder: t('warning.warningGroup.searchPlaceholder')
        },
        {
            fields: "",
            type: "button",
            label: t('warning.warningGroup.query'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                console.log(e)
            }
        }
    ]
})
let tableData = ref([{ name: '这是一个通知组', action_type: '张三、李四、王五' }]) // 列表数据
let isOk = ref(false) // 控制弹窗
let title = ref('') // title名
let page = ref(1)// 分页 页
let size = ref(10)// 分页 条
let total = ref(20)// 总条数

// 新增账号
let addGroup = () => { isOk.value = true, title.value = t('warning.warningGroup.addGroup') }
// 编辑账号
let editGroup = (e: any) => { isOk.value = true, title.value = t('warning.warningGroup.editGroup') }
let handleClose = (e: any) => { isOk.value = false }
// 删除
let dele = (e: any) => {
    ElMessageBox.confirm(`${t('warning.warningGroup.deleteConfirm')}${e.name}?`, t('warning.warningGroup.prompt'), {
        confirmButtonText: t('warning.warningGroup.confirm'),
        cancelButtonText: t('warning.warningGroup.cancel'),
        type: 'warning',
    }).then(() => {
        ElMessage.success(e);
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

<template>
    <div class="userRole">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="addRole">{{ t('warning.warningRule.addRule') }}</el-button>
            </template>
            <template #content>
                <el-table :data="tableData" style="width: 100%;margin-top: 0.5rem;">
                    <el-table-column prop="name" :label="t('warning.warningRule.ruleName')" align="center" />
                    <el-table-column prop="type" :label="t('warning.warningRule.status')" align="center">
                        <template #default="scope">
                            <el-switch v-model="scope.row.type" active-value="1" inactive-value="2" />
                        </template>
                    </el-table-column>
                    <el-table-column prop="" :label="t('warning.warningRule.notificationGroup')" align="center" />
                    <el-table-column prop="creation_time" :label="t('warning.warningRule.createTime')" align="center" />
                    <el-table-column prop="message" :label="t('warning.warningRule.operation')" align="center" width="450">
                        <template #default="scope">
                            <el-button link type="primary" @click="editRole(scope.row)">{{ t('warning.warningRule.edit') }}</el-button>
                            <el-button link type="primary" style="color: red;" @click="dele(scope.row)"> {{ t('warning.warningRule.delete') }}
                            </el-button>
                            <el-button link type="primary" @click="History(scope.row.id)">{{ t('warning.warningRule.alarmHistory') }}</el-button>
                            <el-button plain @click="isOnlineAllocationGroup = true">{{ t('warning.warningRule.allocateGroup') }}</el-button>
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
        <!-- 新增编辑角色 -->
        <addAlarm v-if="isOk" :title="title" @handleClose="handleClose"></addAlarm>
        <!--  分配组弹窗-->
        <onlineAllocationGroup v-if="isOnlineAllocationGroup" @onlineAllocationGroupClose="onlineAllocationGroupClose">
        </onlineAllocationGroup>
        <!-- 告警历史 -->
        <alarmHistory v-if="isalarmHistory" @alarmHistoryClose="alarmHistoryClose" :objid="objid"></alarmHistory>
    </div>
</template>
<script lang="ts" setup>
import facilityHeader from "@/components/facilityHeader.vue";
import onlineAllocationGroup from './onlineAllocationGroup.vue'
import addAlarm from './addAlarm.vue'
import alarmHistory from './alarmHistory.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

// 查询条件
let facilityConfig = () => ({
    title: t('warning.warningRule.pageTitle'),
    search: [
        {
            fields: "c",
            type: "input",
            placeholder: t('warning.warningRule.searchPlaceholder')
        },
        {
            fields: "",
            type: "button",
            label: t('warning.warningRule.query'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                console.log(e)
            }
        }
    ]
})
let tableData = ref([{ name: '管理员', type: '1' }, { name: '管理员', type: '2' }, { name: '管理员', type: '1' }, { name: '管理员', type: '2' }]) // 列表数据
let isOk = ref(false) // 控制弹窗
let title = ref('') // title名
let page = ref(1)// 分页 页
let size = ref(10)// 分页 条
let total = ref(20)// 总条数
let isOnlineAllocationGroup = ref(false) // 分配组弹窗
let isalarmHistory = ref(false) // 告警历史
let objid = ref('')// 告警历史id

// 新增账号
let addRole = () => { isOk.value = true, title.value = t('warning.warningRule.addRule') }
// 编辑账号
let editRole = (e: any) => { isOk.value = true, title.value = t('warning.warningRule.editRule') }
let handleClose = (e: any) => { isOk.value = false }
// 删除
let dele = (e: any) => {
    ElMessageBox.confirm(t('warning.warningRule.deleteConfirm'), t('warning.warningRule.prompt'), {
        confirmButtonText: t('warning.warningRule.confirm'),
        cancelButtonText: t('warning.warningRule.cancel'),
        type: 'warning',
    }).then(() => {
        ElMessage.success(e);
    })
}
// 分配组取消弹窗
let onlineAllocationGroupClose = (e: any) => { isOnlineAllocationGroup.value = false }
// 告警历史
let History = (e: any) => { objid.value = e, isalarmHistory.value = true }
// 告警历史取消弹窗
let alarmHistoryClose = (e: any) => { isalarmHistory.value = false }
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
.userRole {
    background: #fff;
    height: 100%;
    border-radius: 0.7rem;

    :deep(.el-table th.el-table__cell) {
        background-color: #f5f7fa;
    }

    :deep(.el-button.is-plain) {
        border: 1px #409eff solid;
        color: #409eff;
    }
}
</style>

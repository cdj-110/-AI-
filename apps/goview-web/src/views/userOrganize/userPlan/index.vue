<template>
    <div class="userPlan">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="addPlan">{{ $t('userOrganize.addOrganization') }}</el-button>
            </template>
            <template #content>
                <el-table :data="tableData" style="width: 100%;margin-top: 0.5rem;" row-key="id">
                    <el-table-column prop="name" :label="$t('userOrganize.organizationName')" />
                    <el-table-column prop="action_type" :label="$t('userOrganize.permission')" align="center" width="400"/>
                    <el-table-column prop="message" :label="$t('userOrganize.operation')" align="center" >
                        <template #default="scope">
                            <el-button plain @click="isAssignPermissions = true">{{ $t('userOrganize.assignPermissions') }}</el-button>
                            <el-button plain @click="device(scope.row)">{{ $t('userOrganize.assignDevices') }}</el-button>
                            <el-button plain @click="isAllocateDeviceGroups = true">{{ $t('userOrganize.assignDeviceGroups') }}</el-button>
                            <el-button link type="primary" size="small" @click="editPlan(scope.row)">{{ $t('common.edit') }}</el-button>
                            <el-button link type="primary" size="small" style="color: red;" @click="dele(scope.row)">{{ $t('common.delete') }}
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
        <!-- 新增组织 -->
        <addPlanDialog v-if="isAddPlan" :title="title" @handleClose="handleClose"></addPlanDialog>
        <!-- 分配设备组 -->
        <allocateDeviceGroups v-if="isAllocateDeviceGroups" @handleCloseGroups="handleCloseGroups">
        </allocateDeviceGroups>
        <!-- 分配权限 -->
        <assignPermissions v-if="isAssignPermissions" @handleCloseRole="handleCloseRole"></assignPermissions>
        <!-- 分配设备 -->
        <distributionEquipment v-if="isDistributionEquipment" @handleCloseEquipment="handleCloseEquipment" :obj="obj">
        </distributionEquipment>
    </div>
</template>
<script lang="ts" setup>
import { useI18n } from 'vue-i18n'
import facilityHeader from "@/components/facilityHeader.vue";
import addPlanDialog from './addPlanDialog.vue'
import allocateDeviceGroups from './allocateDeviceGroups.vue'
import assignPermissions from './assignPermissions.vue'
import distributionEquipment from './distributionEquipment.vue'

const { t } = useI18n()

let facilityConfig = () => ({
    title: t('userOrganize.userOrganize'),
    search: [
        {
            fields: "c",
            type: "input",
            placeholder: t('userOrganize.organizationName')
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

let tableData = ref([
    {
        id: 1,
        date: '2016-05-02',
        name: '北京微控',
        address: 'No. 189, Grove St, Los Angeles',
        children: [
            {
                id: 2,
                date: '2016-05-01',
                name: '北京微智控',
                address: 'No. 189, Grove St, Los Angeles',
                children: [
                    {
                        id: 3,
                        date: '2016-05-01',
                        name: '技术部',
                        address: 'No. 189, Grove St, Los Angeles',
                    },
                ]
            },

        ],

    },
    {
        id: 4,
        date: '2016-05-04',
        name: 'wangxiaohu',
        address: 'No. 189, Grove St, Los Angeles',
        children: [
            {
                id: 5,
                date: '2016-05-01',
                name: 'wangxiaohu',
                address: 'No. 189, Grove St, Los Angeles',
            },
            {
                id: 6,
                date: '2016-05-01',
                name: 'wangxiaohu',
                address: 'No. 189, Grove St, Los Angeles',
            },
        ],
    },
    {
        id: 7,
        date: '2016-05-01',
        name: 'wangxiaohu',
        address: 'No. 189, Grove St, Los Angeles',
        children: [
            {
                id: 8,
                date: '2016-05-01',
                name: 'wangxiaohu',
                address: 'No. 189, Grove St, Los Angeles',
                children: [
                    {
                        id: 9,
                        date: '2016-05-01',
                        name: 'wangxiaohu',
                        address: 'No. 189, Grove St, Los Angeles',
                    },
                    {
                        id: 10,
                        date: '2016-05-01',
                        name: 'wangxiaohu',
                        address: 'No. 189, Grove St, Los Angeles',
                    },
                ],
            },
            {
                id: 11,
                date: '2016-05-01',
                name: 'wangxiaohu',
                address: 'No. 189, Grove St, Los Angeles',
            },
        ],
    },
    {
        id: 12,
        date: '2016-05-03',
        name: 'wangxiaohu',
        address: 'No. 189, Grove St, Los Angeles',
    },
]) // 列表数据
let isAssignPermissions = ref(false) // 分配权限弹窗
let isAllocateDeviceGroups = ref(false) // 分配设备组
let isDistributionEquipment = ref(false) // 分配设备
let title = ref('') // title名
let page = ref(1)// 分页 页
let size = ref(10)// 分页 条
let total = ref(20)// 总条数
let isAddPlan = ref(false) // 新增组织弹窗
let obj = ref({}) //分配设备弹窗数据

let addPlan = () => { isAddPlan.value = true, title.value = t('userOrganize.addOrganization') }

let dele = (e: any) => {
    ElMessageBox.confirm(t('userOrganize.areYouSureDelete', { name: e.name }), t('common.prompt'), {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
    }).then(() => {
        ElMessage.success(t('common.deleteSuccess'));
    })
}

let editPlan = (e: any) => { isAddPlan.value = true, title.value = t('userOrganize.editOrganization') }

// 分配设备弹窗
let device = (e: any) => { console.log(e),isDistributionEquipment.value = true, obj.value = e }
// 新增组织取消弹窗
let handleClose = (e: any) => { isAddPlan.value = false }

// 分配权限取消弹窗
let handleCloseRole = (e: any) => { isAssignPermissions.value = false }

// 分配权限取消弹窗
let handleCloseGroups = (e: any) => { isAllocateDeviceGroups.value = false }

// 分配权限取消弹窗
let handleCloseEquipment = (e: any) => { isDistributionEquipment.value = false }

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
.userPlan {
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
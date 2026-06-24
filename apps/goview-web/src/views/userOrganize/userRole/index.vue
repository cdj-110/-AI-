<template>
    <div class="userRole">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="addRole">
                    {{ t('userRole.addRole') }}
                </el-button>
            </template>
            <template #content>
                <el-table :data="tableData" style="width: 100%;margin-top: 0.5rem;">
                    <el-table-column prop="name" :label="t('userRole.roleName')" align="center" />
                    <el-table-column prop="creation_time" :label="t('userRole.createTime')" align="center" />
                    <el-table-column :label="t('userRole.operation')" align="center">
                        <template #default="scope">
                            <el-button link type="primary" size="small" @click="editRole(scope.row)">
                                {{ t('userRole.edit') }}
                            </el-button>
                            <el-button link type="primary" size="small" style="color: red;" @click="dele(scope.row)">
                                {{ t('userRole.delete') }}
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
        <addRoleDialog v-if="isOk" :title="title" @handleClose="handleClose"></addRoleDialog>
    </div>
</template>

<script lang="ts" setup>
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { ElMessage, ElMessageBox } from "element-plus";
import facilityHeader from "@/components/facilityHeader.vue";
import addRoleDialog from './addRoleDialog.vue'

const { t } = useI18n();

const facilityConfig = () => ({
    title: t('userRole.pageTitle'),
    search: [
        {
            fields: "c",
            type: "input",
            placeholder: t('userRole.roleName')
        },
        {
            fields: "",
            type: "button",
            label: t('userRole.search'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                console.log(e)
            }
        }
    ]
})

const tableData = ref([
    { name: '管理员' },
    { name: '管理员' },
    { name: '管理员' },
    { name: '管理员' }
])
const isOk = ref(false)
const title = ref('')
const page = ref(1)
const size = ref(10)
const total = ref(20)

const addRole = () => {
    isOk.value = true
    title.value = t('userRole.addRole')
}

const editRole = (e: any) => {
    isOk.value = true
    title.value = t('userRole.editRole')
}

const handleClose = (e: any) => {
    isOk.value = false
}

const dele = (e: any) => {
    ElMessageBox.confirm(
        t('userRole.confirmDelete', { name: e.name }),
        t('userRole.prompt'),
        {
            confirmButtonText: t('userRole.confirm'),
            cancelButtonText: t('userRole.cancel'),
            type: 'warning',
        }
    ).then(() => {
        ElMessage.success(t('userRole.deleteSuccess'))
    })
}

const handleSizeChange = (val: number) => {
    console.log(`${val} items per page`)
}

const handleCurrentChange = (val: number) => {
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
}
</style>

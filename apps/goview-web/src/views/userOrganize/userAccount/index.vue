<template>
    <div class="userAccount">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="addAcction">{{ $t('userAccount.addAccount') }}</el-button>
            </template>
            <template #content>
                <el-table :data="tableData" style="width: 100%;margin-top: 0.5rem;">
                    <el-table-column prop="a" :label="$t('userAccount.userName')" align="center" />
                    <el-table-column prop="b" :label="$t('userAccount.account')" align="center" />
                    <el-table-column prop="level" :label="$t('userAccount.organization')" align="center">
                    </el-table-column>
                    <el-table-column prop="results" :label="$t('userAccount.status')" align="center">
                        <template #default="scope">
                            <el-switch v-model="scope.row.results" class="ml-2"
                                style="--el-switch-on-color: #409EFF; --el-switch-off-color: #DCDFE6" />
                        </template>
                    </el-table-column>
                    <el-table-column prop="creation_time" :label="$t('userAccount.createTime')" align="center" />
                    <el-table-column prop="message" :label="$t('userAccount.operation')" align="center">
                        <template #default="scope">
                            <el-button link type="primary" size="small" @click="editAcction(scope.row)">{{ $t('common.edit') }}</el-button>
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
        <!-- 新增账号弹窗 -->
        <addAcctionDialog v-if="isOk" @handleClose="handleClose" :title="title" :obj="obj"></addAcctionDialog>
    </div>
</template>
<script lang="ts" setup>
import { useI18n } from 'vue-i18n'
import facilityHeader from "@/components/facilityHeader.vue";
import addAcctionDialog from './addAcctionDialog.vue'

const { t } = useI18n()

let facilityConfig = () => ({
    title: t('userAccount.userAccount'),
    search: [
        {
            fields: "c",
            type: "input",
            placeholder: t('userAccount.userName') + '/' + t('userAccount.account')
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
let tableData = ref([{ a: '张三', b: '18210946822' }, { a: '李四', b: '18210946822@163.com' }])
let isOk = ref(false)
let title = ref('')
let page = ref(1)
let size = ref(10)
let total = ref(20)
let obj = ref({})


let addAcction = () => {
    isOk.value = true
    title.value = t('userAccount.addAccount')
}

let editAcction = (e: any) => {
    isOk.value = true
    obj.value = e
    title.value = t('userAccount.editAccount')
}

let handleClose = (e: any) => {
    isOk.value = false
}

let dele = (e: any) => {
    ElMessageBox.confirm(t('userAccount.areYouSureDelete', { name: e.name || e.a }), t('common.prompt'), {
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
.userAccount {
    background: #fff;
    height: 100%;
    border-radius: 0.7rem;

    :deep(.el-table th.el-table__cell) {
        background-color: #f5f7fa;
    }
}
</style>
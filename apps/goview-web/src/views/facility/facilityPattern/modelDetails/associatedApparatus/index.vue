<template>
    <div style="overflow: auto; height: 50vh;padding: 0 0.5rem 0 0;">
        <div style="display: flex;justify-content: space-between;">
            <div>
                <el-button type="primary" icon="Plus" @click="add">{{ t('facilityPattern.associatedDevice') }}</el-button>
                <el-button type="danger" icon="Minus" @click="deleteRow"
                    v-if="multipleSelection.length > 1">{{ t('facilityPattern.removeDevice') }}</el-button>
            </div>
            <div>
                <el-form :inline="true" :model="formInline" class="demo-form-inline">
                    <el-form-item>
                        <el-select v-model="formInline.dev_status" :placeholder="t('facilityPattern.deviceStatus')" clearable style="width: 10rem;">
                            <el-option :label="t('facilityPattern.offline')" :value="0" />
                            <el-option :label="t('facilityPattern.online')" :value="1" />
                        </el-select>
                    </el-form-item>
                    <el-form-item>
                        <el-input v-model="formInline.keyword" :placeholder="t('facilityPattern.deviceName') + '/' + t('facilityPattern.deviceSerialNumber') + '/' + t('facilityPattern.modelName')" clearable
                            style="width: 15rem;" />
                    </el-form-item>
                    <el-form-item>
                        <el-button type="primary" @click="onSubmit">{{ t('facilityPattern.query') }}</el-button>
                    </el-form-item>
                </el-form>
            </div>
        </div>
        <!-- 表格 -->
        <div>
            <el-table :data="tableData" style="width: 100%" @selection-change="handleSelectionChange">
                <el-table-column type="selection" width="55" />
                <el-table-column prop="dev_name" :label="t('facilityPattern.deviceName')" align="center">
                </el-table-column>
                <el-table-column prop="dev_sn" :label="t('facilityPattern.deviceSerialNumber')" align="center" />
                <el-table-column prop="dev_status" :label="t('facilityPattern.status')" align="center">
                    <template #default="scope">
                        <el-tag type="success" v-if="scope.row.dev_status == 1">{{ t('facilityPattern.online') }}</el-tag>
                        <el-tag type="info" v-if="scope.row.dev_status === 0">{{ t('facilityPattern.offline') }}</el-tag>
                    </template>
                </el-table-column>
                <el-table-column prop="create_time" :label="t('facilityPattern.createTime')" align="center"
                    :formatter="((row: any) => itializeUtc(row.create_time))" />
                <el-table-column :label="t('facilityPattern.operation')" align="center">
                    <template #default="scope">
                        <el-button link type="primary" size="small" style="color: red;"
                            @click="dele(scope.row)">{{ t('facilityPattern.remove') }}</el-button>
                    </template>
                </el-table-column>
            </el-table>
            <div style="display: flex;justify-content: flex-end;margin: 0.5rem 0 3rem 0;">
                <el-pagination v-model:current-page="page" v-model:page-size="size"
                    :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next" :total="total"
                    @size-change="handleSizeChange" @current-change="handleCurrentChange" />
            </div>
        </div>
        <apparatusDialog v-if="isShow" @refreshList="list" @handleClose="handleClose" :tltle="tltle" :id="props.id"
            :objData="valueData">
        </apparatusDialog>
    </div>
</template>
<script lang="ts" setup>
import { itializeUtc } from "@/utils/publicFun";
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import apparatusDialog from './apparatusDialog.vue'

const { t } = useI18n()
import { model_device_list, unlink_model_device, batch_unlink_model_devices } from '@/api/facilityPattern/index'
import { useRoute } from 'vue-router'
const route = useRoute()
import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
const props = defineProps({
    id: {
        type: String,
        required: true
    },
    valueData: {
        type: Object,
        required: false
    }
});
let formInline = ref({ dev_status: null, keyword: '' })
// 查询条件
let tableData = ref([]) // 列表数据
let multipleSelection = ref([]) // 多选数据
// 分页 页
let page = ref(1)
// 分页 条
let size = ref(10)
// 总条数
let total = ref(0)
let isShow = ref(false) // 关联设备弹窗
let tltle = ref('')
onMounted(() => {
    list()
})

/**
 * 列表
 */
let list = () => {
    model_device_list({ user_id: user_id.value, pagenumber: page.value, pagesize: size.value, dev_status: formInline.value.dev_status, keyword: formInline.value.keyword, query_type: "model", query_id: props.id }).then((res: any) => {
        if (res.code == 200) {
            tableData.value = res.data.devices
            total.value = res.data.total
        }
    })
}
// 查询
let onSubmit = () => { list() }
// 多选
let handleSelectionChange = (val: any) => {
    multipleSelection.value = val.map((item: any) => item.dev_id)
}
// 移除设备
let deleteRow = () => {
    if (multipleSelection.value.length <= 0) {
        ElMessage.warning('请先选择设备')
    } else {
        ElMessageBox.confirm('是否移除该设备?', '提示', {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning',
        }).then(() => {
            batch_unlink_model_devices({ user_id: user_id.value, model_id: props.id, dev_ids: multipleSelection.value }).then((res: any) => {
                if (res.code == 200) {
                    ElMessage.success(res.msg);
                    list()
                } else {
                    ElMessage.error(res.msg);
                }
            })
        })
    }

}
// 删除设备
let dele = (e: any) => {
    ElMessageBox.confirm('是否移除该设备?', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
    }).then(() => {
        unlink_model_device({ user_id: user_id.value, model_id: props.id, dev_id: e.dev_id }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                list()
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}

let add = () => { tltle.value = t('facilityPattern.associatedDevice'), isShow.value = true }
// 分页 条
let handleSizeChange = (val: number) => {
    list()
}
// 分页 页
let handleCurrentChange = (val: number) => {
    page.value = val
    list()
}
// 取消弹窗
let handleClose = (e: any) => {
    isShow.value = false
    list()
}

</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}

:deep(.el-select__placeholder.is-transparent),
:deep(.el-input__inner) {
    font-size: 0.8rem;
}

:deep(.el-form--inline .el-form-item) {
    margin-right: 1rem
}
</style>
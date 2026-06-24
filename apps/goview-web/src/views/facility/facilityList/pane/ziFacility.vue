<template>
    <div style="overflow: auto; height: 58vh;padding: 0 0.5rem 0 0;">
        <el-button type="primary" icon="Search" style="margin: 1rem 0;"
            @click="selectConfig.isTrue = true;">添加子设备</el-button>
        <el-button type="danger" icon="Minus" v-if="selectList && selectList.length > 0"
            style="margin: 1rem 0 1rem 1rem;">移除子设备</el-button>
        <el-table ref="multipleTableRef" :data="tableData" row-key="id" @selection-change="handleSelectionChange">
            <el-table-column type="selection" width="55" />
            <el-table-column property="sub_dev_name" label="子设备名称" align="center" />
            <el-table-column property="slave_address" label="从机地址" align="center">
                <template #default="scope">
                    <p v-if="scope.row.isTrue == undefined">{{ scope.row.slave_address }}</p>
                    <el-input-number v-else v-model="scope.row.slave_address" controls-position="right" />
                </template>
            </el-table-column>
            <el-table-column property="online_status" label="在线状态" align="center">
                <!-- <template #default="scope">
                    {{ scope.row.online_status == 0 ? '离线' : scope.row.online_status == 1 ? '在线' : null }}
                </template> -->
                    <template #default="scope">
                        <el-tag type="success" v-if="scope.row.online_status == 1">在线</el-tag>
                        <el-tag type="info" v-if="scope.row.online_status === 0">离线</el-tag>
                    </template>
            </el-table-column>
            <el-table-column property="active_time" label="活跃时间" align="center" :formatter="((row: any) => itializeUtc(row.active_time))" />
            <el-table-column label="操作" align="center">
                <template #default="scope">
                    <el-button link type="primary" v-if="scope.row.isTrue == undefined"
                        @click="scope.row.isTrue = true;">修改</el-button>
                    <el-button link type="primary" v-else @click="scope.row.isTrue = undefined;">保存</el-button>
                    <el-button link type="danger">移除</el-button>
                </template>
            </el-table-column>
        </el-table>
        <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
            <el-pagination v-model:current-page="pagenumber" v-model:page-size="pagesize"
                :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next" :total="total"
                @size-change="handleSizeChange" @current-change="CurrentChange" />
        </div>
        <selectFacility :selectConfig="selectConfig" @handleCurrentChange="handleCurrentChange"
            :sub_dev_model="props.dev_model" :id="props.dev_id" @handleClose="handleClose"></selectFacility>
    </div>
</template>

<script lang="ts" setup>
import selectFacility from "./selectFacility.vue";
import { device_detail_sublist } from '@/api/facilityList/index'
import useCounterStore from "@/stores/counter";
import { itializeUtc } from "@/utils/publicFun";

const props = defineProps({
    id: {
        type: String,
        required: ''
    },
    dev_model: {
        type: String,
        required: ''
    },
    dev_id: {
        type: String,
        required: ''
    },

});
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);

let selectConfig = ref({ isTrue: false });
let tableData = ref([]);//子设备列表
let selectList = ref([]);//保存多选的子设备列表
// 分页 页
let pagenumber = ref(1)
// 分页 条
let pagesize = ref(10)
// 总条数
let total = ref(0)
onMounted(() => {
    list()
})
/**
 * 子设备列表多选
 */
let handleSelectionChange = (e: any) => {
    console.log(e)
    selectList.value = e;
}

/**
 * 选择子设备的单选
 */
let handleCurrentChange = (e: any) => {
    console.log(e)
}

let list = () => {
    device_detail_sublist({ dev_id: props.dev_id, user_id: user_id.value, pagenumber: pagenumber.value, pagesize: pagesize.value }).then((res: any) => {
        if (res.code == 200) {
            tableData.value = res.data.data
            total.value = res.data.total
        }
    })
}

// 分页 条
let handleSizeChange = (val: number) => {
    list()
}
// 分页 页
let CurrentChange = (val: number) => {
    pagenumber.value = val
    list()
}
// 取消
let handleClose = (e: any) => {
    list()
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
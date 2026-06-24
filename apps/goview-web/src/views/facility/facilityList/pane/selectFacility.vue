<template>
    <div>
        <el-dialog v-model="props.selectConfig.isTrue" title="子设备列表" width="50%" :close-on-click-modal="false"
            :close-on-press-escape="false" draggable>
            <el-form :inline="true" :model="inlineSearch" style="text-align: right;">
                <el-form-item style="width: 17%;">
                    <el-select v-model="inlineSearch.communication" placeholder="联网方式" clearable>
                        <el-option v-for="item in options" :key="item.value" :label="item.label" :value="item.value" />
                    </el-select>
                </el-form-item>
                <el-form-item style="width: 17%;">
                    <el-input v-model="inlineSearch.dev_name" placeholder="设备名称" clearable />
                </el-form-item>
                <el-form-item>
                    <el-button type="primary" icon="Search">查询</el-button>
                </el-form-item>
            </el-form>
            <el-table :data="ziFacilityList" highlight-current-row @selection-change="handleSelectionChange">
                <el-table-column property="dev_name" label="设备名称" align="center" />
                <el-table-column property="communication" label="连网方式" align="center" />
                <el-table-column property="create_time" label="创建时间" align="center"
                    :formatter="((row: any) => itializeUtc(row.create_time))" />
                <el-table-column property="dev_desc" label="设备描述" align="center" />
                <el-table-column type="selection" width="55" align="center" />
            </el-table>
            <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
                <el-pagination v-model:current-page="pagenumber" v-model:page-size="pagesize"
                    :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next" :total="total"
                    @size-change="handleSizeChange" @current-change="handleCurrentChange" />
            </div>
            <template #footer>
                <el-button type="info" @click="clear">取消</el-button>
                <el-button type="primary" @click="selectChange">提交</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { itializeUtc } from "@/utils/publicFun";
import { device_detail_add_subdevice } from '@/api/facilityList/index'
import useCounterStore from "@/stores/counter";
import { sub_device_list, modelDatas } from '@/api/facilityList/index'
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let props = withDefaults(defineProps<{
    selectConfig: {
        isTrue: Boolean
    },
    sub_dev_model: {
        type: String,
        required: ''
    },
    id: {
        type: String,
        required: ''
    },
}>(), {
    selectConfig: () => ({
        isTrue: false
    }),
});
let ziFacilityList = ref([]);//子设备列表
let inlineSearch = ref({ dev_name: "", communication: "", });//添加子设备搜索
let tarList: any = ref({});//添加子设备列表单选的内容
let options = ref([])
let arrList = ref([])
// 分页 页
let pagenumber = ref(1)
// 分页 条
let pagesize = ref(10)
// 总条数
let total = ref(0)
let emit = defineEmits(["handleClose"]);
onMounted(() => {
    list()
    modelDatasList()
})
let list = () => {
    sub_device_list({
        user_id: user_id.value, group_id: props.id, query_type: 'model', dev_name: inlineSearch.value.dev_name,
        communication: inlineSearch.value.communication, pagenumber: pagenumber.value, pagesize: pagesize.value
    }).then((res: any) => {
        if (res.code == 200) {
            ziFacilityList.value = res.data.data
            total.value = res.data.total
        }
    })
}
let modelDatasList = () => {
    modelDatas().then((res: any) => {
        for (const key in res.data.communication) {
            options.value.push({ label: key, value: res.data.communication[key] })
        }
    })
}

// 处理选中变化
let handleSelectionChange = (val: any) => {
    arrList.value = val.map((item: any) => {
        return {
            "sub_dev_id": item.dev_id,
            "sub_dev_name": item.dev_name,
            "slave_address": "0", // 固定值
            "gateway_model_id": props.sub_dev_model, // 从props取
            "gateway_id": props.id, // 从props取
            "dev_desc": item.dev_desc,
            "user_id": user_id.value // 固定值
        };
    });
}

/**
 * 点击确定时触发外部函数传输数据
 */
let selectChange = () => {
    if (arrList.value.length <= 0) {
        ElMessage.warning("请先选择一个子设备！");
    } else {
        device_detail_add_subdevice(arrList.value).then((res: any) => {
            if (res.code == 200) {
                emit('handleClose', false)
                ElMessage.success(res.msg);
                props.selectConfig.isTrue = false
            } else {
                ElMessage.error(res.msg);
            }
        })

    }
}
let clear = () => {
    emit('handleClose', false)
    props.selectConfig.isTrue = false;
}
// 分页 条
let handleSizeChange = (val: number) => {
    list()
}
// 分页 页
let handleCurrentChange = (val: number) => {
    pagenumber.value = val
    list()
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
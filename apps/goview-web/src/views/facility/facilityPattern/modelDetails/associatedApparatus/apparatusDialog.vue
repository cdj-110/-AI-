<template>
    <div>
        <el-dialog v-model="dialogTableVisible" :title="props.tltle" width="900" @close="emit('handleClose', false)">
            <facilityHeader :facilityConfig="facilityConfig" style="padding:0;">
                <template #left>
                </template>
                <template #content>
                    <el-table :data="gridData" @selection-change="handleSelectionChange">
                        <el-table-column property="dev_name" :label="t('facilityPattern.deviceName')" align="center" />
                        <el-table-column property="communication" :label="t('facilityPattern.networkMethod')" align="center" />
                        <el-table-column property="create_time" :label="t('facilityPattern.createTime')" align="center"
                            :formatter="((row: any) => itializeUtc(row.create_time))" />
                        <el-table-column property="dev_desc" :label="t('facilityPattern.deviceDescription')" align="center" />
                        <el-table-column type="selection" width="55" align="center" />
                    </el-table>
                    <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
                        <el-pagination v-model:current-page="pagenumber" v-model:page-size="pagesize"
                            :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next"
                            :total="total" @size-change="handleSizeChange" @current-change="handleCurrentChange" />
                    </div>
                </template>
            </facilityHeader>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleClose', false)">{{ t('facilityPattern.cancel') }}</el-button>
                    <el-button type="primary" @click="handleChangeClick">
                        {{ t('facilityPattern.confirm') }}
                    </el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { itializeUtc } from "@/utils/publicFun";
import { sub_device_list, modelDatas } from '@/api/facilityList/index'
import { add_model_device } from '@/api/facilityPattern/index'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
const props = defineProps({
    tltle: {
        type: String,
        required: false
    },
    id: {
        type: String,
        required: false
    },
    objData: {
        type: Object,
        required: false
    }
});
let emit = defineEmits(["handleClose", "handleChange", 'refreshList']);
let dialogTableVisible = ref(true)
let gridData = ref([])
let objValue = ref({})
let optionlist: any = ref([])
let arrList = ref([]);//选中之后点击确定保存选中的数据
let dev_list = ref([])
// 分页 页
let pagenumber = ref(1)
// 分页 条
let pagesize = ref(10)
// 总条数
let total = ref(0)

// 查询条件
let facilityConfig = () => ({
    // title: "设备模型",
    search: [
        {
            fields: "dev_name",
            type: "input",
            placeholder: "设备名称"
        },
        {
            fields: "communication",
            type: "select",
            placeholder: "连网方式",
            options: optionlist.value
        },
        {
            fields: "",
            type: "button",
            label: "查询",
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                objValue.value = e
                list()
            }
        }
    ]
})

onMounted(() => {
    list()
    modelDatasList()
})
let list = () => {
    let params = {
        user_id: user_id.value,
        dev_name: objValue.value.dev_name,
        communication: objValue.value.communication,
        pagenumber: pagenumber.value,
        pagesize: pagesize.value
    }
    if (props.tltle == '子设备列表') {
        params.query_type = 'model'
    } else if (props.tltle == '关联设备') {
        params.query_type = 'unrelated'
    }
    sub_device_list(params).then((res: any) => {
        if (res.code == 200) {
            gridData.value = res.data.data
            total.value = res.data.total
        }
    })
}
let modelDatasList = () => {
    modelDatas().then((res: any) => {
        for (const key in res.data.communication) {
            optionlist.value.push({ label: key, value: res.data.communication[key] })
        }
    })
}
// 处理选中变化
let handleSelectionChange = (val: any) => {
    let devIdList = val.map((item: any) => item.dev_id)
    arrList.value = val;
    dev_list.value = devIdList
}
/**
 * 确定
 */
let handleChangeClick = () => {
    if (props.tltle == '子设备列表') {
        dialogTableVisible.value = false;
        emit("handleChange", !Array.isArray(arrList.value) || arrList.value.length <= 0 ? [] : arrList.value);
    } else {
        add_model_device({ user_id: user_id.value, dev_model_id: props.id, dev_ids: dev_list.value, model_name: props.objData.model_name, communication: props.objData.communication, dev_type: props.objData.typeId }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                dialogTableVisible.value = false;
                emit('refreshList')
            } else {
                ElMessage.error(res.msg);
            }
        })
    }
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
:deep(.el-button>span) {
    font-size: 13px;
}

:deep(.el-select__placeholder.is-transparent),
:deep(.el-input__inner) {
    font-size: 0.8rem;
}
</style>
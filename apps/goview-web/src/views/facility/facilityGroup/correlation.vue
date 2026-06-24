<template>
    <div>
        <el-dialog v-model="props.config.isTrue" :title="props.config.title" width="50%" @close="emit('handleClose', false)">
            <el-form :inline="true" :model="inlineSearch" style="text-align: right;">
                <el-form-item style="width: 17%;">
                    <el-select v-model="inlineSearch.communication" :placeholder="t('facilityGroup.communication')" clearable>
                        <el-option v-for="item in options" :key="item.value" :label="item.label" :value="item.value" />
                    </el-select>
                </el-form-item>
                <el-form-item style="width: 17%;">
                    <el-input v-model="inlineSearch.dev_name" :placeholder="t('facilityGroup.deviceName')" clearable />
                </el-form-item>
                <el-form-item>
                    <el-button type="primary" icon="Search" @click="submitForm">{{ t('facilityGroup.query') }}</el-button>
                </el-form-item>
            </el-form>
            <el-table :data="ziFacilityList" highlight-current-row @selection-change="handleCurrentChange">
                <el-table-column property="dev_name" :label="t('facilityGroup.deviceName')" align="center" />
                <el-table-column property="communication" :label="t('facilityGroup.communication')" align="center" />
                <el-table-column property="create_time" :label="t('facilityGroup.createTime')" align="center"
                    :formatter="((row: any) => itializeUtc(row.create_time))" />
                <el-table-column property="dev_desc" :label="t('facilityGroup.deviceDesc')" align="center" />
                <el-table-column type="selection" width="55" align="center" />
            </el-table>
            <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
                <el-pagination v-model:current-page="pagenumber" v-model:page-size="pagesize"
                    :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next" :total="total"
                    @size-change="handleSizeChange" @current-change="handleCurrentChangelist" />
            </div>
            <template #footer>
                <el-button type="info" @click="emit('handleClose', false)">{{ t('facilityGroup.cancel') }}</el-button>
                <el-button type="primary" @click="selectChange">{{ t('facilityGroup.confirm') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import useCounterStore from "@/stores/counter";
import { itializeUtc } from "@/utils/publicFun";
import { sub_device_list, modelDatas } from '@/api/facilityList/index'
import { add_model_device } from '@/api/facilityGroup/index'
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

let emit = defineEmits(['refreshList',"handleClose"]);
let props = withDefaults(defineProps<{
    config: {
        isTrue: Boolean,
        title: String
    },
    group_id: {
        type: string,
        required: ""
    },
    reset: {
        isTrue: Boolean,
        title: String
    },
    groupId: {
        type: string,
        required: ""
    },
    label: {
        type: string,
        required: ""
    }
}>(), {
    config: () => ({
        isTrue: false,
        title: ""
    })
});
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let inlineSearch = ref({ dev_name: "", communication: "" });//选择关联的设备
let ziFacilityList = ref([]);//关联的子设备列表
let tarList: any = ref({});//添加子设备列表单选的内容
let options = ref([])
// 分页 页
let pagenumber = ref(1)
// 分页 条
let pagesize = ref(10)
// 总条数
let total = ref(0)

watch(props, (val) => {
    if (val.reset == true) {
        list()
    //     console.log(11111)
    }
})
onMounted(() => {
    list()
    modelDatasList()
})

let list = () => {
    console.log(111111)
    sub_device_list({
        user_id: user_id.value, query_type: 'group', group_id: props.groupId, dev_name: inlineSearch.value.dev_name,
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
/**
 * 添加子设备列表多选
 */
let handleCurrentChange = (e: any) => {
    // tarList.value = e;
    let devIdList = e.map((item: any) => item.dev_id)
    tarList.value = devIdList;
}

/**
 * 点击确定时获取选择的子设备列表
 */
let selectChange = () => {
    add_model_device({ user_id: user_id.value, group_id: props.groupId, group_name: props.label, dev_ids: tarList.value }).then((res: any) => {
        if (res.code == 200) {
            ElMessage.success(res.msg);
            props.config.isTrue = false;
            emit('refreshList', props.groupId)
        } else {
            ElMessage.error(res.msg);
        }
    })
}
/**
 * 查询
 */
let submitForm = () => { list() }
// 分页 条
let handleSizeChange = (val: number) => {
    list()
}
// 分页 页
let handleCurrentChangelist = (val: number) => {
    pagenumber.value = val
    list()
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
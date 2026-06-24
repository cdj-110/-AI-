<template>
    <div style="background: #fff;height: 100%;">
        <facilityHeader :facilityConfig="facilityConfig" v-if="route.path !== '/ziGroup'">
            <template #left>
                <el-button type="primary" icon="Plus"
                    @click="createGroupConfig = { title: t('facilityGroup.addDeviceGroup'), isTrue: true, type: 0, list: {} }">{{ t('facilityGroup.addDeviceGroup') }}</el-button>
                <el-button type="warning" @click="add">{{ t('facilityGroup.saveSort') }}</el-button>
            </template>
            <template #content>
                <el-table :data="tableData" :default-sort="{ prop: 'sort', order: 'ascending' }">
                    <el-table-column prop="group_name" :label="t('facilityGroup.groupName')" align="center" />
                    <el-table-column prop="group_description" :label="t('facilityGroup.groupDescription')" align="center" />
                    <el-table-column prop="device_count" :label="t('facilityGroup.deviceCount')" align="center" />
                    <el-table-column prop="group_order" :label="t('facilityGroup.groupOrder')" sortable align="center" >
                        <template #default="scope">
                            <el-input-number v-model="scope.row.group_order" controls-position="right"
                                v-if="scope.row.group_name != t('facilityGroup.defaultGroup')" />
                        </template>
                    </el-table-column>
                    <el-table-column prop="create_time" :label="t('facilityGroup.createTime')" align="center" />
                    <el-table-column :label="t('facilityGroup.operation')" width="300" align="center">
                        <template #default="scope">
                            <el-button link type="primary" @click="goDetail(scope.row)">{{ t('facilityGroup.details') }}</el-button>
                            <el-button link type="primary" v-if="scope.row.group_name != t('facilityGroup.defaultGroup')"
                                @click="editGroupClick(scope.row)">{{ t('facilityGroup.edit') }}</el-button>
                            <el-button link type="primary" @click="goDetail(scope.row)">{{ t('facilityGroup.subGroup') }}：{{ scope.row.sub_group_count }}</el-button>
                            <el-button link type="danger" v-if="scope.row.group_name != t('facilityGroup.defaultGroup')"
                                @click="groupRem(scope.row)">{{ t('facilityGroup.delete') }}</el-button>
                        </template>
                    </el-table-column>
                </el-table>
                <div style="display: flex;justify-content: right;margin-top: 1rem;">
                    <el-pagination background layout="prev, pager, next, total" :total="paginnation.total"
                        :page-size="paginnation.pageSize" :current-page="paginnation.currentPage"
                        @current-change="currentChange" />
                </div>
            </template>
        </facilityHeader>
        <createGroup :config="createGroupConfig" @refaer="getList({});"></createGroup>
        <editGroup v-if="isOk" :obj="objValue" :title="createGroupConfig.title" @handleClose="handleClose"></editGroup>
    </div>
</template>

<script lang="ts" setup>
import router from "@/router";
import createGroup from "./createGroup.vue";
import editGroup from './editGroup.vue'
import { groupList, delete_devicegroup } from "@/api/facilityList/index";
import { save_group_order } from '@/api/facilityGroup/index'
import useCounterStore from "@/stores/counter";
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);

//分组表格搜索框
let facilityConfig = () => ({
    title: t('facilityGroup.deviceGroup'),
    search: [
        {
            fields: "groupName",
            type: "input",
            placeholder: t('facilityGroup.groupNamePlaceholder')
        },
        {
            fields: "",
            type: "button",
            label: t('facilityGroup.query'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                getList({ group_name: e.groupName ? e.groupName : "" });
            }
        }
    ]
});
let tableData = ref([]);//分组表格
let createGroupConfig = ref({ type: 0, title: "", isTrue: false, list: {} });//新建设备分组弹窗信息 type 0:新建 1:编辑
let paginnation = ref({ total: 0, pageSize: 10, currentPage: 1 });//分组列表分页
let route = useRoute();
let isOk = ref(false)
let objValue = ref({})

onMounted(() => {
    getList({});
})

/**
 * 删除分组
 */
let groupRem = (e: any) => {
    if (e.group_name == t('facilityGroup.defaultGroup')) {
        ElMessage.warning(t('facilityGroup.defaultGroupCannotDelete'));
        return false;
    }

    ElMessageBox.confirm(t('facilityGroup.confirmDelete'), t('facilityGroup.confirmTitle'), {
        confirmButtonText: t('facilityGroup.confirm'),
        cancelButtonText: t('facilityGroup.cancel'),
        type: 'warning',
    }).then(() => {
        delete_devicegroup({ group_id: e.group_id, user_id: user_id.value }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                getList({});
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}

/**
 * 查询分组
 */
let getList = ({ group_id, group_name }: { group_id?: string, group_name?: string }) => {
    groupList({ user_id: user_id.value, group_id: group_id, group_name: group_name, pagenumber: paginnation.value.currentPage, pagesize: paginnation.value.pageSize }).then((res: any) => {
        if (res.code == 200) {
            let defaultGroup = res.data.data.filter((item: any) => item.group_name === t('facilityGroup.defaultGroup'));
            let otherGroups = res.data.data.filter((item: any) => item.group_name !== t('facilityGroup.defaultGroup'));
            tableData.value = [...defaultGroup, ...otherGroups];
            paginnation.value.total = res.data.total;
        }
    })
}
/**
 * 列表编辑
 */
let editGroupClick = (e: any) => {
    createGroupConfig.value.title = t('facilityGroup.editDeviceGroup')
    objValue.value = e
    isOk.value = true
}
let handleClose = (e: any) => {
    getList({});
    isOk.value = false
}
/**
 * 分页显示
 */
let currentChange = (e: any) => {
    paginnation.value.currentPage = e;
    getList({});
}
/**
 * 详情
 */
let goDetail = (e: any) => {
    router.push({
        path: '/facilityGroup/ziGroup',
        query: {
            group_id: e.group_id,
            user_id: user_id.value,
            // 把整个对象转成 JSON 字符串
            rowData: JSON.stringify(e)
        }
    })
}
/**
 * 保存设备分组排序
 */
let add = () => {
    let targetData = tableData.value.map((item: any) => ({
        group_id: item.group_id,        // 保留分组ID
        group_order: item.group_order// 合并的排序字段
    }));
    save_group_order({ user_id: user_id.value, order_list: targetData }).then((res: any) => {
        if (res.code == 200) {
            ElMessage.success(res.msg);
            getList({});
        } else {
            ElMessage.error(res.msg);
        }
    })
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
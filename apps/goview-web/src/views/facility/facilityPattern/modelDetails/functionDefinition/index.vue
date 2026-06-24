<template>
    <div class="functionDefinition">
        <div class="functionDefinitionTop">
            <div style="font-size: 18px;font-weight: bold;">{{ t('facilityPattern.editFunctionDefinition') }}</div>
            <div style="font-size: 12px;padding: 0.5em 0 0 0;color: #827878;">
                {{ t('facilityPattern.functionDefinitionDesc') }}</div>
        </div>
        <div class="property">
            <div class="property_top">
                <div style="font-weight: bold; font-size: 18px;">{{ t('facilityPattern.property') }}</div>
                <div><el-button type="primary" icon="Plus" @click="addProperty('add')">{{ t('facilityPattern.addProperty') }}</el-button></div>
            </div>
            <!-- 表格 -->
            <div>
                <el-table :data="tableData" style="width: 100%">
                    <el-table-column prop="property_name" :label="t('facilityPattern.propertyName')" align="center" />
                    <el-table-column prop="identifier" :label="t('facilityPattern.propertyIdentifier')" align="center" />
                    <el-table-column prop="data_type" :label="t('facilityPattern.dataType')" align="center" />
                    <!-- 扩展信息列：渲染为完整的字符串格式 -->
                    <el-table-column :label="t('facilityPattern.extendedInfo')" align="center" show-overflow-tooltip
                        :formatter="(row: any) => formatExtendedInfo(row.extended_info)" />
                    <el-table-column :label="t('facilityPattern.operation')" width="200" align="center">
                        <template #default="scope">
                            <el-button link type="primary" size="small" style="color: #7FA2F1;"
                                @click="edit(scope.row, 'edit')">{{ t('facilityPattern.edit') }}</el-button>
                            <el-button link type="primary" size="small" style="color: red;"
                                @click="dele(scope.row)">{{ t('facilityPattern.delete') }}</el-button>
                        </template>
                    </el-table-column>
                </el-table>
            </div>
            <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
                <el-pagination v-model:current-page="pagenumber" v-model:page-size="pagesize"
                    :page-sizes="[5, 10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next" :total="total"
                    @size-change="handleSizeChange" @current-change="handleCurrentChange" />
            </div>
        </div>
        <div class="incident" style="margin-bottom: 2rem;">
            <div class="incident_top">
                <div style="font-weight: bold; font-size: 18px;">{{ t('facilityPattern.event') }}</div>
                <div><el-button type="primary" icon="Plus" @click="addIncident">{{ t('facilityPattern.addEvent') }}</el-button></div>
            </div>
            <!-- 表格 -->
            <div>
                <el-table :data="incidentTableData" style="width: 100%">
                    <el-table-column prop="event_name" :label="t('facilityPattern.eventName')" align="center" />
                    <el-table-column prop="identifier" :label="t('facilityPattern.eventIdentifier')" align="center" />
                    <el-table-column prop="data_type" :label="t('facilityPattern.eventParameters')" width="650" align="center" show-overflow-tooltip>
                        <template #default="{ row }">
                            {{
                                row.data_type
                                    ? JSON.stringify(row.data_type).replace(/"([^"]+)":/g, '$1:')
                                    : ''
                            }}
                        </template>
                    </el-table-column>
                    <el-table-column :label="t('facilityPattern.operation')" width="200" align="center">
                        <template #default="scope">
                            <el-button link type="primary" size="small" style="color: #7FA2F1;"
                                @click="editIncident(scope.row)">{{ t('facilityPattern.edit') }}</el-button>
                            <el-button link type="primary" size="small" style="color: red;"
                                @click="deleTncident(scope.row)">{{ t('facilityPattern.delete') }}</el-button>
                        </template>
                    </el-table-column>
                </el-table>
            </div>
            <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
                <el-pagination v-model:current-page="incidentPagenumber" v-model:page-size="incidentPagesize"
                    :page-sizes="[5, 10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next"
                    :total="incidentTotal" @size-change="incidentHandleSizeChange"
                    @current-change="incidentHandleCurrentChange" />
            </div>
        </div>
        <!-- 添加属性弹窗 -->
        <propertyDialog v-if="isShow" @handleClose="handleClose" :title="title" :action="action" :model_id="props.id"
            :model_name="props.model_name" :propertyObj="propertyObj"></propertyDialog>
        <!-- 添加事件弹窗 -->
        <addIncidentDialog v-if="isIncident" @handleIncidentClose="handleIncidentClose" :title="title" :action="action"
            :model_id="props.id" :model_name="props.model_name" :IncidenObj="IncidenObj">
        </addIncidentDialog>
    </div>
</template>
<script lang="ts" setup>
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import propertyDialog from './propertyDialog.vue'
import addIncidentDialog from './addIncidentDialog.vue'

const { t } = useI18n()
import { get_property, model_event_list, delete_event, delete_property } from '@/api/facilityPattern/index'
const props = defineProps({
    id: {
        type: String,
        required: true
    },
    model_name: {
        type: String,
        required: true
    },
});
import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let tableData = ref([])// 属性表格数据
let incidentTableData = ref([]) // 事件表格数据
let isShow = ref(false) // 添加属性弹窗
let isIncident = ref(false) // 添加事件弹窗
let title = ref('') // 弹窗名字
let action = ref('') // 区分添加跟编辑
let propertyObj = ref({}) // 修改属性数据
let IncidenObj = ref({}) // 修改事件数据
let pagenumber = ref(1)// 属性分页 页
let pagesize = ref(5)// 属性分页 条
let total = ref(0)// 属性总条数
let incidentPagenumber = ref(1)// 事件分页 页
let incidentPagesize = ref(5)// 事件分页 条
let incidentTotal = ref(0)// 事件总条数
onMounted(() => {
    get_propertyList()
    IncidentList()
})
// 属性扩展信息处理
let formatExtendedInfo = (obj: any) => {
    if (!obj) return '';
    const parts = [];
    for (const key in obj) {
        if (obj.hasOwnProperty(key)) {
            // 数字键保持数字形式，字符串键直接使用
            const keyStr = isNaN(Number(key)) ? key : Number(key);
            // 值用双引号包裹
            const valueStr = `"${obj[key]}"`;
            parts.push(`${keyStr}: ${valueStr}`);
        }
    }
    return `{${parts.join(', ')}}`;
}
/**
 * 功能属性列表
 */
let get_propertyList = () => {
    get_property({ user_id: user_id.value, mode_id: props.id, pagenumber: pagenumber.value, pagesize: pagesize.value }).then((res: any) => {
        if (res.code == 200) {
            // 预处理每个行的 extended_info
            tableData.value = res.data.data.map((row: any) => ({
                ...row,
                extendedInfoStr: formatExtendedInfo(row.extended_info)
            }))
            total.value = res.data.total
        }
    })
}
/**
 * 功能事件列表
 */
let IncidentList = () => {
    model_event_list({ user_id: user_id.value, mode_id: props.id, pagenumber: incidentPagenumber.value, pagesize: incidentPagesize.value }).then((res: any) => {
        if (res.code == 200) {
            incidentTableData.value = res.data.data
            incidentTotal.value = res.data.total
        }
    })
}
// 添加属性
let addProperty = (e: any) => {
    if (e == 'add') {
        title.value = t('facilityPattern.addProperty')
        action.value = 'add'
        isShow.value = true
    }
}
// 修改属性
let edit = (val: any, e: any) => {
    if (e == 'edit') {
        title.value = t('facilityPattern.editProperty')
        action.value = 'edit'
        propertyObj.value = val
        isShow.value = true
    }

}
// 删除功能属性
let dele = (e: any) => {
    ElMessageBox.confirm(t('facilityPattern.deleteProperty'), t('facilityPattern.tip'), {
        confirmButtonText: t('facilityPattern.confirm'),
        cancelButtonText: t('facilityPattern.cancel'),
        type: 'warning',
    }).then(() => {
        delete_property({ user_id: user_id.value, model_id: props.id, identifier: e.identifier }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                get_propertyList()
                IncidentList()
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}
// 添加属性取消弹窗
let handleClose = (e: any) => {
    isShow.value = false
    get_propertyList()
}
// 添加事件弹窗
let addIncident = () => {
    title.value = t('facilityPattern.addEvent')
    action.value = 'add'
    isIncident.value = true
}
// 修改事件弹窗
let editIncident = (e: any) => {
    title.value = t('facilityPattern.editEventInfo')
    action.value = 'edit'
    IncidenObj.value = e
    isIncident.value = true
}
// 取消事件弹窗
let handleIncidentClose = (e: any) => {
    isIncident.value = false
    IncidentList()
}
// 属性分页 条
let handleSizeChange = (val: number) => {
    get_propertyList()
}
// 属性分页 页
let handleCurrentChange = (val: number) => {
    pagenumber.value = val
    get_propertyList()
}
// 事件分页 条
let incidentHandleSizeChange = (val: number) => {
    IncidentList()
}
// 事件分页 页
let incidentHandleCurrentChange = (val: number) => {
    incidentPagenumber.value = val
    IncidentList()
}
// 事件删除
let deleTncident = (e: any) => {
    ElMessageBox.confirm(t('facilityPattern.deleteEvent'), t('facilityPattern.tip'), {
        confirmButtonText: t('facilityPattern.confirm'),
        cancelButtonText: t('facilityPattern.cancel'),
        type: 'warning',
    }).then(() => {
        delete_event({ user_id: user_id.value, model_id: props.id, identifier: e.identifier }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                get_propertyList()
                IncidentList()
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}

</script>
<style lang="scss" scoped>
.functionDefinition {
    padding: 0 0.5rem 0 1.2rem;
    overflow: auto;
    height: 50vh;

    .property {
        margin-top: 2rem;

        .property_top {
            display: flex;
            justify-content: space-between;
            margin: 0 0 0.5rem 0;
        }
    }

    .incident {
        margin-top: 2rem;

        .incident_top {
            display: flex;
            justify-content: space-between;
            margin: 0 0 0.5rem 0;
        }
    }

    :deep(.el-table th.el-table__cell) {
        background-color: #f5f7fa;
    }
}
</style>
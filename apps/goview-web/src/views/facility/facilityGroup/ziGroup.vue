<template>
    <div class="ziGroup">
        <el-card>
            <div style="padding: 0 0 0.9rem 0;font-size: 14px;display: flex;align-items: center;width: 4rem"  @click="goBack">
                <el-icon>
                    <ArrowLeft />
                </el-icon><span style="padding: 0 0 0 0.3rem;cursor: pointer;">{{ t('facilityGroup.back') }}</span>
            </div>
            <div :query="groupInfo" style="display: flex;width: 100%;align-items: center;">
                <div style="margin-left: 0.2rem;">
                    <div style="display: flex;align-items: center;font-weight: 700;font-size: 1.3rem;">{{ t('facilityGroup.deviceGroup') }} {{ t('facilityGroup.details') }}</div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityGroup.groupName') }}：</p>
                        <p style="color: #7E7779;">{{ groupInfo.group_name }}</p>
                        <el-icon style="margin-left: 0.5rem;cursor: pointer;" :size="20"
                            v-if="groupInfo.group_name != t('facilityGroup.defaultGroup')"
                            @click="createGroupConfig = { title: t('facilityGroup.editDeviceGroup'), isTrue: true, type: 1, list: groupInfo }">
                            <Edit />
                        </el-icon>
                    </div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityGroup.groupDescription') }}：</p>
                        <p style="color: #7E7779;">{{ groupInfo.parent_id ? groupInfo.parent_id : t('facilityGroup.parentGroup') }}</p>
                    </div>
                </div>
                <div style="margin-left: 5rem;">
                    <div style="display: flex;align-items: center;">&nbsp;</div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityGroup.deviceCount') }}：</p>
                        <p style="color: #7E7779;">{{ groupInfo.device_count }}</p>
                    </div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityGroup.groupDescription') }}：</p>
                        <p style="color: #7E7779;">{{ groupInfo.group_description }}</p>
                    </div>
                </div>
                <div style="margin-left: 5rem;">
                    <div style="display: flex;align-items: center;">&nbsp;</div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityGroup.parentGroup') }}：</p>
                        <p style="color: #7E7779;">{{ groupInfo.admin_name }}</p>
                    </div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityGroup.createTime') }}：</p>
                        <p style="color: #7E7779;">{{ groupInfo.create_time }}</p>
                    </div>
                </div>
            </div>
        </el-card>
        <createGroup :config="createGroupConfig" :obj="listObj.group_name" :group_id="listObj.group_id"
            @refaer="list(editableTabsValue); listGroup();"></createGroup>
        <div style="background-color: #ffffff;height: 79%;border-radius:10px;">
            <el-tabs v-model="editableTabsValue" closable @tab-remove="removeTab" style="margin: 1rem;padding: 1rem 0;"
                @tab-click="handleClick">
                <el-tab-pane v-for="item in editableTabs" :key="item.group_id" :label="item.group_name"
                    :name="item.group_id" :closable="item.group_name !== t('facilityGroup.defaultGroup') && item.group_id !== listObj.group_id">
                    <div style="display: flex;justify-content: space-between;">
                        <div>
                            <el-button icon="Plus" type="warning"
                                @click="createGroupConfig = { title: t('facilityGroup.addDeviceGroup'), isTrue: true, type: 3, list: {} }">{{ t('facilityGroup.createSubGroup') }}</el-button>
                            <el-button icon="Plus" type="primary" @click="AssociateDevice">{{ t('facilityGroup.selectDevice') }}</el-button>
                            <el-button icon="Delete" type="danger" @click="cutoffChange">{{ t('facilityGroup.unbindDevice') }}</el-button>
                        </div>
                        <el-form :inline="true" :model="formInline"
                            style="display: flex;justify-content: space-between;">
                            <el-form-item style="width: 7rem;">
                                <el-select v-model="formInline.facilityType" :placeholder="t('facilityGroup.status')" clearable>
                                    <el-option :label="t('facilityGroup.online')" :value="1" />
                                    <el-option :label="t('facilityGroup.offline')" :value="0" />
                                </el-select>
                            </el-form-item>
                            <el-form-item style="width: 15rem;">
                                <el-input v-model="formInline.keyword" :placeholder="t('facilityGroup.pleaseSelectDevice')" clearable />
                            </el-form-item>
                            <el-button icon="Search" type="primary" @click="onSubmit">{{ t('facilityGroup.query') }}</el-button>
                        </el-form>
                    </div>
                    <el-table :data="tableData" :height="heightCount()" border
                        @selection-change="handleSelectionChange">
                        <el-table-column type="selection" width="55" align="center" />
                        <el-table-column prop="dev_name" :label="t('facilityGroup.deviceName')" align="center" />
                        <el-table-column prop="dev_sn" :label="t('facilityGroup.deviceSerial')" align="center" />
                        <el-table-column prop="dev_status" :label="t('facilityGroup.status')" align="center">
                            <template #default="scope"><span
                                    :style="scope.row.type === '1' ? 'color: #51C873' : 'color: #B2B2B2'">{{
                                        scope.row.dev_status
                                            == '1' ? t('facilityGroup.online') : t('facilityGroup.offline') }}</span></template>
                        </el-table-column>
                        <el-table-column prop="create_time" :label="t('facilityGroup.createTime')" align="center"
                            :formatter="((row: any) => itializeUtc(row.create_time))" />
                    </el-table>
                    <div style="display: flex;justify-content: flex-end;margin: 0.5rem 0 3rem 0;">
                        <el-pagination v-model:current-page="page" v-model:page-size="size"
                            :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next"
                            :total="total" @size-change="handleSizeChange" @current-change="handleCurrentChange" />
                    </div>
                </el-tab-pane>
            </el-tabs>
        </div>
        <correlation v-if="correlationConfig.isTrue" :config="correlationConfig" :group_id="listObj.group_id"
            :reset="reset" :groupId="editableTabsValue" :label="label" @refreshList="refreshList"
            @handleClose="handleClose">
        </correlation>
    </div>
</template>

<script lang="ts" setup>
import createGroup from "./createGroup.vue";
import correlation from "./correlation.vue";
import { groupList, delete_devicegroup } from "@/api/facilityList/index";
import { model_device_list, group_info, unlink_device, batch_unlink_devices } from '@/api/facilityGroup/index'
import useCounterStore from "@/stores/counter";
import { itializeUtc } from "@/utils/publicFun";
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let createGroupConfig = ref({ type: 0, title: "", isTrue: false, list: {} });//新建设备分组弹窗信息 type 0:新建 1:编辑
let correlationConfig = ref({ title: "", isTrue: false });//关联设备
let editableTabs: any = ref([]);//子分组列表
let editableTabsValue = ref("");//默认选中哪个子分组
let formInline = ref({ facilityType: undefined, keyword: "" });//查询分组中的设备信息
let tableData = ref([])
let cutoffList = ref<Array<Object>>([]);//子设备表格选中的列表
let route = useRoute();
let router = useRouter()
// 分页 页
let page = ref(1)
// 分页 条
let size = ref(10)
// 总条数
let total = ref(0)
let reset = ref(false)
let listObj = ref(JSON.parse(route.query.rowData as string))
let label = ref('')
let group_id = ref("");
let groupInfo = ref({ group_name: "", device_count: "", group_description: "", parent_id: "", admin_name: "", create_time: "" });//分组详情信息

onMounted(() => {
    list(listObj.value.group_id);
    listGroup();
})

/**
 * 查询分组详情
 */
let listGroup = () => {
    group_info({ group_id: route.query.group_id, user_id: user_id.value }).then((res: any) => {
        groupInfo.value = res.data;
    })
}

/**
 * 查询分组
 */

let list = (e: any) => {
    group_id.value = e
    groupList({ group_id: route.query.group_id, user_id: user_id.value, group_name: '' }).then((res: any) => {
        if (res.code == 200) {
            editableTabs.value = res.data.data;
            const isExist = editableTabs.value.some((item: any) => item.group_id === listObj.value.group_id)

            // 如果不存在，就添加到数组的第一条位置b0378fe424ba4ed397b22d2ab975b8f2
            if (!isExist && Object.keys(listObj.value).length > 0) {
                editableTabs.value.unshift(listObj.value) // unshift 是添加到开头，push 是添加到末尾
            }
            // 设置默认激活的标签
            // if (editableTabs.value.length > 0 ) {
            editableTabsValue.value = e;
            // }
        }
    })
    model_device_list({
        user_id: user_id.value,
        pagenumber: page.value,
        pagesize: size.value,
        dev_status: Number(formInline.value.facilityType),
        keyword: formInline.value.keyword,
        query_type: "group",
        query_id: e
    }).then((res: any) => {
        if (res.code == 200) {
            tableData.value = res.data.devices
            total.value = res.data.total
        }
    })
}

// 返回
let goBack = () => {
    // router.go(-1)
    router.push('/facilityGroup')
}
// 查询
let onSubmit = () => {
    list(editableTabsValue.value)
}

/**
 * 根据分辨率计算表格应该显示的高度
 */
let heightCount = () => {
    return window.screen.height >= 1000 ? window.screen.height * 0.4 :
        window.screen.height < 1000 && window.screen.height >= 900 ? window.screen.height * 0.25 :
            window.screen.height < 900 && window.screen.height >= 700 ? window.screen.height * 0.15 :
                null;
}

/**
 * 删除子分组
 */
let removeTab = (targetName: any) => {
    let title = editableTabs.value.find((item: any) => item.group_id == targetName);
    if (!title) return;
    if (title.group_name == t('facilityGroup.defaultGroup') || targetName == listObj.value.group_id) {
        ElMessage.warning(t('facilityGroup.defaultGroupCannotDelete'));
        return false;
    }
    console.log(title.group_name);
    ElMessageBox.confirm(`${t('facilityGroup.confirmDelete')}${title.group_name || 'Unknown'}?`, t('facilityGroup.confirmTitle'), {
        confirmButtonText: t('facilityGroup.confirm'),
        cancelButtonText: t('facilityGroup.cancel'),
        type: 'warning',
    }).then(() => {
        delete_devicegroup({ group_id: targetName, user_id: user_id.value }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                list(route.query.group_id);
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}

/**
 * 子设备多选表格选中的子设备
 */
let handleSelectionChange = (e: any) => {
    cutoffList.value = e.map((item: any) => item.dev_id)
}

/**
 * 批量移除
 */
let cutoffChange = () => {
    ElMessageBox.confirm(t('facilityGroup.confirmUnbind'), t('facilityGroup.confirmTitle'), {
        confirmButtonText: t('facilityGroup.confirm'),
        cancelButtonText: t('facilityGroup.cancel'),
        type: 'warning',
    }).then(() => {
        batch_unlink_devices({ dev_ids: cutoffList.value, user_id: user_id.value, group_id: group_id.value }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                list(editableTabsValue.value)
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}

let handleClick = (tab: any) => {
    editableTabsValue.value = tab.props.name
    label.value = tab.props.label
    list(editableTabsValue.value)
}

let refreshList = (e: any) => {
    list(e)
}
/**
 * 取消弹窗
 */
let handleClose = () => {
    correlationConfig.value.isTrue = false
}
/**
 * 关联设备弹窗
 */
let AssociateDevice = () => {
    correlationConfig.value.title = t('facilityGroup.selectDevice'),
        correlationConfig.value.isTrue = true
}
/**
 * 单个删除
 */
let dele = (e: any) => {
    console.log(e.dev_id,'e.dev_id',group_id.value)
    console.log(listObj.value.group_id,'listObj.value.group_id')
    ElMessageBox.confirm(t('facilityGroup.confirmUnbind'), t('facilityGroup.confirmTitle'), {
        confirmButtonText: t('facilityGroup.confirm'),
        cancelButtonText: t('facilityGroup.cancel'),
        type: 'warning',
    }).then(() => {
        unlink_device({ dev_id: e.dev_id, user_id: user_id.value, group_id: group_id.value }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                list(editableTabsValue.value)
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}

// 分页 条
let handleSizeChange = (val: number) => {
    list(editableTabsValue.value)
}
// 分页 页
let handleCurrentChange = (val: number) => {
    page.value = val
    list(editableTabsValue.value)
}
</script>

<style lang="scss" scoped>
.ziGroup {
    background-color: rgb(227, 227, 227);
    height: 100%;
}
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
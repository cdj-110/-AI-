<template>
    <div style="background-color: #ffffff;height: 100%;">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus"
                    @click="config = { isTrue: true, title: t('largeScreen.addLargeScreen'), list: {} }">{{
                        t('largeScreen.addLargeScreen') }}</el-button>
            </template>
            <template #content>
                <div style="display: flex;flex-wrap: wrap;">
                    <div v-for="(item, index) in screenList" :key="index"
                        style="width: calc(100% / 5 - 1.2rem);border: #409eff 1px solid;border-radius: 10px;margin: 1rem 0.3rem;">
                        <img src="@/assets/qwe.jpg" style="width: 100%;border-radius: 10px 10px 0 0;">
                        <p style="font-size: 0.9rem;font-weight: 500;padding: 0.7rem;">{{ item.config_name }}</p>
                        <div class="configClass">
                            <p @click="proviewChange(item)">{{ t('largeScreen.preview') }}</p>
                            <p
                                @click="config = { isTrue: true, title: t('largeScreen.editLargeScreen'), list: item }; form.title = item.config_name">
                                {{ t('largeScreen.edit') }}</p>
                            <p @click="planChange(item)">{{ t('largeScreen.design') }}</p>
                            <p @click="copyClick(item)">{{ t('largeScreen.copy') }}</p>
                            <el-popover placement="bottom" trigger="click" :teleported="false">
                                <template #reference>
                                    <p style="border-radius: 5px;cursor: pointer;font-size: 0.9rem;"><el-icon>
                                            <More />
                                        </el-icon></p>
                                </template>
                                <div class="optionClass">
                                    <p v-if="item.on_workbench == 0" @click="createWorkChange(item)">{{
                                        t('largeScreen.addToWorkbench') }}</p>
                                    <p v-if="item.on_workbench == 1" @click="createWorkChange(item)">{{
                                        t('largeScreen.removeFromWorkbench') }}</p>
                                    <p @click="verTionChange(item)">{{ t('largeScreen.delete') }}</p>
                                </div>
                            </el-popover>
                        </div>
                    </div>
                </div>
            </template>
        </facilityHeader>
        <el-dialog v-model="config.isTrue" :title="config.title" width="30%" :close-on-press-escape="false"
            :close-on-click-modal="false" align-center>
            <el-form :model="form">
                <el-form-item :label="t('largeScreen.largeScreenName')">
                    <el-input v-model="form.title" @keyup.enter="createList()"
                        :placeholder="t('largeScreen.pleaseEnterName')" />
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="config.isTrue = false">{{ t('largeScreen.cancel') }}</el-button>
                <el-button type="primary" @click="createList()">{{ JSON.stringify(config.list) == "{}" ?
                    t('largeScreen.create') : t('largeScreen.edit') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import router from '@/router';
import { apiV1ConfigCatalog, apiconfConfigCatalogRename, getApiV1ConfigCatalog, apiV1ConfigDelete, configCatalogWorkbenchPin, configCatalogWorkbenchUnpin, copy, search } from "@/api/configuration/index";
import useCounterStore from "@/stores/counter";
import screenRootFun from "./hooks/index";
import { luyou } from '@/utils/request';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

let { screenList, getList } = screenRootFun();

let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let config: any = ref({ isTrue: false, title: "", list: {} });//弹窗配置
let form = ref({ title: "" });//编辑大屏名称的表单

/**
 * 顶部公用搜索头
 */
let facilityConfig = () => ({
    title: t('largeScreen.pageTitle'),
    search: [
        {
            fields: "config_name",
            type: "input",
            placeholder: t('largeScreen.searchPlaceholder')
        },
        {
            fields: "",
            type: "button",
            label: t('largeScreen.search'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                search({ user_id: user_id.value, config_name: e.config_name }).then((res: any) => {
                    screenList.value = res
                    ElMessage.success(t('largeScreen.querySuccess'));
                })
            }
        },
    ]
});

onMounted(() => {
    getList({ config_name: "" });
})

/**
 * 添加数据大屏
 */
let createList = () => {
    config.value.isTrue = false;
    let reuest = JSON.stringify(config.value.list) == "{}" ? apiV1ConfigCatalog : apiconfConfigCatalogRename;
    reuest({ user_id: user_id.value, config_name: form.value.title, new_config_name: form.value.title, config_id: config.value.list.config_id }).then((res: any) => {
        form.value.title = "";
        ElMessage.success(JSON.stringify(config.value.list) == "{}" ? t('largeScreen.addSuccess') : t('largeScreen.editSuccess'));
        getList({ config_name: "" });
    }).catch(err => {
        ElMessage.error(err.response.data.detail);
    })
}

/**
 * 删除数据大屏
 */
let verTionChange = (e: any) => {
    ElMessageBox.confirm(`${t('largeScreen.deleteConfirm')} [${e.config_name}]?`, t('largeScreen.deleteTitle'), {
        confirmButtonText: t('largeScreen.confirmDelete'),
        cancelButtonText: t('largeScreen.cancelDelete'),
        closeOnClickModal: false,
        closeOnPressEscape: false,
    }).then(ver => {
        apiV1ConfigDelete({ user_id: user_id.value, config_id: e.config_id }).then((res: any) => {
            ElMessage.success(res.message);
            getList({ config_name: "" });
        }).catch((err: any) => {
            ElMessage.warning(err.response.data.detail);
        })
    }).catch(() => { })
}

/**
 * 添加到工作台
 */
let createWorkChange = (e: any) => {
    let request = e.on_workbench == 0 ? configCatalogWorkbenchPin : configCatalogWorkbenchUnpin;
    request({ user_id: user_id.value, config_id: e.config_id }).then((res: any) => {
        if (res.success == true) {
            e.on_workbench == 0 ? ElMessage.success(res.message) : ElMessage.warning(res.message);
            getList({ config_name: "", type: 1 });
        }
    })
}

/**
 * 打开大屏设计器
 */
let planChange = (e: any) => {
    let page = router.resolve({ path: `/screenRoot`, query: {}, });
    window.open(`${page.href}?config_id=${e.config_id}&mode=design&config_name=${e.config_name}`, "_blank");
}

/**
 * 预览
 */
let proviewChange = (e: any) => {
    let { token } = sessionStorage.getItem("wk_Token") ? JSON.parse(sessionStorage.getItem("wk_Token") as any) : { token: void 0 };
    // window.open(`${luyou}/#/chart/preview/${e.config_id}?token=${token}&userId=${user_id.value}&config_id=${e.config_id}`, "_blank"); // 源代码
    //改动新代码 把 userId 改成 user_id，和预览页面的参数名完全一致
    window.open(`${luyou}/#/chart/preview/${e.config_id}?token=${token}&user_id=${user_id.value}&config_id=${e.config_id}&config_name=${e.config_name}`, '_blank');
}

/**
 * 复制
 */
let copyClick = (e: any) => {
    copy({ user_id: user_id.value, config_id: e.config_id }).then((res: any) => {
        getList({ config_name: "" });
        ElMessage.success(t('largeScreen.copySuccess'));
    })
}
</script>

<style lang="scss" scoped>
.configClass {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background-color: #f2f3f5;
    border-radius: 0 0 10px 10px;

    p {
        cursor: pointer;
        border-right: 1px solid #cccfe7;
        width: 100%;
        text-align: center;
        padding: 7px 0;
        font-size: 0.9rem;
    }
}

.optionClass {
    p {
        padding: 0.7rem 1rem;
        white-space: nowrap;

        &:hover {
            cursor: pointer;
            background-color: #eeeeee;
        }
    }
}

:deep(.el-popover.el-popper) {
    padding: 0;
}
</style>

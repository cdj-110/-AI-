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
                                    <p @click="handleShare(item)">{{ t('largeScreen.share') }}</p>
                                    <p @click="verTionChange(item)" style="color: #f56c6c;">{{ t('largeScreen.delete')
                                        }}</p>
                                </div>
                            </el-popover>
                        </div>
                    </div>
                    <el-empty v-if="screenList.length === 0" description="暂无数据" style="width: 100%; margin-top: 3rem;" />
                </div>
            </template>
        </facilityHeader>

        <!-- 新增/编辑大屏弹窗 -->
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

        <!-- 分享大屏弹窗 -->
        <el-dialog v-model="showShareModal" :title="t('largeScreen.share')" width="480px" :close-on-press-escape="false"
            :close-on-click-modal="false" align-center>
            <div v-if="shareLoading" style="text-align: center; padding: 40px 0;">
                <el-icon size="40" class="is-loading">
                    <Loading />
                </el-icon>
                <p style="margin-top: 16px; color: #666;">{{ t('largeScreen.generatingShareLink') }}</p>
            </div>
            <div v-else>
                <div style="display: flex; gap: 12px; margin: 20px 0;">
                    <el-input v-model="shareUrl" readonly placeholder="分享链接" style="flex: 1;" />
                    <el-button @click="handlePreviewInShare" :disabled="shareLoading">{{ t('largeScreen.preview')
                        }}</el-button>
                    <el-button type="primary" @click="copyPublicChange(shareUrl)" :disabled="shareLoading">{{
                        t('largeScreen.copyLink') }}</el-button>
                </div>
                <!-- <p style="color: #999; font-size: 12px; text-align: center;">
                    {{ t('largeScreen.shareTip') }}
                </p> -->
            </div>
            <template #footer>
                <!-- 重新生成按钮，和关闭按钮左右对称 -->
                <el-button @click="handleRegenerateShare" :loading="shareLoading">{{ t('largeScreen.regenerate')
                    }}</el-button>
                <!-- 取消分享 -->
                <el-button @click="stopSharingClick" :loading="shareLoading" style="margin-left:  0.3rem;">{{ t('largeScreen.stopSharing')
                    }}</el-button>
                <span style="flex: 1;"></span>
                <el-button @click="showShareModal = false">{{ t('largeScreen.close') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { ref, onMounted } from 'vue';
import { storeToRefs } from 'pinia';
import router from '@/router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { More, Loading } from '@element-plus/icons-vue';
import { copyPublicChange } from "@/utils/publicFun";
import facilityHeader from "@/components/facilityHeader.vue";
// 预留：分享接口导入
// import { shareBigScreenApi } from "@/api/configuration/index";
import { apiV1ConfigCatalog, apiconfConfigCatalogRename, apiV1ConfigDelete, configCatalogWorkbenchPin, configCatalogWorkbenchUnpin, copy, search, create, revoke } from "@/api/configuration/index";
import useCounterStore from "@/stores/counter";
import screenRootFun from "./hooks/index";
import { luyou } from '@/utils/request';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

let { screenList, getList } = screenRootFun();

let store = useCounterStore();
let { user_id } = storeToRefs(store);
let config: any = ref({ isTrue: false, title: "", list: {} });
let form = ref({ title: "" });

// 分享相关状态
const showShareModal = ref(false);
const shareLoading = ref(false);
const shareUrl = ref('');
const currentShareItem = ref<any>(null);

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
    const page = router.resolve({
        path: `/screenRoot`,
        query: {
            config_id: e.config_id,
            mode: 'design',
            config_name: e.config_name
        }
    });
    window.open(page.href, "_blank");
}

/**
 * 预览（全局共用）
 */
let proviewChange = (e: any) => {
    let { token } = sessionStorage.getItem("wk_Token") ? JSON.parse(sessionStorage.getItem("wk_Token") as any) : { token: void 0 };
    window.open(`${luyou}/#/chart/preview/${e.config_id}?token=${token}&user_id=${user_id.value}&config_id=${e.config_id}&config_name=${encodeURIComponent(e.config_name)}`, '_blank');
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

/**
 * 生成分享链接（通用方法，首次生成和重新生成都调用）
 * 生成公开预览链接，不需要登录凭证
 */
const generateShareUrl = async () => {
    if (!currentShareItem.value) return;

    shareLoading.value = true;
    shareUrl.value = '';

    try {
        await new Promise(resolve => setTimeout(resolve, 800));
        let { token } = sessionStorage.getItem("wk_Token") ? JSON.parse(sessionStorage.getItem("wk_Token") as any) : { token: void 0 };
        // const baseUrl = window.location.origin + window.location.pathname;
        const baseUrl = window.location.href.split('#')[0];
        console.log('baseUrl', baseUrl);
        let params = {
            user_id: user_id.value,
            config_id: currentShareItem.value.config_id,
            ttl_hours: '168',
            token: token
        }

        create(params).then((res: any) => {
            shareUrl.value = `${baseUrl}#/publicPreview/${currentShareItem.value.config_id}?share_id=${encodeURIComponent(res.share_id)}&config_name=${encodeURIComponent(currentShareItem.value.config_name || '')}`
        })
        // shareUrl.value = `${baseUrl}#/publicPreview/${currentShareItem.value.config_id}?token=${token}&user_id=${user_id.value}&config_id=${currentShareItem.value.config_id}&config_name=${encodeURIComponent(currentShareItem.value.config_name)}`;

        ElMessage.success(t('largeScreen.generateShareSuccess'));
    } catch (err: any) {
        ElMessage.error(t('largeScreen.generateShareFailed'));
    } finally {
        shareLoading.value = false;
    }
}

/**
 * 首次打开分享弹窗
 */
const handleShare = async (item: any) => {
    currentShareItem.value = item;
    showShareModal.value = true;
    await generateShareUrl();
}

/**
 * 重新生成分享链接
 * 调用同一个generateShareUrl方法，传完全一样的参数
 */
const handleRegenerateShare = async () => {
    await ElMessageBox.confirm(
        t('largeScreen.regenerateConfirmMessage'),
        t('largeScreen.regenerateConfirmTitle'),
        {
            confirmButtonText: t('largeScreen.confirm'),
            cancelButtonText: t('largeScreen.cancel'),
            type: 'warning'
        }
    ).then(async () => {
        await generateShareUrl();
    }).catch(() => { });
}
/**
 * 取消分享
 */
let stopSharingClick = () => {
    console.log(currentShareItem.value.config_id, 'currentShareItem.value');
    revoke({ user_id: user_id.value, config_id: currentShareItem.value.config_id }).then((res: any) => {
        shareUrl.value = ''
        ElMessage.success(t('largeScreen.copySuccess'));
    })
}
/**
 * 分享弹窗内预览
 */
const handlePreviewInShare = () => {
    if (currentShareItem.value) {
        proviewChange(currentShareItem.value);
    }
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

:deep(.el-button+.el-button) {
    margin-left: 0;
}

/* 优化弹窗footer布局，让重新生成和关闭按钮左右分开 */
:deep(.el-dialog__footer) {
    display: flex;
    justify-content: space-between;
    align-items: center;
}
</style>

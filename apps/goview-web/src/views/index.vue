<template>
    <div class="index" ref="enlargeRef">
        <el-menu mode="horizontal" :ellipsis="false">
            <el-menu-item index="0">
                <img style="width: 7rem;" src="@/assets/logos.png">
            </el-menu-item>
            <el-menu-item index="4">
                <a style="font-size: 1rem;text-decoration: none;font-weight: 450;" target="_blank"
                    href="https://www.yuque.com/jiayou-4wqsy/px7zph?#">
                    {{ $t('common.documentation') }}
                </a>
            </el-menu-item>

            <el-menu-item index="2">
                <el-dropdown ref="dropdown1" trigger="click" placement="bottom-end" @command="handleCommand">
                    <span class="dropdown-trigger">
                        <el-badge :value="3" class="item">
                            <img src="@/assets/tongzhi.png"
                                style="width: 1.7rem; vertical-align: middle; margin-left: 5px;">
                        </el-badge>
                    </span>
                    <template #dropdown>
                        <el-dropdown-menu>
                            <el-dropdown-item command="1">
                                <el-badge is-dot class="item"> {{ $t('common.alarmNotification') }} </el-badge>
                            </el-dropdown-item>
                            <el-dropdown-item command="2">{{ $t('common.systemMessage') }}</el-dropdown-item>
                        </el-dropdown-menu>
                    </template>
                </el-dropdown>
            </el-menu-item>
            <!-- 头像 -->
            <!-- <el-menu-item index="1">
                <img src="@/assets/user.png" style="width: 2rem;border-radius: 50%;" v-if="!img"></img>
                <img :src="'data:image/png;base64,' + img" style="width: 2rem;border-radius: 50%;" v-else />
            </el-menu-item> -->
             <el-menu-item index="5">
                <el-dropdown @command="handleLocaleChange">
                    <span class="lang-switcher">
                        <el-icon>
                            <Setting />
                        </el-icon>
                        <span class="lang-text">{{ currentLocaleLabel }}</span>
                    </span>
                    <template #dropdown>
                        <el-dropdown-menu>
                            <el-dropdown-item command="zh-CN" :disabled="appStore.locale === 'zh-CN'">
                                中文简体
                            </el-dropdown-item>
                            <el-dropdown-item command="en-US" :disabled="appStore.locale === 'en-US'">
                                English
                            </el-dropdown-item>
                        </el-dropdown-menu>
                    </template>
                </el-dropdown>
            </el-menu-item>
            <el-sub-menu index="3">
                <template #title>{{ nameValue }}</template>
                <el-menu-item index="3-1"
                    @click="accountFitConfig = { isTrue: true, title: $t('common.accountSettings') };">
                    {{ $t('common.accountSettings') }}
                </el-menu-item>
                <el-menu-item index="3-2" @click="outputChange">
                    {{ $t('common.logout') }}
                </el-menu-item>
            </el-sub-menu>
        </el-menu>
        <el-row>
            <el-col :span="3">
                <layout></layout>
            </el-col>
            <el-col :span="21">
                <div
                    style="height: var(--page-height);background-color: #eeeeee;padding: 0.7rem 0.5rem 0 0.5rem;overflow: hidden;">
                    <el-breadcrumb separator="/">
                        <el-breadcrumb-item><el-icon>
                                <HomeFilled />
                            </el-icon>{{ $t('menu.home') }}</el-breadcrumb-item>
                        <el-breadcrumb-item v-for="(item, index) in technical" :key="index">{{ getBreadcrumbTitle(item.title)
                        }}</el-breadcrumb-item>
                    </el-breadcrumb>
                    <div style="margin-top: 1rem;height: var(--content-height);">
                        <router-view :key="appStore.locale"></router-view>
                    </div>
                </div>
            </el-col>
        </el-row>
        <accountFit :config="accountFitConfig"></accountFit>
        <alertMessage v-if="alertValue === '1'" @alarmHistoryClose="alarmHistoryClose"></alertMessage>
        <system v-if="alertValue === '2'" @alarmHistoryClose="alarmHistoryClose"></system>
    </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { Setting } from "@element-plus/icons-vue";
import { HomeFilled } from "@element-plus/icons-vue";
import layout from "@/components/layout.vue";
import useCounterStore from "@/stores/counter";
import { useAppStore } from "@/stores/usePinia"; //  导入我们在Pinia中定义的全局状态
import accountFit from "@/views/userOrganize/accountFit/index.vue";
import alertMessage from '@/views/warning/alertMessage/index.vue'
import system from '@/views/dataCenter/system/index.vue'
import { logout } from '@/api/login/index'
import { ElMessage, ElMessageBox } from "element-plus";
import { storeToRefs } from "pinia";

const store = useCounterStore();
const { technical } = storeToRefs(store);
const { setToken } = useCounterStore();
const enlargeRef = ref("");
const accountFitConfig = ref({ isTrue: false, title: "" });
const alertValue = ref('');
const useStore = useCounterStore();
const { nameValue, vxImg } = storeToRefs(useStore);
const img = ref('');
const { locale: i18nLocale, t } = useI18n();
const appStore = useAppStore(); //  使用Pinia全局状态

const currentLocaleLabel = computed(() => {
    return appStore.locale === 'zh-CN' ? '中文' : 'EN';
});

//  语言切换函数
const handleLocaleChange = (newLocale: string) => {
    // 同步更新i18n语言
    i18nLocale.value = newLocale;

    // 同步更新Pinia中的全局语言状态（自动同步所有Element Plus组件）
    appStore.changeLocale(newLocale);
};

onMounted(() => {
    img.value = vxImg.value;
    // 初始化i18n语言，保持和Pinia一致
    i18nLocale.value = appStore.locale;
})

/**
 * 退出登录
 */
const outputChange = () => {
    ElMessageBox.confirm(
        t('common.confirmLogout'),
        t('common.prompt'),
        {
            confirmButtonText: t('common.confirm'),
            cancelButtonText: t('common.cancel'),
            type: 'warning',
        }
    ).then(() => {
        logout().then((res: any) => {
            if (res.code == 200) {
                setToken("");
                nameValue.value = '';
                ElMessage.success(t('common.logoutSuccess'));
            }
        })
    })
}

/**
 * 告警通知取消弹窗
 */
const alarmHistoryClose = (e: any) => { alertValue.value = '' }

const handleCommand = (e: any) => {
    alertValue.value = e
}

const breadcrumbTitleMap: Record<string, string> = {
    '工作台': 'menu.workbench',
    '概览': 'menu.overview',
    '设备': 'menu.device',
    '设备列表': 'menu.deviceList',
    '设备模型': 'menu.deviceModel',
    '设备分组': 'menu.deviceGroup',
    '告警': 'menu.warning',
    '告警历史': 'menu.warningChronicle',
    '告警规则': 'menu.warningRule',
    '通知组': 'menu.warningGroup',
    '规则': 'menu.rules',
    '设备联动': 'menu.facilityLinkage',
    '定时任务': 'menu.taskPage',
    '云组态': 'menu.configuration',
    '数据大屏': 'menu.largeScreen',
    '数据中心': 'menu.dataCenter',
    '数据报表': 'menu.dataReport',
    '用户': 'menu.user',
    '账号管理': 'menu.userAccount',
    '组织架构': 'menu.userPlan',
    '角色权限': 'menu.userRole',
    '操作日志': 'menu.userLog'
};

const getBreadcrumbTitle = (title: string) => {
    const i18nKey = breadcrumbTitleMap[title];
    return i18nKey ? t(i18nKey) : title;
}
</script>

<style lang="scss" scoped>
.index {
    width: 100vw;
    height: 100vh;

    .el-menu--horizontal>.el-menu-item:nth-child(1) {
        margin-right: auto;
    }
}

// 语言切换组件样式
.lang-switcher {
    display: flex;
    align-items: center;
    cursor: pointer;
    color: var(--el-text-color-regular);
    font-size: 14px;
    transition: color 0.2s;

    &:hover {
        color: var(--el-color-primary);
    }

    .el-icon {
        margin-right: 4px;
        font-size: 16px;
    }

    .lang-text {
        font-weight: 500;
    }
}
</style>
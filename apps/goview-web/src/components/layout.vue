<template>
    <div class="layout">
        <el-menu :default-active="activeMenu" :default-openeds="openKeys" @select="handleMenuSelect">
            <template v-for="item in menuItems" :key="item.path">
                <!-- 父级菜单 -->
                <el-sub-menu :index="item.path" v-if="item.children">
                    <template #title>
                        <div style="display: flex;align-items:center;gap:8px">
                            <img :src="isParentActive(item) ? item.activeIcon : item.icon"
                                style="width:0.9rem;height:0.9rem">
                            <span>{{ t(item.i18nKey) }}</span>
                        </div>
                    </template>
                    <!-- 子级菜单 -->
                    <el-menu-item v-for="child in item.children" :key="child.path" :index="child.path">
                        {{ child.i18nKey ? t(child.i18nKey) : child.label }}
                    </el-menu-item>
                </el-sub-menu>

                <!-- 无子菜单 -->
                <el-menu-item :index="item.path" v-else>
                    {{ t(item.i18nKey) }}
                </el-menu-item>
            </template>
        </el-menu>
    </div>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import useCounterStore from "@/stores/counter";
import { storeToRefs } from 'pinia';
import { luyou } from '@/utils/request';

// 图片导入
import img1 from '@/assets/img/img1.png'
import img2 from '@/assets/img/img2.png'
import img3 from '@/assets/img/img3.png'
import img4 from '@/assets/img/img4.png'
import img5 from '@/assets/img/img5.png'
import img6 from '@/assets/img/img6.png'
import img7 from '@/assets/img/img7.png'
import img1Active from '@/assets/img/img1Active.png'
import img2Active from '@/assets/img/img2Active.png'
import img3Active from '@/assets/img/img3Active.png'
import img4Active from '@/assets/img/img4Active.png'
import img5Active from '@/assets/img/img5Active.png'
import img6Active from '@/assets/img/img6Active.png'
import img7Active from '@/assets/img/img7Active.png'

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const store = useCounterStore();
const { createWorkList, user_id } = storeToRefs(store);

const currentConfigPath = ref('');

// 菜单数据（使用i18n key）
const menuItems = ref([
    {
        i18nKey: 'menu.workbench',
        path: "/workPage",
        icon: img1,
        activeIcon: img1Active,
        children: []
    },
    {
        i18nKey: 'menu.device',
        path: "/facility",
        icon: img2,
        activeIcon: img2Active,
        children: [
            { i18nKey: 'menu.deviceList', path: "/facilityList" },
            { i18nKey: 'menu.deviceModel', path: "/facilityPattern" },
            { i18nKey: 'menu.deviceGroup', path: "/facilityGroup" }
        ]
    },
    {
        i18nKey: 'menu.warning',
        path: "/warning",
        icon: img3,
        activeIcon: img3Active,
        children: [
            { i18nKey: 'menu.warningChronicle', path: "/warningChronicle" },
            { i18nKey: 'menu.warningRule', path: "/warningRule" },
            { i18nKey: 'menu.warningGroup', path: "/warningGroup" }
        ]
    },
    {
        i18nKey: 'menu.rules',
        path: "/rulesPage",
        icon: img4,
        activeIcon: img4Active,
        children: [
            { i18nKey: 'menu.facilityLinkage', path: "/facilityLinkage" },
            { i18nKey: 'menu.taskPage', path: "/taskPage" }
        ]
    },
    {
        i18nKey: 'menu.configuration',
        path: "/configuration",
        icon: img5,
        activeIcon: img5Active,
        children: [
            { i18nKey: 'menu.largeScreen', path: "/largeScreen" }
        ]
    },
    {
        i18nKey: 'menu.dataCenter',
        path: "/dataCenter",
        icon: img6,
        activeIcon: img6Active,
        children: [
            { i18nKey: 'menu.dataReport', path: "/dataReport" }
        ]
    },
    {
        i18nKey: 'menu.user',
        path: "/userOrganize",
        icon: img7,
        activeIcon: img7Active,
        children: [
            { i18nKey: 'menu.userAccount', path: "/userAccount" },
            { i18nKey: 'menu.userPlan', path: "/userPlan" },
            { i18nKey: 'menu.userRole', path: "/userRole" },
            { i18nKey: 'menu.userLog', path: "/userLog" }
        ]
    }
]);

watch(createWorkList, (newval) => {
    menuItems.value[0].children = [
        { i18nKey: 'menu.overview', path: "/workPageOverview" },
        ...createWorkList.value.map((item: any, index: any) => ({ i18nKey: '', label: item.config_name, path: `config_id=${item.config_id}&config_name=${item.config_name}` }))
    ]
}, { immediate: true })

const activeMenu = computed(() => {
    if (currentConfigPath.value) {
        return currentConfigPath.value;
    }
    const path = route.path;
    if (path.startsWith('/facilityPattern')) return '/facilityPattern';
    if (path.startsWith('/facilityGroup')) return '/facilityGroup';
    if (path.startsWith('/facilityList')) return '/facilityList';
    return path || "/workPageOverview";
});

const openKeys = computed(() => {
    const keys: string[] = [];
    if (currentConfigPath.value) {
        keys.push('/workPage');
        return keys;
    }
    menuItems.value.forEach(item => {
        if (item.children?.some(c => c.path === route.path)) {
            keys.push(item.path);
        }
    });
    return keys;
});

const isParentActive = (item: any) => {
    if (currentConfigPath.value) {
        return item.path === '/workPage';
    }
    return item.children?.some((c: any) => c.path === route.path) ||
        item.path === route.path ||
        route.path.startsWith(item.path + '/');
};

const handleMenuSelect = (index: string) => {
    if (index.includes('config_id')) {
        const urlParams = new URLSearchParams(index);
        const configId = urlParams.get('config_id');
        const configName = urlParams.get('config_name');
        if (configId) {
            currentConfigPath.value = index;
            const tokenData = sessionStorage.getItem("wk_Token");
            const token = tokenData ? JSON.parse(tokenData).token || tokenData : '';
            const uid = user_id.value;
            const timestamp = Date.now();
            const previewUrl = `${luyou}/#/chart/preview/${configId}?token=${token}&user_id=${uid}&config_id=${configId}&t=${timestamp}&config_name=${configName}`;
            window.open(previewUrl, '_blank');
            return;
        }
    }
    currentConfigPath.value = '';
    router.push(index);
};
</script>

<style lang="scss" scoped>
.layout {
    width: 100%;
    height: auto !important;
    overflow: hidden !important;
    position: relative;
}

:deep(.el-menu) {
    border: none !important;
    max-height: calc(100vh - 60px) !important;
    overflow-y: scroll !important;
    padding: 0 0 0.5rem 0 !important;
    box-sizing: border-box !important;
    overflow-y: auto !important;
    overflow-x: hidden !important;
    scrollbar-width: none !important;
    -ms-overflow-style: none !important;
}

:deep(.el-menu::-webkit-scrollbar) {
    width: 6px !important;
    height: 6px !important;
    background: transparent !important;
}

:deep(.el-menu::-webkit-scrollbar-track) {
    background: transparent !important;
}

:deep(.el-menu::-webkit-scrollbar-thumb) {
    background: transparent !important;
    border-radius: 3px !important;
    transition: background 0.3s ease !important;
}

:deep(.el-menu:hover::-webkit-scrollbar-thumb) {
    background: #c0c4cc !important;
}

:deep(.el-menu:hover::-webkit-scrollbar-thumb:hover) {
    background: #909399 !important;
}

:deep(.el-menu:hover) {
    scrollbar-width: thin !important;
    scrollbar-color: #c0c4cc transparent !important;
}

:deep(.el-sub-menu.is-active .el-sub-menu__title) {
    color: #409eff !important;
}

:deep(.el-menu-item.is-active) {
    color: #409eff !important;
    background-color: #ecf5ff !important;
}

:deep(.el-sub-menu.is-active .el-sub-menu__title) {
    color: #409eff !important;
}

:deep(.el-menu-item.is-active) {
    color: #409eff !important;
    background-color: #ecf5ff !important;
}
</style>

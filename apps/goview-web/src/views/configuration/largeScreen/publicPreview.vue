<template>
    <div class="public-preview">
        <iframe ref="iframeRef" :src="iframeUrl" frameborder="0" width="100%" height="100%"
            @load="handleIframeLoad"></iframe>
        <div v-if="loading" class="loading-overlay">
            <el-icon size="48" class="is-loading">
                <Loading />
            </el-icon>
            <p>加载中...</p>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { luyou } from '@/utils/request';
// 1. 补全缺失的Vue核心导入（之前漏掉了，会导致编译失败）
import { ref, computed, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { Loading } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus'; // 导入消息提示
import { bootstrap } from '@/api/configuration/index';

const route = useRoute();
const iframeRef = ref<HTMLIFrameElement | null>(null);
const loading = ref(false);

const shareData = ref({
    preview_token: '',
    user_id: '',
    config_id: '',
    config_name: ''
});

// const iframeUrl = computed(() => {
//     const configId = route.params.configId || route.query.config_id || shareData.value.config_id;
//     const token = route.query.token || shareData.value.preview_token || '';
//     const userId = route.query.user_id || shareData.value.user_id || '';
//     const configName = shareData.value.config_name || '';
//     const timestamp = Date.now();

//     if (!configId) return '';

//     // 动态获取当前页面的协议和主机名，确保分享链接能被其他人访问
//     const protocol = window.location.protocol;
//     const hostname = window.location.hostname;
//     const configUrl = `${protocol}//${hostname}:3020/#/chart/preview/${configId}?config_id=${configId}&token=${token}&user_id=${userId}&config_name=${encodeURIComponent(configName)}&t=${timestamp}`;

//     return configUrl;
// });
const iframeUrl = computed(() => {
    const configId = route.params.configId || route.query.config_id || shareData.value.config_id;
    const token = route.query.token || shareData.value.preview_token || '';
    const userId = route.query.user_id || shareData.value.user_id || '';
    const configName = shareData.value.config_name || '';
    const timestamp = Date.now();

    if (!configId || !token) return '';
    console.log(luyou, 'luyou')
    // 直接复用全局 luyou 地址，和普通预览按钮完全一致
    return `${luyou}/#/chart/preview/${configId}?config_id=${configId}&token=${token}&user_id=${userId}&config_name=${encodeURIComponent(configName)}&t=${timestamp}`;
});

const handleIframeLoad = () => {
    console.log('iframe加载完成');
    loading.value = false;
};

onMounted(() => {
    const configName = route.query.config_name ? decodeURIComponent(route.query.config_name as string) : '';
    if (configName) {
        document.title = configName;
    }

    const shareId = route.query.share_id;

    if (shareId) {
        bootstrap({ share_id: shareId })
            .then((res: any) => {
                if (res) {
                    shareData.value = res;
                    if (!configName && res.config_name) {
                        document.title = res.config_name;
                    }
                }
            })
    }

    setTimeout(() => {
        if (loading.value) {
            console.warn('接口请求超时，强制关闭加载');
            loading.value = false;
            ElMessage.warning('加载超时，请刷新页面重试');
        }
    }, 10000);
});
</script>

<style lang="scss" scoped>
.public-preview {
    width: 100vw;
    height: 100vh;
    position: relative;
    overflow: hidden;
}

.loading-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(255, 255, 255, 0.9);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    z-index: 100;

    p {
        margin-top: 16px;
        color: #666;
        font-size: 16px;
    }
}
</style>
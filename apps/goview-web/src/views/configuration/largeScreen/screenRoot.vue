<template>
    <div>
        <iframe ref="iframeRef" :src="iframeUrl" frameborder="0" style="width: 100%;height: 100vh;" @load="onIframeLoad"></iframe>
    </div>
</template>

<script lang="ts" setup>
import { computed, ref, onMounted } from "vue";
import { useRoute } from "vue-router";
import { storeToRefs } from 'pinia';
import useCounterStore from "@/stores/counter";
import { luyou } from "@/utils/request";

const route = useRoute();
const iframeRef = ref<HTMLIFrameElement | null>(null);
const store = useCounterStore();
const { user_id } = storeToRefs(store);

const normalizeBaseUrl = (url: string) => url.replace(/\/$/, '');

const getToken = () => {
    const tokenData = sessionStorage.getItem("wk_Token");
    if (!tokenData) return '';
    try {
        const parsed = JSON.parse(tokenData);
        return parsed.token || parsed.tokenData || tokenData;
    } catch {
        return tokenData;
    }
};

const buildGoViewUrl = (mode: string | undefined, configId: any) => {
    const baseUrl = normalizeBaseUrl(luyou);
    const userId = route.query.user_id || route.query.userId || store.user_id || user_id?.value || '';
    const token = getToken();
    const configName = route.query.config_name || '';
    const params = new URLSearchParams({
        config_id: String(configId || ''),
        user_id: String(userId || ''),
        token: String(token || ''),
        config_name: String(configName || ''),
        mode: mode === 'design' ? 'design' : 'preview',
        t: String(Date.now())
    });

    if (mode === 'design') {
        return `${baseUrl}/#/chart/home/${configId}?${params.toString()}`;
    }
    return `${baseUrl}/#/chart/preview/${configId}?${params.toString()}`;
};

const iframeUrl = computed(() => {
    const configId = route.query.config_id || route.params.id;
    const mode = route.query.mode;
    if (!configId) return luyou;
    return buildGoViewUrl(mode as string | undefined, configId);
});

const onIframeLoad = () => {
    sendToIframe();
};

const sendToIframe = () => {
    const tokenItem = sessionStorage.getItem("wk_Token");
    if (!tokenItem) {
        return;
    }

    let tokenData = '';
    let userId = '';
    try {
        const parsed = JSON.parse(tokenItem);
        tokenData = parsed.token || parsed.tokenData || tokenItem;
        userId = parsed.userId || parsed.user_id || '';
    } catch {
        tokenData = tokenItem;
    }

    if (!userId) {
        userId = user_id?.value || store.user_id || '';
    }

    if (!tokenData) {
        return;
    }

    const configId = route.query.config_id || route.params.id;
    
    const iframe = iframeRef.value;
    if (iframe?.contentWindow) {
        iframe.contentWindow.postMessage(
            {
                type: "auth",
                token: tokenData,
                userId: userId,
                config_id: configId
            },
            '*'
        );
    } else {
        console.warn('iframe 或 contentWindow 不存在');
    }
};

onMounted(() => {
    setTimeout(() => {
        sendToIframe();
    }, 500);
});
</script>

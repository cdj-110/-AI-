<template>
    <div>
        <!-- <iframe src="http://localhost:3020?aaa=111" frameborder="0" style="width: 100%;height: 100vh;"></iframe> -->
        <iframe ref="iframeRef" src="http://localhost:3020?aaa=111" frameborder="0" style="width: 100%;height: 100vh;"
            @load="onIframeLoad"></iframe>
    </div>
</template>

<script lang="ts" setup>
import useCounterStore from "@/stores/counter";
import { ref, onMounted } from "vue";

let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
const token: any = sessionStorage.getItem("wk_Token");
const iframeRef = ref<HTMLIFrameElement | null>(null);

onMounted(() => {
    // 延迟发送确保 iframe 完全加载
    setTimeout(() => {
        sendTokenToIframe();
    }, 500);
})
// iframe 加载完成后发送 token
const onIframeLoad = () => {
    console.log('iframe 加载完成');
    sendTokenToIframe();
};

// 发送 token 到 iframe
const sendTokenToIframe = () => {
   // ✅ 修正后
const tokenItem = sessionStorage.getItem('wk_Token');
console.log('a 项目 sessionStorage wk_Token:', tokenItem); // 添加调试日志

let tokenData = '';
if (tokenItem) {
    try {
        const parsed = JSON.parse(tokenItem);
        tokenData = parsed.token || parsed.tokenData || tokenItem;
    } catch {
        tokenData = tokenItem;
    }
}
console.log('a 项目解析后的 tokenData:', tokenData);
};

</script>
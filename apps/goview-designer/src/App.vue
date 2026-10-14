<template>
    <n-config-provider :theme="darkTheme" :hljs="hljsTheme" :locale="locale" :date-locale="dateLocale"
        :theme-overrides="overridesTheme">
        <go-app-provider>
            <I18n></I18n>
            <router-view></router-view>
            <SessionRecovery />
        </go-app-provider>
    </n-config-provider>
</template>

<script lang="ts" setup>
import { NConfigProvider } from 'naive-ui'
// 恢复弹框依赖编辑器 Store，必须在 setupStore 完成、应用挂载后再加载。
const SessionRecovery = defineAsyncComponent(() => import('@/components/SessionRecovery.vue'))
import { GoAppProvider } from '@/components/GoAppProvider'
import { I18n } from '@/components/I18n'
import { useDarkThemeHook, useThemeOverridesHook, useCode, useLang } from '@/hooks'
import { defineAsyncComponent, onMounted, ref } from 'vue'
import { apiV1MediaImagesList } from '@/api/index'

// 暗黑主题
const darkTheme = useDarkThemeHook()

// 主题配置
const overridesTheme = useThemeOverridesHook()

// 代码主题
const hljsTheme = useCode()

// 全局语言
const { locale, dateLocale } = useLang()
//测试提交

onMounted(() => {
    // 云平台目前没有旧版私有图片列表接口；不在首屏请求不存在的接口。
    if (sessionStorage.getItem('wk_ApiBaseUrl')) return
    // let userId = ref(sessionStorage.getItem('wk_UserId'));
    let userId = ref(new URLSearchParams((window.location.hash.split('?')[1] ?? '')).get('user_id') || sessionStorage.getItem('wk_UserId') || '');
      apiV1MediaImagesList({user_id: userId.value}).then((res: any) => {
        if(res.items && Array.isArray(res.items) && res.items.length > 0){
          sessionStorage.setItem("tupianchunchu", JSON.stringify(res.items));
        }
      }).catch(error => { console.warn('私有图片列表加载失败', error?.message || '未知错误'); });
})
</script>

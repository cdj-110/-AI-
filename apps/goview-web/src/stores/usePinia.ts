import { createPinia, defineStore } from "pinia";
import { ref, computed } from "vue";
import persistedstate from "pinia-plugin-persistedstate";
import zhCn from "element-plus/dist/locale/zh-cn.mjs"
import enUs from "element-plus/dist/locale/en.mjs"

// 创建Pinia实例
const pinia = createPinia();
pinia.use(persistedstate);

//  新增：全局应用状态Store（包含语言管理）
export const useAppStore = defineStore('app', () => {
  // 语言状态（自动持久化到localStorage）
  const locale = ref(localStorage.getItem('locale') || 'zh-CN');

  // 计算属性：获取Element Plus对应的语言包
  const elementLocale = computed(() => {
    return locale.value === 'zh-CN' ? zhCn : enUs;
  });

  // 切换语言方法
  const changeLocale = (newLocale: string) => {
    locale.value = newLocale;
    localStorage.setItem('locale', newLocale);
    document.documentElement.lang = newLocale;
  };

  return {
    locale,
    elementLocale,
    changeLocale
  };
}, {
  //  配置持久化：只持久化locale字段
  persist: {
    key: 'app-store',
    paths: ['locale']
  }
});

export default pinia;
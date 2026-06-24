import { createApp, nextTick } from "vue";
import App from "./App.vue";
import "@/utils/root.scss";
import routerr from "./router";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import * as ElementPlusIconsVue from "@element-plus/icons-vue";
import usePinia from "@/stores/usePinia";
import "echarts";
import ECharts from "vue-echarts";
import DataVVue3 from "@kjgl77/datav-vue3";
import createPattern from "@/views/facility/facilityList/createPattern.vue";
import getPattern from "@/views/facility/facilityList/getPattern.vue";
import i18n from './locales/index'

const app = createApp(App);

// 注册所有Element Plus图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    app.component(key, component);
}

// 缩放功能
const preventCtrlChange = (e: WheelEvent) => {
    if (e.ctrlKey) {
        e.preventDefault();
        nextTick(() => {
            const systemPixel = Math.round((window.devicePixelRatio || 1) * 100);
            const currentZoomStr = document.body.style.zoom || `${systemPixel}%`;
            const currentZoom = Number(currentZoomStr.replace('%', ''));

            if (e.deltaY < 0) {
                if (currentZoom < 200) {
                    document.body.style.zoom = `${currentZoom + 20}%`;
                }
            } else {
                if (currentZoom > 50) {
                    if (systemPixel <= 100) {
                        document.body.style.zoom = `${Math.max(100, currentZoom - 20)}%`;
                    } else if (systemPixel <= 125) {
                        document.body.style.zoom = `${Math.max(105, currentZoom - 20)}%`;
                    } else if (systemPixel <= 150) {
                        document.body.style.zoom = `${Math.max(100, currentZoom - 10)}%`;
                    } else {
                        document.body.style.zoom = `${currentZoom - 10}%`;
                    }
                }
            }
        });
    }
};

document.addEventListener("wheel", preventCtrlChange, { passive: false });

// 注册全局组件
app.component("VueEcharts", ECharts);
app.component("createPattern", createPattern);
app.component("getPattern", getPattern);

// 插件注册
app.use(DataVVue3);
app.use(usePinia);
app.use(i18n);
app.use(ElementPlus);
app.use(routerr);

app.mount('#app');
<template>
    <div class="chart-item" :id="item.id" v-for="(item, index) in listAtter"
        :class="animationsClass(item.styles.animations)" :key="item.id" :style="{
            ...getComponentAttrStyle(item.attr, index),
            ...getTransformStyle(item.styles),
            ...getStatusStyle(item.status),
            ...getPreviewConfigStyle(item.preview),
            ...getBlendModeStyle(item.styles) as any,
            ...getSizeStyle(item.attr)
        }">

        <!-- 分组 -->
        <preview-render-group v-if="item.isGroup" :groupData="(item as CreateComponentGroupType)" :groupIndex="index"
            :themeSetting="themeSetting" :themeColor="themeColor"></preview-render-group>
        <!-- 单组件 -->
        <component v-else :is="item.chartConfig.chartKey" :id="item.id" :chartConfig="item" :themeSetting="themeSetting"
            :themeColor="themeColor" :style="{
                ...getSizeStyle(item.attr),
                ...getFilterStyle(item.styles)
            }" v-on="useLifeHandler(item)"></component>
    </div>
</template>

<script setup lang="ts">
import { PropType, computed, onMounted, ref } from 'vue'
import { useChartDataPondFetch } from '@/hooks'
import { ChartEditStorageType } from '../../index.d'
import { PreviewRenderGroup } from '../PreviewRenderGroup/index'
import { CreateComponentGroupType } from '@/packages/index.d'
import { chartColors } from '@/settings/chartThemes/index'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { animationsClass, getFilterStyle, getTransformStyle, getBlendModeStyle, colorCustomMerge, JSONParse } from '@/utils'
import { getSizeStyle, getComponentAttrStyle, getStatusStyle, getPreviewConfigStyle } from '../../utils'
import { useLifeHandler } from '@/hooks'
import { apiV1configDatas_test } from '@/api/index'
import { allConnection } from "@/utils/allWebSocketConnction";
import { baseurlAddress } from "@/utils/request";
import { useRoute } from 'vue-router'
import { getUrlSearch, JSONStringify } from "@/utils/utils";
import { eventTypeChange, eventHeapListChange } from "./event";

// 初始化数据池
const { initDataPond, clearMittDataPondMap } = useChartDataPondFetch()
const chartEditStore = useChartEditStore()
let listZhong = ref();
let listAtter = ref();
let listSocket = ref<Array<object>>([]);//保存实时数据
let userId = ref(sessionStorage.getItem('wk_UserId') as any);
let wk_Config_id = ref(sessionStorage.getItem('wk_Config_id') as any);
let listCount = ref(0);//保存单个数值上一次的数值

// const props = defineProps({
//   localStorageInfo: {
//     type: Object as PropType<ChartEditStorageType>,
//     required: true
//   }
// })

// 主题色
const themeSetting = computed(() => {
    const chartThemeSetting = chartEditStore.editCanvasConfig.chartThemeSetting
    return chartThemeSetting
})

// 配置项
const themeColor = computed(() => {
    const colorCustomMergeData = colorCustomMerge(chartEditStore.editCanvasConfig.chartCustomThemeColorInfo)
    return colorCustomMergeData[chartEditStore.editCanvasConfig.chartThemeColor]
})

/**
* 查询存储的组态数据(并渲染到页面上面)
*/
let getList = async (ear: any) => {
    let res: any = await apiV1configDatas_test({ user_id: userId.value && userId.value != "undefined" ? userId.value : getUrlSearch("user_id"), config_id: wk_Config_id.value });
    // let chartList = res.main_data.item[0].componentList.filter((item: any) => item.option.dataset && Array.isArray(item.request.requestParams.Params));
    let aaaaa = JSONParse(sessionStorage.getItem("chunyixiazhutai") as any);
    let chartList;
    let chart_list;
    if (aaaaa) {
        chartList = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0].componentList.filter((item: any) => (item.option.dataset || item.option.dataset === 0) && Array.isArray(item.request.requestParams.Params));
        chart_list = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0].componentList;
    } else {
        chartList = res.main_data.item[0].componentList.filter((item: any) => (item.option.dataset || item.option.dataset === 0) && Array.isArray(item.request.requestParams.Params));
        chart_list = res.main_data.item[0].componentList;
    }
    shi_shi_shu_ju(chartList, ear);

    chart_list.forEach((vtent: any)=>{
        if(vtent.interactActions && vtent.interactActions.length != 0){
            eventHeapListChange(vtent);
        }
        console.log(vtent)
        if(vtent.groupList){
            let ver = vtent.groupList.filter((item: any) => (item.option.dataset || item.option.dataset === 0) && Array.isArray(item.request.requestParams.Params));
            shi_shi_shu_ju(ver, ear);
        }
    })

    // listZhong.value = res.main_data.item[0];
    // listAtter.value = res.main_data.item[0].componentList;
    listZhong.value = res.main_data.item[0];
    aaaaa ? listAtter.value = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0].componentList :
        listAtter.value = res.main_data.item[0].componentList;
}

// 组件渲染结束初始化数据池
clearMittDataPondMap()
onMounted(() => {
    let wsUrl = `${baseurlAddress.wss}/api/configuration/point-bindings/realtime?config_id=${wk_Config_id.value}`;
    allConnection(wsUrl).then((ws) => {
        ws.onmessage = (e: any) => {
            let data = JSONParse(e.data);
            listSocket.value.push(data.data);
            listSocket.value = listSocket.value.length > 5 ? listSocket.value.splice(-5) : listSocket.value;
            getList(listSocket.value);
        };
    })
    initDataPond(useChartEditStore);
})

/**
 * 处理渲染时候的实时数据
 */
let shi_shi_shu_ju = (list: any,ear: any) => {
    list.forEach((item: any) => {
        if (item.option.dataset.dimensions && item.option.dataset.source) {
            item.option.dataset.dimensions = ["product", ...item.request.requestParams.Params.map((key: any) => key.value2)];
            item.option.dataset.source = transformData(ear);
        } else if (typeof item.option.dataset == "number" || typeof item.option.dataset == "string") {
            if (Array.isArray(item.request.requestParams.Params)) {
                let list = ear[ear.length - 1].filter((countList: any) => countList.dev_id == item.request.requestParams.Params[0].value1 && countList.identifier == item.request.requestParams.Params[0].value2);
                if (list.length > 0) {
                    let typeType = typeof list[0].value == "number" ? parseFloat(list[0].value) : typeof list[0].value == "string" || typeof list[0].value == "boolean" ? list[0].value : undefined;
                    // item.option.dataset = parseFloat(list[0].value);
                    // listCount.value = parseFloat(list[0].value);
                    item.option.dataset = typeType;
                    listCount.value = typeType;
                } else {
                    item.option.dataset = listCount.value;
                }
            }
        }
    })
}

/**
 * 处理实时数据
 */
function transformData(data: any) {
    let allIdentifiers = new Set();
    for (let group of data) {
        for (let item of group) {
            allIdentifiers.add(item.identifier);
        }
    }
    let result = [];
    for (let group of data) {
        if (group.length === 0) continue;
        let first = group[0];
        let newObj = {
            dev_id: first.dev_id,
            timestamp: first.timestamp,
            product: first.timestamp
        };
        let valueMap = new Map();
        for (let item of group) {
            valueMap.set(item.identifier, parseFloat(item.value));
        }
        for (let id of allIdentifiers) {
            newObj[id] = valueMap.has(id) ? valueMap.get(id) : 0;
        }
        result.push(newObj);
    }
    return result;
}
</script>

<style lang="scss" scoped>
.chart-item {
    position: absolute;
}
</style>

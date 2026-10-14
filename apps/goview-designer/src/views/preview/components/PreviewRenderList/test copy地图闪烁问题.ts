import { computed, onMounted, ref } from 'vue'
import { useChartDataPondFetch } from '@/hooks'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { apiV1configDatas_test, apiAnalysisQueryV3_2 } from '@/api/index'
import { allConnection } from "@/utils/allWebSocketConnction";
import { baseurlAddress } from "@/utils/request";
import { eventHighway, getUrlSearch, JSONParse, JSONStringify } from "@/utils/utils";
import { functionTypeChange, eventTypeGroup } from "./event";
import { colorCustomMerge } from '@/utils';

export default function previewRenderList(){
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
        const colorCustomMergeData: any = colorCustomMerge(chartEditStore.editCanvasConfig.chartCustomThemeColorInfo)
        return colorCustomMergeData[chartEditStore.editCanvasConfig.chartThemeColor]
    })

    /**
    * 查询存储的组态数据(并渲染到页面上面)
    */
    let getList = async (ear: any, _list?: any, off?: any, initialLoad = false) => {
        let res: any = await apiV1configDatas_test({ user_id: userId.value && userId.value != "undefined" ? userId.value : getUrlSearch("user_id"), config_id: wk_Config_id.value });
        // let chartList = res.main_data.item[0].componentList.filter((item: any) => item.option.dataset && Array.isArray(item.request.requestParams.Params));
        let aaaaa = _list ? {title: _list} : JSONParse(sessionStorage.getItem("chunyixiazhutai") as any);
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
            eventHeapListChange(vtent, off);
            if(vtent.groupList){
                let ver = vtent.groupList.filter((item: any) => (item.option.dataset || item.option.dataset === 0) && Array.isArray(item.request.requestParams.Params));
                shi_shi_shu_ju(ver, ear);
            }

            if(vtent.request.requestHttpType == "历史数据"){
                apiAnalysisQueryV3_2({
                    user_id: userId.value,
                    start: vtent.request.requestInterval[0],
                    end: vtent.request.requestInterval[1],
                    pairs: vtent.request.requestParams.Params
                }).then(res => {
                    vtent = res;
                })
            }
        })

        if(off && off.changeAttr == "显隐切换" && Array.isArray(off.targetList)){
            if(off.xianTrueVVV == "隐藏"){
                if(aaaaa){
                    let aaa = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0].componentList;
                    res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0].componentList = aaa.filter((key1: any) => !off.targetList.map((key: any) => key.id).includes(key1.id));
                }else{
                    let aaa = res.main_data.item[0].componentList;
                    res.main_data.item[0].componentList = aaa.filter((key1: any) => !off.targetList.map((key: any) => key.id).includes(key1.id));
                }
            }
        }

        // 首次加载：直接赋值，建立完整引用
        if (initialLoad || !listAtter.value || listAtter.value.length === 0) {
            listZhong.value = res.main_data.item[0];
            aaaaa ? listAtter.value = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0].componentList :
                listAtter.value = res.main_data.item[0].componentList;
        } else {
            // 非首次加载：只把新数据合并到已有组件中，不替换整个数组引用，避免所有组件被卸载重绘
            const newList = aaaaa
                ? res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0].componentList
                : res.main_data.item[0].componentList;
            listZhong.value = res.main_data.item[0];
            // 用新数据逐项覆盖，保留原有数组引用与组件实例
            const existingList: any[] = listAtter.value;
            for (let i = 0; i < newList.length; i++) {
                const oldItem = existingList[i];
                const newItem = newList[i];
                if (!oldItem || oldItem.id !== newItem.id) {
                    // 结构发生变化时才替换对应下标的项
                    existingList[i] = newItem;
                } else {
                    // 只更新会变化的字段，其它配置保持不变
                    oldItem.option.dataset = newItem.option.dataset;
                    if (oldItem.option.series && newItem.option.series) {
                        oldItem.option.series = newItem.option.series;
                    }
                    if (oldItem.option.visualMap && newItem.option.visualMap) {
                        oldItem.option.visualMap = newItem.option.visualMap;
                    }
                    if (oldItem.request) {
                        oldItem.request = newItem.request;
                    }
                    if (oldItem.status) {
                        oldItem.status = newItem.status;
                    }
                    if (oldItem.preview) {
                        oldItem.preview = newItem.preview;
                    }
                    if (oldItem.groupList && newItem.groupList) {
                        oldItem.groupList = newItem.groupList;
                    }
                    if (oldItem.interactActions !== undefined) {
                        oldItem.interactActions = newItem.interactActions;
                    }
                    if (oldItem.events) {
                        oldItem.events = newItem.events;
                    }
                }
            }
            // 后端返回的组件数量少于当前时，截断多余的
            if (existingList.length > newList.length) {
                existingList.length = newList.length;
            }
            // 如果后端组件数变多，追加
            if (existingList.length < newList.length) {
                for (let i = existingList.length; i < newList.length; i++) {
                    existingList.push(newList[i]);
                }
            }
        }

        localStorage.setItem("zhutaineirong",JSONStringify(res));//存在这是因为本页面的事件修改[chunyixiazhutai]本地存储超限，别的地方不可使用
    }

    // 组件渲染结束初始化数据池
    clearMittDataPondMap()
    onMounted(async () => {
        // 首次加载：先调一次 getList 建立完整的 listAtter 引用
        await getList(listSocket.value, undefined, undefined, true);
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

        eventHighway.to("openPage", (e: any) => {
            let arr = JSONParse(localStorage.getItem("zhutaineirong") as any).main_data.item.filter((item: any) => item.title === e.detail.pageValue)[0];
            sessionStorage.removeItem("zhutaineirong");
            sessionStorage.setItem("chunyixiazhutai",JSONStringify(arr));//将当前正在编辑的组态临时保存一下，点击保存的时候读取
            getList(listSocket.value, e.detail.pageValue, undefined, true);
        })
        eventHighway.to("noGoLive", (e: any) => {
            let title = JSONParse(sessionStorage.getItem("chunyixiazhutai") as any);
            getList(listSocket.value, title.title, e.detail, true);
        })
        eventHighway.to("gotoShow", (e: any) => {
            let title = JSONParse(sessionStorage.getItem("chunyixiazhutai") as any);
            getList(listSocket.value, title.title, e.detail, true);
        })
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
        let allIdentifiers: any = new Set();
        for (let group of data) {
            for (let item of group) {
                allIdentifiers.add(item.identifier);
            }
        }
        let result = [];
        for (let group of data) {
            if (group.length === 0) continue;
            let first = group[0];
            let newObj: any = {
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

    /**
     * 将多个事件拆分，并且添加到函数中(实现事件的堆栈)
     * @inner(eventList) 要拆分的数据源配置项
     * @inner(off) 要禁用的数据源配置项
     */
    let eventHeapListChange = (eventList: any, off: any) => {
        let startTrueVVV = 1;
        if(off && off.changeAttr == "禁启用" && Array.isArray(off.targetList)){
            let jinyong = off.targetList.map((item: any) => item.id).includes(eventList.id);//筛选出要禁用的所有数据
            if(!jinyong){
                startTrueVVV = 1;
            }else if(jinyong && off.startTrueVVV == "禁用"){
                startTrueVVV = 2;
            }else if(jinyong && off.startTrueVVV == "启用"){
                startTrueVVV = 1;
            }
        }
        // if(off && off.changeAttr == "显隐切换" && Array.isArray(off.targetList)){
        //     let jinyong = off.targetList.map((item: any) => item.id).includes(eventList.id);//筛选出要禁用的所有数据
        //     console.log(eventList)
        // }
        if(eventList.interactActions && Array.isArray(eventList.interactActions) && startTrueVVV == 1) {
            eventList.events.baseEvent = Object.assign({}, eventList.events.baseEvent, eventTypeGroup(eventList.interactActions, "changeType", true));
        }
    }
    return {
        listAtter,
        listSocket,

        themeSetting,
        themeColor,
        getList
    };
}
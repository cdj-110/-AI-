import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useChartDataPondFetch } from '@/hooks'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { apiV1configDatas_test, apiAnalysisQueryV3_2, deviceinfo } from '@/api/index'
import { allConnection } from "@/utils/allWebSocketConnction";
import { baseurlAddress } from "@/utils/request";
import { eventHighway, getUrlSearch, JSONParse, JSONStringify } from "@/utils/utils";
import { functionTypeChange, eventTypeGroup } from "./event";
import { colorCustomMerge } from '@/utils';
import { usePlatformTelemetry } from '@/utils/platformTelemetry';
import { isHistoryQueryChart } from '@/utils/platformHistoryChart';
import { installPreviewComponents } from '../../hooks/useComInstall.hook';
import { createConditionEventRunner, conditionEventSnapshot } from '@/utils/conditionEvents';
import { eventFunctions } from '@/hooks/eventFunctions';

export default function previewRenderList(){
    // 初始化数据池
    const { initDataPond, clearMittDataPondMap } = useChartDataPondFetch()
    const chartEditStore = useChartEditStore()
    let listZhong = ref();
    let listAtter = ref();
    const conditionRunner=createConditionEventRunner();
    const stopConditions=watch(()=>conditionEventSnapshot(listAtter.value||[]),()=>{
        conditionRunner.evaluate(listAtter.value||[],(component:any,event:any)=>{
            const fn=eventFunctions[functionTypeChange[event.changeAttr]];
            if(fn)fn({component,event:{type:'满足条件'}},event);
        });
    },{flush:'post'});
    usePlatformTelemetry(() => listAtter.value || []);
    let listSocket = ref<Array<object>>([]);//保存实时数据
    let eventOverrideIds = new Set<string>();//记录被事件（更新属性）临时改过样式的组件id，下一帧实时数据自动还原
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
    let getList = async (ear: any, _list?: any, off?: any, initialLoad = false, pageOverride?: any) => {
        // 页面事件切换时直接使用已经解析出的目标画布，避免再次请求期间被实时刷新覆盖。
        // 普通实时刷新仍从服务端读取完整项目数据。
        let res: any = pageOverride
            ? { main_data: { item: [pageOverride] } }
            : await apiV1configDatas_test({ user_id: userId.value && userId.value != "undefined" ? userId.value : getUrlSearch("user_id"), config_id: wk_Config_id.value });
        let aaaaa = _list ? {title: _list} : JSONParse(sessionStorage.getItem("chunyixiazhutai") as any);
        const requestedTitle = aaaaa?.title;
        const activeTitle = JSONParse(sessionStorage.getItem("chunyixiazhutai") as any)?.title;
        if (requestedTitle && activeTitle && requestedTitle !== activeTitle) return;
        let chartList: any[] = [];
        let chart_list: any[] = [];
        if (aaaaa && Array.isArray(res?.main_data?.item)) {
            const matched = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0];
            if (matched && Array.isArray(matched.componentList)) {
                chartList = matched.componentList.filter((item: any) => (item.option?.dataset || item.option?.dataset === 0 || item.chartConfig?.key === 'CirclePoint') && item.request?.requestParams && Array.isArray(item.request.requestParams.Params));
                chart_list = matched.componentList;
            }
        } else if (Array.isArray(res?.main_data?.item) && res.main_data.item[0] && Array.isArray(res.main_data.item[0].componentList)) {
            chartList = res.main_data.item[0].componentList.filter((item: any) => (item.option?.dataset || item.option?.dataset === 0 || item.chartConfig?.key === 'CirclePoint') && item.request?.requestParams && Array.isArray(item.request.requestParams.Params));
            chart_list = res.main_data.item[0].componentList;
        }

        // 用 sessionStorage 里最新的 interactActions 覆盖后端数据（编辑器改事件后不依赖后端保存）
        try {
            const ss2 = JSONParse(sessionStorage.getItem("chunyixiazhutai") as any);
            if (ss2?.componentList && Array.isArray(ss2.componentList)) {
                chart_list.forEach((c: any) => {
                    const ssComp = ss2.componentList.find((s: any) => s.id === c.id);
                    if (ssComp?.interactActions) c.interactActions = ssComp.interactActions;
                });
            }
        } catch (e) {  }

        shi_shi_shu_ju(chartList, ear);

        chart_list.forEach((vtent: any)=>{
            if(vtent.request.requestHttpType == "地图标点-高德"){
                deviceinfo({user_id: userId.value}).then((res: any) => {
                    let valueNo = vtent.request.requestParams.Params.map((v1v1v1: any) => v1v1v1.value1);
                    let idSet: any = new Set(valueNo);
                    let nonono = res.data.data.filter((item: any) => idSet.has(item.dev_sn));
                    vtent.option.dataset.markers = nonono.map((eqeqeq: any) => ({
                        name: eqeqeq.install_location,
                        position: [eqeqeq.lng, eqeqeq.lat],
                        value: eqeqeq.dev_sn,
                        ...eqeqeq
                    }))
                })
            }
            eventHeapListChange(vtent, off);
            if(Array.isArray(vtent.groupList)){
                let ver = vtent.groupList.filter((item: any) => (item.option?.dataset || item.option?.dataset === 0 || item.chartConfig?.key === 'CirclePoint') && item.request?.requestParams && Array.isArray(item.request.requestParams.Params));
                shi_shi_shu_ju(ver, ear);
            }

            if(initialLoad && vtent.request?.requestHttpType == "历史数据" && !isHistoryQueryChart(vtent) && Array.isArray(vtent.request.requestInterval) && vtent.request.requestInterval.length >= 2){
                apiAnalysisQueryV3_2({
                    user_id: userId.value,
                    start: vtent.request.requestInterval[0],
                    end: vtent.request.requestInterval[1],
                    pairs: vtent.request.requestParams?.Params || [],
                    page: vtent.option?.pagination?.page || 1,
                    pageSize: vtent.option?.pagination?.pageSize || 10
                }).then((history: any) => {
                    if (history?.dataset) {
                        vtent.option.dataset = history.dataset;
                        if (vtent.option?.pagination) {
                            vtent.option.pagination.page = history.page || 1;
                            vtent.option.pagination.pageSize = history.pageSize || vtent.option.pagination.pageSize;
                            vtent.option.pagination.itemCount = Number(history.total) || 0;
                        }
                    }
                })
            }
        })

        if(off && off.changeAttr == "显隐切换" && Array.isArray(off.targetList) && Array.isArray(res?.main_data?.item)){
            if(off.xianTrueVVV == "隐藏"){
                if(aaaaa){
                    const matched = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0];
                    if (matched && Array.isArray(matched.componentList)) {
                        matched.componentList = matched.componentList.filter((key1: any) => !off.targetList.map((key: any) => key.id).includes(key1.id));
                    }
                }else if (res.main_data.item[0] && Array.isArray(res.main_data.item[0].componentList)){
                    res.main_data.item[0].componentList = res.main_data.item[0].componentList.filter((key1: any) => !off.targetList.map((key: any) => key.id).includes(key1.id));
                }
            }
        }

        // 首次加载：直接赋值，建立完整引用
        let targetItem: any = null;
        let targetComponentList: any[] = [];
        if (Array.isArray(res?.main_data?.item)) {
            if (aaaaa) {
                targetItem = res.main_data.item.filter((item: any) => item.title == aaaaa.title)[0] || null;
            } else {
                targetItem = res.main_data.item[0] || null;
            }
            if (targetItem && Array.isArray(targetItem.componentList)) {
                targetComponentList = targetItem.componentList;
            }
        }

        if (initialLoad || !listAtter.value || listAtter.value.length === 0) {
            listZhong.value = targetItem || res?.main_data?.item?.[0];
            listAtter.value = targetComponentList;
        } else {
            // 非首次加载：只把新数据合并到已有组件中，不替换整个数组引用，避免所有组件被卸载重绘
            const newList = targetComponentList || [];
            listZhong.value = targetItem || res?.main_data?.item?.[0];
            // 用新数据逐项覆盖，保留原有数组引用与组件实例
            const existingList: any[] = listAtter.value || [];
            for (let i = 0; i < newList.length; i++) {
                const oldItem = existingList[i];
                const newItem = newList[i];
                if (!oldItem || oldItem.id !== newItem.id) {
                    // 结构发生变化时才替换对应下标的项
                    existingList[i] = newItem;
                } else {
                    // 只更新会变化的字段，其它配置保持不变
                    if (oldItem.option && newItem.option) {
                        if (off) {
                            // 按钮事件帧：eventHeapListChange 已在 newItem（chart_list）上套好按钮样式，
                            // 全量 copy 到 oldItem（listAtter 渲染对象），dataset 已在 shi_shi_shu_ju 更新过
                            for (const key in newItem.option) {
                                oldItem.option[key] = newItem.option[key];
                            }
                        } else {
                            // WS 实时帧：只 copy dataset/series/visualMap
                            oldItem.option.dataset = newItem.option.dataset;
                            if (oldItem.option.series && newItem.option.series) {
                                oldItem.option.series = newItem.option.series;
                            }
                            if (oldItem.option.visualMap && newItem.option.visualMap) {
                                oldItem.option.visualMap = newItem.option.visualMap;
                            }
                            // 实时数据优先级最高：按钮临时改过样式的组件，WS 帧里还原为后端原始配置
                            if (eventOverrideIds.has(oldItem.id)) {
                                for (const key in newItem.option) {
                                    if (key === 'dataset' || key === 'series' || key === 'visualMap') continue;
                                    oldItem.option[key] = newItem.option[key];
                                }
                                eventOverrideIds.delete(oldItem.id);
                            }
                        }
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

        if (!pageOverride) {
            localStorage.setItem("zhutaineirong",JSONStringify(res));//存在这是因为本页面的事件修改[chunyixiazhutai]本地存储超限，别的地方不可使用
        }
    }

    // 组件渲染结束初始化数据池
    clearMittDataPondMap()

    const currentPageTitle = () => JSONParse(sessionStorage.getItem("chunyixiazhutai") as any)?.title
    const syncPreviewLocation = (targetTitle: string) => {
        const [hashPath, hashQuery] = window.location.hash.split('?');
        const pathParts = hashPath.split('/');
        if (pathParts.length < 2) return;
        pathParts[pathParts.length - 1] = encodeURIComponent(targetTitle);
        const nextHash = `${pathParts.join('/')}${hashQuery ? `?${hashQuery}` : ''}`;
        window.history.replaceState(window.history.state, '', `${window.location.pathname}${window.location.search}${nextHash}`);
        document.title = targetTitle;
    }
    const handleOpenPage = async (event: Event) => {
        const targetTitle = String((event as CustomEvent)?.detail?.pageValue || '').trim();
        if (!targetTitle) {
            window['$message']?.warning('未配置目标页面');
            return;
        }
        const cachedProject = JSONParse(localStorage.getItem("zhutaineirong") as any);
        const targetPage = Array.isArray(cachedProject?.main_data?.item)
            ? cachedProject.main_data.item.find((item: any) => item?.title === targetTitle)
            : undefined;
        if (!targetPage) {
            window['$message']?.warning(`目标页面“${targetTitle}”不存在，请保存项目后重试`);
            return;
        }
        // 预览启动时只注册了初始画布的组件。目标画布可能包含不同类型的组件，
        // 必须在切换前补充注册，否则数据已切换但动态组件无法渲染，页面会只剩背景。
        const pageData = JSONParse(JSONStringify(targetPage));
        const componentList = Array.isArray(pageData.componentList) ? pageData.componentList : [];
        installPreviewComponents(componentList);

        // 同步完整画布状态。部分图表会从 Pinia 读取画布配置或组件列表，
        // 只替换 PreviewRenderList 的数组会导致目标画布有数据但组件无法渲染。
        chartEditStore.editCanvasConfig = pageData.editCanvasConfig || chartEditStore.editCanvasConfig;
        chartEditStore.requestGlobalConfig = pageData.requestGlobalConfig || chartEditStore.requestGlobalConfig;
        chartEditStore.componentList = componentList;
        chartEditStore.setCurrentCanvasTitle(targetTitle);
        sessionStorage.setItem("chunyixiazhutai", JSONStringify(pageData));
        await getList(listSocket.value, targetTitle, undefined, true, pageData);
        syncPreviewLocation(targetTitle);
    }
    const handleNoGoLive = (event: Event) => {
        const title = currentPageTitle();
        if (title) void getList(listSocket.value, title, (event as CustomEvent).detail, true);
    }
    const handleGotoShow = (event: Event) => {
        const title = currentPageTitle();
        if (title) void getList(listSocket.value, title, (event as CustomEvent).detail, true);
    }
    const handleAttrChange = (event: Event) => {
        const title = currentPageTitle();
        if (title) void getList(listSocket.value, title, (event as CustomEvent).detail, false);
    }

    onMounted(async () => {
        // 事件监听必须先于任何异步数据请求注册。
        // 从包含历史表格的画布直接进入预览时，历史接口较慢或失败不能阻断按钮点击。
        eventHighway.to("openPage", handleOpenPage)
        eventHighway.to("noGoLive", handleNoGoLive)
        eventHighway.to("gotoShow", handleGotoShow)
        eventHighway.to("attrChange", handleAttrChange)

        // 首次加载：先调一次 getList 建立完整的 listAtter 引用
        if (sessionStorage.getItem('wk_ApiBaseUrl')) {
            listAtter.value = chartEditStore.componentList;
            listAtter.value.forEach((component: any) => eventHeapListChange(component, undefined));
            const historyComponents = listAtter.value.flatMap((component: any) =>
                Array.isArray(component?.groupList) ? component.groupList : [component]
            ).filter((component: any) => component?.request?.requestHttpType === '历史数据' && !isHistoryQueryChart(component));
            await Promise.allSettled(historyComponents.map(async (component: any) => {
                const range = component.request?.requestInterval;
                if (!Array.isArray(range) || range.length < 2) return;
                const history: any = await apiAnalysisQueryV3_2({
                    start: range[0],
                    end: range[1],
                    pairs: component.request?.requestParams?.Params || [],
                    page: component.option?.pagination?.page || 1,
                    pageSize: component.option?.pagination?.pageSize || 10
                });
                if (history?.dataset) {
                    component.option.dataset = history.dataset;
                    if (component.option?.pagination) {
                        component.option.pagination.page = history.page || 1;
                        component.option.pagination.pageSize = history.pageSize || component.option.pagination.pageSize;
                        component.option.pagination.itemCount = Number(history.total) || 0;
                    }
                }
            }));
            const hashPath = window.location.hash.split('?')[0].split('/');
            const routeCanvasTitle = decodeURIComponent(hashPath[4] || '');
            const activeCanvasTitle = routeCanvasTitle || chartEditStore.getCurrentCanvasTitle || '画布1';
            chartEditStore.setCurrentCanvasTitle(activeCanvasTitle);
            const page = { title: activeCanvasTitle, ...chartEditStore.getStorageInfo() };
            listZhong.value = page;
            sessionStorage.setItem('chunyixiazhutai', JSONStringify(page));
            // 页面跳转依赖完整项目页面列表。当前编辑页用内存中的最新内容覆盖后端版本，
            // 既保留尚未保存的修改，也避免只缓存当前页造成目标页查找失败。
            try {
                const project = await apiV1configDatas_test({ config_id: wk_Config_id.value });
                const pages = Array.isArray(project?.main_data?.item) ? [...project.main_data.item] : [];
                const currentIndex = pages.findIndex((item: any) => item?.title === page.title);
                if (currentIndex >= 0) pages[currentIndex] = page;
                else pages.push(page);
                localStorage.setItem('zhutaineirong', JSONStringify({ ...project, main_data: { ...(project?.main_data || {}), item: pages } }));
            } catch (error) {
                // 项目页列表临时加载失败时仍保留当前画布，避免预览直接白屏。
                localStorage.setItem('zhutaineirong', JSONStringify({ main_data: { item: [page] } }));
                window['$message']?.warning('项目页面加载失败，页面跳转暂不可用，请稍后重试');
            }
        } else {
            await getList(listSocket.value, undefined, undefined, true);
        }
        let wsUrl = `${baseurlAddress.wss}/api/configuration/point-bindings/realtime?config_id=${wk_Config_id.value}`;
        if (!sessionStorage.getItem('wk_ApiBaseUrl')) allConnection(wsUrl).then((ws) => {
            ws.onmessage = (e: any) => {
                let data = JSONParse(e.data);
                listSocket.value.push(data.data);
                listSocket.value = listSocket.value.length > 5 ? listSocket.value.splice(-5) : listSocket.value;
                getList(listSocket.value);
            };
        })
        initDataPond(useChartEditStore);
    })

    onBeforeUnmount(() => {
        stopConditions();conditionRunner.reset();
        window.removeEventListener("openPage", handleOpenPage)
        window.removeEventListener("noGoLive", handleNoGoLive)
        window.removeEventListener("gotoShow", handleGotoShow)
        window.removeEventListener("attrChange", handleAttrChange)
    })

    /**
     * 处理渲染时候的实时数据
     */
    let shi_shi_shu_ju = (list: any,ear: any) => {
        list.forEach((item: any) => {
            if (!item?.option) return
            // 历史表格由服务端分页接口独立加载，不能再套用实时数据帧。
            // 否则每次 WebSocket 刷新都会重写表头并把历史数据清空。
            if (item.request?.requestHttpType === '历史数据') return
            // CirclePoint 状态圆点：按匹配值驱动颜色（必须在最前并 return，避免落入下方数字分支被 listCount 污染）
            if (item.chartConfig?.key === 'CirclePoint' || item.chartConfig?.key === 'SwitchFun') {
                const params = item.request?.requestParams?.Params || []
                const latestBatch = Array.isArray(ear) && ear.length > 0 ? ear[ear.length - 1] : null
                if (!Array.isArray(latestBatch)) return
                for (const param of params) {
                    if (!param?.value1 || !param?.value2) continue
                    const matched = latestBatch.filter((countList: any) => countList.dev_id == param.value1 && countList.identifier == param.value2)
                    if (matched.length > 0) {
                        const val = matched[0].value
                        const isNumeric = typeof val === 'number' || (typeof val === 'string' && /^-?\d+(\.\d+)?$/.test(val))
                        item.option.dataset = item.chartConfig?.key === 'SwitchFun' ? val : isNumeric ? parseFloat(val) : val
                    }
                }
                return
            }
            if (item.option.dataset && item.option.dataset.dimensions && item.option.dataset.source) {
                item.option.dataset.dimensions = ["product", ...((item.request?.requestParams?.Params || []) as any[]).map((key: any) => key.value2)];
                item.option.dataset.source = transformData(ear);
            } else if (typeof item.option.dataset == "number" || typeof item.option.dataset == "string" || typeof item.option.dataset == "boolean") {
                const params = item.request?.requestParams?.Params;
                if (Array.isArray(params) && params.length > 0 && params[0]?.value1 && params[0]?.value2) {
                    const latestBatch = Array.isArray(ear) && ear.length > 0 ? ear[ear.length - 1] : null;
                    if (Array.isArray(latestBatch)) {
                        const matched = latestBatch.filter((countList: any) => countList.dev_id == params[0].value1 && countList.identifier == params[0].value2);
                        if (matched.length > 0) {
                            let typeType = typeof matched[0].value == "number" ? parseFloat(matched[0].value) : typeof matched[0].value == "string" || typeof matched[0].value == "boolean" ? matched[0].value : undefined;
                            item.option.dataset = typeType;
                            listCount.value = typeType;
                        } else {
                            item.option.dataset = listCount.value;
                        }
                    }
                }
            }
        })
    }

    /**
     * 处理实时数据
     */
    function transformData(data: any) {
        if (!Array.isArray(data)) return [];
        let allIdentifiers: any = new Set();
        for (let group of data) {
            if (!Array.isArray(group)) continue;
            for (let item of group) {
                allIdentifiers.add(item.identifier);
            }
        }
        let result = [];
        for (let group of data) {
            if (!Array.isArray(group) || group.length === 0) continue;
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
        // 更新属性：套用事件保存的样式（跳过 dataset，保持实时数据追踪链），下一帧 WS 自动还原
        if(off && off.changeAttr == "更新属性"){
            let sAttr = off.saveAttrArr;
            if(sAttr && typeof sAttr === 'object' && 'value' in sAttr && !('option' in sAttr)){
                sAttr = sAttr.value;
            }
            if(!sAttr) return;
            let matched = false;
            let targetIds: string[] = [];
            if(Array.isArray(off.targetList) && off.targetList.length){
                targetIds = off.targetList.map((i: any) => i?.id).filter((v: any) => !!v);
            }
            if(targetIds.length === 0 && off.form?.targetComponent){
                const tc = off.form.targetComponent;
                targetIds = Array.isArray(tc) ? tc : [tc];
            }
            if(targetIds.includes(eventList.id)){
                matched = true;
            } else if(sAttr.chartConfig?.title && eventList.chartConfig?.title === sAttr.chartConfig.title && eventList.chartConfig?.conKey === sAttr.chartConfig.conKey){
                matched = true;
            }
            if(matched){
                eventOverrideIds.add(eventList.id);
                const srcOpt = sAttr.option || {};
                for (const key in srcOpt) {
                    if (key !== 'dataset') eventList.option[key] = srcOpt[key];
                }
                if(sAttr.attr) Object.assign(eventList.attr, sAttr.attr);
                if(sAttr.styles) Object.assign(eventList.styles, sAttr.styles);
                if(sAttr.chartConfig) Object.assign(eventList.chartConfig, sAttr.chartConfig);
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

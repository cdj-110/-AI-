<template>
    <div class="binding-config">
        <setting-item-box v-if="!isGeoTrue()" name="数据模型" :itemRightStyle="{ gridTemplateColumns: '1fr' }">
            <setting-item>
                <div v-if="currentSelectOptions.length === 1" class="single-data-model">{{ currentSelectOptions[0].label }}</div>
                <n-select v-else v-model:value="requestHttpType" :options="currentSelectOptions" placeholder="请选择数据模型" />
                <div class="section-desc">{{ capabilityDescription }}</div>
            </setting-item>
        </setting-item-box>
        <setting-item-box v-if="isGeoTrue()" name="设备列表" :itemRightStyle="{ gridTemplateColumns: '1fr' }">
            <setting-item>
                <n-select v-model:value="selecteValue" filterable multiple placeholder="搜索设备列表" :options="select" />
            </setting-item>
        </setting-item-box>
        <div v-if="!isGeoTrue()" class="point-section">
            <div class="section-heading">
                <div>
                    <div class="section-title">绑定点位</div>
                    <div class="section-desc">{{ bindingDescription }}</div>
                </div>
                <div class="section-actions">
                    <n-tag size="small" type="info" :bordered="false">已选 {{ validPointCount }} 个</n-tag>
                    <n-button size="small" secondary type="primary" @click="propertyChange(1, propertyList.length)">
                        <template #icon><n-icon><AddIcon /></n-icon></template>
                        添加点位
                    </n-button>
                </div>
            </div>

            <div class="point-table">
                <div class="point-table__header">
                    <span>序号</span>
                    <span>设备</span>
                    <span>点位</span>
                    <span>操作</span>
                </div>
                <div class="point-row" v-for="(item, index) in propertyList" :key="index">
                    <div class="point-index">{{ index + 1 }}</div>
                    <div>
                        <n-select v-model:value="item.value1" filterable :options="select" placeholder="请选择设备" clearable @update:value="((e: any) => handleUpdateValue(e,index))"/>
                        <span class="field-error" v-if="!item.value1 && props.isRuleTrue">请选择设备</span>
                    </div>
                    <div>
                        <n-select v-model:value="item.value2" filterable :disabled="!item.value1" :options="item.TypeOptions" placeholder="请选择点位" />
                        <span class="field-error" v-if="!item.value2 && props.isRuleTrue">请选择点位</span>
                    </div>
                    <n-button quaternary type="error" size="small" @click="propertyChange(2,index)">移除</n-button>
                </div>
            </div>

            <n-alert v-if="duplicatePointCount" type="warning" :show-icon="false" class="duplicate-alert">
                存在 {{ duplicatePointCount }} 个重复点位，请移除后再绑定。
            </n-alert>
        </div>

        <div class="time-section" v-if="requestHttpType == '历史数据' && !isGeoTrue()">
            <div v-if="supportsHistoryChartQuery(props.targetList)" class="section-heading">
                <div>
                    <div class="section-title">历史查询配置</div>
                    <div class="section-desc">在预览页面显示时间筛选、快捷时间、查询与重置；关闭后按默认时间查询</div>
                </div>
                <n-switch v-model:value="showHistoryQuery" />
            </div>
            <div class="section-heading section-heading--time">
                <div>
                    <div class="section-title">默认查询时间</div>
                    <div class="section-desc">预览时默认读取此时间范围，开启查询栏后可再次调整</div>
                </div>
                <n-space size="small">
                    <n-button v-for="range in quickRanges" :key="range.key" size="tiny"
                        :type="selectedQuickRange === range.key ? 'primary' : 'default'"
                        :secondary="selectedQuickRange === range.key"
                        :disabled="!historyHoursAllowed(range.hours)" @click="applyQuickRange(range.key, range.hours)">{{ range.label }}</n-button>
                </n-space>
            </div>
            <n-date-picker class="time-picker" v-model:formatted-value="requestInterval" value-format="yyyy-MM-dd HH:mm:ss"
                type="datetimerange" :is-date-disabled="historyDateDisabled" clearable @update:formatted-value="selectedQuickRange = 'custom'" />
            <div class="section-desc">{{ historyWindowDescription() }}。缺失点位值不补0；升级不能恢复已清理数据。</div>
        </div>

        <div v-if="requestHttpType == '历史数据' && !isGeoTrue()" class="test-section">
            <div class="test-section__copy">
                <div class="section-title">连接测试</div>
                <div class="section-desc">保存前验证当前点位和时间范围能否查询到数据</div>
            </div>
            <n-button type="primary" secondary :loading="testLoading" :disabled="!canTestQuery" @click="testQuery">测试查询</n-button>
            <n-alert v-if="testResult" :type="testResult.type" :show-icon="true" class="test-result">
                {{ testResult.message }}
            </n-alert>
        </div>
    </div>
</template>

<script setup lang="ts">
import { PropType, toRefs, onMounted, ref, watch, computed } from 'vue'
import { SettingItemBox, SettingItem } from '@/components/Pages/ChartItemSetting'
import { selectTypeOptions, selectTimeOptions } from '@/views/chart/ContentConfigurations/components/ChartData/index.d'
import { RequestConfigType } from '@/store/modules/chartEditStore/chartEditStore.d'
import { icon } from '@/plugins'
import { deviceinfo, device_detail_function, apiAnalysisQueryV3_2 } from '@/api/index'
import { useMessage } from 'naive-ui'
import { componentKeyOf, platformDataModelsOf } from '@/utils/platformBindingCapabilities'
import { supportsHistoryChartQuery } from '@/utils/platformHistoryChart'
import { refreshHistoryWindow, historyDateDisabled, historyHoursAllowed, historyWindowDescription } from '@/utils/historyRetentionWindow'
onMounted(() => { void refreshHistoryWindow().catch(() => Elmessage.warning('无法读取历史保存期限，请重新打开绑定配置')) })

let Elmessage = useMessage();
const props = defineProps({
    isRuleTrue: Boolean,
    targetDataRequest: Object as PropType<RequestConfigType>,
    targetList: Object
})

const isSwitchComponent = computed(() => componentKeyOf(props.targetList) === 'SwitchFun')
const currentSelectOptions = computed(() => {
    const supported = platformDataModelsOf(props.targetList)
    return selectTypeOptions.filter(item => supported.includes(item.value as '实时数据' | '历史数据'))
})
const capabilityDescription = computed(() => currentSelectOptions.value.length === 1
    ? `此组件仅支持${currentSelectOptions.value[0].label}绑定`
    : '此组件支持实时数据与历史数据，可按展示需求选择')
const showHistoryQuery = computed({
    get: () => props.targetDataRequest?.historyQuery?.enabled !== false,
    set: (enabled: boolean) => {
        if (props.targetDataRequest) props.targetDataRequest.historyQuery = { enabled }
    }
})

const quickRanges = [
    { key: '1h', label: '最近 1 小时', hours: 1 },
    { key: '6h', label: '最近 6 小时', hours: 6 },
    { key: '24h', label: '最近 24 小时', hours: 24 },
    { key: '7d', label: '最近 7 天', hours: 24 * 7 }
]
const selectedQuickRange = ref('custom')
const testLoading = ref(false)
const testResult = ref<{ type: 'success' | 'warning' | 'error', message: string } | null>(null)

const { AddIcon, CloseIcon } = icon.ionicons5;
const { requestInterval, requestIntervalUnit, requestParams, requestHttpType, requestUrl } = toRefs(
    props.targetDataRequest as RequestConfigType
)
const bindingDescription = computed(() => requestHttpType.value === '历史数据'
    ? '查询所选点位的历史记录，按时间展示并支持分页'
    : isSwitchComponent.value ? '开关控制只绑定一个实时点位；需在开关配置中开启允许下发，运行时按网关可写能力校验'
    : '接收所选点位的最新上报值；单值组件展示第一个点位，图表持续更新')
let select: any = ref([]);
let selecteValue = ref([]);//地图选择的所有id
let selecteArr = ref([]);//保存地图匹配的数据项
let propertyList:any = ref([ { value1: "", value2:"" } ]);//设备、属性选择

let userId = ref(sessionStorage.getItem('wk_UserId'))

const validPoints = computed(() => propertyList.value.filter((item: any) => item.value1 && item.value2))
const validPointCount = computed(() => validPoints.value.length)
const duplicatePointCount = computed(() => {
    const keys = validPoints.value.map((item: any) => `${item.value1}::${item.value2}`)
    return keys.length - new Set(keys).size
})
const canTestQuery = computed(() => {
    return validPointCount.value > 0 &&
        duplicatePointCount.value === 0 &&
        Array.isArray(requestInterval.value) &&
        requestInterval.value.length === 2
})

const formatDateTime = (date: Date) => {
    const pad = (value: number) => String(value).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const applyQuickRange = (key: string, hours: number) => {
    const end = new Date()
    const start = new Date(end.getTime() - hours * 60 * 60 * 1000)
    selectedQuickRange.value = key
    requestInterval.value = [formatDateTime(start), formatDateTime(end)] as any
    testResult.value = null
}

const testQuery = async () => {
    if (!canTestQuery.value) return
    testLoading.value = true
    testResult.value = null
    try {
        const result: any = await apiAnalysisQueryV3_2({
            user_id: userId.value,
            start: (requestInterval.value as any)[0],
            end: (requestInterval.value as any)[1],
            pairs: validPoints.value,
            page: 1,
            pageSize: 5
        })
        const total = Number(result?.total) || 0
        const previewCount = Array.isArray(result?.dataset?.source) ? result.dataset.source.length : 0
        testResult.value = total > 0
            ? { type: 'success', message: `连接成功，共查询到 ${total} 条记录，已预览验证 ${previewCount} 条。` }
            : { type: 'warning', message: '连接成功，但当前点位在所选时间范围内没有数据。' }
    } catch (error: any) {
        testResult.value = { type: 'error', message: error?.message || '查询失败，请检查设备点位或时间范围。' }
    } finally {
        testLoading.value = false
    }
}

watch(selecteValue,(val) => {
    let idSet: any = new Set(val);
    selecteArr.value = select.value.filter((item: any) => idSet.has(item.dev_sn));
    requestParams.value['Params'] = val.map(item => ({value1: item, value2: "地图"}));
    requestHttpType.value = "地图标点-高德";
},{deep: true});

watch(currentSelectOptions, (options) => {
    if (!isGeoTrue() && options.length && !options.some(item => item.value === requestHttpType.value)) {
        requestHttpType.value = options[0].value
    }
}, { immediate: true })

/**
 * 判断是否是高德地图
 */
 function isGeoTrue() {
    if(props.targetList && props.targetList.chartConfig){
        return props.targetList.chartConfig.chartKey == 'VMapAmap' && props.targetList.chartConfig.title == '高德地图';
    }else{
        return false;
    }
}

/**
 * 渲染设备
 */
let list1 = () => {
    deviceinfo({user_id: userId.value}).then((res: any) => {
        if(res.code == 200 ){
            select.value = (res.data.data || []).map((item: any) => ({
                label: item.dev_name,
                value: item.dev_id,
                ...item
            })).filter((item: any) => item.model_id);
        }
    })
}

/**
 * 渲染属性
 */
let handleUpdateValue = (e:any,i: any) => {
    propertyList.value[i].TypeOptions = [];
    const found = select.value.filter((item:any) => item.dev_id == e)[0]
    if (!found) return
    device_detail_function({user_id: userId.value,model_id: found.model_id}).then((res:any) => {
         if(res.code == 200 ){
            propertyList.value[i].TypeOptions = (res.data.data || []).map((item: any) => {
                const dataType = item.data_type || ''
                const typeLabel = dataType ? ` (${dataType})` : ''
                const disabled = false // 开关支持可写数值点位；写入能力由运行时及后端再次校验。
                return {
                    label: item.identifier + typeLabel,
                    value: item.identifier,
                    disabled,
                    dataType
                }
            })
        }
    })

}

watch(propertyList,(val) => {
    requestParams.value['Params'] = val
    testResult.value = null
},{deep:true});

// 将 Params 规范化为数组（兼容 lodash merge 可能产生的对象结构）
const normalizeParams = (params: any): any[] => {
    if (!params) return []
    if (Array.isArray(params)) return params
    if (typeof params === 'object') {
        return Object.values(params).filter((v: any) => v && typeof v === 'object')
    }
    return []
}

// 初始化时恢复已保存数据（不放在deep watch内部，避免赋值触发循环）
onMounted(() => {
    list1()
    const savedParams = normalizeParams(props.targetDataRequest?.requestParams["Params"])
    if (savedParams.length > 0) {
        propertyList.value = savedParams;
        // 延迟到list1完成后再加载TypeOptions（设备列表需要先就绪）
        setTimeout(() => {
            propertyList.value.forEach((item: any, idx: number) => {
                if (item.value1) {
                    handleUpdateValue(item.value1, idx)
                }
            })
            if(isGeoTrue()){
                selecteValue.value = savedParams.map((item: any) => item.value1);
            }
        }, 200)
    }
    if (requestHttpType.value === '历史数据' && (!Array.isArray(requestInterval.value) || requestInterval.value.length < 2)) {
        applyQuickRange('1h', 1)
    }
})

/**
 * 新增删除设备/属性
 * @inner (type) 1:新增 2:删除
 * @inner (i) 要删除的下标
 */
let propertyChange = (type:number, i:string | number) => {
    if(type == 1) {
        propertyList.value.push({ value1: "", value2:"" });
    }else if(type == 2) {
        if(propertyList.value.length <= 1){
            Elmessage.warning("至少选择一个点位！");
        }else{
            propertyList.value.splice(i,1);
        }
    }
}
</script>

<style lang="scss" scoped>
.binding-config {
    padding: 4px 6px 18px 0;
}
.single-data-model {
    min-height: 34px;
    display: flex;
    align-items: center;
    font-size: 14px;
    font-weight: 500;
}

.point-section,
.time-section,
.test-section {
    margin-top: 18px;
    padding: 16px;
    border: 1px solid rgba(128, 128, 128, 0.22);
    border-radius: 8px;
    background: rgba(128, 128, 128, 0.04);
}

.section-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 14px;
}

.section-heading--time {
    align-items: flex-end;
}

.section-title {
    font-size: 14px;
    font-weight: 600;
}

.section-desc {
    margin-top: 4px;
    color: rgba(128, 128, 128, 0.9);
    font-size: 12px;
}

.section-actions {
    display: flex;
    align-items: center;
    gap: 10px;
}

.point-table {
    overflow: hidden;
    border: 1px solid rgba(128, 128, 128, 0.18);
    border-radius: 6px;
}

.point-table__header,
.point-row {
    display: grid;
    grid-template-columns: 52px minmax(220px, 1fr) minmax(220px, 1fr) 68px;
    align-items: start;
    gap: 12px;
}

.point-table__header {
    padding: 9px 12px;
    color: rgba(128, 128, 128, 0.95);
    font-size: 12px;
    background: rgba(128, 128, 128, 0.08);
}

.point-row {
    min-height: 34px;
    padding: 10px 12px;
    border-top: 1px solid rgba(128, 128, 128, 0.14);
}

.point-index {
    padding-top: 7px;
    color: rgba(128, 128, 128, 0.95);
    text-align: center;
}

.field-error {
    display: block;
    margin-top: 4px;
    color: rgba(255, 74, 74, 0.9);
    font-size: 12px;
}

.duplicate-alert,
.test-result {
    margin-top: 12px;
}

.time-picker {
    width: 100%;
}

.test-section {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 12px;
}

.test-result {
    grid-column: 1 / -1;
}

@media (max-width: 900px) {
    .section-heading,
    .section-heading--time {
        align-items: flex-start;
        flex-direction: column;
    }

    .point-table__header,
    .point-row {
        grid-template-columns: 42px minmax(170px, 1fr) minmax(170px, 1fr) 60px;
    }
}
</style>

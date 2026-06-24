<template>
    <div style="overflow: auto; height: 58vh;padding: 0 0.5rem 0 0;">
        <el-alert type="warning" :closable="false" v-if="props.status == 0">
            <template #title>{{ t('generalView.deviceOffline') }}<span
                    style="color: #3C93E7;cursor: pointer;font-weight: 700;margin-left: 1rem;"
                    @click="emit('activeChange', 2)">⌜{{ t('generalView.deviceNetworking') }}⌟</span></template>
        </el-alert>
        <div style="display: flex;align-items: center;justify-content: space-between;margin: 2rem 0;">
            <div style="font-size: 1.3rem;font-weight: 700;">{{ t('generalView.currentProperties') }}</div>
            <div style="display: flex;align-items: center;">
                <el-button type="primary" icon="Refresh" style="margin-left: 1rem;" @click="getList()">{{ t('generalView.refresh') }}</el-button>
            </div>
        </div>
        <el-alert type="warning" :closable="false" v-if="!natureList || natureList.length <= 0">
            <template #title>{{ t('generalView.noProperties') }}<span
                    style="color: #3C93E7;cursor: pointer;font-weight: 700;margin-left: 1rem;"
                    @click="emit('activeChange', 3)">⌜{{ t('generalView.functionalProperties') }}⌟</span></template>
        </el-alert>
        <div v-if="natureList && natureList.length > 0"
            style="display: flex;justify-content: flex-start;flex-wrap: wrap;gap: 1.4rem;padding: 0 0 2rem 0;">
            <div v-for="(item, index) in natureList" :key="index"
                style="width: calc((100% - 4 * 1.4rem) / 5);border: 1px solid #c0c5cc;border-radius: 7px;margin:0.7rem 0;overflow: hidden;">
                <!-- 标题栏 -->
                <div
                    style="display: flex;align-items: center;justify-content: space-between;padding: 0.7rem 1rem;border-bottom: 1px solid #c0c5cc;">
                    <p style="font-size: 1rem;font-weight: 500;flex:1;min-width: 0;">
                        <el-tooltip :content="item.property_name" placement="top"
                            v-if="item.property_name.length >= 10">
                            <span :class="{ 'ellipsis-text': item.property_name.length >= 10 }">{{ item.property_name
                                }}</span>
                        </el-tooltip>
                        <span v-else>{{ item.property_name }}</span>
                    </p>
                    <p style="display: flex;align-items: center;">
                        <el-dropdown trigger="click" @command="handleCommand">
                            <el-icon size="20" style="cursor: pointer;">
                                <MoreFilled />
                            </el-icon>
                            <template #dropdown>
                                <el-dropdown-menu>
                                    <el-dropdown-item command="add-to-panel">
                                        <el-icon style="margin-right: 4px;">
                                            <Histogram />
                                        </el-icon>
                                        {{ t('generalView.addToPanel') }}
                                    </el-dropdown-item>
                                </el-dropdown-menu>
                            </template>
                        </el-dropdown>
                    </p>
                </div>
                <div style="padding: 1.5rem 1rem;">
                    <div v-if="item.data_type == 'number'">
                        <div style="font-size: 0.8rem;color:#CAD4E4;margin-bottom: 0.5rem;">{{ t('generalView.currentValue') }}</div>
                        <div>
                            <span style="color: #5771E7;font-size: 1.3rem;font-weight: 500;margin: 0 0.2rem 0 0;">
                                {{ formatPropertyValue(item.property_value) }}
                            </span>
                            <span style="font-size: 14px;margin-top: 0.3rem;">{{ item.unit }}</span>
                        </div>
                    </div>

                    <!-- 开关类型（空气开关） -->
                    <div v-else>
                        <div style="font-size: 0.8rem;color:#CAD4E4;margin-bottom: 0.5rem;">{{ t('generalView.currentStatus') }}</div>
                        <div>
                            <span style="font-size: 1rem;font-weight: 500;">
                                <el-tooltip class="box-item" effect="dark" :content="item.displayText || '--'"
                                    placement="top-start">
                                    <span class="text"> {{ item.displayText || '--' }} </span>
                                </el-tooltip>
                            </span>
                            <el-switch style="margin-left: 0.5rem;" v-model="item.property_value" active-value="1"  v-if="item.read_write_type !== 'r'"
                                inactive-value="0" @change="handleSwitchChange(item)" />
                        </div>
                    </div>
                </div>

                <!-- 底部：更新时间 + 下发按钮 -->
                <div
                    style="display: flex;align-items: center;justify-content: space-between;padding: 0.7rem 1rem;background-color: #f8f9fa;border-top: 1px solid #c0c5cc;">
                    <span style="font-size: 12px;color: #909399;">
                        <!-- <el-icon style="margin-right: 4px;" size="14">
                            <Clock />
                        </el-icon> -->
                        <span v-if="item.time">更新于:刚刚</span>
                        <span v-else>
                            <span v-if="item.collection_times">
                                {{ formatTime(item.collection_times) }}
                            </span>
                        </span>
                    </span>
                    <el-button type="primary" size="small" @click="handleSendProperty(item)"
                        v-if="item.read_write_type !== 'r'">
                        下发数据
                    </el-button>
                </div>
            </div>
        </div>
        <sendProperty v-if="isSendProperty" @handleClose="handleClose" :SendPropertyObj="SendPropertyObj" :dev_id="props.dev_id" @sendData="sendData">
        </sendProperty>
    </div>
</template>

<script lang="ts" setup>
import { socketUrl } from '@/utils/request';
import { ElTooltip, ElIcon, ElDropdown, ElDropdownMenu, ElDropdownItem, ElSwitch, ElButton } from 'element-plus'
import { Download, MoreFilled, Histogram, Clock } from '@element-plus/icons-vue'
import { allConnection } from '@/utils/allWebSocketConnction';
import { device_detail_function, push_attributes } from "@/api/facilityList/index";
import useCounterStore from "@/stores/counter";
import sendProperty from './sendProperty.vue'
import { formatTime } from '@/utils/publicFun'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
const props = defineProps({
    dev_id: {
        type: String,
        required: ''
    },
    model_id: {
        type: String,
        required: ''
    },
    status: {
        type: Number,
        required: ''
    },

});
let emit = defineEmits(["activeChange"]);
let natureList = ref([]);//设备概览 - 采集的数据
let isSendProperty = ref(false);
let SendPropertyObj = ref({})
// 处理嵌套对象数据（修改版：保留顶层 send_time / send time）
const processNestedData = (data: any) => {
    const result = [];
    // 先把顶层的时间字段存下来
    const sendTime = data.send_time || data['send time'];

    Object.keys(data).forEach(key => {
        const value = data[key];

        // 跳过时间字段，避免重复处理
        if (key === 'send_time' || key === 'send time') return;

        // 如果是对象且包含多个键值对
        if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
            const firstKey = Object.keys(value)[0];
            const firstValue = value[firstKey];

            // 添加到结果数组，同时带上时间
            result.push({
                title: firstKey,
                value: firstValue,
                send_time: sendTime // 把时间存进去
            });
            // 将剩余的键值对提取为新的对象
            const remainingKeys = Object.keys(value).filter(k => k !== firstKey);
            if (remainingKeys.length > 0) {
                result.push({
                    title: '其他属性',
                    value: JSON.stringify(remainingKeys.map(k => ({ [k]: value[k] })), null, 2),
                    send_time: sendTime // 其他属性也带上时间
                });
            }
        } else {
            // 普通键值对，同时带上时间
            result.push({
                title: key,
                value: value,
                send_time: sendTime // 把时间存进去
            });
        }
    });
    return result;
};

const formatPropertyValue = (value: any): string | number => {
    // 定义空值规则：空串、undefined、null 视为无值
    if (value === '' || value === undefined || value === null) {
        return '--'
    }
    // 其他情况（包括0、false、数字、字符串等）返回原值
    return value
}

interface BoolPropertyItem {
    data_type: string;
    extended_info?: { 0: string; 1: string } | string;
    property_value: string | number | null;
    displayText?: string;
    [key: string]: any;
}

const DEFAULT_BOOL_CONFIG = { 0: '关', 1: '开' };
const formatBoolDisplay = (item: BoolPropertyItem): { displayText: string; property_value: string } => {
    const rawValue = item.property_value;

    // ✅ 这里就是默认值为0的地方
    const valueStr = (rawValue === '' || rawValue === null || rawValue === undefined) ? '0' : String(rawValue);
    const boolConfig = (item.extended_info && typeof item.extended_info === 'object' && !Array.isArray(item.extended_info))
        ? item.extended_info as { 0: string; 1: string }
        : DEFAULT_BOOL_CONFIG;

    const configKeys = Object.keys(boolConfig) as Array<'0' | '1'>;
    const displayText = configKeys.includes(valueStr as '0' | '1')
        ? boolConfig[valueStr as '0' | '1']
        : DEFAULT_BOOL_CONFIG[valueStr as '0' | '1'] || '--';

    return { displayText, property_value: valueStr };
};

let list = () => {
     device_detail_function({ user_id: user_id.value, model_id: props.model_id, dev_id: props.dev_id }).then((res: any) => {
            natureList.value = res.data.data
            // const WS_BASE_URL = import.meta.env.VITE_WEBSOCKET_API.replace(/\/$/, '')
            const WS_BASE_URL = socketUrl.replace(/\/$/, '')
            const dev_id = `${WS_BASE_URL}/api/mqtt_collection/data/realtime/${props.dev_id}`
            let list = []
            natureList.value.forEach((item: BoolPropertyItem) => {
                if (item.data_type === 'bool') {
                    const { displayText, property_value } = formatBoolDisplay(item);
                    item.displayText = displayText;
                    item.property_value = property_value;
                }
            });
            allConnection(dev_id).then((res) => {
                res.onmessage = (e: any) => {
                    const data = JSON.parse(e.data);
                    list = processNestedData(data);
                    natureList.value.forEach((item: BoolPropertyItem) => {
                        const matchingItem = list.find(l => l.title === item.identifier);
                        if (matchingItem) {
                            item.property_value = matchingItem.value;
                            item.time = matchingItem.send_time;
                        }
                        if (item.data_type === 'bool') {
                            const { displayText, property_value } = formatBoolDisplay(item);
                            item.displayText = displayText;
                            item.property_value = property_value;
                        }
                    });

                }
            })
        })
}

watch(props, (val) => {
    if (val.model_id) {
       list()
    }
}, { immediate: true, deep: true })
let getList = () => {
    // const WS_BASE_URL = import.meta.env.VITE_WEBSOCKET_API.replace(/\/$/, '')
    const WS_BASE_URL = socketUrl.replace(/\/$/, '')
    const dev_id = `${WS_BASE_URL}/api/mqtt_collection/data/realtime/${props.dev_id}`
    let vertest: any = []
    allConnection(dev_id).then((res) => {
        res.onmessage = (e: any) => {
            const data = JSON.parse(e.data);
            vertest = processNestedData(data);
            natureList.value.forEach((item: any) => {
                const matchingItem = vertest.find(l => l.title === item.identifier);
                if (matchingItem) {
                    item.property_value = matchingItem.value;
                }
            });
        }
    })
}
let sendData = () => {  list() }

// 下拉菜单操作
const handleCommand = (command: any) => {
    if (command === 'add-to-panel') {
    }
}

// 开关状态变更
const handleSwitchChange = (item: any) => {
    console.log(item.property_value)
    // 开关状态变化后，自动同步显示用户配置的对应文字
    const boolConfig = item.extended_info || { 0: '关', 1: '开' };
    item.displayText = boolConfig[item.property_value] || boolConfig['0'] || '--';
    let params = {
        "user_id": user_id.value,
        "dev_id": props.dev_id,
        "items": [
            {
                "identifier": item.identifier,
                "value":Number(item.property_value)
            }
        ]
    }
    push_attributes(params).then((res: any) => {
        if (res.code == 200) {
            getList()
            ElMessage.success(res.msg)
        } else {
            ElMessage.error(res.msg)
        }
    })
};

// 下发属性
const handleSendProperty = (item: any) => {
    SendPropertyObj.value = item
    isSendProperty.value = true

}
let handleClose = () => {
    isSendProperty.value = false
}
</script>
<style lang="scss" scoped>
:deep(.ellipsis-text) {
    white-space: nowrap !important;
    overflow: hidden !important;
    text-overflow: ellipsis !important;
    display: block !important;
    width: 70% !important;
}

:deep(.text) {
    display: inline-block;
    max-width: 80px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: middle;
}
</style>
<template>
    <div class="workPageOverview">
        <el-row :gutter="20">
            <el-col :span="15">
                <!-- 设备数 -->
                <div style="display: flex;justify-content: space-between;margin-top: 1rem;">
                    <div class="huizong" v-for="(item, index) in aggregation" :key="index">
                        <div>
                            <p style="font-size: 1.2rem;font-weight: 500;">{{ item.title }}</p>
                            <p style="font-size: 1.7rem;font-weight: 700;color: #7A7FFF;">{{ item.value }}</p>
                        </div>
                        <img :src="item.img" width="17%">
                    </div>
                </div>
                <!-- 设备接入类型 设备告警统计 -->
                <div style="display: flex;justify-content: space-between;margin-top: 2vh;">
                    <div
                        style="width: calc(100% / 2 - 1.4rem);height: 30vh;border: rgb(212, 212, 212) 1px solid;border-radius: 10px;background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);box-shadow: 0px 0px 12px rgba(0, 0, 0, 0.2);">
                        <VueEcharts style="width: 100%;height: 100%;" :option="pieOption" autoresize />
                    </div>
                    <div
                        style="width: calc(100% / 2 - 1.4rem);height: 30vh;border: rgb(212, 212, 212) 1px solid;border-radius: 10px;background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);box-shadow: 0px 0px 12px rgba(0, 0, 0, 0.2);">
                        <VueEcharts style="width: 100%;height: 100%;" :option="graphOption" autoresize />
                    </div>
                </div>

                <!-- 设备监控看板 -->
                <div
                    style="overflow: auto;padding: 1vh;margin-top: 2vh;height: 40vh;border: rgb(212, 212, 212) 1px solid;border-radius: 10px;background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);box-shadow: 0px 0px 12px rgba(0, 0, 0, 0.2);">
                    <div style="font-size: 1.2rem; font-weight: 700; color: #303133; margin: 0.3rem;">{{
                        t('overview.deviceMonitoringBoard') }}</div>
                    <!-- indicator-position="outside" overflow: auto;-->
                    <el-carousel :interval="5000">
                        <el-carousel-item v-for="(chunk, index) in chunkedOptions" :key="index">
                            <div style="display: flex; flex-wrap: wrap; justify-content: flex-start;">
                                <div class="option" v-for="(item, idx) in chunk" :key="idx"
                                    :style="getItemBorderColor(item.onOff)">
                                    <div style="display: flex; align-items: center;">
                                        <img :src="getItemIcon(item.onOff)" width="40px"
                                            style="margin-right: 0.8rem; border-radius: 4px;">
                                        <div>
                                            <p style="font-size: 0.9rem; font-weight: 700; color: #303133;">{{
                                                item.keyTitle }}</p>
                                            <p style="font-size: 0.7rem; color: #909399; margin-top: 0.2rem;">{{
                                                item.keyType }}</p>
                                        </div>
                                    </div>
                                    <div v-if="!item.onOff"
                                        style="font-size: 0.85rem; color: #303133; margin: 0.5rem 0; display: flex; align-items: center;">
                                        {{ t('overview.currentTemperature') }}：
                                        <span style="font-size: 1.2rem; font-weight: 700; margin-left: 0.3rem;">{{
                                            item.valueKey }}</span><span style="color:#DFDEDB">℃</span>
                                    </div>
                                    <div v-if="item.onOff"
                                        style="font-size: 0.8rem; color: #303133; margin: 0.5rem 0; display: flex; align-items: center;">
                                        {{ t('overview.switchStatus') }}： <p
                                            style="font-size: 1rem; font-weight: 700; margin-left: 0.3rem;">{{
                                                item.onOff == '2' ? t('overview.open') : t('overview.closed') }}</p>
                                        <el-switch style="margin-left: 0.5rem;" v-model="item.onOff" active-value="2"
                                            inactive-value="1" />
                                    </div>
                                    <div style="display: flex; align-items: center; justify-content: space-between;">
                                        <div
                                            style="font-size: 0.65rem; color: #909399; display: flex; align-items: center;">
                                            <el-icon style="margin-right: 0.2rem;">
                                                <Clock />
                                            </el-icon>{{ t('overview.updated') }}{{ item.time }}{{
                                                t('overview.minutesAgo') }}
                                        </div>
                                        <div @click="chartIsTrue = true;"
                                            style="font-size: 0.7rem; color: #ffffff; background: #2D8CF0; padding: 0.2rem 0.6rem; border-radius: 4px; display: flex; align-items: center; cursor: pointer;">
                                            <el-icon size="14" style="margin-right: 0.2rem;">
                                                <DataLine />
                                            </el-icon>{{ item.keyTitle == t('overview.airSwitch') ? t('overview.history') :
                                                t('overview.trend') }}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </el-carousel-item>
                    </el-carousel>
                </div>

            </el-col>
            <el-col :span="9">
                <!-- 版本信息 -->
                <div
                    style="overflow: auto;padding: 1rem;margin-top: 1rem;height: 43vh;border: 1px solid #e0e0e0;border-radius: 8px; background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);box-shadow: 0px 0px 12px rgba(0, 0, 0, 0.2);">
                    <div
                        style="font-size: 1rem;color: #333;white-space: nowrap;overflow: hidden;text-overflow: ellipsis;cursor: pointer;">
                        {{ t('overview.versionInfo') }}</div>
                    <div
                        style="font-size: 1.2rem;font-weight: 700;color: #7A7FFF;padding: 0.3rem 0;white-space: nowrap;overflow: hidden;text-overflow: ellipsis;cursor: pointer;">
                        {{ t('overview.standardVersion') }}</div>
                    <div v-for="(item, index) in progList" :key="index"
                        style="display: flex;align-items: center;margin: 0.3rem;">
                        <div
                            style="font-size: 0.8rem;color: #666;width: 13%;margin-left: 0.5rem;white-space: nowrap;overflow: hidden;text-overflow: ellipsis;">
                            <el-tooltip class="box-item" effect="dark" :content="item.title" placement="top"> {{
                                item.title }}： </el-tooltip>
                        </div>
                        <el-progress :percentage="item.value" :format="() => item.key"
                            :color="item.value > 80 ? '#f9c74f' : (item.value > 70 ? '#ff6b6b' : '#71B8D1')"
                            style="width: 87%;" />
                    </div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;justify-content: space-between;">
                        <p style="font-size: 0.9rem;color: #333;">{{ t('overview.upgradeToEnterprise') }}</p>
                        <el-button type="primary" size="small"
                            style="background: linear-gradient(270deg, #2AC2FF 0%, #5586FB 70%);border: none;font-size: 0.7rem;">{{
                                t('overview.buyNow') }}</el-button>
                    </div>
                    <ul style="font-size: 0.8rem;margin-left: 2rem;line-height: 2.5;color: #666;">
                        <li>{{ t('overview.moreDevices') }}</li>
                        <li>{{ t('overview.enterpriseSecurity') }}</li>
                        <li style="display: flex;justify-content: space-between;">
                            <span>{{ t('overview.dedicatedService') }}</span>
                            <span style="color: #7A7FFF;cursor: pointer;">「{{ t('overview.contactNow') }}」</span>
                        </li>
                    </ul>
                </div>
                <!-- 微控云移动端 -->
                <div
                    style="overflow: auto;padding: 1rem;margin-top: 1.2rem;height: 20vh;border: 1px solid #e0e0e0;border-radius: 8px; background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);box-shadow: 0px 0px 12px rgba(0, 0, 0, 0.2);">
                    <div style="font-size: 1.1rem;font-weight: 700;color: #333;">{{ t('overview.mobileApp') }}</div>
                    <div style="display: flex;align-items: center;margin-top: 1.4rem;">
                        <div style="width: 55%;font-size: 0.8rem;line-height: 2;color: #666;">
                            <p style="width: 200px;overflow: hidden;text-overflow: ellipsis;white-space: nowrap;">
                                <el-tooltip class="box-item" effect="dark" :content="t('overview.viewDeviceStatus')"
                                    placement="top"> {{ t('overview.viewDeviceStatus') }}</el-tooltip></p>
                            <p>{{ t('overview.scanToExperience') }}</p>
                        </div>
                        <div style="width: 70%;display: flex;align-items: center;justify-content: space-between;">
                            <div style="text-align: center;overflow: hidden;text-overflow: ellipsis;white-space: nowrap;">
                                <img src="@/assets/aaaaaaa.png" width="75%">
                                <p style="font-size: 0.7rem;color: #666;">
                                        {{ t('overview.wechatMiniProgram') }}
                                        </p>
                            </div>
                            <div style="text-align: center;">
                                <img src="@/assets/aaaaaaa.png" width="75%">
                                <p style="font-size: 0.7rem;color: #666;">{{ t('overview.androidApp') }}</p>
                            </div>
                            <div style="text-align: center;">
                                <img src="@/assets/aaaaaaa.png" width="75%">
                                <p style="font-size: 0.7rem;color: #666;">{{ t('overview.iosApp') }}</p>
                            </div>
                        </div>
                    </div>
                </div>
                <!-- 今日告警通知统计 -->
                <div
                    style="overflow: auto;padding: 1rem;margin-top: 1rem;height: 18vh;border: 1px solid #e0e0e0;border-radius: 8px; background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);box-shadow: 0px 0px 12px rgba(0, 0, 0, 0.2);">
                    <div style="font-size: 1.1rem;font-weight: 700;color: #333;">{{ t('overview.todayAlarmStatistics')
                        }}</div>
                    <div
                        style="display: flex;align-items: center;justify-content: space-between;width: 75%;margin: 0.9rem auto;">
                        <div>
                            <p style="display: flex;align-items: center;font-size: 0.8rem;color: #666;">
                                <img src="@/assets/qwe3.png" width="25rem" style="margin-right: 0.5rem;">
                                <span>{{ t('overview.wechatOfficialAccount') }}：1</span>
                            </p>
                            <p
                                style="display: flex;align-items: center;margin-top: 1.5rem;font-size: 0.8rem;color: #666;">
                                <img src="@/assets/qwe2.png" width="25rem" style="margin-right: 0.5rem;">
                                <span>{{ t('overview.email') }}：1</span>
                            </p>
                        </div>
                        <div>
                            <p style="display: flex;align-items: center;font-size: 0.8rem;color: #666;">
                                <img src="@/assets/qwe4.png" width="25rem" style="margin-right: 0.5rem;">
                                <span>{{ t('overview.sms') }}：1</span>
                            </p>
                            <p
                                style="display: flex;align-items: center;margin-top: 1.5rem;font-size: 0.8rem;color: #666;">
                                <img src="@/assets/qwe1.png" width="25rem" style="margin-right: 0.5rem;">
                                <span>{{ t('overview.phone') }}：1</span>
                            </p>
                        </div>
                    </div>
                </div>
            </el-col>
        </el-row>
        <el-dialog v-model="chartIsTrue" :title="t('overview.historyData')" width="50%">
            <div style="display: flex;align-items: center;justify-content: space-between;">
                <p>{{ t('overview.propertyName') }}：{{ t('overview.temperature') }}</p>
                <div style="display: flex;align-items: center;justify-content: space-between;">
                    <el-date-picker v-model="searchValue" type="daterange" range-separator="-"
                        :start-placeholder="t('overview.startTime')" :end-placeholder="t('overview.endTime')" />
                    <el-button type="primary" @click="chartIsTrue = false" style="margin-left: 1rem;">{{
                        t('overview.refresh') }}</el-button>
                    <p
                        style="margin-left: 1rem;border: 1px solid #507eff;padding: 0.3rem;display: flex;align-items: center;">
                        <el-icon size="20" color="#507eff">
                            <Download />
                        </el-icon>
                    </p>
                </div>
            </div>
            <div style="height: 50vh;">
                <VueEcharts style="width: 100%;height: 100%;" :option="graphOption" autoresize />
            </div>
            <template #footer>
                <el-button type="primary" @click="chartIsTrue = false">{{ t('overview.table') }}</el-button>
                <el-button type="primary" @click="chartIsTrue = false">{{ t('overview.barChart') }}</el-button>
                <el-button type="primary" @click="chartIsTrue = false">{{ t('overview.lineChart') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import * as echarts from "echarts";
import { workbench_overview } from "@/api/workPage/index";
import img1 from '@/assets/aq1.png'
import img2 from '@/assets/worging.png'
import useCounterStore from "@/stores/counter";
import { storeToRefs } from 'pinia';
import img3 from '@/assets/vv4.png'
import img4 from '@/assets/online.png'
import img5 from '@/assets/alert.png'
import img6 from '@/assets/rule.png'
import Color from 'element-plus/es/components/color-picker-panel/src/utils/color.mjs';

const { t } = useI18n();
const store = useCounterStore();
const { user_id } = storeToRefs(store);

let STAT_FIELD_MAP = [
    { field: 'total_devices', title: t('overview.totalDevices'), defaultValue: 0, img: img3 },
    { field: 'online_devices', title: t('overview.onlineDevices'), defaultValue: 0, img: img4 },
    { field: 'alarm_devices', title: t('overview.alarmDevices'), defaultValue: 0, img: img5 },
    { field: 'rule_count', title: t('overview.rules'), defaultValue: 0, img: img6 }
]

let aggregation = ref<Array<{ title: string; value: number, img: any }>>([])

let optionConfigList = ref([
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2 },
    { keyTitle: t('overview.airSwitch'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2, onOff: '1' },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2 },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2, onOff: '1' },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2 },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2, onOff: '1' },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2 },
    { keyTitle: t('overview.airSwitch'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2, onOff: '1' },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2 },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2 },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2, onOff: '1' },
    { keyTitle: t('overview.temperatureSensor'), keyType: t('overview.workshopElectricBox'), valueTitle: t('overview.currentTemperature'), valueKey: "25", time: 2 }
]);

const chunkedOptions = computed(() => {
    const chunkSize = 6;
    const chunks = [];
    for (let i = 0; i < optionConfigList.value.length; i += chunkSize) {
        chunks.push(optionConfigList.value.slice(i, i + chunkSize));
    }
    return chunks;
});

let progList = ref([
    { title: t('overview.deviceTotal'), value: 70, key: "1/5台" },
    { title: t('overview.dataStorage'), value: 35, key: "1350000/5000000条/月" },
    { title: t('overview.alarmSMS'), value: 90, key: "8/10条" },
    { title: t('overview.alarmVoice'), value: 75, key: "9/10条" },
    { title: t('overview.userCount'), value: 50, key: "1/3人" },
    { title: t('overview.configurationScreen'), value: 50, key: "1/2个" }
]);

let chartIsTrue = ref(false);
let searchValue = ref([]);

let DEVICE_TYPE_MAP = {
    GW: t('overview.gateway'),
    GSD: t('overview.gatewaySubDevice'),
    DD: t('overview.directDevice'),
    unknown: t('overview.unknownDevice')
}

let deviceData = ref({ total_devices: 0, by_dev_type: {} })

onMounted(() => {
    overviewFaclity()
})

let overviewFaclity = () => {
    workbench_overview({ user_id: user_id.value }).then((res: any) => {
        deviceData.value = res.data
        aggregation.value = STAT_FIELD_MAP.map(config => ({
            title: config.title,
            value: res.data[config.field] ?? config.defaultValue,
            img: config.img
        }))
    })
}

const getItemBorderColor = (val: any) => {
    switch (val) {
        case '1': return 'border-left: 4px solid #FBCB58;'
        case '2': return 'border-left: 4px solid #FBCB58;'
        default: return 'border-left: 4px solid #39AFD1;'
    }
}

const getItemIcon = (val: any) => {
    switch (val) {
        case '1': return img2
        case '2': return img2
        default: return img1
    }
}

const pieOption = computed(() => {
    const { total_devices = 0, by_dev_type = {} } = deviceData.value || {}
    const pieData = Object.entries(by_dev_type).map(([key, value]) => {
        const name = DEVICE_TYPE_MAP[key] || key
        const count = value || 0
        const itemStyle = key === 'unknown' ? { color: '#B4B4B4' } : key === 'GSD' ? { color: '#FFB085' } : undefined
        return { value: count, name: `${name} ${count}台 `, itemStyle }
    })

    return {
        title: { text: t('overview.deviceAccessType'), top: 10, left: 10 },
        tooltip: { show: true, trigger: "item" },
        legend: { orient: "vertical", right: 7, top: 20, bottom: 20 },
        series: [{
            type: "pie",
            radius: ["35%", "70%"],
            center: ["30%", "55%"],
            avoidLabelOverlap: false,
            label: { show: false, position: "center" },
            emphasis: { label: { show: true, fontSize: 20, fontWeight: "bold" } },
            data: pieData
        }]
    }
})

let graphOption = computed(() => {
    return {
        title: {
            text: t('overview.deviceAlarmStatistics'),
            top: 10,
            left: 10
        },
        tooltip: {
            show: true,
            trigger: "item"
        },
        xAxis: {
            type: "category",
            data: [t('overview.emergencyAlarm'), t('overview.importantAlarm'), t('overview.normalAlarm'), t('overview.resolvedAlarm')]
        },
        yAxis: {
            type: "value",
            axisTick: {
                show: true
            },
            axisLine: {
                show: true
            },
            splitLine: {
                show: false
            }
        },
        grid: {
            top: "20%",
            bottom: "7%",
            left: "3%",
            right: "3%",
            containLabel: true
        },
        series: [
            {
                type: "bar",
                barWidth: "30%",
                data: [
                    {
                        value: 70,
                        itemStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                                { offset: 0, color: "#FF7A7A" },
                                { offset: 1, color: "#CE0C0C" }
                            ])
                        }
                    }, {
                        value: 50,
                        itemStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                                { offset: 0, color: "#FFE957" },
                                { offset: 1, color: "#C99517" }
                            ])
                        }
                    }, {
                        value: 70,
                        itemStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                                { offset: 0, color: "#5FFCFC" },
                                { offset: 1, color: "#15C1C7" }
                            ])
                        }
                    }, {
                        value: 55,
                        itemStyle: {
                            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                                { offset: 0, color: "#59FCCA" },
                                { offset: 1, color: "#1EC17C" }
                            ])
                        }
                    }
                ]
            }
        ]
    };
});
</script>

<style lang="scss" scoped>
.workPageOverview {
    padding: 0 1rem;
    background: #fff;
    height: 100%;

    .huizong {
        background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);
        box-shadow: 0px 0px 12px rgba(0, 0, 0, 0.2);
        border: rgb(212, 212, 212) 1px solid;
        color: #303133;
        display: flex;
        justify-content: space-between;
        align-items: center;
        border-radius: 10px;
        padding: 1rem;
        width: calc(100% / 4 - 2rem);
        cursor: pointer;
    }

    .option {
        width: calc(100% / 3 - 1rem);
        background: linear-gradient(to bottom, #E4ECFE 0%, #F8FFFE 30%);
        padding: 0.8rem 1rem;
        border-radius: 8px;
        margin: 0.5rem;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
        box-sizing: border-box;
    }
}
</style>

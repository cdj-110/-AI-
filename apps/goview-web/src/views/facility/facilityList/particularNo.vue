<template>
    <div>
        <el-card>
            <div style="cursor: pointer;display: flex;align-items: center;color: black;margin-bottom: 0.7rem;font-size: 14px;width: 4rem;"
                @click="goBack()">
                <el-icon>
                    <ArrowLeft />
                </el-icon><span style="margin-left: 0.5rem;">{{ t('facilityList.back') }}</span>
            </div>
            <div style="display: flex;width: 100%;">
                <div
                    style="border: 1px solid #c0c5cc;padding: 0.7rem 1rem;border-radius: 10px;display: flex;align-items: center;">
                    <el-image class="device-img" :src="deviceMap[queryTitleConfig.device_type]?.img || aaa" fit="cover"
                        style="width: 95px;height: 95px;" />
                </div>

                <div style="margin-left: 3rem;" :query="queryTitleConfig">
                    <div style="display: flex;align-items: center;">
                        <p style="font-weight: 700; margin: 0 0.4rem 0 0;">{{ queryTitleConfig.dev_name }}</p>
                        <el-tag type="success" v-if="queryTitleConfig.dev_status == 1">{{ t('facilityList.online') }}</el-tag>
                        <el-tag type="info" v-if="queryTitleConfig.dev_status === 0">{{ t('facilityList.offline') }}</el-tag>
                        <el-icon style="margin-left: 1rem;cursor: pointer;" :size="20"
                            @click="alterConfig.isTrue = true; alterConfig.list = queryTitleConfig;">
                            <Edit />
                        </el-icon>
                        <div
                            style="display: flex;align-items: center;justify-content: space-between;margin-left: 1rem;">
                            <el-icon size="20" color="red">
                                <Warning />
                            </el-icon>
                            <el-button link type="danger"
                                style="text-align: right;color: #f57c7c;cursor: auto;">{{ t('facilityList.emergencyAlarm') }}</el-button>
                        </div>
                    </div>
                    <div style="display: flex;align-items: center;font-size: 0.9rem;margin-top: 0.8rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityList.deviceID') }}：</p>
                        <p style="color: #7E7779;">{{ queryTitleConfig.dev_id }}</p>
                        <p style="color: #7E7779;">
                            <el-icon style="margin-left: 0.5rem;cursor: pointer;">
                                <CopyDocument @click="copyPublicChange(queryTitleConfig.dev_id)" />
                            </el-icon>
                        </p>
                    </div>
                    <div style="display: flex;align-items: center;font-size: 0.9rem;margin-top: 0.8rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityList.deviceType') }}：</p>
                        <p style="color: #7E7779;">{{ queryTitleConfig.device_type == "GW" ? t('facilityList.gateway') :
                            queryTitleConfig.device_type == "GSD" ? t('facilityList.gatewaySubDevice') : queryTitleConfig.device_type == "DD" ?
                                t('facilityList.directDevice') : "" }}</p>
                    </div>
                    <div style="margin-top: 0.8rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityList.createTime') }}：</p>
                        <p style="color: #7E7779;">{{ itializeUtc(queryTitleConfig.create_time) }}</p>
                    </div>

                </div>
                <div style="margin-left: 5rem;">
                    <div style="display: flex;align-items: center;">&nbsp;</div>
                    <div style="display: flex;align-items: center;font-size: 0.9rem;margin-top: 0.8rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityList.modelName') }}：</p>
                        <p style="color: #7E7779;">{{ queryTitleConfig.dev_model_name }}</p>
                    </div>
                    <div style="margin-top: 0.8rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityList.deviceAddress') }}：</p>
                        <p style="color: #7E7779;">
                            <el-tooltip class="box-item" effect="dark" :content="queryTitleConfig.install_location"
                                placement="top-start">
                                <span class="box-ellipsis">
                                    {{ queryTitleConfig.install_location }}
                                </span>
                            </el-tooltip>

                        </p>
                    </div>

                    <div style="margin-top: 0.8rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityList.deviceDesc') }}：</p>
                        <!-- <p style="color: #7E7779;" v-if="queryTitleConfig.dev_desc.length < 28">{{
                            queryTitleConfig.dev_desc }}</p> -->
                        <p style="color: #7E7779;">
                            <el-tooltip class="box-item" effect="dark" :content="queryTitleConfig.dev_desc"
                                placement="top-start">
                                <span class="box-ellipsis">
                                    {{ queryTitleConfig.dev_desc }}
                                </span>
                            </el-tooltip>
                        </p>
                    </div>
                </div>
                <div>
                    <div style="display: flex;align-items: center;">&nbsp;</div>
                    <div style="margin-top: 1rem;display: flex;align-items: center;font-size: 0.9rem;">
                        <p style="color: #AAAAAA;">{{ t('facilityList.belongGroup') }}：{{ queryTitleConfig.dev_group?.join(',') || t('facilityList.none') }}</p>
                        <p style="color: #7E7779;"></p>
                    </div>
                </div>
            </div>
        </el-card>

        <el-card style="margin-top: 0.5rem;height: 67vh;">
            <el-tabs v-model="activeName" type="card">
                <el-tab-pane :label="t('facilityList.deviceOverview')" :name="1">
                    <generalView :config="props.particularConfig" :dev_id="listObj.dev_id"
                        :model_id="queryTitleConfig.dev_model" v-if="activeName == 1"
                        :status="queryTitleConfig.dev_status" @activeChange="activeChange"></generalView>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityList.deviceNetworking')" :name="2">
                    <networking :config="props.particularConfig" :dev_id="listObj.dev_id"
                        :device_type="queryTitleConfig.device_type" v-if="activeName == 2" @infoClick="infoClick">
                    </networking>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityList.functionalProperties')" :name="3">
                    <functional :config="props.particularConfig" :activeName="activeName"
                        :model_id="queryTitleConfig.dev_model" :model_name="queryTitleConfig.dev_model_name"
                        :dev_id="listObj.dev_id" :device_type="queryTitleConfig.device_type" v-if="activeName == 3" @getText="getText">
                    </functional>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityList.deviceEvents')" :name="4">
                    <deviceEvents :config="props.particularConfig" :activeName="activeName"
                        :model_id="queryTitleConfig.dev_model" :model_name="queryTitleConfig.dev_model_name"
                        :dev_id="listObj.dev_id" :device_type="queryTitleConfig.device_type" v-if="activeName == 4">
                    </deviceEvents>
                </el-tab-pane>
                <!-- GSD -->
                <el-tab-pane :label="t('facilityList.modbusConfig')" :name="5" v-if="listObj.device_type === 'GSD'">
                    <modbusConfig :config="props.particularConfig" v-if="activeName == 5"></modbusConfig>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityList.subDevice')" :name="6" v-if="listObj.device_type === 'GW'">
                    <ziFacility :config="props.particularConfig" :dev_id="listObj.dev_id"
                        :id="queryTitleConfig.dev_model" :dev_model="queryTitleConfig.dev_model" v-if="activeName == 6">
                    </ziFacility>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityList.deviceLogs')" :name="7">
                    <facilityLog :config="props.particularConfig" v-if="activeName == 7" :dev_id="listObj.dev_id">
                    </facilityLog>
                </el-tab-pane>
                <!-- <el-tab-pane label="设备调试" :name="9" v-if="!listObj.device_type || listObj.device_type === 'GW'">
                    <facilityTryout :config="props.particularConfig" v-if="activeName == 9"></facilityTryout>
                </el-tab-pane>  -->
            </el-tabs>
        </el-card>
        <geoAlter :alterConfig="alterConfig" @refaer="getList();"></geoAlter>
    </div>
</template>

<script lang="ts" setup>
import { socketUrl } from "@/utils/request";
import { copyPublicChange } from "@/utils/publicFun";
import { allConnection } from '@/utils/allWebSocketConnction';
import generalView from "./pane/generalView.vue";
import networking from "./pane/networking.vue";
import functional from "./pane/functional.vue";
import deviceEvents from "./pane/deviceEvents.vue";
import ziFacility from "./pane/ziFacility.vue";
import facilityLog from "./pane/facilityLog.vue";
import modbusConfig from "./pane/modbusConfig.vue";
import facilityTryout from "./pane/facilityTryout.vue";
import geoAlter from "./pane/geoAlter.vue";
import { device_detail } from "@/api/facilityList/index";
import useCounterStore from "@/stores/counter";
import { itializeUtc } from "@/utils/publicFun";
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
import img1 from '@/assets/wgmx.png'
import img2 from '@/assets/zsbmx.png'
import img3 from '@/assets/zlsb.png'
import aaa from '@/assets/aaa.png'
let route = useRoute();
let router = useRouter()
let props: any = withDefaults(defineProps<{
    particularConfig: Object
}>(), {
    particularConfig: () => ({
        isTrue: false,
        list: {}
    })
});
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);

let activeName = ref(1);//详情标签默认选中哪个
let alterConfig = ref({ isTrue: false, list: {} });
let queryTitleConfig = ref({ dev_name: "", install_location: "", device_type: "", create_time: "", dev_desc: "" });//设备头部详情
let listObj = ref(JSON.parse(route.query.list as string))
let statusSocket = ref<WebSocket | null>(null);

// 核心映射表
const deviceMap = {
    GW: { name: '网关', img: img1 },
    GSD: { name: '网关子设备', img: img2 },
    DD: { name: '直连设备', img: img3 }
}

let closeStatusSocket = () => {
    if (statusSocket.value) {
        statusSocket.value.close();
        statusSocket.value = null;
    }
}

let connectStatusSocket = (devId: string) => {
    closeStatusSocket();
    // const WS_BASE_URL = import.meta.env.VITE_WEBSOCKET_API.replace(/\/$/, '')
    const WS_BASE_URL = socketUrl.replace(/\/$/, '')
    const wsUrl = `${WS_BASE_URL}/api/mqtt_collection/status/realtime/${devId}`
    allConnection(wsUrl).then((ws) => {
        statusSocket.value = ws;
        ws.onmessage = (e: any) => {
            const data = JSON.parse(e.data);
            queryTitleConfig.value.dev_status = data.Online;
        };
    })
}

let getList = () => {
    device_detail({
        dev_id: listObj.value.dev_id,
        user_id: user_id.value
    }).then((val: any) => {
        if (val.data.install_location == null) {
            val.data.install_location = ''
        }
        queryTitleConfig.value = val.data;
    })
}

watch(() => listObj.value.dev_id, (devId) => {
    if (devId) {
        getList();
        connectStatusSocket(devId);
    }
}, { immediate: true });

onUnmounted(() => {
    closeStatusSocket();
});


/**
 * 返回
 */
let goBack = () => {
    // router.go(-1)
    // emit("goBack");
    if (listObj.value.geoRow) {
        router.push('/facilityList?geoRow=true');
    } else {
        router.push('/facilityList');
    }
}

let infoClick = () => {
    getList()
}
let getText = () => {
    getList()
}

let activeChange = (val: number) => {
    activeName.value = val;
}
</script>

<style lang="scss" scoped>
:deep(.el-tabs--card>.el-tabs__header) {
    border-bottom: none;
}

:deep(.el-tabs--card>.el-tabs__header .el-tabs__item) {
    border-bottom: none;
    border-left: none;
    height: 35px;
}

:deep(.el-tabs--card>.el-tabs__header .el-tabs__nav) {
    border: none;
}

:deep(.el-tabs--card>.el-tabs__header .el-tabs__item.is-active) {
    border: var(--el-color-primary) 1px solid;
    border-radius: 7px;
    margin-top: 0;
}

:deep(.el-alert--warning.is-light) {
    background-color: #FCF2C7;
}

:deep(.el-alert--warning.is-light, .el-alert--warning.is-light .el-alert__description) {
    color: #5C3C1C;
}

:deep(.el-card) {
    border-radius: 10px;
}

:deep(.el-card__body) {
    padding: 12px;
}

.box-ellipsis {
    display: inline-block;
    width: 400px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
</style>

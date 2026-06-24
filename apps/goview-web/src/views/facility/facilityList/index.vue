<template>
    <div style="background: #fff;height: 100%;">
        <div style="background: #fff;height: 100%;" v-if="particularConfig.isTrue">
            <facilityHeader :facilityConfig="facilityConfig" v-if="platTrue">
               <template #left>
                    <el-button type="primary" icon="Plus" @click="tagOnConfig.bool = true;">{{ t('facilityList.addDevice') }}</el-button>
                    <!-- <el-button type="warning" icon="Plus" @click="quantityTagOnConfig.bool = true;">批量添加设备</el-button> -->
                </template>
                <template #content>
                    <el-table :data="tableData" style="width: 100%">
                        <el-table-column align="center" prop="dev_name" :label="t('facilityList.deviceName')">
                            <template #default="scope">
                                <div v-if="scope.row.is_delete == 0">{{ scope.row.dev_name }}</div>
                                <div v-if="scope.row.is_delete == 1"
                                    style="text-decoration: line-through;color: #cccccc;">{{ scope.row.dev_name }}</div>
                            </template>
                        </el-table-column>
                        <el-table-column align="center" prop="dev_sn" :label="t('facilityList.deviceSerial')" show-overflow-tooltip>
                            <template #default="scope">
                                <div v-if="scope.row.is_delete == 0">{{ scope.row.dev_sn }}</div>
                                <div v-if="scope.row.is_delete == 1"
                                    style="text-decoration: line-through;color: #cccccc;">{{ scope.row.dev_sn }}</div>
                            </template>
                        </el-table-column>
                        <el-table-column align="center" prop="dev_model_name" :label="t('facilityList.deviceModelName')" show-overflow-tooltip>
                            <template #default="scope">
                                <div v-if="scope.row.is_delete == 0">{{ scope.row.dev_model_name }}</div>
                                <div style="text-decoration: line-through;color: #cccccc;"
                                    v-if="scope.row.is_delete == 1">
                                    {{ scope.row.dev_model_name }}
                                </div>
                            </template>
                        </el-table-column>
                        <el-table-column align="center" prop="dev_status" :label="t('facilityList.status')">
                            <template #default="scope">
                                <div v-if="scope.row.is_delete == 0">
                                    <el-tag type="success" v-if="scope.row.dev_status == 1">{{ t('facilityList.online') }}</el-tag>
                                    <el-tag type="info" v-if="scope.row.dev_status === 0">{{ t('facilityList.offline') }}</el-tag>
                                </div>
                                <div v-if="scope.row.is_delete === 1"
                                    style="text-decoration: line-through;color: #cccccc;">已删除</div>
                            </template>
                        </el-table-column>
                        <el-table-column align="center" prop="create_time" :label="t('facilityList.createTime')">
                            <template #default="scope">
                                <div v-if="scope.row.is_delete == 0">{{ itializeUtc(scope.row.create_time) }}</div>
                                <div v-if="scope.row.is_delete == 1"
                                    style="text-decoration: line-through;color: #cccccc;">{{
                                        itializeUtc(scope.row.create_time) }}</div>
                            </template>
                        </el-table-column>
                        <el-table-column align="center" :label="t('facilityList.operation')" width="350">
                            <template #default="scope">
                                <div>
                                    <div>
                                        <el-button link :type="scope.row.is_delete == 1 ? 'info' : 'primary'"
                                            @click="particularConfigDetails(scope.row)"
                                            :disabled="scope.row.is_delete == 1">{{ t('facilityList.details') }}</el-button>
                                        <el-button link type="danger" v-if="scope.row.is_delete == 0"
                                            @click="cutOutChange(scope.row, 0)">{{ t('facilityList.delete') }}</el-button>
                                        <el-button link type="primary" v-if="scope.row.is_delete == 1"
                                            @click="recovery(scope.row)">{{ t('facilityList.recoverDevice') }}</el-button>
                                        <el-button link type="danger" v-if="scope.row.is_delete == 1"
                                            @click="cutOutChange(scope.row, 1)">{{ t('facilityList.permanentlyDelete') }}</el-button>
                                    </div>
                                    <!-- <div style="display: flex;align-items: center;justify-content: space-between;">
                                        <el-icon size="20" color="red">
                                            <Warning />
                                        </el-icon>
                                        <el-button link type="danger"
                                            style="text-align: right;color: #f57c7c;cursor: auto;">紧急告警</el-button>
                                    </div> -->
                                </div>
                            </template>
                        </el-table-column>
                    </el-table>
                    <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
                        <el-pagination v-model:current-page="paginnation.currentPage"
                            v-model:page-size="paginnation.pageSize" :page-sizes="[10, 50, 100, 200]"
                            layout="total, sizes, prev, pager,next" :total="paginnation.total"
                            @size-change="handleSizeChange" @current-change="currentChange" />
                    </div>
                </template>
            </facilityHeader>
            <div v-else ref="mapRef" style="height: 100%;width: 100%;position: relative;">
                <div
                    style="position: absolute;z-index: 100;display: flex;justify-content: space-between;left: 1.5%;top: 3rem;">
                    <div style="background: #fff;border-radius: 10px;padding: 1rem 0.5rem;width: 25vw;">
                        <div :query="onlineTotal"
                            style="display: flex;align-items: center;justify-content: space-between;">
                            <div style="display: flex;align-items: center;justify-content: center;text-align: center;">
                                <img src="@/assets/v1.png" style="width: 35%;margin-right: 0.7rem;">
                                <div>
                                    <p>{{ t('facilityList.deviceName') }}</p>
                                    <p>{{ onlineTotal.total }}</p>
                                </div>
                            </div>
                            <div style="display: flex;align-items: center;justify-content: center;text-align: center;">
                                <img src="@/assets/v2.png" style="width: 35%;margin-right: 0.7rem;">
                                <div>
                                    <p>{{ t('facilityList.online') }}</p>
                                    <p>{{ onlineTotal.yesOnline }}</p>
                                </div>
                            </div>
                            <div style="display: flex;align-items: center;justify-content: center;text-align: center;">
                                <img src="@/assets/v3.png" style="width: 35%;margin-right: 0.7rem;">
                                <div>
                                    <p>{{ t('facilityList.offline') }}</p>
                                    <p>{{ onlineTotal.noOnline }}</p>
                                </div>
                            </div>
                        </div>
                        <div
                            style="width: 94%;margin: auto;display: flex;align-items: center;justify-content: space-between;">
                            <el-input v-model="selectGeoText" :placeholder="t('facilityList.deviceName') + '、' + t('facilityList.deviceGroup')" @input="grapeChange(groupId)"
                                clearable style="width: 70%;margin: 2rem 0;" />
                            <el-button type="primary" icon="Search" @click="grapeChange(groupId)">{{ t('facilityList.query') }}</el-button>
                        </div>
                        <div style="display: flex;justify-content: space-between;height: 45vh;">
                            <div style="width: 50%;overflow: auto;padding: 0 1rem;">
                                <el-tabs v-model="activeName" stretch>
                                    <el-tab-pane :label="t('facilityList.deviceGroup')" :name="t('facilityList.deviceGroup')" class="pean">
                                        <el-tabs v-model="groupId" @tab-change="grapeChange(groupId)"
                                            tab-position="left" stretch>
                                            <el-tab-pane v-for="(item, index) in grapeList" :key="index"
                                                :label="item.group_name" :name="item.group_id"></el-tab-pane>
                                        </el-tabs>
                                    </el-tab-pane>
                                    <el-tab-pane :label="t('facilityList.region')" :name="t('facilityList.region')">
                                        <el-tree :data="regionList" :props="{ children: 'city', label: 'province' }"
                                            @node-click="regionChange" ref="treeRef" />
                                    </el-tab-pane>
                                </el-tabs>
                            </div>
                            <div style="border-left: 1px solid #eeeeee;width: 50%;overflow: auto;padding: 0 1rem;"
                                class="pean online">
                                <el-select v-model="onLineText" :placeholder="t('facilityList.status')" @change="grapeChange(groupId)"
                                    clearable>
                                    <el-option :label="t('facilityList.online')" :value="1" />
                                    <el-option :label="t('facilityList.offline')" :value="0" />
                                </el-select>
                                <div style="overflow: hidden;text-overflow: ellipsis;margin-top: 1rem;">
                                    <p v-for="(item, index) in grapeTextList" :key="index"
                                        @click="grapeTextChange(item)"
                                        style="padding: 0.5rem 0;border-bottom: 2px dashed #eee;cursor: pointer;font-size: 13px;">
                                        {{ item.dev_name }}
                                    </p>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div>
                        <el-button type="primary" icon="Operation" @click="platTrue = true;"
                            style="position: fixed;left: 90%;">{{ t('facilityList.deviceList') }}</el-button>
                    </div>
                </div>
            </div>
        </div>
        <div v-else style="background-color: rgb(227, 227, 227);height: 100%;">
            <!-- @goBack="particularConfig = { isTrue: true, list: {} };" -->
            <particularNo :particularConfig="particularConfig"></particularNo>
        </div>
        <tagOn v-if="tagOnConfig.bool" :tagOnConfig="tagOnConfig" @refaer="getList({});"></tagOn>
        <quantityTagOn v-if="quantityTagOnConfig.bool" :quantityTagOnConfig="quantityTagOnConfig"></quantityTagOn>
    </div>
</template>

<script lang="ts" setup>
import { socketUrl } from '@/utils/request'
import { connection } from '@/utils/webSocketConnction';
import { useRouter, useRoute } from 'vue-router'
const router = useRouter()
import { geoInit, itializeUtc } from "@/utils/publicFun";
import facilityHeader from "@/components/facilityHeader.vue";
import tagOn from "./tagOn.vue";
import quantityTagOn from "./quantityTagOn.vue";
import particularNo from "./particularNo.vue";
import { deviceinfo, groupList, delete_device, soft_delete_device, recovery_device, locationProvinces, locationDevices } from "@/api/facilityList";
import { model_device_list } from "@/api/facilityGroup";
import useCounterStore from "@/stores/counter";
import { geoLoactionVer } from "@/utils/geo/chain";
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let platTrue = ref(true);//是否显示地图
let mapRef: any = ref("");//地图Ref
let geoPrototype: any = ref({});//地图的实例
let geoPunctuation: any = ref([]);//标点集合
let particularConfig = ref({ isTrue: true, list: {} });//详情和列表的切换
let tableData: any = ref([]);//设备列表的信息
let tagOnConfig = ref({ bool: false });//添加设备的弹窗信息
let quantityTagOnConfig = ref({ bool: false });//批量添加设备的弹窗信息
let selectGeoText = ref("");//地图查询
let activeName = ref("地域");//地图设备分组和地域默认选中的值
let groupId = ref("");//地图设备分组默认选中的值
//地图地域数据源
let regionList: any = ref([]);
let grapeList: any = ref([]);//设备分组数据源
let onLineText: any = ref("");//地图设备状态选择框
let onlineTotal = ref({ total: 0, yesOnline: 0, noOnline: 0 });//数量/在线/离线
let grapeTextList: any = ref([]);//地图设备状态数据源
let groupContent = ref([]);//分组查询列表下拉框
let paginnation = ref({ total: 0, pageSize: 10, currentPage: 1 });//设备列表分页
// 1. 定义 iframe 的 ref（替代 getElementById）
const iframeRef = ref<HTMLIFrameElement | null>(null);
/**
 * 顶部公用搜索头
 */
let facilityConfig = () => ({
    title: t('facilityList.deviceList'),
    search: [
        {
            fields: "dev_group",//字段
            type: "select",//类型
            placeholder: t('facilityList.deviceGroup'),//提示
            options: groupContent.value
        },
        {
            fields: "dev_status",
            type: "select",
            placeholder: t('facilityList.status'),
            options: [
                { label: t('facilityList.offline'), value: 0 },
                { label: t('facilityList.online'), value: 1 }
            ]
        },
        {
            fields: "keyword",
            type: "input",
            placeholder: t('facilityList.deviceName') + "/" + t('facilityList.deviceSerial') + "/" + t('facilityList.deviceModelName')
        },
        {
            fields: "",
            type: "button",
            label: t('facilityList.query'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                getList({ dev_group: e.dev_group, dev_status: e.dev_status, keyword: e.keyword });
            }
        },
        {
            fields: "",
            type: "button",
            label: t('facilityList.deviceList'),
            status: "primary",
            icon: "Location",
            onClick: (e: any) => {
                geoChangeInit();
            }
        }
    ]
})

onMounted(() => {
    let route = useRoute();
    if (route.query.geoRow) {
        geoChangeInit();
        router.replace("/facilityList");
    }
    getList({});
    groupList({ user_id: user_id.value }).then((res: any) => {
        groupContent.value = res.data.data.map((item: any) => {
            item.label = item.group_name;
            item.value = item.group_id;
            return item;
        });
        grapeList.value = res.data.data.reverse();
    })
    setTimeout(() => {
        // websocket链接
        // let wss = "wss://192.168.1.101/api/mqtt_collection/status/realtime/all/devices"
        const WS_BASE_URL = socketUrl.replace(/\/$/, '')
        const wsUrl = `${WS_BASE_URL}/api/mqtt_collection/status/realtime/all/devices`
        connection(wsUrl).then(res => {
            res.onmessage = (e: any) => {
                // console.log('收到消息:', e.data)
                const data = JSON.parse(e.data);
                tableData.value = tableData.value.map((item: any) =>
                    item.dev_id === data.dev_id ? { ...item, dev_status: data.Online } : item
                );
            }
        })
    }, 1000)

    locationProvinces({ user_id: user_id.value }).then((res: any) => {
        regionList.value = res.data;
    })
})

let treeRef: any = ref(null);

/**
 * 切换到设备地图页面
 */
let geoChangeInit = () => {
    platTrue.value = false;
    nextTick(() => {
        geoInit({ dom: mapRef.value }).then((res: any) => {
            geoPrototype.value = res.map;
        })
    })
}

/**
 * 地域列表点击选中事件
 */
let regionChange = (tree1: any, tree2: any) => {
    let parents = []
    let parent = tree2.parent;
    while (parent && parent.data) {
        parents.unshift(parent.data);
        parent = parent.parent;
    }
    let verlist = parents.map(p => p.province).filter(item => item).concat([tree1.province]);
    locationDevices({
        user_id: user_id.value,
        province: verlist.length >= 1 ? verlist[0] : "",
        city: verlist.length >= 2 ? verlist[1] : "",
        district: verlist.length >= 3 ? verlist[2] : ""
    }).then((res: any) => {
        grapeTextList.value = res.data.data;
        geoPrototype.value.map.clearMap();
        geoPunctuation.value = [];
        res.data.data.forEach((item: any) => {
            if (item.lng && item.lat) {
                poiGetChange(geoPrototype.value.AMap, geoPrototype.value.map, [item.lng, item.lat]);
            }
        })
    })
}

/**
 * 点击设备地图里面的分组
 */
let grapeChange = (e: any) => {
    if (!e) {
        ElMessage.error("请先选择分组！");
        return false;
    }
    //查询有多少在线离线的
    model_device_list({ user_id: user_id.value, query_type: "group", query_id: e }).then((res: any) => {
        onlineTotal.value.total = res.data.total;
        onlineTotal.value.yesOnline = res.data.devices.filter((item: any) => item.dev_status === 1).length;
        onlineTotal.value.noOnline = res.data.devices.filter((item: any) => item.dev_status === 0).length;
    })
    model_device_list({
        user_id: user_id.value,
        keyword: selectGeoText.value,
        dev_status: onLineText.value === 0 || onLineText.value === 1 ? onLineText.value : undefined,
        query_type: "group",
        query_id: e
    }).then((res: any) => {
        grapeTextList.value = res.data.devices;
        // geoPrototype.value.map.remove(geoPunctuation.value);
        geoPrototype.value.map.clearMap();
        geoPunctuation.value = [];
        res.data.devices.forEach((item: any) => {
            if (item.lng && item.lat) {
                poiGetChange(geoPrototype.value.AMap, geoPrototype.value.map, [item.lng, item.lat]);
            }
        })
    })
}

/**
 * 点击设备地图里面的设备
 */
let grapeTextChange = (e: any) => {
    // geoPrototype.value.map.remove(geoPunctuation.value);
    geoPrototype.value.map.clearMap();
    geoPunctuation.value = [];
    if (e.lng && e.lat) {
        poiGetChange(geoPrototype.value.AMap, geoPrototype.value.map, [e.lng, e.lat]);
        geoPrototype.value.map.setCenter([e.lng, e.lat])
    } else {
        ElMessage.error("此设备没有选择经纬度！");
    }
}

/**
 * 通过经纬度标点，传经纬度就显示经纬度的标点，不传就点击标点
 */
let poiGetChange = (AMap: any, map: any, arr: Array<string>) => {
    let lngLat = new AMap.LngLat(arr[0], arr[1]);//经纬度
    return new Promise(resolve => {
        let marker = new AMap.Marker({
            map: map,
            position: lngLat,
            anchor: 'top-center',
        });
        map.add(marker);
        geoPunctuation.value.push(marker);
        marker.on("click", (e: any) => {
            let textContent = grapeTextList.value.filter((item: any) => item.lat == e.target.getPosition().lat && item.lng == e.target.getPosition().lng)[0];
            let infoWindow = new AMap.InfoWindow({
                position: lngLat,
                content: `<div>
                    <div style="display: flex;justify-content: space-between;align-items: center;border-bottom:1px solid #eeeeee;padding: 1vh 1vw;">${textContent.dev_name}</p><p>${textContent.dev_status == 0 ? "离线" : "在线"}</p></div>
                    <p style="margin-top: 1vh;font-size: 13px;color: #241313;">设备模型：${textContent.dev_model_name}</p>
                    <p style="margin-top: 1vh;font-size: 13px;color: #241313;">设备唯一码：${textContent.dev_sn}</p>
                    <p style="margin-top: 1vh;font-size: 13px;color: #241313;">创建时间：${itializeUtc(textContent.create_time)}</p>
                    <p style="text-align: right;"><button id="getLngLat" style="background-color: #409eff;padding: 0.5vh 1vw;color: #ffffff;margin-top: 1vh;border: none;border-radius: 0.3rem;cursor: pointer;">详情</button></p>
                </div>`,
            });
            map.setCenter([arr[0], arr[1]])
            infoWindow.open(map);
            document.getElementById("getLngLat")?.addEventListener("click", () => {
                textContent.geoRow = true;
                particularConfigDetails(textContent);
            })
        })
    })
}

/**
 * 分页显示
 */
let currentChange = (e: any) => {
    paginnation.value.currentPage = e;
    getList({});
}
let handleSizeChange = (val: number) => {
    getList({});
}

/**
 * 设备列表查询
 */
let getList = ({ dev_group, dev_status, keyword }: { dev_group?: string, dev_status?: number, keyword?: number }) => {
    deviceinfo({ user_id: user_id.value, dev_group: dev_group, dev_status: dev_status, keyword: keyword, pagenumber: paginnation.value.currentPage, pagesize: paginnation.value.pageSize }).then((res: any) => {
        tableData.value = res.data.data;
        paginnation.value.total = res.data.total;
    })
}

/**
 * 删除
 */
let cutOutChange = (e: any, type: number) => {
    ElMessageBox.confirm("是否删除该设备", "提示", {
        confirmButtonText: "确认",
        cancelButtonText: "取消",
        type: "warning",
        // center: true,
    }).then(() => {
        if (type === 0) {
            soft_delete_device({ device_id: e.dev_id, user_id: user_id.value }).then((res: any) => {
                if (res.code == 200) {
                    getList({})
                    ElMessage.success(res.msg);
                } else {
                    ElMessage.error(res.msg);
                }
            })
        } else {
            delete_device({ device_id: e.dev_id, user_id: user_id.value }).then((res: any) => {
                if (res.code == 200) {
                    getList({})
                    ElMessage.success(res.msg);
                } else {
                    ElMessage.error(res.msg);
                }
            })
        }
    }).catch(() => { });
}

/**
 * 回复设备
 */
let recovery = (e: any) => {
    ElMessageBox.confirm("是否确认恢复设备", "提示", {
        confirmButtonText: "确认",
        cancelButtonText: "取消",
        type: "warning",
        center: true,
    }).then(() => {
        recovery_device({ device_id: e.dev_id, user_id: user_id.value }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                getList({})
            } else {
                ElMessage.error(res.msg);
            }
        })
    })

}

// isTrue: false, list: scope.row
let particularConfigDetails = (e: any) => {
    router.push({
        path: '/facilityList/particularNo', // 相对路径，自动拼接到当前路径后
        query: {
            list: JSON.stringify(e), // 传递详情ID
            isTrue: false as any
        }
    })
}
</script>

<style lang="scss" scoped>
.pean {
    :deep(.el-tabs__active-bar) {
        background-color: transparent;
    }

    :deep(.el-tabs__nav-wrap:after) {
        background-color: transparent;
    }

    :deep(.el-tabs--left .el-tabs__item.is-left, .el-tabs--right .el-tabs__item.is-left) {
        justify-content: start;
    }
}

.online {
    :deep(.el-tabs--left .el-tabs__item.is-left) {
        border-bottom: dashed 2px #eeeeee;
    }

    :deep(.el-tabs__item) {
        padding: 0 44px;
        margin-left: 15px;
    }
}

:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}

:deep(.el-table .el-table__cell) {
    padding: 1rem 0;
}
</style>
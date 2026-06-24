<template>
    <div style="overflow: auto; height: 60vh;padding: 0 0.5rem 0 0;">
        <div v-if="props.config.list.facilityPattern === 1"
            style="display: flex;align-items: center;padding: 0.7rem 1rem;">
            <p style="font-size: 0.9rem;">{{ t('functional.bindModel') }}</p>
            <el-button type="primary" plain style="margin-left: 1rem;">{{ t('functional.selectCreatedModel')
            }}</el-button>
        </div>
        <el-alert v-if="props.config.list.facilityPattern === 1" type="warning" :closable="false"
            style="margin: 0.7rem 1rem;">
            <template #title>{{ t('functional.noModelAlert') }}</template>
        </el-alert>
        <div v-if="props.config.list.facilityPattern !== 1" style="padding: 0.8rem;">
            <div style="display: flex;align-items: center;justify-content: space-between;">
                <el-button type="primary" icon="Edit" @click="updateList"
                    :disabled="!props.model_id || !props.model_name" v-if="props.model_id && props.model_name">{{
                        t('functional.updateDeviceModel') }}</el-button>
                <div style="display: flex;align-items: center;" v-if="!props.model_id || !props.model_name">
                    <p style="font-size: 0.9rem;"><span style="color: red;margin-right: 0.2rem;">*</span>{{
                        t('functional.bindModel') }}</p>
                    <div style="margin-left: 1rem;"></div>
                    <el-button type="primary" plain style="margin-left: 1rem;" @click="getPatternConfig.bool = true;">{{
                        t('functional.selectCreatedModel') }}</el-button>
                </div>
                <div style="display: flex;align-items: center;">
                    <el-input v-model="tarvalueText" :placeholder="t('functional.propertyNamePlaceholder')"
                        style="margin-right: 1rem;width: 15vw;" clearable />
                    <el-button type="primary" icon="Search"
                        @click="getFunction({ pagenumber: paginnation.currentPage, pagesize: paginnation.pageSize, keyword: tarvalueText });">{{
                            t('functional.query') }}</el-button>
                    <el-button type="primary" icon="Refresh"
                        @click="getFunction({ pagenumber: 1, pagesize: 10, keyword: '' });">{{ t('functional.refresh')
                        }}</el-button>
                </div>
            </div>
            <el-table :data="propertyList" style="width: 100%;padding: 0.7rem 0;">
                <el-table-column prop="property_name" :label="t('functional.propertyName')" width="150" align="center">
                    <template #default="scope">
                        {{ scope.row.property_name }} ({{ scope.row.data_type }})
                    </template>
                </el-table-column>
                <el-table-column prop="identifier" :label="t('functional.propertyType')" align="center">
                    <template #default="scope">
                        <span>
                            <el-tag
                                :type="scope.row.read_write_type === 'r' ? 'primary' : scope.row.read_write_type === 'w' ? 'warning' : scope.row.read_write_type === 'rw' ? 'success' : ''">
                                {{
                                    scope.row.read_write_type == 'r' ? t('functional.readOnly') : scope.row.read_write_type
                                        == 'w' ? t('functional.writeOnly') :
                                        scope.row.read_write_type == 'rw' ? t('functional.readWrite') : '' }}</el-tag>
                        </span>
                    </template>
                </el-table-column>
                <el-table-column prop="identifier" :label="t('functional.identifier')" align="center" />
                <el-table-column prop="property_value" :label="t('functional.propertyValue')" align="center">
                    <template #default="scope">
                        <div v-if="scope.row.data_type == 'bool'">
                            <span style="color: #5771E7;">{{ scope.row.property_value }}</span>
                            <span style="margin-top: 0.3rem;font-size: 0.7rem;">{{ scope.row.unit }}</span>
                        </div>
                        <div v-if="scope.row.data_type == 'number'">
                            <span style="color: #5771E7;">{{ scope.row.property_value }}</span>
                            <span style="margin-top: 0.3rem;font-size: 0.7rem;">{{ scope.row.property_value == 1 ?
                                t('functional.on') : scope.row.property_value == 0 ? t('functional.off') : '' }}</span>
                        </div>
                    </template>
                </el-table-column>
                <el-table-column prop="create_time" :label="t('functional.updateTime')" width="200" align="center">
                    <template #default="scope">
                        <span v-if="scope.row.collection_times">{{ scope.row.collection_times }}</span>
                        <span v-else>{{ scope.row.create_time.replace('T', ' ') }}</span>
                    </template>
                </el-table-column>
                <el-table-column prop="a" :label="t('functional.historicalData')" align="center">
                    <template #default="scope">
                        <el-icon @click="historicalDataClick(scope.row)" size="20" color="#509eff"
                            style="cursor: pointer;margin-top: 0.3rem;">
                            <Clock />
                        </el-icon>
                    </template>
                </el-table-column>
                <el-table-column :label="t('functional.operation')" align="center">
                    <template #default="scope">
                        <!-- <el-button link type="primary">详情</el-button> listTrue = true;-->
                        <el-button link type="primary" @click="detalClick(scope.row)"
                            v-if="scope.row.read_write_type !== 'r'">{{ t('functional.sendData') }}</el-button>
                    </template>
                </el-table-column>
            </el-table>
            <!-- <div style="display: flex;justify-content: right;margin-top: 1rem;">
                <el-pagination background layout="prev, pager, next, total" :total="paginnation.total"
                    :page-size="paginnation.pageSize" :current-page="paginnation.currentPage"
                    @current-change="currentChange" />
            </div> -->
            <div style="display: flex;justify-content: flex-end;">
                <el-pagination v-model:current-page="paginnation.currentPage" v-model:page-size="paginnation.pageSize"
                    :page-sizes="[10, 50, 100, 200]" layout="total, sizes, prev, pager,next" :total="paginnation.total"
                    @size-change="handleSizeChange" @current-change="currentChange" />
            </div>
        </div>
        <historicalData v-if="isHistoricalData" @handleClose="handleClose" :text="text" :identifier="identifier"
            :dev_id="props.dev_id"></historicalData>

        <createPattern :createPatternConfig="createPatternConfig" @refear="getList();"></createPattern>
        <getPattern ref="getPatternRef" :getPatternConfig="getPatternConfig" @paneOnChange="paneOnChange"
            :dev_id="props.dev_id" @createPatternChange="createPatternConfig.bool = true;" @sucssClick="sucssClick">
        </getPattern>

        <sendProperty v-if="listTrue" @handleClose="sendPropertyHandleClose" :SendPropertyObj="SendPropertyObj"
            :dev_id="props.dev_id" @sendData="sendData">
        </sendProperty>
    </div>
</template>

<script lang="ts" setup>
import { socketUrl } from '@/utils/request';
import { useRouter } from 'vue-router'
import { itializeUtc } from "@/utils/publicFun";
import monacoEditor from "@/components/monacoEditor.vue";
import sendProperty from './sendProperty.vue'
import historicalData from './historicalData.vue'
import { device_detail_function } from "@/api/facilityList/index";
import useCounterStore from "@/stores/counter";
import { allConnection } from '@/utils/allWebSocketConnction';
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const router = useRouter()
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let emit = defineEmits(["getText"]);

// let propertyText = ref([]);//功能属性 - 历史数据搜索框
let propertyList = ref([]);//功能属性表格
let tarvalueText = ref("");//功能属性搜索框
let text = ref('') // 历史数据弹窗属性名称
let props: any = withDefaults(defineProps<{
    config: Object,
    model_id: {
        type: String,
        required: true
    },
    model_name: {
        type: String,
        required: true
    },
    device_type: {
        type: String,
        required: true
    },
    dev_id: {
        type: String,
        required: true
    },
    activeName: any
}>(), {
    config: () => ({
        isTrue: false,
        list: {}
    })
});
let isHistoricalData = ref(false); // 历史数据弹窗
// let dialogTableVisible = ref(false);//功能属性的历史数据弹窗
let listTrue = ref(false);//功能属性的下发数据弹窗
// let gridData = ref([]);//功能属性的历史数据表格
let identifier = ref('');
let createPatternConfig = ref({ bool: false, modal: true, title: "创建设备模型", width: "55%", draggable: true, showClose: true });//创建设备模型的配置项 - 传入子组件中的值
let getPatternConfig = ref({ bool: false, modal: true, title: "设备模型", width: "55%", draggable: true, showClose: true, type: '1' });//选择设备模型的配置项 - 传入子组件中的值
let getPatternRef: any = ref("");//选择模型Ref
let paginnation = ref({ total: 0, pageSize: 10, currentPage: 1 });//设备列表分页
let isSendProperty = ref(false);
let SendPropertyObj = ref({})


/**
 * 渲染功能属性列表数据
 */
let getFunction = ({ keyword, pagenumber, pagesize }: { pagenumber?: number, pagesize?: number, keyword?: string }) => {
    device_detail_function({
        user_id: user_id.value,
        model_id: props.model_id,
        pagenumber: pagenumber,
        pagesize: pagesize,
        keyword: keyword,
        dev_id: props.dev_id
    }).then((res: any) => {
        propertyList.value = res.data.data;
        paginnation.value.total = res.data.total;
    })
}
let sendData = () => { getFunction({ pagenumber: paginnation.value.currentPage, pagesize: paginnation.value.pageSize, keyword: tarvalueText.value }) }

/**
 * 分页显示
 */
let currentChange = (e: any) => {
    paginnation.value.currentPage = e;
    getFunction({ pagenumber: paginnation.value.currentPage, pagesize: paginnation.value.pageSize, keyword: tarvalueText.value });
}
let handleSizeChange = (e: number) => {
    getFunction({ pagenumber: paginnation.value.currentPage, pagesize: paginnation.value.pageSize, keyword: tarvalueText.value });
}

watch(props, (val) => {
    if (val.model_id) {
        getFunction({ pagenumber: paginnation.value.currentPage, pagesize: paginnation.value.pageSize, keyword: tarvalueText.value });
    }
    if (val.dev_id) {
        const WS_BASE_URL = socketUrl.replace(/\/$/, '')
        const dev_id = `${WS_BASE_URL}/api/mqtt_collection/data/realtime/${val.dev_id}`
        allConnection(dev_id).then(res => {
            res.onmessage = (e: any) => {
                const data = JSON.parse(e.data);
                propertyList.value.forEach((item: any) => {
                    if (data[item.identifier] !== undefined) {
                        item.property_value = data[item.identifier];
                        item.create_time = data.send_time
                        item.collection_times = data.send_time
                    }
                });
            }
        })
    }
}, { immediate: true, deep: true })

/**
 * 点击模型
 */
let paneOnChange = (list1: any, list2: any, list3: any) => {
    ElMessage({
        message: h('div', { style: 'line-height: 1; font-size: 14px' }, [
            h('p', { style: 'color: red' }, `选择了【${list3.model_name}】模型`),
            h('p', { style: 'color: red;margin-top:1rem;' }, `模型ID是【${list3.model_id}】`),
            h('p', { style: 'color: red;margin-top:1rem;' }, `模型类型是【${list3.device_type}】`),
        ]),
        type: "success",
    })
}
let sucssClick = () => {
    emit("getText");
}

/**
 * 渲染模型和触发子组件渲染模型
 */
let getList = () => {
    getPatternRef.value.getList();
}



/**
 * json编辑器数据
 */
let jsonOnChange = (e: any) => {
    // console.log(toRaw(e).getValue())
}
let updateList = () => {
    router.push({
        path: '/facilityPattern/modelDetails', // 相对路径，自动拼接到当前路径后
        query: {
            model_id: props.model_id,
            model_name: props.model_name,
            device_type: props.device_type,
            upDataStr: 'updata'
        }
    })
}
// 历史数据弹窗
let historicalDataClick = (val: any) => {
    text.value = val.property_name;
    identifier.value = val.identifier;
    isHistoricalData.value = true;
}

let handleClose = (e: any) => {
    isHistoricalData.value = false;
}
let detalClick = (e: any) => {
    SendPropertyObj.value = e
    listTrue.value = true
}
// 取消下发弹窗
let sendPropertyHandleClose = () => {
    listTrue.value = false
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
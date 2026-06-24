<template>
    <div style="overflow: auto; height: 58vh;padding: 0 0.5rem 0 0;">
        <div v-if="props.config.list.facilityPattern === 1"
            style="display: flex;align-items: center;padding: 0.7rem 1rem;">
            <p style="font-size: 0.9rem;">{{ t('deviceEvents.bindModel') }}</p>
            <el-button type="primary" plain style="margin-left: 1rem;">{{ t('deviceEvents.selectCreatedModel') }}</el-button>
        </div>
        <el-alert v-if="props.config.list.facilityPattern === 1" type="warning" :closable="false"
            style="margin: 0.7rem 1rem;">
            <template #title>{{ t('deviceEvents.noModelAlert') }}</template>
        </el-alert>
        <div v-if="props.config.list.facilityPattern !== 1" style="padding: 1rem;">
            <div style="display: flex;align-items: center;justify-content: space-between;">
                <el-button type="primary" icon="Edit" @click="updateList"
                    :disabled="!props.model_id || !props.model_name"
                    v-if="props.model_id && props.model_name">{{ t('deviceEvents.updateDeviceModel') }}</el-button>
                <div style="display: flex;align-items: center;" v-if="!props.model_id || !props.model_name">
                    <p style="font-size: 0.9rem;"><span style="color: red;margin-right: 0.2rem;">*</span>{{ t('deviceEvents.bindModel') }}</p>
                    <div style="margin-left: 1rem;"></div>
                    <el-button type="primary" plain style="margin-left: 1rem;"
                        @click="getPatternConfig.bool = true;">{{ t('deviceEvents.selectCreatedModel') }}</el-button>
                </div>
                <div style="display: flex;align-items: center;">
                    <el-input v-model="tarvalueText" :placeholder="t('deviceEvents.eventNamePlaceholder')" style="margin-right: 1rem;width: 15vw;" clearable/>
                    <el-button type="primary" icon="Search" @click="getFunction">{{ t('deviceEvents.query') }}</el-button>
                    <el-button type="primary" icon="Refresh" @click="refresh">{{ t('deviceEvents.refresh') }}</el-button>
                </div>
            </div>
            <el-table :data="list" style="width: 100%;padding: 1rem 0;">
                <el-table-column prop="event_name" :label="t('deviceEvents.eventName')" align="center" width="150" show-overflow-tooltip/>
                <el-table-column prop="identifier" :label="t('deviceEvents.eventIdentifier')" align="center" />
                <el-table-column prop="timestamp" :label="t('deviceEvents.eventReportTime')" align="center" />
                <el-table-column :label="t('deviceEvents.operation')" align="center">
                    <template #default="scope">
                        <el-button link type="primary" @click="detalis(scope.row)">{{ t('deviceEvents.viewDetails') }}</el-button>
                    </template>
                </el-table-column>
            </el-table>
            <div style="display: flex;justify-content: right;margin-top: 1rem;">
                <el-pagination background layout="prev, pager, next, total" :total="paginnation.total"
                    :page-size="paginnation.pageSize" :current-page="paginnation.currentPage"
                    @current-change="currentChange" />
            </div>
        </div>
        <!-- 核心修改点：加上 destroy-on-close 和 key -->
        <el-dialog v-model="listTrue" :title="t('deviceEvents.viewEvent')" width="30%" :before-close="handleClose" :destroy-on-close="true">

            <div class="text">{{ t('deviceEvents.eventName') }}</div>
            <div class="text_Color">{{ detaliForm.event_name }}</div>
            <div class="text">{{ t('deviceEvents.eventMessage') }}</div>

            <!-- 核心修改点：外层包 div 并动态绑定 key，强制编辑器重新渲染 -->
            <div :key="monacoKey">
                <monacoEditor height="30vh" :config="monacoConfig" :readOnly="true">
                </monacoEditor>
            </div>

            <div class="text">{{ t('deviceEvents.eventReportTime') }}</div>
            <div class="text_Color">{{ detaliForm.timestamp }}</div>
            <template #footer>
                <el-button plain @click="handleClose" style="font-size: 0.8rem;">{{ t('deviceEvents.close') }}</el-button>
            </template>
        </el-dialog>

        <createPattern :createPatternConfig="createPatternConfig" @refear="getList();"></createPattern>
        <getPattern ref="getPatternRef" :getPatternConfig="getPatternConfig" @paneOnChange="paneOnChange"
            @createPatternChange="createPatternConfig.bool = true;"></getPattern>
    </div>
</template>

<script lang="ts" setup>
import { useRouter } from 'vue-router'
import monacoEditor from "@/components/monacoEditor.vue";
import { function_event_log } from "@/api/facilityList/index";
import { h, nextTick } from 'vue' // 确保引入了 h 和 nextTick
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const router = useRouter()

let list = ref([]);//功能属性表格
let tarvalueText = ref("");//功能属性搜索框
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
let listTrue = ref(false);//功能属性的下发数据弹窗
let createPatternConfig = ref({ bool: false, modal: true, title: "创建设备模型", width: "55%", draggable: true, showClose: true });//创建设备模型的配置项 - 传入子组件中的值
let getPatternConfig = ref({ bool: false, modal: true, title: "设备模型", width: "55%", draggable: true, showClose: true, type: '1' });//选择设备模型的配置项 - 传入子组件中的值
let getPatternRef: any = ref("");//选择模型Ref
let paginnation = ref({ total: 0, pageSize: 10, currentPage: 1 });//设备列表分页
let detaliForm = ref({ event_name: '', identifier: '', log_type: '', timestamp: '', value: {} });
const monacoConfig = ref({
    value: '',
    language: 'json',
    theme: 'vs-dark'
});
const monacoKey = ref(0);
onMounted(() => {
    getFunction();
})
/**
 * 渲染功能事件列表数据
 */
let getFunction = () => {
    function_event_log({
        dev_id: props.dev_id,
        pagenumber: paginnation.value.currentPage,
        pagesize: paginnation.value.pageSize,
        keyword: tarvalueText.value
    }).then((res: any) => {
        list.value = res.data.items;
        paginnation.value.total = res.data.total;
    })
}

const detalis = (row: any) => {
    // 第一步：先赋值数据
    detaliForm.value = row;

    // 处理 JSON 字符串
    const rawValue = row.value;
    monacoConfig.value.value = typeof rawValue === 'string'
        ? rawValue
        : JSON.stringify(rawValue, null, 2);

    // 第二步：Key + 1，强制 DOM 节点替换，彻底解决缓存问题
    monacoKey.value++;

    // 第三步：直接打开弹窗
    listTrue.value = true;
}
let handleClose = () => {
    listTrue.value = false;
}
// 刷新
let refresh = () => { getFunction(); }

/**
 * 分页显示
 */
let currentChange = (e: any) => {
    paginnation.value.currentPage = e;
    getFunction();
}

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
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
.text {
    color: #878787;
    font-size: 0.8rem;
}

.text_Color {
    margin: 0.4rem 0.5rem;
    color: #252525;
}
</style>
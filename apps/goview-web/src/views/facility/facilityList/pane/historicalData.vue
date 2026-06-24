<template>
    <div>
        <el-dialog v-model="dialogTableVisible" title="历史数据" width="40%" @close="emit('handleClose', false)"
            :close-on-press-escape="false" draggable>
            <div style="display: flex;align-items: center;justify-content: space-between;margin: 1rem 0 2rem 0;">
                <div style="font-size: 1rem;font-weight: 700;">属性名称：{{ props.text }}</div>
                <div style="display: flex;align-items: center;">
                    <el-date-picker v-model="propertyText" type="datetimerange" range-separator="至"
                        start-placeholder="开始时间" end-placeholder="结束时间" :default-time="defaultTime1"
                        value-format="YYYY-MM-DD HH:mm:ss" style="width:17rem" @change="resetForm" />
                    <el-button type="primary" icon="Search" style="margin-left: 0.7rem;"
                        @click="resetForm">刷新</el-button>
                    <el-button type="primary" icon="Download" @click="downloadCont">导出</el-button>
                </div>
            </div>
            <div v-if="graphicType === 1">
                <el-table :data="gridData">
                    <el-table-column prop="identifier" label="标识符" />
                    <el-table-column prop="value" label="属性值" />
                    <el-table-column prop="timestamp" label="时间" />
                </el-table>
                <div style="display: flex;justify-content:flex-end;margin-top: 1rem;" v-if="gridData.length > 0">
                    <el-pagination background layout="prev, pager, next, total" :total="paginnation.total"
                        :page-size="paginnation.pageSize" :current-page="paginnation.currentPage"
                        @current-change="currentChange" />
                </div>
            </div>
            <div v-if="graphicType === 2" id="barId" style="width: 100%;height: 50vh;">柱状图</div>
            <div v-if="graphicType === 3" id="lineId" style="width: 100%;height: 50vh;">折线图</div>
            <template #footer>
                <el-radio-group v-model="graphicType">
                    <el-radio-button :value="1">表格</el-radio-button>
                    <el-radio-button :value="2">柱状图</el-radio-button>
                    <el-radio-button :value="3">折线图</el-radio-button>
                </el-radio-group>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { historical_deviceanalysis, exports } from '@/api/facilityList/index'
import useCounterStore from "@/stores/counter";
import { postDownload, parseFilenameFromContentDisposition } from '@/utils/publicFun'
let store = useCounterStore();
let { user_id } = storeToRefs(store);
import * as echarts from "echarts";
import { ref, watch, onMounted, nextTick } from 'vue';

const props = defineProps({
    text: {
        type: String,
        required: false
    },
    identifier: {
        type: String,
        required: false
    },
    dev_id: {
        type: String,
        required: false
    }
});

let emit = defineEmits(["handleClose"]);

let dialogTableVisible = ref(true);
let propertyText = ref([]);
let gridData = ref([]);
let graphicType = ref(1);
let paginnation = ref({ total: 0, pageSize: 10, currentPage: 1 }); // 这里修复：pageSize 默认 10 更合理

let defaultTime1 = [
    new Date(new Date().setHours(new Date().getHours(), new Date().getMinutes(), new Date().getSeconds())),
    new Date(2000, 1, 1, 0, 0, 0)
]

let timestampArray = ref([]);
let valueArray = ref([]);

watch(graphicType, (val) => {
    nextTick(() => {
        if (val === 2) getBar();
        if (val === 3) getLine();
    })
})

onMounted(() => {
    list();
})

/**
 * 柱状图
 */
let getBar = () => {
    let chartDom = document.getElementById("barId");
    if (!chartDom) return;
    let myChart = echarts.init(chartDom);
    let option = {
        tooltip: { trigger: 'axis' }, // 悬浮提示
        xAxis: { type: "category", data: timestampArray.value },
        yAxis: { type: "value" },
        series: [{
            data: valueArray.value,
            type: "bar",
            showBackground: true,
            backgroundStyle: { color: "rgba(170,170,170,0.2)" },
            barWidth: 40,
        }]
    };
    myChart.setOption(option);
}

/**
 * 折线图
 */
let getLine = () => {
    let chartDom = document.getElementById("lineId");
    if (!chartDom) return;
    let myChart = echarts.init(chartDom);
    let option = {
        tooltip: { trigger: 'axis' }, // 悬浮提示
        xAxis: { type: 'category', data: timestampArray.value },
        yAxis: { type: 'value' },
        series: [{ data: valueArray.value, type: 'line' }]
    };
    myChart.setOption(option);
}

/**
 * 分页
 */
let currentChange = (e: any) => {
    paginnation.value.currentPage = e;
    list();
}

/**
 * 获取数据列表
 */
let list = () => {
    let params = {
        identifier: props.identifier,
        dev_id: props.dev_id,
        pagenumber: paginnation.value.currentPage,
        pagesize: paginnation.value.pageSize
    }
    if (propertyText.value?.length === 2) {
        params.start_time = propertyText.value[0];
        params.end_time = propertyText.value[1];
    }

    historical_deviceanalysis(params).then((res: any) => {
        if (res.code == 200) {
            // 赋值新数据（自动覆盖旧的第四页数据）
            gridData.value = res.data.items || [];
            paginnation.value.total = res.data.total || 0;

            // 更新图表数据
            timestampArray.value = gridData.value.map((item: any) => item.timestamp);
            valueArray.value = gridData.value.map((item: any) => item.value);

            // 切换图表时重新渲染
            nextTick(() => {
                if (graphicType.value === 2) getBar();
                if (graphicType.value === 3) getLine();
            });
        }
    });
}

// 查询 / 刷新（修复点）
let resetForm = () => {
    paginnation.value.currentPage = 1;
    gridData.value = [];
    timestampArray.value = [];
    valueArray.value = [];
    list();
}

// 导出
let downloadCont = () => {
    let params = {
        identifier: props.identifier,
        dev_id: props.dev_id,
        pagenumber: paginnation.value.currentPage,
        pagesize: paginnation.value.pageSize,
        export_format: "csv",
        total:paginnation.value.total
    }
    if (propertyText.value?.length === 2) {
        params.start_time = propertyText.value[0];
        params.end_time = propertyText.value[1];
    }
    exports(params).then((res: any) => {

        // const contentDisposition = res.headers.get('content-disposition');
        // console.log('contentDisposition', contentDisposition)
        // const filenameStarMatch = contentDisposition.match(/filename\*=UTF-8''(.+)/);
        // console.log('filenameStarMatch', filenameStarMatch)
        // let fileName = decodeURIComponent(filenameStarMatch[1]);
        // 2. 安全解析文件名（兜底方案）
        // 2. 安全解析文件名（兜底方案）
        let fileName = `历史数据_${new Date().toISOString().slice(0, 10)}.csv`;
        const contentDisposition =
            res.headers.get('content-disposition') ||
            res.headers.get('Content-Disposition');

        if (contentDisposition) {
            const utf8Match = contentDisposition.match(/filename\*=UTF-8''(.+)/);
            const normalMatch = contentDisposition.match(/filename="?([^";]+)"?/);
            if (utf8Match?.[1]) {
                fileName = decodeURIComponent(utf8Match[1]);
            } else if (normalMatch?.[1]) {
                fileName = decodeURIComponent(normalMatch[1]);
            }
        }

        // 3. 关键！传 res.data 而不是 res
        postDownload(res.data, fileName);
    })
}
</script>

<style lang="scss" scoped>
:deep(.el-date-editor .el-range-input),
:deep(.el-button [class*=el-icon]+span) {
    font-size: 0.8rem;
}
</style>
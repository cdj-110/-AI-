<template>
    <div style="overflow: auto; height: 58vh;padding: 0 0.5rem 0 0;">
        <el-form :inline="true" :model="formInline" style="text-align: right;">
            <el-form-item>
                <el-date-picker v-model="formInline.time" type="datetimerange" :range-separator="t('facilityLog.to')"
                    :start-placeholder="t('facilityLog.startTime')" :end-placeholder="t('facilityLog.endTime')" style="width:17rem" :disabled-date="disabledDate"
                    value-format="YYYY-MM-DD HH:mm:ss" />
            </el-form-item>
            <el-form-item style="width: 10vw;">
                <el-select v-model="formInline.log_type" :placeholder="t('facilityLog.logType')" clearable>
                    <el-option v-for="(item, index) in attributeList" :key="index" :label="item.label"
                        :value="item.value" />
                </el-select>
            </el-form-item>
            <el-form-item>
                <el-button type="primary" icon="Search" @click="resetForm">{{ t('facilityLog.query') }}</el-button>
                <el-button type="primary" icon="Refresh" @click="refresh">{{ t('facilityLog.refresh') }}</el-button>
            </el-form-item>
        </el-form>
        <el-table :data="facilityList">
            <el-table-column prop="log_type" :label="t('facilityLog.logType')" align="center" width="200">
                <template #default="scope">
                    <div style="display: flex; justify-content: center; align-items: center; gap: 4px;">
                        <el-icon :color="logConfig[scope.row.log_type]?.color">
                            <component :is="logConfig[scope.row.log_type]?.icon" />
                        </el-icon>
                        {{ logConfig[scope.row.log_type]?.label || scope.row.log_type }}
                        <img v-if="logConfig[scope.row.log_type]?.arrowImg"
                            :src="logConfig[scope.row.log_type]?.arrowImg"
                            :style="{ width: '14px', height: '14px', objectFit: 'contain' }" />
                    </div>
                </template>
            </el-table-column>
            <el-table-column prop="raw_data" :label="t('facilityLog.content')">
                <template #default="scope">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <div style="flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                            {{ scope.row.raw_data ? JSON.stringify(scope.row.raw_data).replace(/"([^":]+)":/g,
                                '$1:').replace(/"([^}]+)"/g, '$1') : '' }}
                        </div>
                        <span
                            style="display: flex; align-items: center; gap: 4px; white-space: nowrap; cursor: pointer;"
                            class="detail-btn" @click="detalis(scope.row)">
                            <el-icon>
                                <View />
                            </el-icon>
                            <span style="font-size: 0.7rem;">{{ t('facilityLog.details') }}</span>
                        </span>
                    </div>
                </template>
            </el-table-column>
            <!-- ---------------------------- -->
            <!-- <el-table-column prop="raw_data" label="内容" show-overflow-tooltip>
                <template #default="scope">
                    <div style="display: flex; align-items: center;gap: 8px;">
                        {{ scope.row.raw_data ? JSON.stringify(scope.row.raw_data).replace(/"([^":]+)":/g,
                            '$1:').replace(/"([^}]+)"/g, '$1') : '' }}
                        <span style="display: flex; align-items: center; gap: 4px; white-space: nowrap;cursor: pointer;"
                            class="detail-btn" @click="detalis(scope.row)">
                            <el-icon>
                                <View />
                            </el-icon>
                            <span style="font-size: 0.7rem;">详情</span>
                        </span>
                    </div>
                </template>
            </el-table-column> -->
            <el-table-column prop="timestamp" :label="t('facilityLog.time')" align="center" width="300" />
        </el-table>
        <div style="display: flex;justify-content: flex-end;margin: 0.5rem 0 0.8rem 0;">
            <el-pagination v-model:current-page="page" v-model:page-size="size"
                :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next" :total="total"
                @size-change="handleSizeChange" @current-change="handleCurrentChange" />
        </div>
        <el-dialog v-model="listTrue" :title="t('facilityLog.viewMessage')" width="30%" :before-close="handleClose" :destroy-on-close="true">
            <div class="text">{{ t('facilityLog.messageType') }}</div>
            <div class="text_Color"> {{ logConfig[detaliForm.log_type]?.label || detaliForm.log_type
            }}</div>
            <div class="text">{{ t('facilityLog.messageStatus') }}</div>
            <div class="text_Color" style="display: flex;align-items: center;">
                <el-icon :color="logConfig[detaliForm.log_type]?.color">
                    <component :is="logConfig[detaliForm.log_type]?.icon" />
                </el-icon>
                <span style="font-size: 0.8rem;"> {{ detaliForm.log_type == 'event_reporting_type_error' ? t('facilityLog.failure') : t('facilityLog.success')
                }}</span>
            </div>
            <div class="text">{{ t('facilityLog.messageTime') }}</div>
            <div class="text_Color"> {{ detaliForm.timestamp }}</div>
            <div class="text">{{ t('facilityLog.messageContent') }}</div>

            <!-- 核心修改 2：外层包 div 并绑定 key -->
            <div :key="monacoKey">
                <monacoEditor height="30vh" :config="monacoConfig" :readOnly="true">
                </monacoEditor>
            </div>

            <template #footer>
                <el-button plain @click="handleClose" style="font-size: 0.8rem;">{{ t('facilityLog.close') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>

import { ref, onMounted, nextTick } from 'vue' // 确保引入 nextTick
import { CircleCheck, CircleClose, View } from '@element-plus/icons-vue' // 补全 View 图标引入
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
import arrowUpImg from '@/assets/arrow-up.png'
import arrowDownImg from '@/assets/arrow-down.png'
import monacoEditor from "@/components/monacoEditor.vue";
import { historical_devicelog, all_types } from "@/api/facilityLog/index";

const props = defineProps({
    dev_id: {
        type: String,
        required: true
    },
});

// 数据定义
let formInline = ref({ time: [], log_type: '' });
let facilityList = ref([]);
let page = ref(1)
let size = ref(10)
let total = ref(0)
let listTrue = ref(false)
let detaliForm = ref({});

// 核心修改 3：新增强制刷新 Key 和 Monaco 静态配置
const monacoKey = ref(0);
const monacoConfig = ref({
    value: '',
    language: 'json',
    theme: 'vs-dark'
});

const disabledDate = (time: Date) => {
    return time.getTime() > Date.now()
}

const attributeList = ref([])

onMounted(() => {
    list()
    all_types().then((res: any) => {
        if (res.code == 200) {
            for (const key in res.data) {
                if (Object.prototype.hasOwnProperty.call(res.data, key)) {
                    attributeList.value.push({ label: res.data[key], value: key })
                }
            }
        }
    })
})

const logConfig = {
    attribute_downlink_type: {
        label: t('facilityLog.attributeDownlink'),
        color: '#03DA6B',
        icon: CircleCheck,
        arrowImg: arrowDownImg,
    },
    attribute_reporting_type: {
        label: t('facilityLog.attributeReporting'),
        color: '#03DA6B',
        icon: CircleCheck,
        arrowImg: arrowUpImg,
    },
    event_reporting_type: {
        label: t('facilityLog.eventReporting'),
        color: '#03DA6B',
        icon: CircleCheck,
        arrowImg: arrowUpImg,
    },
    event_reporting_type_error: {
        label: t('facilityLog.eventReporting'),
        color: 'red',
        icon: CircleClose,
        arrowImg: arrowUpImg,
    },
}

let list = () => {
    const params: any = {
        dev_id: props.dev_id,
        log_type: formInline.value.log_type,
        pagenumber: page.value,
        pagesize: size.value
    }

    if (formInline.value.time && formInline.value.time.length === 2) {
        params.start_time = formInline.value.time[0]
        params.end_time = formInline.value.time[1]
    }
    historical_devicelog(params).then((res: any) => {
        if (res.code == 200) {
            facilityList.value = res.data.items
            total.value = res.data.total
        }
    })
}

let resetForm = () => {
    list()
}
let refresh = () => {
    list()
}

// 修复 Bug：这里必须更新 size.value，否则切换每页条数不生效
let handleSizeChange = (val: number) => {
    size.value = val
    list()
}

let handleCurrentChange = (val: number) => {
    page.value = val
    list()
}

// 核心修改 4：完全重写详情打开函数
const detalis = (row: any) => {
    // 1. 直接赋值数据
    detaliForm.value = row

    // 2. 处理 Monaco 显示内容
    const rawValue = row.raw_data
    monacoConfig.value.value = typeof rawValue === 'string'
        ? rawValue
        : JSON.stringify(rawValue, null, 2)

    // 3. Key + 1，强制 DOM 重建
    monacoKey.value++

    // 4. 打开弹窗
    listTrue.value = true
}

let handleClose = () => {
    listTrue.value = false
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}

.detail-btn:hover {
    color: #409eff;
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
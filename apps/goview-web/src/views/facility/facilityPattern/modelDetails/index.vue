<template>
    <div style="background: #eeeeee;height: 100%;">
        <!-- overflow-y: auto; -->
        <el-card>
            <div style="padding: 0 0 0.9rem 0;font-size: 14px;display: flex;align-items: center;width: 4rem;"  @click="goBack">
                <el-icon>
                    <ArrowLeft />
                </el-icon><span style="padding: 0 0 0 0.3rem;cursor: pointer;">{{ t('facilityPattern.back') }}</span>
            </div>
            <div class="top">
                <div class="top_left">
                    <el-image class="device-img" :src="deviceMap[form.device_type]?.img" fit="cover" />
                </div>
                <div class="top_cont">
                    <div class="top_cont_1">
                        <p style="font-weight: 700;">{{ form.model_name }}</p>
                        <el-icon style="margin-left: 1rem;cursor: pointer;" :size="20" @click="editClick">
                            <Edit />
                        </el-icon>
                    </div>
                    <!-- 设备类型：模板里直接转换 -->
                    <div class="top_cont_text">
                        <p class="top_cont_text_col1">{{ t('facilityPattern.deviceType') }}：</p>
                        <p class="top_cont_text_col2">{{ deviceTypeMap[form.device_type] || form.device_type }}</p>
                    </div>

                    <!-- 通信方式：模板里直接转换 -->
                    <div class="top_cont_text">
                        <p class="top_cont_text_col1">{{ t('facilityPattern.communication') }}：</p>
                        <p class="top_cont_text_col2">{{ communicationMap[form.communication] || form.communication }}
                        </p>
                    </div>
                    <div class="top_cont_text">
                        <p class="top_cont_text_col1">{{ t('facilityPattern.accessProtocol') }}：</p>
                        <p class="top_cont_text_col2">{{ form.protocol }}</p>
                    </div>
                </div>
                <div class="top_right">
                    <div style="display: flex;align-items: center;">&nbsp;</div>
                    <div class="top_right_text">
                        <p class="top_right_text_col1">{{ t('facilityPattern.createTime') }}：</p>
                        <p class="top_right_text_col2">{{ form.create_time }}</p>
                    </div>
                    <div class="top_right_text">
                        <p class="top_right_text_col1">{{ t('facilityPattern.modelDescription') }}：</p>
                        <p class="top_right_text_col2">{{ form.model_description }}</p>
                    </div>
                </div>
            </div>
        </el-card>
        <el-card style="margin-top: 1rem;height: 64vh;">
            <!-- overflow: auto; -->
            <el-tabs v-model="activeName" type="card">
                <el-tab-pane :label="t('facilityPattern.associatedDevice')" :name="1"
                    v-if="objData.typeId == 'GW' || objData.typeId == 'GSD' || objData.typeId == 'DD'">
                    <associatedApparatus :id="objData.model_id" :valueData="objData" v-if="activeName == 1">
                    </associatedApparatus>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityPattern.functionDefinition')" :name="2"
                    v-if="objData.typeId == 'GW' || objData.typeId == 'GSD' || objData.typeId == 'DD'">
                    <functionDefinition :id="objData.model_id" v-if="activeName == 2" :model_name="form.model_name">
                    </functionDefinition>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityPattern.dataPointConfig')" :name="3" v-if="objData.typeId == 'GSD'">
                    <pointConfig :id="objData.model_id" v-if="activeName == 3"></pointConfig>
                </el-tab-pane>
            </el-tabs>
        </el-card>
        <!-- 详情编辑 -->
        <editDialog v-if="isShow" @handleClose="handleClose" :detailObj="form" :model_id="objData.model_id" :model_type="objData.model_type">
        </editDialog>
    </div>
</template>
<script lang="ts" setup>
import { useRoute } from 'vue-router';
import { ref, onMounted, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import associatedApparatus from './associatedApparatus/index.vue'
import functionDefinition from './functionDefinition/index.vue'
import pointConfig from './pointConfig/index.vue';
import editDialog from './editDialog.vue'
import { Edit } from '@element-plus/icons-vue';
import { model_info } from '@/api/facilityPattern/index'
import useCounterStore from "@/stores/counter";
import img1 from '@/assets/wgmx.png'
import img2 from '@/assets/zsbmx.png'
import img3 from '@/assets/zlsb.png'

const { t } = useI18n();
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let route = useRoute();
let router = useRouter()
let activeName = ref(1);//详情标签默认选中哪个
let isShow = ref(false) // 详情编辑弹窗
let form = ref({ model_name: '', device_type: '', communication: '', create_time: '', protocol: '', model_description: '' }) // 详情数据
let objData = ref({
    model_type: route.query.model_type,
    model_id: route.query.model_id,
    model_name: route.query.model_name,
    communication: route.query.communication,
    typeId: route.query.typeId
})
// 设备类型ID → 名称映射
const deviceTypeMap = computed(() => ({
    GW: t('facilityPattern.gateway'),
    GSD: t('facilityPattern.gatewaySubDevice'),
    DD: t('facilityPattern.directDevice')
}));

// 通信方式ID → 名称映射
const communicationMap = computed(() => ({
    WIFI: 'WIFI',
    CELLULAR: t('facilityPattern.cellular'),
    BLUETOOTH: t('facilityPattern.bluetooth'),
    'NB-IOT': t('facilityPattern.nbIoT'),
    OTHER: t('facilityPattern.other')
}));
// 核心映射表
const deviceMap = computed(() => ({
    GW: { name: t('facilityPattern.gateway'), img: img1 },
    GSD: { name: t('facilityPattern.gatewaySubDevice'), img: img2 },
    DD: { name: t('facilityPattern.directDevice'), img: img3 }
}))
watch(() => route.query, (val) => {
    if (val) {
        if (val.upDataStr == 'updata') {
            objData.value.typeId = val.device_type
            objData.value.model_id = val.model_id
            objData.value.model_name = val.model_name
            activeName.value = 2
        }
    }
},
    { immediate: true } // 组件初始化时立即执行一次
);
onMounted(() => {
    DetailList()
})

/**
 * 模型详情
 */
let DetailList = () => {
    model_info({ user_id: user_id.value, model_id: objData.value.model_id }).then((res: any) => {
        if (res.code == 200) {
            form.value = res.data;
        }
    })
}
/**
 *编辑
 */
let editClick = () => {
    isShow.value = true
}

// 返回
let goBack = () => {
    router.push('/facilityPattern')
}
// 关闭编辑弹窗
let handleClose = (e: any) => { DetailList(), isShow.value = false }
</script>
<style lang="scss" scoped>
.top {
    display: flex;
    width: 100%;
    align-items: center;

    .top_left {
        border: 1px solid #c0c5cc;
        padding: 0.7rem 1rem;
        border-radius: 10px;

        img {
            width: 7vw;
        }
    }

    .top_cont {
        margin-left: 3rem;

        .top_cont_1 {
            display: flex;
            align-items: center;
        }

        .top_cont_text {
            margin-top: 1rem;
            display: flex;
            align-items: center;
            font-size: 0.9rem;

            .top_cont_text_col1 {
                color: #AAAAAA;
            }

            .top_cont_text_col2 {
                color: #7E7779;
            }
        }
    }

    .top_right {
        margin-left: 12rem;

        .top_right_text {
            margin-top: 1rem;
            display: flex;
            align-items: center;
            font-size: 0.9rem;

            .top_right_text_col1 {
                color: #AAAAAA;
            }

            .top_right_text_col2 {
                color: #7E7779;
            }
        }
    }
}

:deep(.el-tabs--card>.el-tabs__header) {
    border-bottom: none;
}

:deep(.el-tabs--card>.el-tabs__header .el-tabs__item) {
    border-bottom: none;
    border-left: none;
    height: 1.95rem;
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
</style>
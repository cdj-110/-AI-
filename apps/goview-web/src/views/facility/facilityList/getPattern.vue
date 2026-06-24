<template>
    <div>
        <el-dialog v-model="props.getPatternConfig.bool" :modal="props.getPatternConfig.modal"
            :title="props.getPatternConfig.title" :show-close="props.getPatternConfig.showClose"
            :width="props.getPatternConfig.width" :draggable="props.getPatternConfig.draggable"
            :close-on-press-escape="false" :close-on-click-modal="false">
            <el-tabs v-model="activeName" editable @edit="emit('createPatternChange')" @tab-click="handleClick">
                <template #add-icon>
                    <el-button type="primary">{{ t('facilityList.createDeviceModel') }}</el-button>
                </template>
                <el-tab-pane v-for="(item, index) in paneZhongList" :key="index" :label="item.title" :name="item.title">
                    <div v-for="(ccc, i) in item.listTitle" :key="i" :style="i != 0 ? 'margin-top: 2rem;' : ''">
                        <div>{{ ccc.title }}</div>
                        <div style="display: flex;align-items: center;flex-wrap: wrap;">
                            <div class="paneClass" v-for="(vvv, u) in ccc.paneList" :key="u"
                                :class="{ 'active': selectedModel?.model_id === vvv.model_id }"
                                @click="handleModelClick(item, ccc, vvv)">
                                <img src="@/assets/aa.png" style="width: 5rem;padding: 0.5rem;">
                                <div style="margin-left: 0.7rem;">
                                    <p>{{ vvv.model_name }}</p>
                                    <p style="padding-top: 0.7rem;">{{ vvv.protocol }}</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </el-tab-pane>
            </el-tabs>

            <!-- 当 type == '1' 时显示确定按钮 -->
            <template #footer v-if="props.getPatternConfig.type == '1'">
                <el-button @click="props.getPatternConfig.bool = false">{{ t('facilityList.cancel') }}</el-button>
                <el-button type="primary" @click="handleConfirm" :disabled="!selectedModel">{{ t('facilityList.confirm') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue';
import { model_list, device_detail_bind_model } from "@/api/facilityPattern";
import useCounterStore from "@/stores/counter";
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

let store = useCounterStore();
let { user_id } = storeToRefs(store);

/**
 * @function paneOnChange 选择模板时触发，有三个参数。
 * @function createPatternChange 点击创建模板时触发，一般用来打开创建模板弹窗
 */
let emit = defineEmits(["paneOnChange", "createPatternChange", "sucssClick"]);

// 从父组件传过来的值
let props = withDefaults(defineProps<{
    getPatternConfig: {
        bool: Boolean,
        modal: Boolean,
        title: string,
        width: string,
        draggable: Boolean,
        showClose: Boolean,
        type: string
    },
    dev_id: {
        type: String,
        required: false
    }
}>(), {
    getPatternConfig: () => ({
        bool: false,
        modal: true,
        title: "",
        width: "",
        draggable: true,
        showClose: true,
        type: ""
    })
});

// 选中的模型
const selectedModel = ref<any>(null);
let activeName = ref(t('facilityList.weControlGatewayModel'));
let model_type = ref(0)
let device_type = ref('GW')
// 处理模型点击
const handleModelClick = (item: any, ccc: any, vvv: any) => {
    if (props.getPatternConfig.type == '1') {
        // type == '1' 时，只记录选中状态，不调用接口
        selectedModel.value = vvv;
    } else {
        // 原逻辑不变
        emit('paneOnChange', item, ccc, vvv);
    }
};

// 处理确定按钮点击
const handleConfirm = () => {
    if (selectedModel.value) {
        device_detail_bind_model({ user_id: user_id.value, dev_id: props.dev_id , model_id: selectedModel.value.model_id }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                emit('sucssClick')
            } else {
                ElMessage.error(res.msg);
            }
            props.getPatternConfig.bool = false;
        })
    }
};

watch(props.getPatternConfig, (val) => {
    if (val.bool == true) {
        !props.getPatternConfig.title ? props.getPatternConfig.showClose = false : props.getPatternConfig.showClose = true;
        // 重置选中状态
        selectedModel.value = null;
        getList();

    }
}, { immediate: true, deep: true });
// onMounted(() => {
//     getList();
// });


const handleClick = (tab: any, event: Event) => {
    if (tab.props.name == t('facilityList.weControlGatewayModel')) {
        device_type.value = 'GW'
    } else {
        device_type.value = ''
    }
    model_type.value = tab.props.name == t('facilityList.weControlGatewayModel') ? 0 : tab.props.name == t('facilityList.createCustomDeviceModel') ? 1 : 0;
    getList()
}
// 模型的列表
let paneZhongList = ref<Array<{ title: string, listTitle: Array<{ title: string, paneList: Array<any> }> }>>([
    {
        title: t('facilityList.weControlGatewayModel'),
        listTitle: [{ title: "", paneList: [] }]
    },
    {
        title: t('facilityList.createCustomDeviceModel'),
        listTitle: [{ title: "", paneList: [] }, { title: "", paneList: [] }, { title: "", paneList: [] }]
    }
]);

let getList = () => {
    model_list({ user_id: user_id.value, device_name: "", device_type: device_type.value, model_type: model_type.value }).then((res: any) => {
        if (res.code == 200) {
            paneZhongList.value[0].listTitle[0].paneList = [];
            paneZhongList.value[1].listTitle[0].paneList = [];
            paneZhongList.value[1].listTitle[1].paneList = [];
            paneZhongList.value[1].listTitle[2].paneList = [];
            res.data.data.forEach((item: any) => {
                if (item.model_type == 0) {
                    paneZhongList.value[0].listTitle[0].paneList.push(item);
                } else {
                    if (item.device_type == "GW") {
                        paneZhongList.value[1].listTitle[0].paneList.push(item);
                        paneZhongList.value[1].listTitle[0].title = "网关";
                    } else if (item.device_type == "GSD") {
                        paneZhongList.value[1].listTitle[1].paneList.push(item);
                        paneZhongList.value[1].listTitle[1].title = "网关子设备";
                    } else if (item.device_type == "DD") {
                        paneZhongList.value[1].listTitle[2].paneList.push(item);
                        paneZhongList.value[1].listTitle[2].title = "直连设备";
                    }
                }
            });
        }
    });
};

defineExpose({ getList });
</script>

<style lang="scss" scoped>
:deep(.el-tabs__item .is-icon-close) {
    display: none;
}

:deep(.el-tabs__new-tab) {
    width: 8vw;
}

.paneClass {
    display: flex;
    align-items: center;
    border: 1px solid #eeeeee;
    border-radius: 7px;
    padding: 0 0.5rem;
    margin-top: 1rem;
    overflow: auto;
    cursor: pointer;
    // flex: calc(100% / 4 - 3%);
    margin-left: 1%;
    transition: all 0.3s;

    &:hover {
        border-color: #3C93E7;
    }

    &.active {
        border-color: #3C93E7;
        background-color: #e6f3ff;
        // box-shadow: 0 0 10px rgba(60, 147, 231, 0.3);
        // color: ;
    }
}
</style>
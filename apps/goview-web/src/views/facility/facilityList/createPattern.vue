<template>
    <div class="createPattern">
        <el-dialog v-model="props.createPatternConfig.bool" :modal="props.createPatternConfig.modal"
            :title="props.createPatternConfig.title" :show-close="props.createPatternConfig.showClose"
            :width="props.createPatternConfig.width" :draggable="props.createPatternConfig.draggable"
            :close-on-press-escape="false" :close-on-click-modal="false">
            <el-tabs v-model="int1" tab-position="left" @tab-click="handleClick">
                <el-tab-pane :label="t('facilityList.weControlProductModel')" :name="t('facilityList.weControlProductModel')" style="margin-left: 1rem;">
                    <el-tabs v-if="createVer == 0" v-model="int2">
                        <el-tab-pane v-for="(item, index) in generalList" :key="index" :label="item.label"
                            :name="item.name">
                            <div
                                style="display: flex;align-items: center;flex-wrap: wrap;justify-content: space-between;">
                                <div v-for="(vvv, index) in item.generalList" :key="index"
                                    @click="createiniter = { user_id: user_id, ...vvv }; createVer = 1;"
                                    style="display: flex;align-items: center;border: 1px solid #eee;padding: 0.3rem 0.7rem;cursor: pointer;border-radius: 10px;margin: 0.5rem;flex: 27%;">
                                    <img src="@/assets/aaa.png" style="width: 4rem;margin-right: 0.7rem;">
                                    <div>
                                        <p>{{ vvv.model_name }}</p>
                                        <p style="margin-top: 0.5rem;">{{ vvv.protocol }}</p>
                                    </div>
                                </div>
                            </div>
                        </el-tab-pane>
                    </el-tabs>
                    <el-form v-if="createVer == 1" :model="createiniter" label-width="auto" :rules="createRules"
                        ref="createRulesRef" style="width: 70%;margin: auto;">
                        <el-form-item :label="t('facilityList.modelName')" prop="model_name">
                            <el-input v-model="createiniter.model_name" :placeholder="t('facilityList.modelNamePlaceholder')" />
                        </el-form-item>
                        <el-form-item :label="t('facilityList.deviceType')" prop="device_type">
                            <el-radio-group v-model="createiniter.device_type" disabled>
                                <el-radio v-for="(item, index) in deviceTypeList" :key="index" :value="item.value"
                                    size="large">{{ item.label }}</el-radio>
                            </el-radio-group>
                        </el-form-item>
                        <el-alert
                            :title="createiniter.device_type == 'GW' ? t('facilityList.gatewayDesc') : createiniter.device_type == 'DD' ? t('facilityList.directDeviceDesc') : t('facilityList.subDeviceDesc')"
                            type="warning" :closable="false" style="margin-bottom: 1rem;" />
                        <el-form-item :label="t('facilityList.communication')" prop="communication">
                            <el-select v-model="createiniter.communication" disabled :placeholder="t('facilityList.communicationPlaceholder')">
                                <div>
                                    <el-option v-for="(item, index) in communicationList" :key="index"
                                        :label="item.label" :value="item.value" />
                                </div>
                            </el-select>
                        </el-form-item>
                        <el-form-item :label="t('facilityList.accessProtocol')" prop="protocol">
                            <el-select v-model="createiniter.protocol" disabled :placeholder="t('facilityList.protocolPlaceholder')">
                                <el-option v-for="(item, index) in protocolList" :key="index" :label="item.label"
                                    :value="item.value" />
                            </el-select>
                        </el-form-item>
                        <el-form-item :label="t('facilityList.modelDescription')" prop="model_description">
                            <el-input type="textarea" :rows="3" v-model="createiniter.model_description"
                                :placeholder="t('facilityList.modelDescriptionPlaceholder')" />
                        </el-form-item>
                    </el-form>
                </el-tab-pane>
                <el-tab-pane :label="t('facilityList.createCustomDeviceModel')" :name="t('facilityList.createCustomDeviceModel')">
                    <el-form v-if="createVer == 1" :model="createiniter" label-width="auto" :rules="createRules"
                        ref="createRulesRef" style="width: 70%;margin: auto;">
                        <el-form-item :label="t('facilityList.modelName')" prop="model_name">
                            <el-input v-model="createiniter.model_name" :placeholder="t('facilityList.modelNamePlaceholder')" />
                        </el-form-item>
                        <el-form-item :label="t('facilityList.deviceType')" prop="device_type">
                            <el-radio-group v-model="createiniter.device_type">
                                <el-radio v-for="(item, index) in deviceTypeList" :key="index" :value="item.value"
                                    size="large">{{ item.label }}</el-radio>
                            </el-radio-group>
                        </el-form-item>
                        <el-alert
                            :title="createiniter.device_type == 'GW' ? t('facilityList.gatewayDesc') : createiniter.device_type == 'DD' ? t('facilityList.directDeviceDesc') : t('facilityList.subDeviceDesc')"
                            type="warning" :closable="false" style="margin-bottom: 1rem;" />
                        <el-form-item :label="t('facilityList.communication')" prop="communication">
                            <el-select v-model="createiniter.communication" :placeholder="t('facilityList.communicationPlaceholder')">
                                <div>
                                    <el-option v-for="(item, index) in communicationList" :key="index"
                                        :label="item.label" :value="item.value" />
                                </div>
                            </el-select>
                        </el-form-item>
                        <el-form-item :label="t('facilityList.accessProtocol')" prop="protocol">
                            <el-select v-model="createiniter.protocol" :placeholder="t('facilityList.protocolPlaceholder')">
                                <el-option v-for="(item, index) in protocolList" :key="index" :label="item.label"
                                    :value="item.value" />
                            </el-select>
                        </el-form-item>
                        <el-form-item :label="t('facilityList.modelDescription')" prop="model_description">
                            <el-input type="textarea" :rows="3" v-model="createiniter.model_description"
                                :placeholder="t('facilityList.modelDescriptionPlaceholder')" />
                        </el-form-item>
                    </el-form>
                </el-tab-pane>
            </el-tabs>
            <template #footer>
                <el-button @click="props.createPatternConfig.bool = false">{{ t('facilityList.cancel') }}</el-button>
                <el-button type="primary" @click="createChange">{{ t('facilityList.confirm') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { addDevicemodel, modelDatas } from "@/api/facilityList";
import useCounterStore from "@/stores/counter";
import { model_list, product_models } from "@/api/facilityPattern";
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let emit = defineEmits(["refear"]);

//从父组件传过来的值
let props = withDefaults(defineProps<{
    createPatternConfig: {
        bool: Boolean,
        modal?: Boolean,
        title?: string,
        width?: string,
        draggable?: Boolean,
        showClose?: Boolean
    }
}>(), {
    createPatternConfig: () => ({
        bool: false,
        modal: true,
        title: "",
        width: "",
        draggable: true,
        showClose: true
    })
});

let int1 = ref(t('facilityList.weControlProductModel'));//整体切换
let int2 = ref(t('facilityList.gatewayDevice'));//微控产品模型切换
let createiniter = ref({ user_id: "", model_name: "", device_type: "GW", communication: "ETHERNET", protocol: "MQTT", model_description: "" });//创建设备模型数据保存
let createRules = ref({
    model_name: [{ required: true, message: t('facilityList.modelNameRequired'), trigger: "blur" }],
    device_type: [{ required: true, message: t('facilityList.deviceTypeRequired'), trigger: "blur" }],
    communication: [{ required: true, message: t('facilityList.communicationRequired'), trigger: "blur" }],
    protocol: [{ required: true, message: t('facilityList.protocolRequired'), trigger: "blur" }]
});
let createRulesRef: any = ref("");//表单Ref
let createVer = ref(0);//是否点击了默认模板列表 0:未点击 1:点击了
let deviceTypeList: any = ref([]);//设备类型
let communicationList: any = ref([]);//通信方式
let protocolList: any = ref([]);//接入协议
let model_type = ref('GW');
let generalList = ref([
    {
        label: t('facilityList.gatewayDevice'),
        name: t('facilityList.gatewayDevice'),
        generalList: []
    }
]);

onMounted(() => {
    //查询设备类型/接入方式和协议下拉框
    modelDatas().then((res: any) => {
        for (const key in res.data.communication) {
            communicationList.value.push({ label: key, value: res.data.communication[key] })
        }
        for (const key in res.data.protocol) {
            protocolList.value.push({ label: key, value: res.data.protocol[key] })
        }
        for (const key in res.data.device_type) {
            deviceTypeList.value.push({ label: key, value: res.data.device_type[key] })
        }
    })

    getList()
})
watch(props.createPatternConfig, (val) => {
    if (val) {
        createVer.value = 0
    }
})
/**
 * 添加模板
 */
let createChange = () => {
    createiniter.value.user_id = user_id.value;
    if (!createiniter.value.model_description) {
        createiniter.value.model_description = ''
    }
    if (model_type.value == 'GW') {
        const { model_id: wk_product_models, ...rest } = createiniter.value
        createiniter.value = { ...rest, model_type: 0, wk_product_models }
    } else {
        const { ...rest } = createiniter.value
        createiniter.value = { ...rest, model_type: 1, }
    }
    addDevicemodel(createiniter.value).then((res: any) => {
        if (res.code == 200) {
            ElMessage.success(res.msg);
            props.createPatternConfig.bool = false;
            emit("refear");
        } else {
            ElMessage.error(res.msg);
        }
    })
}
const handleClick = (e: any) => {
    if (e.props.name === t('facilityList.weControlProductModel')) {
        model_type.value = 'GW'
        createVer.value = 0
    } else {
        model_type.value = ''
    }
    if (e.props.name === t('facilityList.createCustomDeviceModel')) {
        createVer.value = 1
        createiniter.value.model_name = ''
        createiniter.value.model_description = ''
        createiniter.value.wk_product_models = ''
    } else {
        createVer.value = 0
    }

    getList()
}
let getList = () => {
    // model_list({ user_id: user_id.value, device_name: "", device_type: model_type.value, model_type: createVer.value }).then((res: any) => {
    //     if (res.code == 200) {
    //         generalList.value[0].generalList = res.data.data;
    //     }
    // });
    product_models().then((res: any) => {
        if (res.code == 200) {
            generalList.value[0].generalList = res.data.data;
        }
    })
};
</script>

<style lang="scss" scoped></style>
<template>
    <div style="overflow: auto; height: 58vh;padding: 0 0.5rem 0 0;">
        <div style="display: flex;align-items: center;padding: 0.7rem 1rem;"
            v-if="!props.device_type">
            <p style="font-size: 0.9rem;"><span style="color: red;margin-right: 0.2rem;">*</span>{{ t('networking.bindModel') }}</p>
            <div style="margin-left: 1rem;"></div>
            <el-button type="primary" plain style="margin-left: 1rem;"
                @click="getPatternConfig.bool = true;">{{ t('networking.selectCreatedModel') }}</el-button>
        </div>
        <h3 style="padding: 0.5rem 1rem;">{{ t('networking.deviceCertificate') }}</h3>
        <div style="padding: 0.5rem 1rem;font-size: 0.9rem;">{{ t('networking.accessTokenDesc') }}
        </div>
        <div style="padding: 0.7rem 1rem;">
            <el-button type="primary" icon="CopyDocument"
                @click="copyPublicChange(from.username)">{{ t('networking.copyAccessToken') }}</el-button>
            <el-button type="primary" icon="CopyDocument"
                @click="copyPublicChange(from.password)">{{ t('networking.copyProjectKey') }}</el-button>
        </div>
        <h3 style="padding: 0.5rem 1rem;">{{ t('networking.deviceMqttEndpoint') }}</h3>
        <div style="padding: 0.7rem 1rem;font-size: 0.9rem;">{{ t('networking.mqttDesc') }}</div>
        <div style="padding: 0.7rem 1rem;margin: 0.7rem 1rem;background-color: #423F40;color: #ffffff;">
            {{ from.mqtthost }}:{{ from.mqttport }}</div>
        <div style="padding: 0.7rem 1rem;font-size: 0.9rem;">
            <span style="font-weight: 500;">{{ t('networking.mqttHost') }}：</span>
            <span style="color: #cccccc;">{{ from.mqtthost }}</span>
            <el-icon style="margin-left: 0.5rem;cursor: pointer;">
                <CopyDocument @click="copyPublicChange(from.mqtthost)" />
            </el-icon>
        </div>
        <div style="padding: 0.7rem 1rem;font-size: 0.9rem;">
            <span style="font-weight: 500;">{{ t('networking.mqttPort') }}：</span>
            <span style="color: #cccccc;">{{ from.mqttport }}</span>
            <el-icon style="margin-left: 0.5rem;cursor: pointer;">
                <CopyDocument @click="copyPublicChange(from.mqttport)" />
            </el-icon>
        </div>
        <div style="padding: 0.7rem 1rem;font-size: 0.9rem;">
            <span style="font-weight: 500;">Username：</span>
            <span style="color: #cccccc;cursor: pointer;">
                < {{ from.username }}>
            </span>
            <el-icon style="margin-left: 0.5rem;cursor: pointer;">
                <CopyDocument @click="copyPublicChange(from.username)" />
            </el-icon>
        </div>
        <div style="padding: 0.7rem 1rem;font-size: 0.9rem;">
            <span style="font-weight: 500;">Password：</span>
            <span style="color: #cccccc;cursor: pointer;">
                < {{ from.password }}>
            </span>
            <el-icon style="margin-left: 0.5rem;cursor: pointer;">
                <CopyDocument @click="copyPublicChange(from.password)" />
            </el-icon>
        </div>
        <createPattern :createPatternConfig="createPatternConfig" @refear="getList();"></createPattern>
        <getPattern ref="getPatternRef" :getPatternConfig="getPatternConfig" @paneOnChange="paneOnChange"
            @createPatternChange="createPatternConfig.bool = true;" @sucssClick="sucssClick" :dev_id="props.dev_id"></getPattern>
    </div>

</template>

<script lang="ts" setup>
import { copyPublicChange } from "@/utils/publicFun";
import { device_detail_connect } from '@/api/facilityList/index'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
// let props: any = withDefaults(defineProps<{
//     particularConfig: Object
// }>(),{
//     particularConfig:() => ({
//         isTrue: false,
//         list: {}
//     })
// });
let emit = defineEmits(["infoClick"]);
import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
const props = defineProps({
    dev_id: {
        type: String,
        required: ''
    },
    device_type: {
        type: String,
        required: ''
    }

});
let getPatternRef: any = ref("");//选择模型Ref
let createPatternConfig = ref({ bool: false, modal: true, title: "创建设备模型", width: "55%", draggable: true, showClose: true });//创建设备模型的配置项 - 传入子组件中的值
let getPatternConfig = ref({ bool: false, modal: true, title: "设备模型", width: "55%", draggable: true, showClose: true, type: '1', dev_id: props.dev_id });//选择设备模型的配置项 - 传入子组件中的值
let route: any = useRoute();
let from = ref({ username: '', password: '', mqtthost: '', mqttport: '' })

onMounted(() => {
    list()
})
let list = () => {
    device_detail_connect({ dev_id: props.dev_id, user_id: user_id.value }).then((res: any) => {
        console.log(res, 'res')
        if (res.code == 200) {
            from.value = res.data
        }
    })
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

let sucssClick = () => {
    list()
    emit("infoClick");
}

/**
 * 渲染模型和触发子组件渲染模型
 */
let getList = () => {
    getPatternRef.value.getList();
}

</script>
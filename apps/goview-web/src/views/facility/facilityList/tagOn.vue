<template>
    <div class="tagOn">
        <el-dialog v-model="props.tagOnConfig.bool" :title="t('facilityList.addDeviceTitle')" width="30%" draggable
            :close-on-press-escape="false" :close-on-click-modal="false">
            <div style="padding: 0 20px;">
                <el-form v-if="!facilityTrue" :model="form" label-width="auto" :rules="rules" ref="rulesRef"
                    style="max-height: 50vh;overflow-y: auto;">
                    <el-form-item :label="t('facilityList.deviceName')" prop="device_name">
                        <el-input v-model="form.device_name" :placeholder="t('facilityList.modelNamePlaceholder')" />
                    </el-form-item>
                    <el-form-item :label="t('facilityList.serialNumber')" prop="device_sn">
                        <el-input v-model="form.device_sn" :placeholder="t('facilityList.serialNumberPlaceholder')" />
                    </el-form-item>
                    <el-form-item :label="t('facilityList.modelName')" prop="device_model_value">
                        <el-input v-model="form.device_model_value" @click="getPatternConfig.bool = true;" readonly
                            :placeholder="t('facilityList.selectModel')" />
                    </el-form-item>
                    <el-form-item v-for="(item, index) in ziFacility" :key="index" v-if="paneTrue"
                        :label="t('facilityList.subDevice')">
                        <el-input v-model="item.slave_address" :placeholder="t('facilityList.enterSlaveAddress')"
                            style="width: 40%;" />
                        <el-input v-model="item.sub_dev_name" :placeholder="t('facilityList.selectSubDevice')"
                            @click="add" style="width: 45%;margin-left: 1%;" />
                        <div style="margin-left: 3%;cursor: pointer;color: red;" @click="ziPaneChange(index)">{{
                            t('facilityList.delete') }}</div>
                    </el-form-item>
                    <!-- <el-form-item v-if="paneTrue">
                        <el-button type="primary" style="width: 100%;" @click="ziPaneChange('添加')">{{
                            t('facilityList.addSubDevice') }}</el-button>
                    </el-form-item> -->
                    <div v-if="paneTrue" style="display: flex;justify-content: flex-end;margin: 0 0 1rem 0;">
                        <el-button type="primary" style="width: 80%;" @click="ziPaneChange('添加')">{{
                            t('facilityList.addSubDevice') }}</el-button>
                    </div>
                    <el-form-item :label="t('facilityList.deviceGroup')">
                        <el-select v-model="form.device_group" :placeholder="t('facilityList.selectDeviceGroup')">
                            <el-option v-for="(item, index) in groupConfig" :key="index" :label="item.group_name"
                                :value="item.group_id" />
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="t('facilityList.remarks')">
                        <el-input v-model="form.dev_desc" :rows="3" type="textarea"
                            :placeholder="t('facilityList.enterRemarks')" />
                    </el-form-item>
                </el-form>
                <div v-else>
                    <el-result :title="t('facilityList.addNewDevice') + ' ' + t('facilityList.success')">
                        <template #icon>
                            <img src="@/assets/success.png" width="50%">
                        </template>
                        <template #extra>
                            <div style="display: flex;justify-content: center;align-items: center;">
                                <el-button type="primary" @click="facilityTrue = false;">{{
                                    t('facilityList.continueAdd') }}</el-button>
                                <el-button type="primary" @click="props.tagOnConfig.bool = false;">{{
                                    t('facilityList.viewDevice') }}</el-button>
                            </div>
                        </template>

                    </el-result>
                </div>
            </div>
            <template #footer>
                <el-button type="primary" v-if="!facilityTrue" @click="tagOnChange">{{ t('facilityList.addNewDevice')
                    }}</el-button>
            </template>
        </el-dialog>
        <selectFacility :selectConfig="ziPattern" @handleCurrentChange="handleCurrentChange"></selectFacility>
        <apparatusDialog v-if="isApparatusDialog" @handleClose="handleClose" @handleChange="handleChange" :tltle="tltle"
            :id="user_id"></apparatusDialog>
        <createPattern :createPatternConfig="createPatternConfig" @refear="getList();"></createPattern>
        <getPattern ref="getPatternRef" :getPatternConfig="getPatternConfig" @paneOnChange="paneOnChange"
            @createPatternChange="createPatternConfig.bool = true;"></getPattern>
    </div>
</template>

<script lang="ts" setup>
import selectFacility from "./pane/selectFacility.vue";
import { addDevice, groupList } from "@/api/facilityList";
import useCounterStore from "@/stores/counter";
import apparatusDialog from '@/views/facility/facilityPattern/modelDetails/associatedApparatus/apparatusDialog.vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let createPatternConfig = ref({ bool: false, modal: true, title: t('facilityList.createDeviceModel'), width: "55%", draggable: true, showClose: true });//创建设备模型的配置项 - 传入子组件中的值
let getPatternConfig = ref({ bool: false, modal: true, title: t('facilityList.deviceModel'), width: "55%", draggable: true, showClose: true });//选择设备模型的配置项 - 传入子组件中的值
let isApparatusDialog = ref(false) // 添加子设备弹窗
let emit = defineEmits(["refaer"]);
//从父组件传过来的值
let props = withDefaults(defineProps<{
    tagOnConfig: {
        bool: Boolean
    }
}>(), {
    tagOnConfig: () => ({
        bool: false
    })
})
let form = ref({ user_id: "", device_group: "", device_name: "", device_sn: "", device_model: "", device_model_value: undefined, device_type: "", communication: "", dev_desc: "", sub_devices: [] });//表单内容
let rules = ref({
    device_name: [{ required: true, message: t('facilityList.deviceNameRequired'), trigger: "blur" }],
    device_group: [{ required: true, message: t('facilityList.deviceGroupRequired'), trigger: "blur" }]
});
let rulesRef: any = ref("");//表单Ref
let getPatternRef: any = ref("");//选择模型Ref
let ziPattern = ref({ isTrue: false, curring: 0 });//选择子设备模型的弹窗
let ziFacility = ref([]);//子设备列表 { sub_dev_id: "", sub_dev_name: "", slave_address: "", gateway_model_id: "", description: "" }
let paneTrue = ref(false);//子设备列表是否显示
let facilityTrue = ref(false);//是否成功添加设备，用来判断是否继续添加
let groupConfig: any = ref([]);//分组下拉框列表
let tltle = ref('') // 子设备弹窗title

watch(props.tagOnConfig, (val) => {
    if (val) {
        if (val.bool == true) {
            form.value = { device_name: '', device_sn: '', device_model_value: undefined, sub_devices: [], dev_desc: '', device_group: "" }
            paneTrue.value = false
            // console.log(groupConfig.value,'======')
            // 👇 新增：默认选中第一条数据
            // if (groupConfig.value.length > 0) {

            //     form.value.device_group = groupConfig.value[0].group_id;
            //     console.log(form.value.device_group)
            // }
        } else {
            facilityTrue.value = false;
        }
        // facilityTrue.value = false;
    }
}, { immediate: true, deep: true });

onMounted(() => {
    //渲染设备分组下拉列表
    groupList({ user_id: user_id.value, group_id: "", group_name: "" }).then((res: any) => {
        if (res.code == 200) {
            // 👇 新增：将“默认分组”移到最前面
            let defaultGroup = res.data.data.filter((item: any) => item.group_name === "默认分组");
            let otherGroups = res.data.data.filter((item: any) => item.group_name !== "默认分组");
            // 拼接数组时使用正确的变量名
            groupConfig.value = [...defaultGroup, ...otherGroups];
            if (groupConfig.value.length > 0) {

                form.value.device_group = groupConfig.value[0].group_id;
            }
        }
    })
    getList();
})

/**
 * 渲染模型和触发子组件渲染模型
 */
let getList = () => {
    getPatternRef.value.getList();
}

/**
 * 选择子设备之后，获取所有选中的子设备
 */
let handleChange = (e: any) => {
    ziFacility.value = e.map((item: any) => {
        return {
            sub_dev_id: item.dev_id,
            sub_dev_name: item.dev_name,
            slave_address: item.slave_address,
            gateway_model_id: form.value.device_model,
            description: !item.dev_desc ? "无" : item.dev_desc,
        }
    })
}

/**
 * 添加/减少子设备
 */
let ziPaneChange = (i: any) => {
    if (i == "添加") {
        ziFacility.value.push({ sub_dev_id: "", sub_dev_name: "", slave_address: "", gateway_model_id: "", description: "" });
    } else {
        ziFacility.value.splice(i, 1);
    }
}

/**
 * 添加设备
 */
let tagOnChange = () => {
    rulesRef.value.validate((vali: any) => {
        if (vali) {
            form.value.user_id = user_id.value;
            let arr = ziFacility.value.some(item => !item.slave_address);
            if (paneTrue.value == true) {
                if (arr) {
                    ElMessage.error("从机号不能为空！");
                } else {
                    form.value.sub_devices = ziFacility.value;
                    cont(form.value)
                }
            } else if (paneTrue.value == false) {
                cont(form.value)
            } else {
                ElMessage.error("从机号不能为空！");
            }
        }
    })
}
let cont = (e: any) => {
    addDevice(e).then((res: any) => {
        if (res.code != 200) {
            ElMessage.error(res.msg);
        } else {
            facilityTrue.value = true;
            ElMessage.success(res.msg);
            emit("refaer");
        }
    })
}

/**
 * 点击模型
 */
let paneOnChange = (list1: any, list2: any, list3: any) => {
    form.value.device_model = list3.model_id;
    form.value.device_model_value = list3.model_name;
    form.value.device_type = list3.device_type;
    form.value.communication = list3.communication;
    getPatternConfig.value.bool = false;

    if (list3.device_type == "GW") {
        paneTrue.value = true;
    } else {
        paneTrue.value = false;
    }
}

/**
 * 选择子模型的点击操作
 */
let handleCurrentChange = (val: any) => {
    // console.log("aaa",val)
    // ziFacility.value[ziPattern.value.curring].value2 = val.id;
    // ziPattern.value.isTrue = false;
}
// dev:
let add = () => { tltle.value = '子设备列表', isApparatusDialog.value = true, console.log(props.tagOnConfig) }
/**
 * 子设备取消弹窗
 */
let handleClose = (e: any) => { isApparatusDialog.value = false }
</script>

<style lang="scss" scoped>
:deep(.el-tabs__item .is-icon-close) {
    display: none;
}

:deep(.el-tabs__new-tab) {
    // width: 7vw;
}
</style>
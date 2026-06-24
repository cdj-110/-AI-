<template>
    <div>
        <el-dialog v-model="dialogIstrue" :title="props.title" width="30%" @close="emit('handleClose', false)">
            <el-form :model="form" style="margin: 0 2rem;" label-width="auto">
                <el-form-item :label="t('facilityGroup.groupName')">
                    <el-input v-model="form.group_name" :placeholder="t('facilityGroup.pleaseEnterGroupName')" clearable/>
                </el-form-item>
                <el-form-item :label="t('facilityGroup.groupOrder')">
                    <el-input-number v-model="form.group_order" controls-position="right" clearable/>
                    <span style="margin-left: 0.5rem;">({{ t('facilityGroup.groupOrder') }})</span>
                </el-form-item>
                <el-form-item :label="t('facilityGroup.groupDescription')">
                    <el-input type="textarea" v-model="form.group_description" :rows="5" :placeholder="t('facilityGroup.pleaseEnterGroupDesc')" clearable/>
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button type="info" @click="emit('handleClose', false)">{{ t('facilityGroup.cancel') }}</el-button>
                <el-button type="primary" @click="groupChange">{{ t('facilityGroup.confirm') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { update_devicegroup } from "@/api/facilityList/index";
import useCounterStore from "@/stores/counter";
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let emit = defineEmits(["handleClose"]);
let form = ref({ user_id: "", group_name: "", parent_id: "", group_order: 0, group_description: "" });//添加设备分组表单信息
let dialogIstrue = ref(true)
const props = defineProps({
    obj: {
        type: Object,
        required: false
    },
    title: {
        type: String,
        required: false
    },
});

watch(props, (val) => {
    if (val) {
        form.value = props.obj
        console.log(form.value, 'form.value')
    }
}, { immediate: true, deep: true })

onMounted(() => {
})

/**
 * 新增分组
 */
let groupChange = () => {
    let params = {
        user_id: user_id.value,
        group_id: form.value.group_id,
        group_name: form.value.group_name,
        group_order: form.value.group_order,
        group_description: form.value.group_description

    }
    update_devicegroup(params).then((res: any) => {
        if (res.code == 200) {
            ElMessage.success(res.msg);
            emit("handleClose");
        } else {
            ElMessage.error(res.msg);
        }
    });
}
</script>
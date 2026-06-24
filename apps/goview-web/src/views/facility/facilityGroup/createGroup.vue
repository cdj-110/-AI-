<template>
    <div>
        <el-dialog v-model="props.config.isTrue" :title="props.config.title" width="30%">
            <el-form :model="form" style="margin: 0 2rem;" label-width="auto">
                <el-form-item :label="t('facilityGroup.groupName')">
                    <el-input v-model="form.group_name" :placeholder="t('facilityGroup.pleaseEnterGroupName')" clearable />
                </el-form-item>
                <el-form-item :label="t('facilityGroup.parentGroup')" v-if="props.config.type == 0">
                    <el-select v-model="form.parent_id" :placeholder="t('facilityGroup.parentGroup')" clearable>
                        <el-option v-for="(item, index) in groupAllList" :key="index" :label="item.group_name"
                            :value="item.group_id" />
                    </el-select>
                </el-form-item>
                <el-form-item :label="t('facilityGroup.parentGroup')" v-if="props.config.type == 3">
                    <el-input v-model="form.parent_name" :placeholder="t('facilityGroup.pleaseEnterGroupName')" disabled="true" clearable />
                </el-form-item>
                <el-form-item :label="t('facilityGroup.groupOrder')">
                    <el-input-number v-model="form.group_order" controls-position="right" clearable />
                    <span style="margin-left: 0.5rem;">({{ t('facilityGroup.groupOrder') }})</span>
                </el-form-item>
                <el-form-item :label="t('facilityGroup.groupDescription')">
                    <el-input type="textarea" v-model="form.group_description" :rows="5" :placeholder="t('facilityGroup.pleaseEnterGroupDesc')"
                        clearable />
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button type="info" @click="props.config.isTrue = false">{{ t('facilityGroup.cancel') }}</el-button>
                <el-button type="primary" @click=" groupChange();">{{ t('facilityGroup.confirm') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { addDevicegroup, groupList, update_devicegroup } from "@/api/facilityList/index";
import useCounterStore from "@/stores/counter";
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let emit = defineEmits(["refaer"]);
let form = ref({ user_id: "", group_name: "", parent_name: '', parent_id: "", group_order: 10, group_description: "" });//添加设备分组表单信息
let groupAllList = ref([])
let props = withDefaults(defineProps<{
    config: {
        title: string,
        isTrue: boolean,
        type: number,
        list: any
    },
    obj: {
        type: string,
        required: ""
    },
    group_id: {
        type: string,
        required: ""
    }
}>(), {
    config: () => ({
        title: "",
        isTrue: false,
        type: 0,
        list: {}
    })
});

watch(props, (val) => {
    if (val.config.isTrue == true && val.config.type == 1) {
        form.value = JSON.parse(JSON.stringify(val.config.list));
    }
    if (val.config.type == 0) {
        groupList({ user_id: user_id.value, group_id: "", group_name: "" }).then((res: any) => {
            if (res.code == 200) {
                let defaultGroup = res.data.data.filter((item: any) => item.group_name === t('facilityGroup.defaultGroup'));
                let otherGroups = res.data.data.filter((item: any) => item.group_name !== t('facilityGroup.defaultGroup'));
                groupAllList.value = [...defaultGroup, ...otherGroups];
            }
        })
    }
    else if (val.config.type == 3) {
        form.value.parent_name = props.obj
        form.value.parent_id = props.group_id
    }
}, { immediate: true, deep: true })

onMounted(() => {
})

/**
 * 新增分组
 */
let groupChange = () => {
    form.value.user_id = user_id.value;
    let request = props.config.type == 1 ? update_devicegroup : addDevicegroup;
    request(form.value).then((res: any) => {
        if (res.code == 200) {
            ElMessage.success(res.msg);
            props.config.isTrue = false
            emit("refaer");
            form.value = { user_id: "", group_name: "", parent_id: "", group_order: 10, group_description: "", parent_name: '' };
        } else {
            ElMessage.error(res.msg);
        }
    });
}
</script>
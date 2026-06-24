<template>
    <div>
        <el-dialog v-model="dialogTableVisible" title="下发数据" width="25%" @close="emit('handleClose', false)"
            :close-on-press-escape="false" draggable>
            <div style="display: flex;justify-content: space-between;width: 50%;margin: 0 0 1rem 0;">
                <div>
                    <div style="color: #A4A4A4;font-size: 0.8rem;">属性标识符</div>
                    <div style="color: #000000;margin-top: 0.5rem">{{ formData.identifier }}</div>
                </div>
                <div>
                    <div style="color: #A4A4A4;font-size: 0.8rem;">属性名称</div>
                    <div style="color: #000000;margin-top: 0.5rem">{{ formData.property_name }}</div>
                </div>
            </div>
            <div v-if="formData.data_type == 'number'">
                <div style="color: #A4A4A4;font-size: 0.8rem;margin: 0.3rem 0;">属性值</div>
                <el-input-number v-model="formData.property_value" style="width: 100%;" />
            </div>

            <div v-else>
                <div style="color: #A4A4A4;font-size: 0.8rem;margin: 0.3rem 0;">属性值</div>
                <el-switch v-model="formData.property_value" active-value="1" inactive-value="0" />
            </div>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleClose', false)">取消</el-button>
                    <el-button type="primary" @click="confirmSend">
                        下发
                    </el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue';
import useCounterStore from "@/stores/counter";
import { push_attributes } from "@/api/facilityList/index";
let store = useCounterStore();
let { user_id } = storeToRefs(store);

const props = defineProps({
    SendPropertyObj: {
        type: Object,
        required: false,
        default: () => ({})
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

let emit = defineEmits(["handleClose", "sendData"]);

let dialogTableVisible = ref(true);

let formData = ref({
    identifier: '',
    property_name: '',
    property_value: 0,
    switchValue: '',
    data_type: '',
    ...props.SendPropertyObj
});

// 监听父组件传入的值变化，同步到本地副本
watch(() => props.SendPropertyObj, (val) => {
    formData.value = {
        identifier: '',
        property_name: '',
        property_value: 0,
        switchValue: '',
        data_type: '',
        ...val
    };
}, { deep: true });

// 确认下发
const confirmSend = () => {
    let params = {
        "user_id": user_id.value,
        "dev_id": props.dev_id,
        "items": [
            {
                "identifier": formData.value.identifier,
                "value":Number(formData.value.property_value)
            }
        ]
    }
    push_attributes(params).then((res: any) => {
        if (res.code == 200) {
            emit("sendData");
            ElMessage.success(res.msg)
            dialogTableVisible.value = false;
        } else {
            ElMessage.error(res.msg)
        }
    })
};

</script>

<style lang="scss" scoped></style>
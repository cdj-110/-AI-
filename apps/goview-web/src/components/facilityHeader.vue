<template>
    <div style="padding: 1rem 2rem;">
        <div style="font-size: 1.3rem;font-weight: 700;">{{ typeof props.facilityConfig == "function" ? props.facilityConfig().title : props.facilityConfig.title }}</div>
        <div style="padding: 1rem 0;display: flex;align-items: center;justify-content: space-between;">
            <div>
                <slot name="left"></slot>
            </div>
            <div :model="form" style="display: flex;align-items: center;justify-content: space-between;">
                <div v-for="(item, index) in typeof props.facilityConfig == 'function' ? props.facilityConfig().search : props.facilityConfig.search" :key="index">
                    <el-select v-model="form[item.fields]" v-if="item.type == 'select'" :placeholder="item.placeholder" style="width: 7vw;margin-left: 1rem;" clearable>
                        <el-option v-for="(v, index) in item.options" :key="index" :label="v.label" :value="v.value" />
                    </el-select>

                    <el-input v-model="form[item.fields]" v-if="item.type == 'input'" style="width: 13vw;margin-left: 1rem;" :placeholder="item.placeholder" clearable />

                    <el-button :icon="item.icon" @click="item.onClick ? item.onClick(form) : () => {}" :type="item.status" style="margin-left: 1rem;" v-if="item.type == 'button'">
                        {{ item.label }}
                    </el-button>
                </div>
            </div>
        </div>
        <div style="overflow: auto;max-height: 77vh;">
            <slot name="content"></slot>
        </div>
    </div>
</template>

<script lang="ts" setup>
let props = withDefaults(defineProps<{
    facilityConfig: {
        title: string,
        search?: Array<{
            fields: string,
            label?: string,
            type?: string,
            placeholder?: string,
            icon?: string,
            status?: string,
            options?: Array<{ label: string, value: string }> | Function,
            onClick?: Function
        }>
    } | any
}>(),{
    facilityConfig: () => ({
        title: "",
        search: []
    })
});

let form: any = ref({});
</script>